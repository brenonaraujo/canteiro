package rental

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/account"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/listing"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
)

// Repository persists rentals, payment intents, webhook events and receipts.
type Repository interface {
	CreateIntent(ctx context.Context, r rental.Rental, snapBytes []byte) (rental.Rental, error)
	GetByID(ctx context.Context, id string) (rental.Rental, error)
	GetByIntentKey(ctx context.Context, tenantID, listingID, intentKey string) (rental.Rental, bool, error)
	ListForOwner(ctx context.Context, ownerID string, states []rental.State) ([]rental.Rental, error)
	ListForTenant(ctx context.Context, tenantID string, states []rental.State) ([]rental.Rental, error)
	UpdateState(ctx context.Context, id string, from, to rental.State, mutate func(r *rental.Rental)) (rental.Rental, error)
	ListActiveOverlapping(ctx context.Context, listingID string, start, end time.Time) ([]rental.Rental, error)
	ListOwnerBlocks(ctx context.Context, listingID string, start, end time.Time) ([]Block, error)
	SaveReceipt(ctx context.Context, rec rental.Receipt) (rental.Receipt, error)
	GetReceipt(ctx context.Context, rentalID string) (rental.Receipt, bool, error)
	UpsertPaymentIntent(ctx context.Context, intent PaymentIntent) (PaymentIntent, error)
	GetPaymentIntent(ctx context.Context, rentalID string) (PaymentIntent, bool, error)
	RecordWebhookEvent(ctx context.Context, ev WebhookEvent) (WebhookEvent, error)
	// F4 cancellation persistence.
	SaveCancellation(ctx context.Context, c CancellationRecord) (CancellationRecord, error)
	GetCancellationByRental(ctx context.Context, rentalID string) (CancellationRecord, bool, error)
	// F4 chargeback blocks the tenant's account until manual unblock (EC-5).
	SetTenantChargebackBlocked(ctx context.Context, tenantID string, blocked bool) error
}

// PaymentIntent is the persisted representation of a PSP intent.
type PaymentIntent struct {
	CreatedAt time.Time
	UpdatedAt time.Time

	ID                string
	RentalID          string
	Provider          string
	ProviderPaymentID string
	IdempotencyKey    string
	Status            string
	FailureCode       string
	FailureMessage    string

	Attempt            int
	AmountCents        int64
	DepositCents       int64
	ExpectedTotalCents int64
}

// WebhookEvent is the persisted record of a PSP event.
type WebhookEvent struct {
	ReceivedAt  time.Time
	ProcessedAt *time.Time

	ID              string
	Provider        string
	ProviderEventID string
	EventType       string
	RentalID        string
	PaymentIntentID string

	Payload []byte

	SignatureValid bool
}

// CancellationRecord is the immutable, audit-ready F4 record persisted
// per rental. The row is created once and never updated (ADR-lite #2);
// a correction is a new event referencing the original.
//
//nolint:fieldalignment // F4 surface has many int64 fields; the optimal ordering is not stable across govet releases.
type CancellationRecord struct {
	IssuedAt time.Time

	TenantRefundCents                    int64
	OwnerPayoutCentsAfterCancellation    int64
	OperatorPayoutCentsAfterCancellation int64
	CancellationFeeCents                 int64
	CommissionCents                      int64
	DepositCaptureCents                  int64
	DepositReleaseCents                  int64
	DepositPartialCaptureCents           int64

	ID                   string
	RentalID             string
	ActorID              string
	ProcessorOperationID string
	ReversalReason       string
	ActorKind            rental.ActorKind
	WindowCode           rental.WindowCode
	DepositState         rental.DepositState
}

// ToReceiptFields copies the F4 AC-11 fields onto a Receipt. The receipt
// is the tenant-visible write-once surface; the cancellation record is
// the platform-side audit row.
func (c CancellationRecord) ToReceiptFields() (rental.ActorKind, rental.WindowCode, int64, int64, int64, int64, int64, rental.DepositState, int64, int64, int64, string, time.Time) {
	return c.ActorKind, c.WindowCode, c.CancellationFeeCents, c.TenantRefundCents,
		c.OwnerPayoutCentsAfterCancellation, c.OperatorPayoutCentsAfterCancellation,
		c.CommissionCents, c.DepositState, c.DepositCaptureCents, c.DepositReleaseCents,
		c.DepositPartialCaptureCents, c.ProcessorOperationID, c.IssuedAt
}

