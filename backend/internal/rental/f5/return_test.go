package f5

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
)

// --- fakes ---------------------------------------------------------------

type fakeRental struct {
	rentals map[string]rental.Rental
	mu      sync.Mutex
}

func newFakeRental() *fakeRental { return &fakeRental{rentals: map[string]rental.Rental{}} }

func (f *fakeRental) Get(_ context.Context, id string) (rental.Rental, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.rentals[id]; ok {
		return r, nil
	}
	return rental.Rental{}, rental.ErrNotFound
}

func (f *fakeRental) put(r rental.Rental) { f.mu.Lock(); f.rentals[r.ID] = r; f.mu.Unlock() }

type fakeReturn struct {
	byID   map[string]rental.Return
	byRent map[string]string
	mu     sync.Mutex
}

func newFakeReturn() *fakeReturn {
	return &fakeReturn{byID: map[string]rental.Return{}, byRent: map[string]string{}}
}

func (f *fakeReturn) Create(_ context.Context, ret rental.Return) (rental.Return, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byRent[ret.RentalID]; ok {
		return rental.Return{}, rental.ErrF5ReturnAlreadyExists
	}
	ret.CreatedAt = time.Now().UTC()
	ret.UpdatedAt = ret.CreatedAt
	f.byID[ret.ID] = ret
	f.byRent[ret.RentalID] = ret.ID
	return ret, nil
}

func (f *fakeReturn) GetByRental(_ context.Context, rentalID string) (rental.Return, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.byRent[rentalID]
	if !ok {
		return rental.Return{}, false, nil
	}
	return f.byID[id], true, nil
}

func (f *fakeReturn) UpdateState(_ context.Context, id string, from, to rental.ReturnState, mutate func(ret *rental.Return)) (rental.Return, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cur, ok := f.byID[id]
	if !ok {
		return rental.Return{}, rental.ErrF5ReturnNotFound
	}
	if cur.State != from {
		return rental.Return{}, rental.ErrF5ReturnInvalidState
	}
	if !rental.CanReturnTransition(from, to) {
		return rental.Return{}, rental.ErrF5ReturnInvalidState
	}
	cur.State = to
	if mutate != nil {
		mutate(&cur)
	}
	cur.UpdatedAt = time.Now().UTC()
	f.byID[id] = cur
	return cur, nil
}

func (f *fakeReturn) Mutate(_ context.Context, id string, mutate func(ret *rental.Return)) (rental.Return, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cur, ok := f.byID[id]
	if !ok {
		return rental.Return{}, rental.ErrF5ReturnNotFound
	}
	if mutate != nil {
		mutate(&cur)
	}
	cur.UpdatedAt = time.Now().UTC()
	f.byID[id] = cur
	return cur, nil
}

type fakeDamage struct{}

func (fakeDamage) Create(context.Context, rental.DamageClaim) (rental.DamageClaim, error) {
	return rental.DamageClaim{}, nil
}
func (fakeDamage) GetByID(context.Context, string) (rental.DamageClaim, error) {
	return rental.DamageClaim{}, rental.ErrF5DamageNotFound
}
func (fakeDamage) GetByRental(context.Context, string) (rental.DamageClaim, bool, error) {
	return rental.DamageClaim{}, false, nil
}
func (fakeDamage) UpdateState(context.Context, string, rental.DamageState, rental.DamageState, func(*rental.DamageClaim)) (rental.DamageClaim, error) {
	return rental.DamageClaim{}, nil
}
func (fakeDamage) ListExpiring(context.Context, time.Time) ([]rental.DamageClaim, error) {
	return nil, nil
}

type fakeDebt struct{}

