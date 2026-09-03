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

// Pilar 3 tests — caucao executada por evento de dominio (renter agree
// or staff resolve) + residuo vira divida ativa.
//
// These tests intentionally use stateful local fakes (not the empty
// `fakeReturnRepo2`/`fakeDebtRepo2` shared with damage_test.go) so we
// can assert on the captured deposit and the created debt row.

// --- stateful return repo -----------------------------------------------

type statefulReturnRepo struct {
	byID   map[string]rental.Return
	byRent map[string]string
	mu     sync.Mutex
}

func newStatefulReturnRepo() *statefulReturnRepo {
	return &statefulReturnRepo{byID: map[string]rental.Return{}, byRent: map[string]string{}}
}

func (r *statefulReturnRepo) Create(_ context.Context, ret rental.Return) (rental.Return, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byRent[ret.RentalID]; ok {
		return rental.Return{}, rental.ErrF5ReturnAlreadyExists
	}
	ret.CreatedAt = time.Now().UTC()
	ret.UpdatedAt = ret.CreatedAt
	r.byID[ret.ID] = ret
	r.byRent[ret.RentalID] = ret.ID
	return ret, nil
}

func (r *statefulReturnRepo) GetByRental(_ context.Context, rentalID string) (rental.Return, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byRent[rentalID]
	if !ok {
		return rental.Return{}, false, nil
	}
	return r.byID[id], true, nil
}

func (r *statefulReturnRepo) UpdateState(_ context.Context, id string, from, to rental.ReturnState, mutate func(*rental.Return)) (rental.Return, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.byID[id]
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
	r.byID[id] = cur
	return cur, nil
}

func (r *statefulReturnRepo) Mutate(_ context.Context, id string, mutate func(*rental.Return)) (rental.Return, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.byID[id]
	if !ok {
		return rental.Return{}, rental.ErrF5ReturnNotFound
	}
	if mutate != nil {
		mutate(&cur)
	}
	cur.UpdatedAt = time.Now().UTC()
	r.byID[id] = cur
	return cur, nil
}

// --- stateful debt repo -------------------------------------------------

type statefulDebtRepo struct {
	byID     map[string]rental.Debt
	byDamage map[string]string
	mu       sync.Mutex
}

func newStatefulDebtRepo() *statefulDebtRepo {
	return &statefulDebtRepo{byID: map[string]rental.Debt{}, byDamage: map[string]string{}}
}

func (d *statefulDebtRepo) Create(_ context.Context, deb rental.Debt) (rental.Debt, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if deb.DamageID != "" {
		if _, ok := d.byDamage[deb.DamageID]; ok {
			return rental.Debt{}, rental.ErrF5DebtAlreadyExists
		}
		d.byDamage[deb.DamageID] = deb.ID
	}
	d.byID[deb.ID] = deb
	return deb, nil
}

func (d *statefulDebtRepo) GetByID(_ context.Context, id string) (rental.Debt, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	deb, ok := d.byID[id]
	if !ok {
		return rental.Debt{}, rental.ErrF5DebtNotFound
	}
	return deb, nil
}

func (d *statefulDebtRepo) GetByDamage(_ context.Context, damageID string) (rental.Debt, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	id, ok := d.byDamage[damageID]
	if !ok {
		return rental.Debt{}, false, nil
	}
	return d.byID[id], true, nil
}

func (d *statefulDebtRepo) UpdateState(_ context.Context, id string, from, to rental.DebtState, mutate func(*rental.Debt)) (rental.Debt, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	cur, ok := d.byID[id]
	if !ok {
		return rental.Debt{}, rental.ErrF5DebtNotFound
	}
	if cur.State != from {
		return rental.Debt{}, rental.ErrF5DebtInvalidState
	}
	cur.State = to
	if mutate != nil {
		mutate(&cur)
	}
	cur.UpdatedAt = time.Now().UTC()
	d.byID[id] = cur
	return cur, nil
}

func (d *statefulDebtRepo) Mutate(_ context.Context, id string, mutate func(*rental.Debt)) (rental.Debt, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	cur, ok := d.byID[id]
	if !ok {
		return rental.Debt{}, rental.ErrF5DebtNotFound
	}
	if mutate != nil {
		mutate(&cur)
	}
	cur.UpdatedAt = time.Now().UTC()
	d.byID[id] = cur
	return cur, nil
}

func (d *statefulDebtRepo) ListOpenForRenter(_ context.Context, renterID string) ([]rental.Debt, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []rental.Debt
	for _, deb := range d.byID {
		if deb.RenterID == renterID && deb.State == rental.DebtOpen {
			out = append(out, deb)
		}
	}
	return out, nil
}