// CancelInput is the input to the F4 cancellation use case.
type CancelInput struct {
	CallerAccountID      string
	RentalID             string
	ActorKind            rental.ActorKind
	Reason               string
	ProcessorOpID        string // set when the trigger is a webhook
	IsChargebackReversal bool
	FeeBPSOverride       int64 // 0 = use config default
}

// CancellationResult is what Cancel returns. The state machine moves the
// rental through two transitions (→ cancellation_in_progress → cancelled);
// both writes are atomic from the caller's perspective.
type CancellationResult struct {
	Cancellation CancellationRecord
	Rental       rental.Rental
}

// Block is a thin wrapper around listing.Block.
type Block struct {
	StartsAt  time.Time
	EndsAt    time.Time
	ID        string
	ListingID string
}

// ListingLookup is the slice of listing.Service this service needs.
type ListingLookup interface {
	GetByID(ctx context.Context, id string) (listing.Listing, error)
}

// AccountLookup is the slice of account.Service this service needs.
type AccountLookup interface {
	GetByID(ctx context.Context, id string) (account.Account, error)
}

// DebtGate is the read-only slice of the F5 debt service that F3 consumes
// (Pilar 5). A renter carrying an open avaria debt cannot open a new
// reservation intent. F5 owns every write to the debt lifecycle — this port
// exists so F3 never imports the F5 package and never mutates debt state.
type DebtGate interface {
	HasOpenDebt(ctx context.Context, renterID string) (bool, error)
}

// IDGenerator produces rental + receipt + intent + webhook ids.
type IDGenerator interface {
	String() string
}

// Config holds the knobs the service reads from the platform env.
type Config struct {
	IDGen              IDGenerator
	Now                func() time.Time
	DebtGate           DebtGate // Pilar 5 F5 gate; nil disables the check
	DefaultCurrency    string
	ProviderName       string
	ProviderWebhookKey string
	AcceptanceWindow   time.Duration
	MinLeadTime        time.Duration
	CommissionBPS      int64

	// F4 knobs.
	CancellationFeeBPS  int64 // 10% default = 1000
	CancellationWindowH int   // 24h default
	MinFractionHours    int   // EC-2: minimum fraction when after-start
	FeatureF4Enabled    bool  // R1 rollback flag
}

// Defaults fills the zero-valued fields with the documented defaults.
func (c *Config) Defaults() {
	if c.AcceptanceWindow == 0 {
		c.AcceptanceWindow = 12 * time.Hour
	}
	if c.CommissionBPS == 0 {
		c.CommissionBPS = rental.DefaultCommissionBPS
	}
	if c.DefaultCurrency == "" {
		c.DefaultCurrency = "BRL"
	}
	if c.ProviderName == "" {
		c.ProviderName = "noop"
	}
	if c.Now == nil {
		c.Now = func() time.Time { return time.Now().UTC() }
	}
	if c.IDGen == nil {
		c.IDGen = defaultIDGen{}
	}
	if c.CancellationFeeBPS == 0 {
		c.CancellationFeeBPS = 1000
	}
	if c.CancellationWindowH == 0 {
		c.CancellationWindowH = 24
	}
	if c.MinFractionHours == 0 {
		c.MinFractionHours = 4
	}
}

// Service orchestrates the rental lifecycle.
type Service struct {
	repo     Repository
	listing  ListingLookup
	accounts AccountLookup
	payment  PaymentProvider
	cfg      Config
}

// PaymentProvider is the adapter boundary.
type PaymentProvider interface {
	CreateIntent(ctx context.Context, req CreateIntentRequest) (CreateIntentResponse, error)
	VerifyWebhookSignature(ctx context.Context, rawBody []byte, signature string) (ProviderWebhookEvent, error)
}

// CreateIntentRequest is the input to the PSP.
type CreateIntentRequest struct {
	Metadata         map[string]string
	RentalID         string
	IdempotencyKey   string
	Currency         string
	AcceptanceWindow time.Duration
	AmountCents      int64
	DepositCents     int64
}

// CreateIntentResponse is the PSP's response.
type CreateIntentResponse struct {
	Provider          string
	ProviderPaymentID string
	Status            string
	FailureCode       string
	FailureMessage    string
}

// ProviderWebhookEvent is the (already-verified) PSP event.
type ProviderWebhookEvent struct {
	Provider        string
	ProviderEventID string
	EventType       string
	RentalID        string
	PaymentIntentID string
	FailureCode     string
	FailureMessage  string
	AmountCents     int64
	DepositCents    int64
	Authorized      bool
}