func (fakeDebt) Create(context.Context, rental.Debt) (rental.Debt, error) { return rental.Debt{}, nil }
func (fakeDebt) GetByID(context.Context, string) (rental.Debt, error) {
	return rental.Debt{}, rental.ErrF5DebtNotFound
}
func (fakeDebt) GetByDamage(context.Context, string) (rental.Debt, bool, error) {
	return rental.Debt{}, false, nil
}
func (fakeDebt) UpdateState(context.Context, string, rental.DebtState, rental.DebtState, func(*rental.Debt)) (rental.Debt, error) {
	return rental.Debt{}, nil
}
func (fakeDebt) Mutate(context.Context, string, func(*rental.Debt)) (rental.Debt, error) {
	return rental.Debt{}, nil
}
func (fakeDebt) ListOpenForRenter(context.Context, string) ([]rental.Debt, error) { return nil, nil }
func (fakeDebt) ListDueBy(context.Context, time.Time) ([]rental.Debt, error)      { return nil, nil }

// fixedClock returns a fixed time for deterministic window tests.
type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// seqIDGen returns deterministic ids.
type seqIDGen struct{ n int }

func (g *seqIDGen) String() string { g.n++; return "id-" + string(rune('a'+g.n-1)) }

// --- fixtures ------------------------------------------------------------

func confirmedRental(id string) rental.Rental {
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	return rental.Rental{
		ID:              id,
		ListingID:       "listing-1",
		TenantAccountID: "tenant-1",
		State:           rental.StateConfirmed,
		StartsAt:        ends.Add(-2 * 24 * time.Hour),
		EndsAt:          ends,
		DepositCents:    50000,
		ConfirmedAt:     ptr(ends.Add(-2 * 24 * time.Hour)),
	}
}

func ptr(t time.Time) *time.Time { return &t }

// --- tests ---------------------------------------------------------------

func TestService_RegisterPickup_HappyPath(t *testing.T) {
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	rent := newFakeRental()
	rent.put(confirmedRental("r-1"))
	ret := newFakeReturn()
	svc := NewService(Config{Now: fixedClock{t: now}, IDGen: &seqIDGen{}}, rent, ret, fakeDamage{}, fakeDebt{})

	got, err := svc.RegisterPickup(context.Background(), "r-1", rental.EvidencePayload{
		Photos:      []string{"k1", "k2"},
		Description: "asset in good shape",
	})
	require.NoError(t, err)
	require.Equal(t, rental.ReturnInProgress, got.State)
	require.NotEmpty(t, got.PickupEvidence)
}

func TestService_RegisterPickup_RejectsNonConfirmedRental(t *testing.T) {
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	rent := newFakeRental()
	r := confirmedRental("r-1")
	r.State = rental.StatePending
	rent.put(r)
	ret := newFakeReturn()
	svc := NewService(Config{Now: fixedClock{t: now}}, rent, ret, fakeDamage{}, fakeDebt{})

	_, err := svc.RegisterPickup(context.Background(), "r-1", rental.EvidencePayload{})
	require.ErrorIs(t, err, rental.ErrF5RentalNotConfirmed)
}

func TestService_RegisterPickup_RejectsDuplicate(t *testing.T) {
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	rent := newFakeRental()
	rent.put(confirmedRental("r-1"))
	ret := newFakeReturn()
	svc := NewService(Config{Now: fixedClock{t: now}}, rent, ret, fakeDamage{}, fakeDebt{})

	_, err := svc.RegisterPickup(context.Background(), "r-1", rental.EvidencePayload{})
	require.NoError(t, err)
	_, err = svc.RegisterPickup(context.Background(), "r-1", rental.EvidencePayload{})
	require.ErrorIs(t, err, rental.ErrF5ReturnAlreadyExists)
}

