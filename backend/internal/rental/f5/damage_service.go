package f5

import (
	"context"
	"fmt"
	"time"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
)

// ResponseKind is the renter's choice when responding to an open claim.
type ResponseKind string

const (
	// ResponseAgree is the renter's choice to accept the proposed value.
	ResponseAgree ResponseKind = "agree"
	// ResponseContest is the renter's choice to dispute the claim; deposit stays held.
	ResponseContest ResponseKind = "contest"
	// ResponseCounter is the renter's counter-proposal. For v1 we treat a
	// counter as agreement at the countered value if it is positive, else
	// as a contest if it is zero. The handler can map UI semantics to one
	// of these two.
	ResponseCounter ResponseKind = "counter"
)

// OpenDamageClaimInput is the input to Service.OpenDamageClaim.
//
// call site; alignment is micro-optimization here, not a correctness or
// performance issue.
//
//nolint:govet // fieldalignment: the input struct is consumed by one
type OpenDamageClaimInput struct {
	Evidence      rental.EvidencePayload
	Description   string
	OwnerID       string
	RentalID      string
	ProposedCents int64
	Nature        rental.DamageNature
}

// RenterRespondInput is the input to Service.RenterRespond.
type RenterRespondInput struct {
	Note        string
	ClaimID     string
	RenterID    string
	Response    ResponseKind
	AgreedCents int64
}

// StaffResolveInput is the input to Service.StaffResolve. StaffResolve is
// the only place where a contested claim can be finalized by the staff;
// the decision is final in v1 (Pilar 4).
type StaffResolveInput struct {
	Note        string
	ClaimID     string
	AgreedCents int64
}

// damageCapsByNature returns the maximum ProposedCents the owner can claim
// for the given nature. v1 policy: cosmetic and functional are capped at
// the rental deposit; loss is capped at the deposit (declared-asset-value
// support is a follow-up; the snapshot does not yet carry a declared
// value). The cap is a hard ceiling — anything above is rejected.
func damageCapsByNature(nature rental.DamageNature, deposit int64) int64 {
	return deposit
}

// OpenDamageClaim creates a new damage claim in the open state. The
// 48h owner claim window (AC-3) is enforced from the rental's ends_at.
// The 48h renter defense window (AC-4) starts on the open timestamp and
// is checked in RenterRespond.
func (s *Service) OpenDamageClaim(ctx context.Context, in OpenDamageClaimInput) (rental.DamageClaim, error) {
	r, err := s.rentals.Get(ctx, in.RentalID)
	if err != nil {
		return rental.DamageClaim{}, err
	}
	if !r.IsOwner(in.OwnerID) {
		return rental.DamageClaim{}, fmt.Errorf("%w: caller is not the owner", rental.ErrForbidden)
	}
	if r.State != rental.StateConfirmed {
		return rental.DamageClaim{}, fmt.Errorf("%w: state=%s", rental.ErrF5RentalNotConfirmed, r.State)
	}
	now := s.cfg.Now.Now()
	if now.After(r.EndsAt.Add(s.cfg.OwnerClaimWindow)) {
		return rental.DamageClaim{}, fmt.Errorf("%w: ends_at=%s window=%s", rental.ErrF5DamageWindowExpired, r.EndsAt, s.cfg.OwnerClaimWindow)
	}
	if !in.Nature.IsValid() {
		return rental.DamageClaim{}, fmt.Errorf("%w: nature=%s", rental.ErrF5DamageInvalidNature, in.Nature)
	}
	cap := damageCapsByNature(in.Nature, r.DepositCents)
	if in.ProposedCents <= 0 || in.ProposedCents > cap {
		return rental.DamageClaim{}, fmt.Errorf("%w: proposed=%d cap=%d", rental.ErrF5DamageAmountInvalid, in.ProposedCents, cap)
	}
	if len(in.Evidence.Photos) == 0 || in.Description == "" {
		return rental.DamageClaim{}, rental.ErrF5DamageEvidenceRequired
	}
	evBytes, err := rental.MarshalReturnEvidence(in.Evidence)
	if err != nil {
		return rental.DamageClaim{}, fmt.Errorf("%w: marshal evidence: %v", rental.ErrInvalidInput, err)
	}
	claim, err := s.damage.Create(ctx, rental.DamageClaim{
		ID:            s.cfg.IDGen.String(),
		RentalID:      in.RentalID,
		OwnerID:       in.OwnerID,
		RenterID:      r.TenantAccountID,
		State:         rental.DamageOpen,
		Nature:        in.Nature,
		Description:   in.Description,
		Evidence:      evBytes,
		ProposedCents: in.ProposedCents,
		OpenedAt:      now,
	})
	if err != nil {
		return rental.DamageClaim{}, err
	}
	return claim, nil
}

