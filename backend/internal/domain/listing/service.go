package listing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/account"
)

// AccountLookup is the slice of account.Service the listing Service needs.
// Defined here to avoid an import cycle (listing → account, not the other way).
type AccountLookup interface {
	GetByID(ctx context.Context, id string) (account.Account, error)
}

// currentTermsVersion is the version of the owner terms accepted in this
// release. Once v2 ships, surface it through config.
const currentTermsVersion = "v1"

// IDGenerator produces new listing / block ids. Tests pin a fixed id via
// SetIDFunc; production uses crypto-random UUID-shaped strings.
type IDGenerator interface {
	String() string
}

// defaultIDGen produces random UUID-shaped strings (8-4-4-4-12).
type defaultIDGen struct{}

func (defaultIDGen) String() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

// DefaultIDGenerator exposes the production id generator for tests that
// want to assert on the format (length, dashes).
func DefaultIDGenerator() IDGenerator { return defaultIDGen{} }

// Service is the F2 use-case orchestrator. It owns the listing lifecycle
// transitions (draft → published → paused), the publish gates and the
// owner onboarding state. Persistence is delegated to Repository.
type Service struct {
	repo  Repository
	acc   AccountLookup
	now   func() time.Time
	idGen IDGenerator
}

// NewService wires the dependencies. clock and idGen are replaceable in tests.
func NewService(repo Repository, acc AccountLookup, now time.Time) *Service {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return &Service{
		repo:  repo,
		acc:   acc,
		now:   func() time.Time { return now },
		idGen: defaultIDGen{},
	}
}

// SetIDFunc replaces the id generator (tests pin a static id).
func (s *Service) SetIDFunc(g IDGenerator) { s.idGen = g }

// --- use cases -------------------------------------------------------------

// CreateDraft persists a new draft. The caller-supplied OwnerAccountID is
// overwritten with the authenticated owner — never trust the body.
func (s *Service) CreateDraft(ctx context.Context, ownerID string, draft Listing) (Listing, error) {
	draft.OwnerAccountID = ownerID
	draft.State = StateDraft
	draft.ID = s.idGen.String()
	if err := draft.Validate(); err != nil {
		return Listing{}, err
	}
	return s.repo.Create(ctx, draft)
}

// Update edits a draft or paused listing. Published listings must be
// paused first (AC-3); editing a published listing returns ErrAlreadyPublished.
func (s *Service) Update(ctx context.Context, ownerID string, patch Listing) (Listing, error) {
	patch.OwnerAccountID = ownerID
	cur, err := s.repo.GetByID(ctx, patch.ID)
	if err != nil {
		return Listing{}, err
	}
	if cur.OwnerAccountID != ownerID {
		return Listing{}, ErrForbidden
	}
	if !cur.CanEdit() {
		return Listing{}, ErrAlreadyPublished
	}
	// Preserve immutable fields.
	patch.State = cur.State
	patch.OwnerAccountID = cur.OwnerAccountID
	if err := patch.Validate(); err != nil {
		return Listing{}, err
	}
	patch.CreatedAt = cur.CreatedAt
	patch.UpdatedAt = s.now()
	return s.repo.Update(ctx, patch)
}

// Publish moves a draft or paused listing to published after running the
// publish gates. Account-level gates (active + profile + onboarding) are
// evaluated here; listing-content gates come from Listing.PublishGates.
func (s *Service) Publish(ctx context.Context, ownerID, id string) (Listing, error) {
	cur, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Listing{}, err
	}
	if cur.OwnerAccountID != ownerID {
		return Listing{}, ErrForbidden
	}
	if !cur.CanPublishFrom(cur.State) {
		return Listing{}, ErrAlreadyPublished
	}
	if err := s.canOwnerPublish(ctx, ownerID); err != nil {
		return Listing{}, err
	}
	cfg, _, err := s.repo.CategoryByName(ctx, cur.Category)
	if err != nil {
		return Listing{}, err
	}
	if missing := cur.PublishGates(cfg); len(missing) > 0 {
		return Listing{}, ErrPublishGates
	}
	if err := s.repo.UpdateState(ctx, id, StatePublished); err != nil {
		return Listing{}, err
	}
	cur.State = StatePublished
	return cur, nil
}

// Pause reverses publish without losing data. EC-7.
func (s *Service) Pause(ctx context.Context, ownerID, id string) (Listing, error) {
	cur, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Listing{}, err
	}
	if cur.OwnerAccountID != ownerID {
		return Listing{}, ErrForbidden
	}
	if !cur.CanPause() {
		return Listing{}, ErrNotPublished
	}
	if err := s.repo.UpdateState(ctx, id, StatePaused); err != nil {
		return Listing{}, err
	}
	cur.State = StatePaused
	return cur, nil
}

// GetMine is the owner view of a single listing (includes draft).
func (s *Service) GetMine(ctx context.Context, ownerID, id string) (Listing, error) {
	l, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Listing{}, err
	}
	if l.OwnerAccountID != ownerID {
		return Listing{}, ErrForbidden
	}
	return l, nil
}

// ListMine returns every listing owned by the caller.
func (s *Service) ListMine(ctx context.Context, ownerID string) ([]Listing, error) {
	return s.repo.ListByOwner(ctx, ownerID)
}

