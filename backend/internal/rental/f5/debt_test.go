package f5_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
	f5svc "github.com/brenonaraujo/canteiro/backend/internal/rental/f5"
)

// fakeDebtRepo is a real (non-stub) implementation of DebtRepository for
// the debt service tests. The damage service tests use fakeDebtRepo2
// (the stub) because they only need the contract surface; the debt
// service tests need a stateful fake.
type fakeDebtRepo struct {
	byID     map[string]rental.Debt
	byRenter map[string][]string
	mu       sync.Mutex
}

func newFakeDebtRepo() *fakeDebtRepo {
	return &fakeDebtRepo{byID: map[string]rental.Debt{}, byRenter: map[string][]string{}}
}

func (f *fakeDebtRepo) Create(_ context.Context, d rental.Debt) (rental.Debt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, existing := range f.byID {
		if existing.DamageID == d.DamageID {
			return rental.Debt{}, rental.ErrF5DebtAlreadyExists //nolint:govet
		}
	}
	if d.ID == "" {
		return rental.Debt{}, rental.ErrInvalidInput
	}
	f.byID[d.ID] = d
	f.byRenter[d.RenterID] = append(f.byRenter[d.RenterID], d.ID)
	return d, nil
}

func (f *fakeDebtRepo) GetByID(_ context.Context, id string) (rental.Debt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if d, ok := f.byID[id]; ok {
		return d, nil
	}
	return rental.Debt{}, rental.ErrF5DebtNotFound
}

func (f *fakeDebtRepo) GetByDamage(_ context.Context, damageID string) (rental.Debt, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, d := range f.byID {
		if d.DamageID == damageID {
			return d, true, nil
		}
	}
	return rental.Debt{}, false, nil
}

func (f *fakeDebtRepo) UpdateState(_ context.Context, id string, from, to rental.DebtState, mutate func(d *rental.Debt)) (rental.Debt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cur, ok := f.byID[id]
	if !ok {
		return rental.Debt{}, rental.ErrF5DebtNotFound
	}
	if cur.State != from {
		return rental.Debt{}, rental.ErrF5DebtInvalidState
	}
	if !rental.CanDebtTransition(from, to) {
		return rental.Debt{}, rental.ErrF5DebtInvalidState
	}
	cur.State = to
	if mutate != nil {
		mutate(&cur)
	}
	cur.UpdatedAt = time.Now().UTC()
	f.byID[id] = cur
	return cur, nil
}

func (f *fakeDebtRepo) Mutate(_ context.Context, id string, mutate func(d *rental.Debt)) (rental.Debt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cur, ok := f.byID[id]
	if !ok {
		return rental.Debt{}, rental.ErrF5DebtNotFound
	}
	if mutate != nil {
		mutate(&cur)
	}
	cur.UpdatedAt = time.Now().UTC()
	f.byID[id] = cur
	return cur, nil
}

func (f *fakeDebtRepo) ListOpenForRenter(_ context.Context, renterID string) ([]rental.Debt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []rental.Debt
	for _, d := range f.byID {
		if d.RenterID == renterID && d.State == rental.DebtOpen {
			out = append(out, d)
		}
	}
	return out, nil
}

func (f *fakeDebtRepo) ListDueBy(_ context.Context, before time.Time) ([]rental.Debt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []rental.Debt
	for _, d := range f.byID {
		if d.State == rental.DebtOpen && !d.DueAt.After(before) {
			out = append(out, d)
		}
	}
	return out, nil
}

// --- tests ---------------------------------------------------------------

func debtSvc() (f5svc.RentalLookup, *fakeDamageRepo, *fakeReturnRepo2, *fakeDebtRepo) {
	_ = newFakeRentalLookup() // not used by debt flows but kept for symmetry
	return nil, nil, nil, newFakeDebtRepo()
}

func TestDebt_Create_HappyPath(t *testing.T) {
	now := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	debts := newFakeDebtRepo()
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, nil, fakeReturnRepo2{}, newFakeDamageRepo(), debts)

	got, err := svc.CreateDebt(context.Background(), f5svc.CreateDebtInput{
		RentalID: "r-1", DamageID: "d-1", RenterID: "renter-1", OriginalCents: 10000,
	})
	require.NoError(t, err)
	require.Equal(t, rental.DebtOpen, got.State)
	require.Equal(t, int64(10000), got.OriginalCents)
	require.Equal(t, now.Add(5*24*time.Hour).Format(time.RFC3339), got.DueAt.Format(time.RFC3339))
}