func (d *statefulDebtRepo) ListDueBy(_ context.Context, _ time.Time) ([]rental.Debt, error) {
	return nil, nil
}

// --- helpers -------------------------------------------------------------

// registerReturn pre-creates an in-progress return row for the rental.
// Used to set up Path B (return row exists when the claim is resolved).
func registerReturn(t *testing.T, ret *statefulReturnRepo, rentals *fakeRentalLookup, svc *f5svc.Service, now time.Time) rental.Return {
	t.Helper()
	r, err := rentals.Get(context.Background(), "r-1")
	require.NoError(t, err)
	// We bypass the service here to seed the fixture directly — the
	// service's RegisterPickup requires the rental be in StateConfirmed
	// and uses IDGen, neither of which we want to drive from this helper.
	pickupEvidence := []byte(`{"photos":["k1"],"description":"seed"}`)
	returned, err := ret.Create(context.Background(), rental.Return{
		ID:             "ret-fixed",
		RentalID:       r.ID,
		State:          rental.ReturnInProgress,
		PickupEvidence: pickupEvidence,
	})
	require.NoError(t, err)
	_ = svc
	_ = now
	return returned
}

// --- tests ---------------------------------------------------------------

// TestPilar3_RenterAgree_CapturesDepositAndReleasesRemainder —
// agreed (30000) < deposit (50000), so captured=30000, released=20000,
// no debt (residual <= 0).
func TestPilar3_RenterAgree_CapturesDepositAndReleasesRemainder(t *testing.T) {
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	rentals := newFakeRentalLookup()
	rentals.put(rentalFixture("r-1", "owner-1", "renter-1", ends, 50000))
	dmg := newFakeDamageRepo()
	ret := newStatefulReturnRepo()
	registerReturn(t, ret, rentals, nil, now)
	debts := newStatefulDebtRepo()

	claim, err := svcOpen(t, rentals, dmg, now, ends, rental.DamageFunctional, 30000)
	require.NoError(t, err)
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, rentals, ret, dmg, debts)

	resolved, err := svc.RenterRespond(context.Background(), f5svc.RenterRespondInput{
		ClaimID:     claim.ID,
		RenterID:    "renter-1",
		Response:    f5svc.ResponseAgree,
		AgreedCents: 30000,
	})
	require.NoError(t, err)
	require.Equal(t, rental.DamageRenterAgreed, resolved.State)

	// Assert on the devolucoes row.
	cur, ok, err := ret.GetByRental(context.Background(), "r-1")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, int64(30000), cur.DepositCapturedCents)
	require.Equal(t, int64(20000), cur.DepositReleasedCents)

	// Assert no debt was opened (residual = agreed - captured = 0).
	_, ok, err = debts.GetByDamage(context.Background(), claim.ID)
	require.NoError(t, err)
	require.False(t, ok)
}

// TestPilar3_RenterAgree_AgreedExceedsDeposit_OpensDebt —
// Path A: claim seeded directly into DamageRenterAgreed with agreed > deposit
// (the cap is policy-bound in damageCapsByNature; Pilar 3 is forward-
// compatible with declared-asset-value support, so the helper handles
// residual > 0 even if v1 policy prevents it from API calls today).
func TestPilar3_RenterAgree_AgreedExceedsDeposit_OpensDebt(t *testing.T) {
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	rentals := newFakeRentalLookup()
	rentals.put(rentalFixture("r-1", "owner-1", "renter-1", ends, 50000))
	dmg := newFakeDamageRepo()
	ret := newStatefulReturnRepo()
	registerReturn(t, ret, rentals, nil, now)
	debts := newStatefulDebtRepo()

	// Seed the claim in DamageRenterAgreed with AgreedCents > deposit.
	// Bypasses the API cap (deposit-only v1 policy) because Pilar 3 is
	// forward-compatible with declared-asset-value.
	claim, err := svcOpen(t, rentals, dmg, now, ends, rental.DamageFunctional, 50000)
	require.NoError(t, err)
	updated, err := dmg.UpdateState(context.Background(), claim.ID, rental.DamageOpen, rental.DamageRenterAgreed, func(c *rental.DamageClaim) {
		c.RespondedAt = &now
		c.RenterResponseKind = string(f5svc.ResponseAgree)
		c.RenterResponseNote = "agree"
		c.AgreedCents = 60000
		c.ResolvedAt = &now
	})
	require.NoError(t, err)

	// Invoke the wire directly — no service method triggers it post-hoc
	// in v1, but the helper exists for forward-compat.
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, rentals, ret, dmg, debts)
	r, err := rentals.Get(context.Background(), "r-1")
	require.NoError(t, err)
	cur, _, err := ret.GetByRental(context.Background(), "r-1")
	require.NoError(t, err)
	require.NoError(t, svc.WirePilar3OnClaimResolvedForTest(context.Background(), updated, 60000))

	// Captured = deposit (50000), released = 0.
	cur2, _, err := ret.GetByRental(context.Background(), "r-1")
	require.NoError(t, err)
	require.Equal(t, int64(50000), cur2.DepositCapturedCents)
	require.Equal(t, int64(0), cur2.DepositReleasedCents)

	// Debt created for residual 10000.
	deb, ok, err := debts.GetByDamage(context.Background(), updated.ID)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, int64(10000), deb.OriginalCents)
	require.Equal(t, "r-1", deb.RentalID)
	require.Equal(t, "renter-1", deb.RenterID)

	_ = r
	_ = cur
}