// RenterRespond is the renter's reply within 48h of the claim opening.
// Three outcomes:
//
//   - agree at the proposed value: claim transitions to DamageRenterAgreed
//     (capture the proposed value against the deposit; residual > deposit
//     becomes a debt in the follow-up commit).
//   - agree at a counter value: same as agree with the counter value.
//   - contest: claim transitions to DamageContested; the staff must
//     decide via StaffResolve.
func (s *Service) RenterRespond(ctx context.Context, in RenterRespondInput) (rental.DamageClaim, error) {
	cur, err := s.damage.GetByID(ctx, in.ClaimID)
	if err != nil {
		return rental.DamageClaim{}, err
	}
	if cur.RenterID != in.RenterID {
		return rental.DamageClaim{}, fmt.Errorf("%w: caller is not the renter", rental.ErrForbidden)
	}
	if cur.State == rental.DamageContested {
		return rental.DamageClaim{}, rental.ErrF5DamageAlreadyContested
	}
	if cur.State == rental.DamageRenterAgreed {
		return rental.DamageClaim{}, rental.ErrF5DamageAlreadyAgreed
	}
	now := s.cfg.Now.Now()
	if now.After(cur.OpenedAt.Add(s.cfg.RenterDefenseWindow)) {
		return rental.DamageClaim{}, fmt.Errorf("%w: opened_at=%s window=%s", rental.ErrF5DamageWindowExpired, cur.OpenedAt, s.cfg.RenterDefenseWindow)
	}
	r, err := s.rentals.Get(ctx, cur.RentalID)
	if err != nil {
		return rental.DamageClaim{}, err
	}
	cap := damageCapsByNature(cur.Nature, r.DepositCents)
	return s.applyRenterResponse(ctx, cur, in, cap, now)
}

// applyRenterResponse is the dispatch table that turns the renter's
// ResponseKind into the matching state transition. Pulled out of
// RenterRespond so the guard clauses above stay readable (pre-impl skill).
//
// pieces; the dispatch table itself is the right granularity for the test
// matrix.
//
//nolint:revive // argument-limit: helpers below break this into manageable
func (s *Service) applyRenterResponse(ctx context.Context, cur rental.DamageClaim, in RenterRespondInput, cap int64, now time.Time) (rental.DamageClaim, error) {
	switch in.Response {
	case ResponseAgree:
		agreed := cur.ProposedCents
		if in.AgreedCents > 0 {
			agreed = in.AgreedCents
		}
		if agreed <= 0 || agreed > cap {
			return rental.DamageClaim{}, fmt.Errorf("%w: agreed=%d cap=%d", rental.ErrF5DamageAmountInvalid, agreed, cap)
		}
		return s.markRenterAgreed(ctx, cur, markAgreed{kind: ResponseAgree, note: in.Note, agreed: agreed, now: now})
	case ResponseContest:
		return s.markRenterContested(ctx, cur, in.Note, now)
	case ResponseCounter:
		if in.AgreedCents <= 0 || in.AgreedCents > cap {
			return rental.DamageClaim{}, fmt.Errorf("%w: counter=%d cap=%d", rental.ErrF5DamageAmountInvalid, in.AgreedCents, cap)
		}
		return s.markRenterAgreed(ctx, cur, markAgreed{kind: ResponseCounter, note: in.Note, agreed: in.AgreedCents, now: now})
	default:
		return rental.DamageClaim{}, fmt.Errorf("%w: response=%s", rental.ErrInvalidInput, in.Response)
	}
}