func TestDebt_Create_RejectsZeroAmount(t *testing.T) {
	debts := newFakeDebtRepo()
	svc := f5svc.NewService(f5svc.Config{}, nil, fakeReturnRepo2{}, newFakeDamageRepo(), debts)
	_, err := svc.CreateDebt(context.Background(), f5svc.CreateDebtInput{
		RentalID: "r-1", DamageID: "d-1", RenterID: "renter-1", OriginalCents: 0,
	})
	require.ErrorIs(t, err, rental.ErrF5DebtAmountInvalid)
}

func TestDebt_Create_ForgivenessRequiresReason(t *testing.T) {
	debts := newFakeDebtRepo()
	svc := f5svc.NewService(f5svc.Config{}, nil, fakeReturnRepo2{}, newFakeDamageRepo(), debts)
	_, err := svc.CreateDebt(context.Background(), f5svc.CreateDebtInput{
		RentalID: "r-1", DamageID: "d-1", RenterID: "renter-1",
		OriginalCents: 10000, ForgivenCents: 1000,
	})
	require.ErrorIs(t, err, rental.ErrF5DebtForgiveRequiresReason)
}

func TestDebt_Settle_HappyPath(t *testing.T) {
	now := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	debts := newFakeDebtRepo()
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, nil, fakeReturnRepo2{}, newFakeDamageRepo(), debts)

	created, err := svc.CreateDebt(context.Background(), f5svc.CreateDebtInput{
		RentalID: "r-1", DamageID: "d-1", RenterID: "renter-1", OriginalCents: 5000,
	})
	require.NoError(t, err)

	settled, err := svc.SettleDebt(context.Background(), f5svc.SettleDebtInput{
		DebtID: created.ID, SettledCents: 5000,
	})
	require.NoError(t, err)
	require.Equal(t, rental.DebtSettled, settled.State)
	require.Equal(t, int64(5000), settled.SettledCents)
	require.NotNil(t, settled.SettledAt)
}

func TestDebt_Settle_RejectsPartialAmount(t *testing.T) {
	now := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	debts := newFakeDebtRepo()
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, nil, fakeReturnRepo2{}, newFakeDamageRepo(), debts)
	created, err := svc.CreateDebt(context.Background(), f5svc.CreateDebtInput{
		RentalID: "r-1", DamageID: "d-1", RenterID: "renter-1", OriginalCents: 5000,
	})
	require.NoError(t, err)
	_, err = svc.SettleDebt(context.Background(), f5svc.SettleDebtInput{
		DebtID: created.ID, SettledCents: 2000,
	})
	require.ErrorIs(t, err, rental.ErrF5DebtAmountInvalid)
}

func TestDebt_Forgive_RequiresReason(t *testing.T) {
	now := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	debts := newFakeDebtRepo()
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, nil, fakeReturnRepo2{}, newFakeDamageRepo(), debts)
	created, err := svc.CreateDebt(context.Background(), f5svc.CreateDebtInput{
		RentalID: "r-1", DamageID: "d-1", RenterID: "renter-1", OriginalCents: 5000,
	})
	require.NoError(t, err)
	_, err = svc.ForgiveDebt(context.Background(), f5svc.ForgiveDebtInput{
		DebtID: created.ID, Cents: 1000, Reason: "",
	})
	require.ErrorIs(t, err, rental.ErrF5DebtForgiveRequiresReason)
}

func TestDebt_Forgive_PartialKeepsOpen(t *testing.T) {
	now := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	debts := newFakeDebtRepo()
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, nil, fakeReturnRepo2{}, newFakeDamageRepo(), debts)
	created, err := svc.CreateDebt(context.Background(), f5svc.CreateDebtInput{
		RentalID: "r-1", DamageID: "d-1", RenterID: "renter-1", OriginalCents: 5000,
	})
	require.NoError(t, err)

	updated, err := svc.ForgiveDebt(context.Background(), f5svc.ForgiveDebtInput{
		DebtID: created.ID, Cents: 2000, Reason: "renter proof of pre-existing condition",
	})
	require.NoError(t, err)
	require.Equal(t, rental.DebtOpen, updated.State)
	require.Equal(t, int64(2000), updated.ForgivenCents)
}