func TestService_RegisterReturn_HappyPath_ReleasesFullDepositWhenNoDamage(t *testing.T) {
	now := time.Date(2026, 9, 10, 13, 0, 0, 0, time.UTC) // 1h after ends_at
	rent := newFakeRental()
	rent.put(confirmedRental("r-1"))
	ret := newFakeReturn()
	svc := NewService(Config{Now: fixedClock{t: now}}, rent, ret, fakeDamage{}, fakeDebt{})

	_, err := svc.RegisterPickup(context.Background(), "r-1", rental.EvidencePayload{})
	require.NoError(t, err)
	got, err := svc.RegisterReturn(context.Background(), "r-1", rental.EvidencePayload{Description: "ok"})
	require.NoError(t, err)
	require.Equal(t, rental.ReturnClosed, got.State)
	require.Equal(t, int64(50000), got.DepositReleasedCents)
	require.Equal(t, int64(0), got.DepositCapturedCents)
	require.NotNil(t, got.ReturnedAt)
}

func TestService_RegisterReturn_BeforeEndsAt_Blocked(t *testing.T) {
	now := time.Date(2026, 9, 10, 11, 0, 0, 0, time.UTC) // 1h BEFORE ends_at
	rent := newFakeRental()
	rent.put(confirmedRental("r-1"))
	ret := newFakeReturn()
	svc := NewService(Config{Now: fixedClock{t: now}}, rent, ret, fakeDamage{}, fakeDebt{})

	_, err := svc.RegisterPickup(context.Background(), "r-1", rental.EvidencePayload{})
	require.NoError(t, err)
	_, err = svc.RegisterReturn(context.Background(), "r-1", rental.EvidencePayload{})
	require.ErrorIs(t, err, rental.ErrF5ReturnWindowOpen)
}

func TestService_RegisterReturn_AlreadyClosed(t *testing.T) {
	now := time.Date(2026, 9, 10, 13, 0, 0, 0, time.UTC)
	rent := newFakeRental()
	rent.put(confirmedRental("r-1"))
	ret := newFakeReturn()
	svc := NewService(Config{Now: fixedClock{t: now}}, rent, ret, fakeDamage{}, fakeDebt{})

	_, err := svc.RegisterPickup(context.Background(), "r-1", rental.EvidencePayload{})
	require.NoError(t, err)
	_, err = svc.RegisterReturn(context.Background(), "r-1", rental.EvidencePayload{})
	require.NoError(t, err)
	_, err = svc.RegisterReturn(context.Background(), "r-1", rental.EvidencePayload{})
	require.ErrorIs(t, err, rental.ErrF5ReturnAlreadyClosed)
}

func TestService_RegisterReturn_OnUnknownRental(t *testing.T) {
	now := time.Date(2026, 9, 10, 13, 0, 0, 0, time.UTC)
	rent := newFakeRental()
	ret := newFakeReturn()
	svc := NewService(Config{Now: fixedClock{t: now}}, rent, ret, fakeDamage{}, fakeDebt{})

	_, err := svc.RegisterReturn(context.Background(), "missing", rental.EvidencePayload{})
	require.ErrorIs(t, err, rental.ErrNotFound)
}

func TestService_RegisterReturn_BothPartiesAfterWindow_ClosesWithFullDeposit(t *testing.T) {
	// After ReturnConfirmationWindow without both parties confirming, the
	// service can be called explicitly with a forced close. In v1 the
	// renter calling RegisterReturn is enough to close (the owner is
	// silent); the staff can mediate via damage claim.
	now := time.Date(2026, 9, 12, 13, 0, 0, 0, time.UTC) // 49h after ends_at
	rent := newFakeRental()
	rent.put(confirmedRental("r-1"))
	ret := newFakeReturn()
	svc := NewService(Config{Now: fixedClock{t: now}}, rent, ret, fakeDamage{}, fakeDebt{})

	_, err := svc.RegisterPickup(context.Background(), "r-1", rental.EvidencePayload{})
	require.NoError(t, err)
	got, err := svc.RegisterReturn(context.Background(), "r-1", rental.EvidencePayload{Description: "ok"})
	require.NoError(t, err)
	require.Equal(t, rental.ReturnClosed, got.State)
	require.Equal(t, int64(50000), got.DepositReleasedCents)
}
