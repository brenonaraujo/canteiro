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

// --- fakes (independent copy for damage tests) ---

type fakeRentalLookup struct {
	rentals map[string]rental.Rental
	mu      sync.Mutex
}

func newFakeRentalLookup() *fakeRentalLookup {
	return &fakeRentalLookup{rentals: map[string]rental.Rental{}}
}
func (f *fakeRentalLookup) Get(_ context.Context, id string) (rental.Rental, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.rentals[id]; ok {
		return r, nil
	}
	return rental.Rental{}, rental.ErrNotFound
}
func (f *fakeRentalLookup) put(r rental.Rental) { f.mu.Lock(); f.rentals[r.ID] = r; f.mu.Unlock() }

type fakeDamageRepo struct {
	byID   map[string]rental.DamageClaim
	byRent map[string]string
	mu     sync.Mutex
}

func newFakeDamageRepo() *fakeDamageRepo {
	return &fakeDamageRepo{byID: map[string]rental.DamageClaim{}, byRent: map[string]string{}}
}
func (f *fakeDamageRepo) Create(_ context.Context, c rental.DamageClaim) (rental.DamageClaim, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byRent[c.RentalID]; ok {
		return rental.DamageClaim{}, rental.ErrF5DamageAlreadyExists
	}
	c.CreatedAt = time.Now().UTC()
	c.UpdatedAt = c.CreatedAt
	if c.OpenedAt.IsZero() {
		c.OpenedAt = c.CreatedAt
	}
	f.byID[c.ID] = c
	f.byRent[c.RentalID] = c.ID
	return c, nil
}
func (f *fakeDamageRepo) GetByID(_ context.Context, id string) (rental.DamageClaim, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c, ok := f.byID[id]; ok {
		return c, nil
	}
	return rental.DamageClaim{}, rental.ErrF5DamageNotFound
}
func (f *fakeDamageRepo) GetByRental(_ context.Context, rentalID string) (rental.DamageClaim, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.byRent[rentalID]
	if !ok {
		return rental.DamageClaim{}, false, nil
	}
	return f.byID[id], true, nil
}
func (f *fakeDamageRepo) UpdateState(_ context.Context, id string, from, to rental.DamageState, mutate func(c *rental.DamageClaim)) (rental.DamageClaim, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cur, ok := f.byID[id]
	if !ok {
		return rental.DamageClaim{}, rental.ErrF5DamageNotFound
	}
	if cur.State != from {
		return rental.DamageClaim{}, rental.ErrF5DamageInvalidState
	}
	if !rental.CanDamageTransition(from, to) {
		return rental.DamageClaim{}, rental.ErrF5DamageInvalidState
	}
	cur.State = to
	if mutate != nil {
		mutate(&cur)
	}
	cur.UpdatedAt = time.Now().UTC()
	f.byID[id] = cur
	return cur, nil
}
func (f *fakeDamageRepo) ListExpiring(_ context.Context, before time.Time) ([]rental.DamageClaim, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []rental.DamageClaim
	for _, c := range f.byID {
		if (c.State == rental.DamageOpen || c.State == rental.DamageContested) && c.OpenedAt.Before(before) {
			out = append(out, c)
		}
	}
	return out, nil
}

type fakeDebtRepo2 struct{}

func (fakeDebtRepo2) Create(context.Context, rental.Debt) (rental.Debt, error) {
	return rental.Debt{}, nil
}
func (fakeDebtRepo2) GetByID(context.Context, string) (rental.Debt, error) {
	return rental.Debt{}, rental.ErrF5DebtNotFound
}
func (fakeDebtRepo2) GetByDamage(context.Context, string) (rental.Debt, bool, error) {
	return rental.Debt{}, false, nil
}
func (fakeDebtRepo2) UpdateState(context.Context, string, rental.DebtState, rental.DebtState, func(*rental.Debt)) (rental.Debt, error) {
	return rental.Debt{}, nil
}
func (fakeDebtRepo2) ListOpenForRenter(context.Context, string) ([]rental.Debt, error) {
	return nil, nil
}
func (fakeDebtRepo2) ListDueBy(context.Context, time.Time) ([]rental.Debt, error) { return nil, nil }
func (fakeDebtRepo2) Mutate(context.Context, string, func(*rental.Debt)) (rental.Debt, error) {
	return rental.Debt{}, nil
}

