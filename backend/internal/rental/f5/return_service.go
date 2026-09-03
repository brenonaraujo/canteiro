package f5

import (
	"context"
	"errors"
	"fmt"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
)

// RegisterPickup creates a new Return row in the awaiting_pickup state
// and immediately advances it to in_progress. Both transitions are
// performed in the same call (Pilar 1: pickup registration is the
// definitive act of "the asset has changed hands").
//
// Preconditions:
//   - The rental must be in StateConfirmed (ErrF5RentalNotConfirmed).
//   - The caller is the renter OR the owner (Pilar 1). For v1 the API
//     layer is responsible for the authz check; the service trusts the
//     rental loader and does not re-verify.
//   - No return row exists for the rental (ErrF5ReturnAlreadyExists).
//
// Errors:
//   - rental.ErrNotFound if the rental does not exist
//   - rental.ErrF5RentalNotConfirmed if the rental is not confirmed
//   - rental.ErrF5ReturnAlreadyExists if a return row already exists
func (s *Service) RegisterPickup(ctx context.Context, rentalID string, ev rental.EvidencePayload) (rental.Return, error) {
	r, err := s.rentals.Get(ctx, rentalID)
	if err != nil {
		return rental.Return{}, err
	}
	if r.State != rental.StateConfirmed {
		return rental.Return{}, fmt.Errorf("%w: state=%s", rental.ErrF5RentalNotConfirmed, r.State)
	}
	evBytes, err := rental.MarshalReturnEvidence(ev)
	if err != nil {
		return rental.Return{}, fmt.Errorf("%w: marshal evidence: %v", rental.ErrInvalidInput, err)
	}
	ret, err := s.returns.Create(ctx, rental.Return{
		ID:             s.cfg.IDGen.String(),
		RentalID:       rentalID,
		State:          rental.ReturnInProgress,
		PickupEvidence: evBytes,
	})
	if err != nil {
		return rental.Return{}, err
	}
	return ret, nil
}

// RegisterReturn marks the asset as returned. In v1 a single call from
// either party closes the return; the deposit handling depends on
// whether a damage claim already captured part of the deposit (Pilar 3).
// Two paths feed the deposit columns:
//
//   - Path A (claim resolved first): captureFromPreResolvedClaim runs
//     before close and writes DepositCaptured/DepositReleased on the
//     existing devolucoes row. If residual > 0 it also opens a debt.
//   - Path B (return registered first): the damage flow's
//     markRenterAgreed / StaffResolve already wrote the columns and
//     this method preserves them when computing DepositReleasedCents.
//
// Preconditions:
//   - The rental must be in StateConfirmed (ErrF5RentalNotConfirmed).
//   - The return must exist (ErrF5ReturnNotFound) and be in InProgress
//     (ErrF5ReturnAlreadyClosed / ErrF5ReturnInvalidState).
//   - The current time must be at or after ends_at (ErrF5ReturnWindowOpen)
//     — no early return.
//
// On success the return is closed. If no damage claim was resolved
// (DepositCapturedCents == 0), the full deposit is released. If a
// damage claim already captured part of the deposit, that capture is
// preserved and the remainder is released.
func (s *Service) RegisterReturn(ctx context.Context, rentalID string, ev rental.EvidencePayload) (rental.Return, error) {
	r, err := s.rentals.Get(ctx, rentalID)
	if err != nil {
		return rental.Return{}, err
	}
	if r.State != rental.StateConfirmed {
		return rental.Return{}, fmt.Errorf("%w: state=%s", rental.ErrF5RentalNotConfirmed, r.State)
	}
	now := s.cfg.Now.Now()
	if now.Before(r.EndsAt) {
		return rental.Return{}, fmt.Errorf("%w: now=%s ends_at=%s", rental.ErrF5ReturnWindowOpen, now, r.EndsAt)
	}
	cur, ok, err := s.returns.GetByRental(ctx, rentalID)
	if err != nil {
		return rental.Return{}, err
	}
	if !ok {
		return rental.Return{}, rental.ErrF5ReturnNotFound
	}
	if cur.State == rental.ReturnClosed {
		return rental.Return{}, rental.ErrF5ReturnAlreadyClosed
	}
	if !rental.CanReturnTransition(cur.State, rental.ReturnClosed) {
		return rental.Return{}, rental.ErrF5ReturnInvalidState
	}
	evBytes, err := rental.MarshalReturnEvidence(ev)
	if err != nil {
		return rental.Return{}, fmt.Errorf("%w: marshal evidence: %v", rental.ErrInvalidInput, err)
	}
	// Path A: if a damage claim was already resolved, capture the deposit
	// now. Idempotent. The helper no-ops when no claim exists or the
	// claim isn't resolved.
	cur, err = s.captureFromPreResolvedClaim(ctx, r, cur, rentalID)
	if err != nil {
		return rental.Return{}, err
	}
	// Deposit handling: if no damage claim captured part of the deposit
	// (Pilar 3 happy path), release the full deposit. Otherwise the
	// damage flow wrote the columns already; preserve them.
	released := r.DepositCents - cur.DepositCapturedCents
	if released < 0 {
		released = 0
	}
	returnedAt := now
	closed, err := s.returns.UpdateState(ctx, cur.ID, cur.State, rental.ReturnClosed, func(ret *rental.Return) {
		ret.ReturnEvidence = evBytes
		ret.ReturnedAt = &returnedAt
		ret.DepositReleasedCents = released
	})
	if err != nil {
		return rental.Return{}, err
	}
	return closed, nil
}