// markRenterAgreed persists the renter's acceptance. The struct keeps
// the call signature under the linter's argument-limit (pre-impl skill).
//
// micro-optimization.
//
//nolint:govet // fieldalignment: 4-field local helper, alignment is a
type markAgreed struct {
	kind   ResponseKind
	note   string
	agreed int64
	now    time.Time
}

func (s *Service) markRenterAgreed(ctx context.Context, cur rental.DamageClaim, p markAgreed) (rental.DamageClaim, error) {
	return s.damage.UpdateState(ctx, cur.ID, cur.State, rental.DamageRenterAgreed, func(c *rental.DamageClaim) {
		c.RespondedAt = &p.now
		c.RenterResponseKind = string(p.kind)
		c.RenterResponseNote = p.note
		c.AgreedCents = p.agreed
		c.ResolvedAt = &p.now
	})
}

func (s *Service) markRenterContested(ctx context.Context, cur rental.DamageClaim, note string, now time.Time) (rental.DamageClaim, error) {
	respondedAt := now
	return s.damage.UpdateState(ctx, cur.ID, cur.State, rental.DamageContested, func(c *rental.DamageClaim) {
		c.RespondedAt = &respondedAt
		c.RenterResponseKind = string(ResponseContest)
		c.RenterResponseNote = note
	})
}

// StaffResolve finalizes a contested claim. The decision is final in v1
// (Pilar 4). The staff must record a note for audit.
func (s *Service) StaffResolve(ctx context.Context, in StaffResolveInput) (rental.DamageClaim, error) {
	cur, err := s.damage.GetByID(ctx, in.ClaimID)
	if err != nil {
		return rental.DamageClaim{}, err
	}
	if cur.State != rental.DamageContested {
		return rental.DamageClaim{}, fmt.Errorf("%w: state=%s", rental.ErrF5DamageInvalidState, cur.State)
	}
	if in.Note == "" {
		return rental.DamageClaim{}, rental.ErrF5DamageEvidenceRequired
	}
	r, err := s.rentals.Get(ctx, cur.RentalID)
	if err != nil {
		return rental.DamageClaim{}, err
	}
	cap := damageCapsByNature(cur.Nature, r.DepositCents)
	if in.AgreedCents <= 0 || in.AgreedCents > cap {
		return rental.DamageClaim{}, fmt.Errorf("%w: agreed=%d cap=%d", rental.ErrF5DamageAmountInvalid, in.AgreedCents, cap)
	}
	now := s.cfg.Now.Now()
	decidedAt := now
	return s.damage.UpdateState(ctx, cur.ID, cur.State, rental.DamageStaffResolved, func(c *rental.DamageClaim) {
		c.DecidedAt = &decidedAt
		c.StaffDecisionNote = in.Note
		c.AgreedCents = in.AgreedCents
		c.ResolvedAt = &decidedAt
	})
}

// ExpireStale transitions any open claims whose defense window has
// elapsed to DamageExpired. D1 of the refinement: silence is NOT
// agreement; expiration moves the claim to mediation, not to
// DamageRenterAgreed. For v1 we keep it as Expired (mediation by staff
// follows via StaffResolve).
func (s *Service) ExpireStale(ctx context.Context) (int, error) {
	now := s.cfg.Now.Now()
	cutoff := now.Add(-s.cfg.RenterDefenseWindow)
	claims, err := s.damage.ListExpiring(ctx, cutoff)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, c := range claims {
		_, err := s.damage.UpdateState(ctx, c.ID, c.State, rental.DamageExpired, nil)
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}