type fakeReturnRepo2 struct{}

func (fakeReturnRepo2) Create(context.Context, rental.Return) (rental.Return, error) {
	return rental.Return{}, rental.ErrF5ReturnNotFound
}
func (fakeReturnRepo2) GetByRental(context.Context, string) (rental.Return, bool, error) {
	return rental.Return{}, false, nil
}
func (fakeReturnRepo2) UpdateState(context.Context, string, rental.ReturnState, rental.ReturnState, func(*rental.Return)) (rental.Return, error) {
	return rental.Return{}, nil
}
func (fakeReturnRepo2) Mutate(context.Context, string, func(*rental.Return)) (rental.Return, error) {
	return rental.Return{}, nil
}

// --- helpers ---

func rentalFixture(id, ownerID, renterID string, endsAt time.Time, deposit int64) rental.Rental {
	return rental.Rental{
		ID:              id,
		ListingID:       "listing-1",
		TenantAccountID: renterID,
		State:           rental.StateConfirmed,
		StartsAt:        endsAt.Add(-48 * time.Hour),
		EndsAt:          endsAt,
		DepositCents:    deposit,
		ConfirmedAt:     ptrTime(endsAt.Add(-48 * time.Hour)),
		ListingSnapshot: rental.ListingSnapshot{
			OwnerID:          ownerID,
			DepositCents:     deposit,
			PriceAmountCents: 10000,
		},
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

// --- tests ---------------------------------------------------------------

func TestDamage_OpenClaim_HappyPath(t *testing.T) {
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	rentals := newFakeRentalLookup()
	rentals.put(rentalFixture("r-1", "owner-1", "renter-1", ends, 50000))
	dmg := newFakeDamageRepo()
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, rentals, fakeReturnRepo2{}, dmg, fakeDebtRepo2{})

	claim, err := svc.OpenDamageClaim(context.Background(), f5svc.OpenDamageClaimInput{
		RentalID:      "r-1",
		OwnerID:       "owner-1",
		Nature:        rental.DamageFunctional,
		ProposedCents: 30000,
		Description:   "broken arm",
		Evidence:      rental.EvidencePayload{Photos: []string{"p1"}},
	})
	require.NoError(t, err)
	require.Equal(t, rental.DamageOpen, claim.State)
	require.Equal(t, "owner-1", claim.OwnerID)
	require.Equal(t, "renter-1", claim.RenterID)
}

func TestDamage_OpenClaim_RejectsAfterWindow(t *testing.T) {
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC) // 70h after ends
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	rentals := newFakeRentalLookup()
	rentals.put(rentalFixture("r-1", "owner-1", "renter-1", ends, 50000))
	dmg := newFakeDamageRepo()
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, rentals, fakeReturnRepo2{}, dmg, fakeDebtRepo2{})

	_, err := svc.OpenDamageClaim(context.Background(), f5svc.OpenDamageClaimInput{
		RentalID:      "r-1",
		OwnerID:       "owner-1",
		Nature:        rental.DamageCosmetic,
		ProposedCents: 1000,
	})
	require.ErrorIs(t, err, rental.ErrF5DamageWindowExpired)
}

func TestDamage_OpenClaim_RejectsNonOwner(t *testing.T) {
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	rentals := newFakeRentalLookup()
	rentals.put(rentalFixture("r-1", "owner-1", "renter-1", ends, 50000))
	dmg := newFakeDamageRepo()
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, rentals, fakeReturnRepo2{}, dmg, fakeDebtRepo2{})

	_, err := svc.OpenDamageClaim(context.Background(), f5svc.OpenDamageClaimInput{
		RentalID:      "r-1",
		OwnerID:       "intruder",
		Nature:        rental.DamageCosmetic,
		ProposedCents: 1000,
	})
	require.ErrorIs(t, err, rental.ErrForbidden)
}