// captureFromPreResolvedClaim implements Path A of the Pilar 3 wire.
// If a damage claim for this rental was already resolved (renter
// agreed or staff resolved) before RegisterReturn was called, the
// deposit is captured here. The helper no-ops when there's nothing to
// capture (no claim, claim not resolved, already captured).
//
// Returns the (possibly updated) current return row.
func (s *Service) captureFromPreResolvedClaim(
	ctx context.Context,
	r rental.Rental,
	cur rental.Return,
	rentalID string,
) (rental.Return, error) {
	if cur.DepositCapturedCents != 0 {
		return cur, nil
	}
	claim, claimOK, claimErr := s.damage.GetByRental(ctx, rentalID)
	if claimErr != nil {
		return cur, claimErr
	}
	if !claimOK || claim.ResolvedAt == nil || claim.AgreedCents <= 0 {
		return cur, nil
	}
	if err := s.captureDepositAndMaybeCreateDebt(ctx, r, cur, claim, claim.AgreedCents); err != nil {
		return cur, err
	}
	// Re-read so the caller sees the freshly-captured columns.
	updated, reloadOK, err := s.returns.GetByRental(ctx, rentalID)
	if err != nil {
		return cur, err
	}
	if !reloadOK {
		return cur, rental.ErrF5ReturnNotFound
	}
	return updated, nil
}

// captureDepositAndMaybeCreateDebt is the Pilar 3 wire: when a damage
// claim resolves (renter agrees or staff resolves), it (a) captures up
// to min(agreed, deposit) from the deposit, (b) releases the remainder
// to the renter, (c) opens a debt for the residual (agreed - captured)
// if it is > 0. Idempotency:
//
//   - The deposit columns on devolucoes are mutated only on the first
//     call (DepositCapturedCents == 0 guard). A second call returns
//     the same state without mutating anything.
//   - dividas has UNIQUE(damage_id); a second call's CreateDebt either
//     hits ErrF5DebtAlreadyExists (we treat as success because the
//     row is already there) or wins the race and inserts the row.
//
// This helper runs inside the natural request flow (no explicit
// transaction) — the B1 fix made the columns NOT NULL safe and the
// UNIQUE on damage_id backstops any concurrent double-call.
//
// Deferral: if no return row exists yet (cur.ID == ""), the helper is
// a no-op. The damage flow runs first in v1's typical ordering; Path
// A (claim resolved before return registration) is covered by the
// inline call from RegisterReturn.
//
// Preconditions:
//   - claim.ResolvedAt is set (the call site already moved the claim
//     to a terminal state); not enforced here.
//
// Errors: bubbles up anything other than ErrF5DebtAlreadyExists.
func (s *Service) captureDepositAndMaybeCreateDebt(
	ctx context.Context,
	r rental.Rental,
	cur rental.Return,
	claim rental.DamageClaim,
	agreed int64,
) error {
	if cur.ID == "" {
		// No return row yet — defer; RegisterReturn will cover Path A.
		return nil
	}
	if cur.DepositCapturedCents != 0 {
		// Already wired for this rental/damage pair — noop.
		return nil
	}
	if agreed <= 0 {
		return fmt.Errorf("%w: agreed=%d", rental.ErrF5DamageAmountInvalid, agreed)
	}
	deposit := r.DepositCents
	captured := agreed
	if captured > deposit {
		captured = deposit
	}
	released := deposit - captured
	if released < 0 {
		released = 0
	}
	residual := agreed - captured

	// 1. Update devolucoes with deposit_captured/released + returned_at.
	now := s.cfg.Now.Now()
	_, err := s.returns.Mutate(ctx, cur.ID, func(ret *rental.Return) {
		ret.DepositCapturedCents = captured
		ret.DepositReleasedCents = released
		if ret.ReturnedAt == nil {
			ret.ReturnedAt = &now
		}
	})
	if err != nil {
		return err
	}

	// 2. If agreed exceeded the deposit, open a debt for the residual.
	//    UNIQUE(damage_id) makes this safe under a second invocation.
	if residual <= 0 {
		return nil
	}
	// If a debt already exists for this damage (e.g., a prior run won
	// the race or this is a retry), CreateDebt returns ErrF5DebtAlreadyExists.
	// Treat that as success.
	if _, debtOK, debtErr := s.debts.GetByDamage(ctx, claim.ID); debtErr != nil {
		return debtErr
	} else if debtOK {
		return nil
	}
	_, err = s.CreateDebt(ctx, CreateDebtInput{
		RentalID:      claim.RentalID,
		DamageID:      claim.ID,
		RenterID:      claim.RenterID,
		OriginalCents: residual,
	})
	if err != nil && !errors.Is(err, rental.ErrF5DebtAlreadyExists) {
		return err
	}
	return nil
}

// wirePilar3OnClaimResolved is the small glue used by the damage flow
// (markRenterAgreed / StaffResolve) to invoke captureDepositAndMaybeCreateDebt.
// It loads the rental and (if any) the return row and forwards to the
// helper. Errors are propagated verbatim.
//
// This helper exists to keep the two call sites (markRenterAgreed +
// StaffResolve) tight: both follow the same shape — update claim
// state, then if a return row exists for the rental, capture the
// deposit and (if needed) open a debt.
func (s *Service) wirePilar3OnClaimResolved(ctx context.Context, claim rental.DamageClaim, agreed int64) error {
	r, err := s.rentals.Get(ctx, claim.RentalID)
	if err != nil {
		return err
	}
	cur, _, err := s.returns.GetByRental(ctx, claim.RentalID)
	if err != nil {
		return err
	}
	return s.captureDepositAndMaybeCreateDebt(ctx, r, cur, claim, agreed)
}