// NewService builds the service with default config filled in.
func NewService(repo Repository, listing ListingLookup, accounts AccountLookup, payment PaymentProvider, cfg Config) *Service {
	cfg.Defaults()
	return &Service{
		repo:     repo,
		listing:  listing,
		accounts: accounts,
		payment:  payment,
		cfg:      cfg,
	}
}

// SetDebtGate wires the F5 Pilar 5 debt gate after construction. The setter
// exists because the F5 service depends on this service for rental lookup,
// so the two cannot both be fully wired at construction time. Calling it
// with nil leaves the gate disabled.
func (s *Service) SetDebtGate(g DebtGate) {
	s.cfg.DebtGate = g
}

// requireNoOpenDebt is the Pilar 5 gate: a renter with an open avaria debt
// cannot open a new reservation intent. The check is best-effort by design —
// a race between this read and the intent write is acceptable (Pilar 5 is a
// gate, not a pessimistic lock). A nil gate means F5 is not wired.
func (s *Service) requireNoOpenDebt(ctx context.Context, renterID string) error {
	if s.cfg.DebtGate == nil {
		return nil
	}
	open, err := s.cfg.DebtGate.HasOpenDebt(ctx, renterID)
	if err != nil {
		return err
	}
	if open {
		return rental.ErrOpenDebt
	}
	return nil
}

type defaultIDGen struct{}

func (defaultIDGen) String() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

func intentKeyFromWindow(listingID string, start, end time.Time) string {
	return listingID + "|" + start.UTC().Format(time.RFC3339) + "|" + end.UTC().Format(time.RFC3339)
}

func tenantHasOpenDebt(r rental.Rental) bool {
	return r.TenantClaimDebt == "open"
}

func (s *Service) requireActiveTenant(ctx context.Context, tenantID string) error {
	acc, err := s.accounts.GetByID(ctx, tenantID)
	if err != nil {
		if errors.Is(err, account.ErrNotFound) {
			return rental.ErrForbidden
		}
		return err
	}
	if acc.Status == account.StatusDeactivated {
		return rental.ErrAccountDeactivated
	}
	if !acc.ProfileComplete() {
		return rental.ErrProfileIncomplete
	}
	return nil
}

func (s *Service) requirePublishedListingSnapshot(ctx context.Context, listingID string) (listing.Listing, rental.ListingSnapshot, error) {
	l, err := s.listing.GetByID(ctx, listingID)
	if err != nil {
		if errors.Is(err, listing.ErrNotFound) {
			return listing.Listing{}, rental.ListingSnapshot{}, rental.ErrNotFound
		}
		return listing.Listing{}, rental.ListingSnapshot{}, err
	}
	if l.State != listing.StatePublished {
		return listing.Listing{}, rental.ListingSnapshot{}, rental.ErrListingUnavailable
	}
	snap := rental.ListingSnapshot{
		OwnerID:           l.OwnerAccountID,
		Title:             l.Title,
		Category:          string(l.Category),
		PriceUnit:         string(l.PriceUnit),
		PriceAmountCents:  l.PriceAmountCents,
		DepositCents:      l.DepositCents,
		MinLeadTimeHours:  l.MinLeadTimeHours,
		PickupCity:        l.PickupCity,
		HeavyLegalCession: l.HeavyLegalCession,
		Operator: rental.OperatorSnapshot{
			Mode:            string(l.Operator.Mode),
			HourlyRateCents: l.Operator.HourlyRateCents,
			MinHours:        l.Operator.MinHours,
			IsOwner:         l.Operator.Identity.IsOwner,
		},
	}
	if l.Operator.Identity.Name != "" {
		snap.Operator.Name = l.Operator.Identity.Name
	}
	if l.Operator.Identity.Phone != "" {
		snap.Operator.Phone = l.Operator.Identity.Phone
	}
	return l, snap, nil
}

func (s *Service) requireNoOverlap(ctx context.Context, listingID string, start, end time.Time) error {
	active, err := s.repo.ListActiveOverlapping(ctx, listingID, start, end)
	if err != nil {
		return err
	}
	if len(active) > 0 {
		return rental.ErrCalendarOverlap
	}
	blocks, err := s.repo.ListOwnerBlocks(ctx, listingID, start, end)
	if err != nil {
		return err
	}
	for _, b := range blocks {
		if rental.HasOverlap(start, end, b.StartsAt, b.EndsAt) {
			return rental.ErrCalendarOverlap
		}
	}
	return nil
}