func TestDamage_OpenClaim_RejectsExceedingCapForFunctional(t *testing.T) {
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	rentals := newFakeRentalLookup()
	rentals.put(rentalFixture("r-1", "owner-1", "renter-1", ends, 50000))
	dmg := newFakeDamageRepo()
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, rentals, fakeReturnRepo2{}, dmg, fakeDebtRepo2{})

	_, err := svc.OpenDamageClaim(context.Background(), f5svc.OpenDamageClaimInput{
		RentalID:      "r-1",
		OwnerID:       "owner-1",
		Nature:        rental.DamageFunctional,
		ProposedCents: 60000, // > deposit
	})
	require.ErrorIs(t, err, rental.ErrF5DamageAmountInvalid)
}

func TestDamage_OpenClaim_RejectsExceedingCapForLoss(t *testing.T) {
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	rentals := newFakeRentalLookup()
	r := rentalFixture("r-1", "owner-1", "renter-1", ends, 50000)
	// Pretend the asset is declared at 200000 cents (price_amount_cents is not the asset value;
	// we use a sentinel: if DepositCents == 50000, declared value defaults to deposit * 4
	// in v1 — this is policy and lives in the service. To keep tests deterministic, set
	// deposit to 50000 and check the cap of 50000 (deposit) + some declared value.
	// For v1 the loss cap is deposit (we don't yet have declared value in the snapshot);
	// the test below proves we cap functional at deposit; for loss we cap at deposit too
	// until the snapshot gains declared_value.
	rentals.put(r)
	dmg := newFakeDamageRepo()
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, rentals, fakeReturnRepo2{}, dmg, fakeDebtRepo2{})

	_, err := svc.OpenDamageClaim(context.Background(), f5svc.OpenDamageClaimInput{
		RentalID:      "r-1",
		OwnerID:       "owner-1",
		Nature:        rental.DamageLoss,
		ProposedCents: 60000, // > deposit cap (loss cap is deposit in v1)
	})
	require.ErrorIs(t, err, rental.ErrF5DamageAmountInvalid)
}

func TestDamage_OpenClaim_RejectsNoEvidence(t *testing.T) {
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	rentals := newFakeRentalLookup()
	rentals.put(rentalFixture("r-1", "owner-1", "renter-1", ends, 50000))
	dmg := newFakeDamageRepo()
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, rentals, fakeReturnRepo2{}, dmg, fakeDebtRepo2{})

	_, err := svc.OpenDamageClaim(context.Background(), f5svc.OpenDamageClaimInput{
		RentalID:      "r-1",
		OwnerID:       "owner-1",
		Nature:        rental.DamageCosmetic,
		ProposedCents: 1000,
		Evidence:      rental.EvidencePayload{}, // empty: no photos, no description
	})
	require.ErrorIs(t, err, rental.ErrF5DamageEvidenceRequired)
}

func TestDamage_RenterAgree_TransitionsToRenterAgreed(t *testing.T) {
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	rentals := newFakeRentalLookup()
	rentals.put(rentalFixture("r-1", "owner-1", "renter-1", ends, 50000))
	dmg := newFakeDamageRepo()
	claim, err := svcOpen(t, rentals, dmg, now, ends, rental.DamageFunctional, 30000)
	require.NoError(t, err)

	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, rentals, fakeReturnRepo2{}, dmg, fakeDebtRepo2{})
	resolved, err := svc.RenterRespond(context.Background(), f5svc.RenterRespondInput{
		ClaimID:     claim.ID,
		RenterID:    "renter-1",
		Response:    f5svc.ResponseAgree,
		AgreedCents: 30000,
	})
	require.NoError(t, err)
	require.Equal(t, rental.DamageRenterAgreed, resolved.State)
	require.Equal(t, int64(30000), resolved.AgreedCents)
}

func TestDamage_RenterContest_TransitionsToContested(t *testing.T) {
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	rentals := newFakeRentalLookup()
	rentals.put(rentalFixture("r-1", "owner-1", "renter-1", ends, 50000))
	dmg := newFakeDamageRepo()
	claim, err := svcOpen(t, rentals, dmg, now, ends, rental.DamageFunctional, 30000)
	require.NoError(t, err)

	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, rentals, fakeReturnRepo2{}, dmg, fakeDebtRepo2{})
	resolved, err := svc.RenterRespond(context.Background(), f5svc.RenterRespondInput{
		ClaimID:  claim.ID,
		RenterID: "renter-1",
		Response: f5svc.ResponseContest,
		Note:     "the asset was already broken",
	})
	require.NoError(t, err)
	require.Equal(t, rental.DamageContested, resolved.State)
}

