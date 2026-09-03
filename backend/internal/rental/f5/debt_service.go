package f5

import (
	"context"
	"fmt"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
)

// CreateDebtInput is the input to Service.CreateDebt. Called by the
// handler (or by the damage service internally) when a damage claim
// resolves with an agreed value that exceeds the deposit — Pilar 3.
//
//nolint:govet // fieldalignment: 6-field input struct; micro-optimization.
type CreateDebtInput struct {
	ForgivenReason string
	RentalID       string
	DamageID       string
	RenterID       string
	OriginalCents  int64
	ForgivenCents  int64
}

// SettleDebtInput marks the debt as paid. The caller (handler) is
// responsible for actually capturing the funds via the PSP; this is
// the audit record (Pilar 5: history of the debt).
type SettleDebtInput struct {
	DebtID       string
	SettledCents int64
}

// ForgiveDebtInput is the staff action. Pilar 5: reason is mandatory.
//
//nolint:govet // fieldalignment: 4-field input struct; micro-optimization.
type ForgiveDebtInput struct {
	Reason  string
	StaffID string
	DebtID  string
	Cents   int64
}

// CreateDebt persists a new debt for the (rental, damage) tuple. UNIQUE
// on damage_id in the schema ensures idempotency: a second call with the
// same damage id returns ErrF5DebtAlreadyExists.
func (s *Service) CreateDebt(ctx context.Context, in CreateDebtInput) (rental.Debt, error) {
	if in.OriginalCents <= 0 {
		return rental.Debt{}, fmt.Errorf("%w: original=%d", rental.ErrF5DebtAmountInvalid, in.OriginalCents)
	}
	if in.ForgivenCents > in.OriginalCents {
		return rental.Debt{}, fmt.Errorf("%w: forgiven=%d > original=%d", rental.ErrF5DebtCapExceeded, in.ForgivenCents, in.OriginalCents)
	}
	if in.ForgivenCents > 0 && in.ForgivenReason == "" {
		return rental.Debt{}, rental.ErrF5DebtForgiveRequiresReason
	}
	now := s.cfg.Now.Now()
	dueAt := now.Add(s.cfg.DebtSettlementWindow)
	d, err := s.debts.Create(ctx, rental.Debt{
		ID:             s.cfg.IDGen.String(),
		RentalID:       in.RentalID,
		DamageID:       in.DamageID,
		RenterID:       in.RenterID,
		State:          rental.DebtOpen,
		OriginalCents:  in.OriginalCents,
		ForgivenCents:  in.ForgivenCents,
		ForgivenReason: in.ForgivenReason,
		CreatedAt:      now,
		UpdatedAt:      now,
		DueAt:          dueAt,
	})
	if err != nil {
		return rental.Debt{}, err
	}
	return d, nil
}

// SettleDebt marks the debt as paid. v1 is single-shot: the entire
// remaining amount is settled at once. Partial settlements are out of
// scope (Pilar 3: a debt is either fully paid or open).
func (s *Service) SettleDebt(ctx context.Context, in SettleDebtInput) (rental.Debt, error) {
	cur, err := s.debts.GetByID(ctx, in.DebtID)
	if err != nil {
		return rental.Debt{}, err
	}
	if cur.State != rental.DebtOpen {
		return rental.Debt{}, fmt.Errorf("%w: state=%s", rental.ErrF5DebtAlreadySettled, cur.State)
	}
	remaining := cur.OriginalCents - cur.ForgivenCents - cur.SettledCents
	if in.SettledCents != remaining {
		return rental.Debt{}, fmt.Errorf("%w: settled=%d remaining=%d", rental.ErrF5DebtAmountInvalid, in.SettledCents, remaining)
	}
	return s.debts.UpdateState(ctx, cur.ID, cur.State, rental.DebtSettled, func(d *rental.Debt) {
		d.SettledCents = in.SettledCents
		now := s.cfg.Now.Now()
		d.SettledAt = &now
	})
}

// ForgiveDebt cancels (part of) the debt. Pilar 5: justification is
// mandatory. Cents is the additional amount to forgive; the new
// ForgivenCents total must not exceed OriginalCents.
func (s *Service) ForgiveDebt(ctx context.Context, in ForgiveDebtInput) (rental.Debt, error) {
	cur, err := s.debts.GetByID(ctx, in.DebtID)
	if err != nil {
		return rental.Debt{}, err
	}
	if cur.State != rental.DebtOpen {
		return rental.Debt{}, fmt.Errorf("%w: state=%s", rental.ErrF5DebtAlreadySettled, cur.State)
	}
	if in.Reason == "" {
		return rental.Debt{}, rental.ErrF5DebtForgiveRequiresReason
	}
	newForgiven := cur.ForgivenCents + in.Cents
	if in.Cents <= 0 || newForgiven > cur.OriginalCents {
		return rental.Debt{}, fmt.Errorf("%w: cents=%d new_total=%d original=%d", rental.ErrF5DebtCapExceeded, in.Cents, newForgiven, cur.OriginalCents)
	}
	to := rental.DebtOpen
	if newForgiven == cur.OriginalCents {
		to = rental.DebtForgiven
	}
	if to == rental.DebtOpen {
		return s.debts.Mutate(ctx, cur.ID, func(d *rental.Debt) {
			d.ForgivenCents = newForgiven
			d.ForgivenReason = in.Reason
		})
	}
	return s.debts.UpdateState(ctx, cur.ID, cur.State, to, func(d *rental.Debt) {
		d.ForgivenCents = newForgiven
		d.ForgivenReason = in.Reason
		now := s.cfg.Now.Now()
		d.ForgivenAt = &now
	})
}

// HasOpenDebt reports whether the renter has at least one open debt.
// This is the gate consumed by F3 CreateIntent (Pilar 5) — the renter
// cannot start a new reservation while an avaria debt is unpaid.
//
// Implementation: the repository returns the open debts; we check
// whether any of them has a non-zero remaining after forgiven.
func (s *Service) HasOpenDebt(ctx context.Context, renterID string) (bool, error) {
	debts, err := s.debts.ListOpenForRenter(ctx, renterID)
	if err != nil {
		return false, err
	}
	for _, d := range debts {
		if d.OriginalCents-d.ForgivenCents-d.SettledCents > 0 {
			return true, nil
		}
	}
	return false, nil
}

// SuspendOverdue is the daily sweep: debts past their due_at that are
// still open mark the renter account for suspension. The actual
// account-state change is out of scope of F5 (the renter's suspension
// status is read by F3 via HasOpenDebt, which is enough for v1:
// open + past-due is still "open" from F3's perspective).
//
// Returns the number of debts that crossed their due_at. The handler
// is expected to log + emit a metric; this is the audit hook.
func (s *Service) SuspendOverdue(ctx context.Context) (int, error) {
	now := s.cfg.Now.Now()
	debts, err := s.debts.ListDueBy(ctx, now)
	if err != nil {
		return 0, err
	}
	return len(debts), nil
}