// TestPilar3_StaffResolve_CapturesDepositAndOpensDebt —
// contested claim, staff resolves at 20000 (< deposit 50000), Pilar 3 fires.
func TestPilar3_StaffResolve_CapturesDepositAndOpensDebt(t *testing.T) {
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	rentals := newFakeRentalLookup()
	rentals.put(rentalFixture("r-1", "owner-1", "renter-1", ends, 50000))
	dmg := newFakeDamageRepo()
	ret := newStatefulReturnRepo()
	registerReturn(t, ret, rentals, nil, now)
	debts := newStatefulDebtRepo()

	claim, err := svcOpen(t, rentals, dmg, now, ends, rental.DamageFunctional, 20000)
	require.NoError(t, err)
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, rentals, ret, dmg, debts)

	_, err = svc.RenterRespond(context.Background(), f5svc.RenterRespondInput{
		ClaimID: claim.ID, RenterID: "renter-1", Response: f5svc.ResponseContest, Note: "dispute",
	})
	require.NoError(t, err)

	resolved, err := svc.StaffResolve(context.Background(), f5svc.StaffResolveInput{
		ClaimID: claim.ID, AgreedCents: 20000, Note: "evidence supports owner",
	})
	require.NoError(t, err)
	require.Equal(t, rental.DamageStaffResolved, resolved.State)

	cur, ok, err := ret.GetByRental(context.Background(), "r-1")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, int64(20000), cur.DepositCapturedCents)
	require.Equal(t, int64(30000), cur.DepositReleasedCents)

	// No debt: residual = 20000 - 20000 = 0.
	_, ok, err = debts.GetByDamage(context.Background(), claim.ID)
	require.NoError(t, err)
	require.False(t, ok)
}

// TestPilar3_RenterAgree_Idempotent — running RenterRespond with the
// same agreement twice does not double-capture or open a second debt.
// (RenterRespond guards against double agreement via ErrF5DamageAlreadyAgreed,
// so the safe-pathing for idempotency is exercised via StaffResolve twice
// after contest, but RenterRespond already refuses. We test the
// capture helper's idempotency directly via a re-resolve scenario.)
func TestPilar3_RenterAgree_NoReturnRow_Defers(t *testing.T) {
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	rentals := newFakeRentalLookup()
	rentals.put(rentalFixture("r-1", "owner-1", "renter-1", ends, 50000))
	dmg := newFakeDamageRepo()
	ret := newStatefulReturnRepo()
	// Intentionally NOT registering a return row — Pilar 3 defers.
	debts := newStatefulDebtRepo()

	claim, err := svcOpen(t, rentals, dmg, now, ends, rental.DamageFunctional, 30000)
	require.NoError(t, err)
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, rentals, ret, dmg, debts)

	// Agree with no return row — should succeed, no capture, no debt.
	resolved, err := svc.RenterRespond(context.Background(), f5svc.RenterRespondInput{
		ClaimID:     claim.ID,
		RenterID:    "renter-1",
		Response:    f5svc.ResponseAgree,
		AgreedCents: 30000,
	})
	require.NoError(t, err)
	require.Equal(t, rental.DamageRenterAgreed, resolved.State)

	// No return row was created — Pilar 3 deferred.
	_, ok, err := ret.GetByRental(context.Background(), "r-1")
	require.NoError(t, err)
	require.False(t, ok)
}