func TestDamage_RenterRespond_RejectsAfterWindow(t *testing.T) {
	opened := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	now := opened.Add(49 * time.Hour)
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	rentals := newFakeRentalLookup()
	rentals.put(rentalFixture("r-1", "owner-1", "renter-1", ends, 50000))
	dmg := newFakeDamageRepo()
	claim, err := svcOpenAt(t, rentals, dmg, opened, ends, rental.DamageFunctional, 30000)
	require.NoError(t, err)

	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, rentals, fakeReturnRepo2{}, dmg, fakeDebtRepo2{})
	_, err = svc.RenterRespond(context.Background(), f5svc.RenterRespondInput{
		ClaimID:     claim.ID,
		RenterID:    "renter-1",
		Response:    f5svc.ResponseAgree,
		AgreedCents: 30000,
	})
	require.ErrorIs(t, err, rental.ErrF5DamageWindowExpired)
}

func TestDamage_StaffResolve_RequiresJustification(t *testing.T) {
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	rentals := newFakeRentalLookup()
	rentals.put(rentalFixture("r-1", "owner-1", "renter-1", ends, 50000))
	dmg := newFakeDamageRepo()
	claim, err := svcOpen(t, rentals, dmg, now, ends, rental.DamageFunctional, 30000)
	require.NoError(t, err)

	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, rentals, fakeReturnRepo2{}, dmg, fakeDebtRepo2{})
	// contest first
	_, err = svc.RenterRespond(context.Background(), f5svc.RenterRespondInput{
		ClaimID: claim.ID, RenterID: "renter-1", Response: f5svc.ResponseContest,
	})
	require.NoError(t, err)
	// resolve with empty note
	_, err = svc.StaffResolve(context.Background(), f5svc.StaffResolveInput{
		ClaimID:     claim.ID,
		AgreedCents: 15000,
		Note:        "",
	})
	require.ErrorIs(t, err, rental.ErrF5DamageEvidenceRequired) // reusing — both require justification
}

// helpers
type openFixture struct {
	rentals  *fakeRentalLookup
	dmg      *fakeDamageRepo
	at, ends time.Time
	nature   rental.DamageNature
	proposed int64
}

func (f openFixture) open(t *testing.T) (rental.DamageClaim, error) {
	t.Helper()
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: f.at}}, f.rentals, fakeReturnRepo2{}, f.dmg, fakeDebtRepo2{})
	return svc.OpenDamageClaim(context.Background(), f5svc.OpenDamageClaimInput{
		RentalID:      "r-1",
		OwnerID:       "owner-1",
		Nature:        f.nature,
		ProposedCents: f.proposed,
		Description:   "broken",
		Evidence:      rental.EvidencePayload{Photos: []string{"p1"}, Description: "broken"},
	})
}

//nolint:revive // test fixture: 7 args is intentional for a builder helper
func svcOpen(t *testing.T, rentals *fakeRentalLookup, dmg *fakeDamageRepo, now time.Time, ends time.Time, nature rental.DamageNature, proposed int64) (rental.DamageClaim, error) {
	t.Helper()
	return openFixture{rentals: rentals, dmg: dmg, at: now, ends: ends, nature: nature, proposed: proposed}.open(t)
}

//nolint:revive // test fixture: 7 args is intentional for a builder helper
func svcOpenAt(t *testing.T, rentals *fakeRentalLookup, dmg *fakeDamageRepo, at, ends time.Time, nature rental.DamageNature, proposed int64) (rental.DamageClaim, error) {
	t.Helper()
	return openFixture{rentals: rentals, dmg: dmg, at: at, ends: ends, nature: nature, proposed: proposed}.open(t)
}

type fakeClock2 struct{ t time.Time }

func (c fakeClock2) Now() time.Time { return c.t }