func TestDebt_Forgive_FullTransitionsToForgiven(t *testing.T) {
	now := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	debts := newFakeDebtRepo()
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, nil, fakeReturnRepo2{}, newFakeDamageRepo(), debts)
	created, err := svc.CreateDebt(context.Background(), f5svc.CreateDebtInput{
		RentalID: "r-1", DamageID: "d-1", RenterID: "renter-1", OriginalCents: 5000,
	})
	require.NoError(t, err)
	updated, err := svc.ForgiveDebt(context.Background(), f5svc.ForgiveDebtInput{
		DebtID: created.ID, Cents: 5000, Reason: "owner withdrew",
	})
	require.NoError(t, err)
	require.Equal(t, rental.DebtForgiven, updated.State)
	require.NotNil(t, updated.ForgivenAt)
}

func TestDebt_HasOpenDebt_TrueWhenOpen(t *testing.T) {
	now := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	debts := newFakeDebtRepo()
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, nil, fakeReturnRepo2{}, newFakeDamageRepo(), debts)
	_, err := svc.CreateDebt(context.Background(), f5svc.CreateDebtInput{
		RentalID: "r-1", DamageID: "d-1", RenterID: "renter-1", OriginalCents: 5000,
	})
	require.NoError(t, err)
	has, err := svc.HasOpenDebt(context.Background(), "renter-1")
	require.NoError(t, err)
	require.True(t, has)
}

func TestDebt_HasOpenDebt_FalseWhenSettled(t *testing.T) {
	now := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	debts := newFakeDebtRepo()
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, nil, fakeReturnRepo2{}, newFakeDamageRepo(), debts)
	created, err := svc.CreateDebt(context.Background(), f5svc.CreateDebtInput{
		RentalID: "r-1", DamageID: "d-1", RenterID: "renter-1", OriginalCents: 5000,
	})
	require.NoError(t, err)
	_, err = svc.SettleDebt(context.Background(), f5svc.SettleDebtInput{
		DebtID: created.ID, SettledCents: 5000,
	})
	require.NoError(t, err)
	has, err := svc.HasOpenDebt(context.Background(), "renter-1")
	require.NoError(t, err)
	require.False(t, has)
}

func TestDebt_HasOpenDebt_FalseWhenForgiven(t *testing.T) {
	now := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	debts := newFakeDebtRepo()
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, nil, fakeReturnRepo2{}, newFakeDamageRepo(), debts)
	created, err := svc.CreateDebt(context.Background(), f5svc.CreateDebtInput{
		RentalID: "r-1", DamageID: "d-1", RenterID: "renter-1", OriginalCents: 5000,
	})
	require.NoError(t, err)
	_, err = svc.ForgiveDebt(context.Background(), f5svc.ForgiveDebtInput{
		DebtID: created.ID, Cents: 5000, Reason: "all",
	})
	require.NoError(t, err)
	has, err := svc.HasOpenDebt(context.Background(), "renter-1")
	require.NoError(t, err)
	require.False(t, has)
}

func TestDebt_SuspendOverdue_CountsOnlyOpenAndPastDue(t *testing.T) {
	now := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	debts := newFakeDebtRepo()
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, nil, fakeReturnRepo2{}, newFakeDamageRepo(), debts)
	_, err := svc.CreateDebt(context.Background(), f5svc.CreateDebtInput{
		RentalID: "r-1", DamageID: "d-1", RenterID: "renter-1", OriginalCents: 5000,
	})
	require.NoError(t, err)
	count, err := svc.SuspendOverdue(context.Background())
	require.NoError(t, err)
	// The fake sets due_at = now + 5d, so the debt is not yet due.
	require.Equal(t, 0, count)
}

// silence unused warning for debtSvc() if it gets unused after edits
var _ = debtSvc
