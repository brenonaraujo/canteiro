package f5

import (
	"context"
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
		ID:            s.cfg.IDGen.String(),
		RentalID:      rentalID,
		State:         rental.ReturnInProgress,
		PickupEvidence: evBytes,
	})
	if err != nil {
		return rental.Return{}, err
	}
	return ret, nil
}

// RegisterReturn marks the asset as returned. In v1 a single call from
// either party closes the return with the full deposit released; the
// damage flow (OpenClaim) is a separate use case that records the
// capture without going through the return state machine.
//
// Preconditions:
//   - The rental must be in StateConfirmed (ErrF5RentalNotConfirmed).
//   - The return must exist (ErrF5ReturnNotFound) and be in InProgress
//     (ErrF5ReturnAlreadyClosed / ErrF5ReturnInvalidState).
//   - The current time must be at or after ends_at (ErrF5ReturnWindowOpen)
//     — no early return.
//
// On success the return is closed and the deposit is fully released.
// DepositCapturedCents stays 0; the damage flow moves the captured amount
// when the claim is resolved.
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
	returnedAt := now
	closed, err := s.returns.UpdateState(ctx, cur.ID, cur.State, rental.ReturnClosed, func(ret *rental.Return) {
		ret.ReturnEvidence = evBytes
		ret.ReturnedAt = &returnedAt
		ret.DepositReleasedCents = r.DepositCents
		ret.DepositCapturedCents = 0
	})
	if err != nil {
		return rental.Return{}, err
	}
	return closed, nil
}