// GetPublic returns the published listing or ErrNotFound.
func (s *Service) GetPublic(ctx context.Context, id string) (Listing, error) {
	return s.repo.GetPublic(ctx, id)
}

// SearchCatalog is the public discovery endpoint.
func (s *Service) SearchCatalog(ctx context.Context, f SearchFilters) ([]Listing, int, error) {
	return s.repo.SearchCatalog(ctx, f)
}

// GetPublicCalendar returns the active blocks in the requested window.
// Empty from/to means "no window filter".
func (s *Service) GetPublicCalendar(ctx context.Context, id string, from, to time.Time) ([]Block, error) {
	if _, err := s.repo.GetPublic(ctx, id); err != nil {
		return nil, err
	}
	if from.IsZero() || to.IsZero() {
		return s.repo.ListBlocks(ctx, id)
	}
	return s.repo.ListBlocksInWindow(ctx, id, from, to)
}

// --- owner onboarding ------------------------------------------------------

// GetOwnerOnboarding returns the onboarding state (payout + terms).
// Missing row → empty onboarding (PayoutSet() == false).
func (s *Service) GetOwnerOnboarding(ctx context.Context, ownerID string) (OwnerOnboarding, error) {
	return s.repo.GetOwnerOnboarding(ctx, ownerID)
}

// UpsertOwnerOnboarding persists payout details and/or terms acceptance.
// AcceptTerms=false preserves the previous accepted-at; setting it requires
// the supplied terms version to match currentTermsVersion.
func (s *Service) UpsertOwnerOnboarding(ctx context.Context, ownerID string, patch OwnerOnboarding) (OwnerOnboarding, error) {
	patch.AccountID = ownerID
	cur, err := s.repo.GetOwnerOnboarding(ctx, ownerID)
	if err != nil {
		return OwnerOnboarding{}, err
	}
	if !cur.TermsAcceptedAt.IsZero() && patch.TermsAcceptedAt.IsZero() {
		patch.TermsAcceptedAt = cur.TermsAcceptedAt
		patch.TermsVersion = cur.TermsVersion
	}
	if !patch.TermsAcceptedAt.IsZero() && patch.TermsVersion != currentTermsVersion {
		return OwnerOnboarding{}, ErrOwnerOnboardingRequired
	}
	return s.repo.UpsertOwnerOnboarding(ctx, patch)
}

// ListCategories returns the platform-defined category config (deposit min).
func (s *Service) ListCategories(ctx context.Context) ([]CategoryConfig, error) {
	return s.repo.CategoryConfig(ctx)
}

// --- blocks ----------------------------------------------------------------

// AddBlock registers an availability block for the owner's listing. Overlap
// is rejected (EC: calendar invariant for F3).
func (s *Service) AddBlock(ctx context.Context, ownerID, listingID string, b Block) (Block, error) {
	if err := s.requireOwnership(ctx, ownerID, listingID); err != nil {
		return Block{}, err
	}
	if !b.EndsAt.After(b.StartsAt) {
		return Block{}, ErrBlockWindow
	}
	existing, err := s.repo.ListBlocks(ctx, listingID)
	if err != nil {
		return Block{}, err
	}
	if HasOverlappingBlock(existing, b.StartsAt, b.EndsAt) {
		return Block{}, ErrBlockOverlap
	}
	b.ID = s.idGen.String()
	b.ListingID = listingID
	return s.repo.AddBlock(ctx, b)
}

// ListBlocks returns every block of an owner's listing.
func (s *Service) ListBlocks(ctx context.Context, ownerID, listingID string) ([]Block, error) {
	if err := s.requireOwnership(ctx, ownerID, listingID); err != nil {
		return nil, err
	}
	return s.repo.ListBlocks(ctx, listingID)
}

// RemoveBlock deletes one block by id. Unknown id → ErrNotFound.
func (s *Service) RemoveBlock(ctx context.Context, ownerID, listingID, blockID string) error {
	if err := s.requireOwnership(ctx, ownerID, listingID); err != nil {
		return err
	}
	return s.repo.RemoveBlock(ctx, listingID, blockID)
}

// --- helpers ---------------------------------------------------------------

// canOwnerPublish enforces the account-level gates before content gates:
// active + profile complete + payout set + current terms accepted.
func (s *Service) canOwnerPublish(ctx context.Context, ownerID string) error {
	acc, err := s.acc.GetByID(ctx, ownerID)
	if err != nil {
		if errors.Is(err, account.ErrNotFound) {
			return ErrForbidden
		}
		return err
	}
	if acc.Status == account.StatusDeactivated {
		return ErrDeactivated
	}
	if acc.Status != account.StatusActive || !acc.ProfileComplete() {
		return ErrProfileIncomplete
	}
	ob, err := s.repo.GetOwnerOnboarding(ctx, ownerID)
	if err != nil {
		return err
	}
	if !ob.PayoutSet() || !ob.TermsAccepted(currentTermsVersion) {
		return ErrOwnerOnboardingRequired
	}
	return nil
}

// requireOwnership verifies the caller owns the listing before block ops.
func (s *Service) requireOwnership(ctx context.Context, ownerID, listingID string) error {
	l, err := s.repo.GetByID(ctx, listingID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(l.OwnerAccountID) != ownerID {
		return ErrForbidden
	}
	return nil
}