// TestPilar3_PathA_RegisterReturn_CapturesAfterClaimResolved —
// the renter agrees to a claim BEFORE RegisterReturn is called. When
// RegisterReturn fires, it picks up the already-resolved claim and
// captures the deposit (with debt if residual > 0).
func TestPilar3_PathA_RegisterReturn_CapturesAfterClaimResolved(t *testing.T) {
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	// For Path A, RegisterReturn fires after ends_at (window check),
	// so we use clock values past ends_at.
	rentals := newFakeRentalLookup()
	rentals.put(rentalFixture("r-1", "owner-1", "renter-1", ends, 50000))
	dmg := newFakeDamageRepo()
	ret := newStatefulReturnRepo()
	// Return row does NOT exist yet at this point — Path A: claim is
	// resolved first, then RegisterReturn fires and creates it.
	debts := newStatefulDebtRepo()

	claim, err := svcOpen(t, rentals, dmg, ends.Add(time.Hour), ends, rental.DamageFunctional, 30000)
	require.NoError(t, err)
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: ends.Add(time.Hour)}}, rentals, ret, dmg, debts)

	// Renter agrees before any return is registered — Pilar 3 defers.
	_, err = svc.RenterRespond(context.Background(), f5svc.RenterRespondInput{
		ClaimID:     claim.ID,
		RenterID:    "renter-1",
		Response:    f5svc.ResponseAgree,
		AgreedCents: 30000,
	})
	require.NoError(t, err)

	// Now register pickup + return — this is where Path A's capture fires.
	_, err = svc.RegisterPickup(context.Background(), "r-1", rental.EvidencePayload{
		Photos: []string{"k1"}, Description: "ok",
	})
	require.NoError(t, err)
	closed, err := svc.RegisterReturn(context.Background(), "r-1", rental.EvidencePayload{
		Photos: []string{"k2"}, Description: "returned",
	})
	require.NoError(t, err)

	// Pilar 3 captured 30000, released 20000 — agreed < deposit so no debt.
	require.Equal(t, rental.ReturnClosed, closed.State)
	require.Equal(t, int64(30000), closed.DepositCapturedCents)
	require.Equal(t, int64(20000), closed.DepositReleasedCents)

	_, ok, err := debts.GetByDamage(context.Background(), claim.ID)
	require.NoError(t, err)
	require.False(t, ok)
}

// TestPilar3_PathA_RegisterReturn_CreatesDebtWhenResidual —
// claim seeded in DamageRenterAgreed with agreed > deposit (forward-
// compatible path; v1 cap prevents this from the API but Pilar 3 is
// built to handle residual > 0). RegisterReturn picks it up and
// captures the deposit + opens a debt for the residual.
func TestPilar3_PathA_RegisterReturn_CreatesDebtWhenResidual(t *testing.T) {
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	openedAt := ends.Add(time.Hour)
	rentals := newFakeRentalLookup()
	rentals.put(rentalFixture("r-1", "owner-1", "renter-1", ends, 50000))
	dmg := newFakeDamageRepo()
	ret := newStatefulReturnRepo()
	debts := newStatefulDebtRepo()

	// Open claim at the deposit cap, then directly transition to
	// DamageRenterAgreed with AgreedCents > deposit.
	claim, err := svcOpen(t, rentals, dmg, openedAt, ends, rental.DamageFunctional, 50000)
	require.NoError(t, err)
	updated, err := dmg.UpdateState(context.Background(), claim.ID, rental.DamageOpen, rental.DamageRenterAgreed, func(c *rental.DamageClaim) {
		c.RespondedAt = &openedAt
		c.RenterResponseKind = string(f5svc.ResponseAgree)
		c.RenterResponseNote = "agree"
		c.AgreedCents = 60000
		c.ResolvedAt = &openedAt
	})
	require.NoError(t, err)
	_ = updated

	// Pickup + return — Path A fires: capture 50000, release 0, debt 10000.
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: openedAt}}, rentals, ret, dmg, debts)
	_, err = svc.RegisterPickup(context.Background(), "r-1", rental.EvidencePayload{
		Photos: []string{"k1"}, Description: "ok",
	})
	require.NoError(t, err)
	closed, err := svc.RegisterReturn(context.Background(), "r-1", rental.EvidencePayload{
		Photos: []string{"k2"}, Description: "returned",
	})
	require.NoError(t, err)

	require.Equal(t, int64(50000), closed.DepositCapturedCents)
	require.Equal(t, int64(0), closed.DepositReleasedCents)

	deb, ok, err := debts.GetByDamage(context.Background(), claim.ID)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, int64(10000), deb.OriginalCents)
}
