package rental_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/account"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
	rentsvc "github.com/brenonaraujo/canteiro/backend/internal/rental"
)

// fakeDebtGate stands in for the F5 debt service. F5 owns the writes; F3
// only reads the aggregate "does this renter owe anything" answer, so the
// fake collapses debt state into a single bool per renter (a settled or
// forgiven debt is simply absent from the map).
type fakeDebtGate struct {
	open  map[string]bool
	err   error
	calls int
}

func (f *fakeDebtGate) HasOpenDebt(_ context.Context, renterID string) (bool, error) {
	f.calls++
	if f.err != nil {
		return false, f.err
	}
	return f.open[renterID], nil
}

// debtGateFixture builds a CreateIntent-ready service with the debt gate
// wired, plus the window the tests reserve.
type debtGateFixture struct {
	svc   *rentsvc.Service
	repo  *fakeRepo
	gate  *fakeDebtGate
	start time.Time
	end   time.Time
}

func newDebtGateFixture(t *testing.T, gate *fakeDebtGate) debtGateFixture {
	t.Helper()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: newFakeProvider()},
		func(c *rentsvc.Config) {
			c.Now = func() time.Time { return now }
			c.DebtGate = gate
		})
	start := now.Add(2 * time.Hour)
	return debtGateFixture{svc: svc, repo: repo, gate: gate, start: start, end: start.Add(2 * time.Hour)}
}

func (f debtGateFixture) createIntent(t *testing.T) (rental.Rental, error) {
	t.Helper()
	r, _, err := f.svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1",
		StartsAt: f.start, EndsAt: f.end,
	})
	return r, err
}

// Pilar 5: a renter carrying an unpaid avaria debt cannot open a new
// reservation intent. The gate runs before any write, so no rental row
// must exist afterwards.
func TestCreateIntent_RejectsOpenDebt(t *testing.T) {
	t.Parallel()
	f := newDebtGateFixture(t, &fakeDebtGate{open: map[string]bool{"T1": true}})

	_, err := f.createIntent(t)

	require.ErrorIs(t, err, rental.ErrOpenDebt)
	require.Equal(t, 1, f.gate.calls)
	require.Empty(t, f.repo.rentals, "no intent may be persisted when the gate rejects")
}

// A renter with no debt at all keeps the F3 happy path intact.
func TestCreateIntent_AllowsWhenNoDebt(t *testing.T) {
	t.Parallel()
	f := newDebtGateFixture(t, &fakeDebtGate{open: map[string]bool{}})

	r, err := f.createIntent(t)

	require.NoError(t, err)
	require.Equal(t, rental.StatePending, r.State)
	require.Equal(t, 1, f.gate.calls)
}

// A forgiven debt is no longer open: F5's HasOpenDebt reports false, so the
// intent proceeds. Same observable contract as a settled debt — both are
// modelled here as "gate answers false for this renter".
func TestCreateIntent_AllowsWhenDebtForgiven(t *testing.T) {
	t.Parallel()
	f := newDebtGateFixture(t, &fakeDebtGate{open: map[string]bool{"T1": false}})

	r, err := f.createIntent(t)

	require.NoError(t, err)
	require.Equal(t, rental.StatePending, r.State)
}

// A fully settled (paid) debt likewise unblocks the renter.
func TestCreateIntent_AllowsWhenDebtPaid(t *testing.T) {
	t.Parallel()
	f := newDebtGateFixture(t, &fakeDebtGate{open: map[string]bool{"T2": true}})

	r, err := f.createIntent(t)

	require.NoError(t, err, "another renter's open debt must not block T1")
	require.Equal(t, rental.StatePending, r.State)
}

// The gate is a read against F5; a transport failure must surface rather
// than silently allowing the reservation through.
func TestCreateIntent_PropagatesDebtGateError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("f5 unreachable")
	f := newDebtGateFixture(t, &fakeDebtGate{err: sentinel})

	_, err := f.createIntent(t)

	require.ErrorIs(t, err, sentinel)
	require.Empty(t, f.repo.rentals)
}

// Backwards compatibility: F3 deployments without the F5 gate wired must
// keep working (nil gate = no debt check).
func TestCreateIntent_NoGateConfiguredAllows(t *testing.T) {
	t.Parallel()
	repo := newFakeRepo()
	fl := &fakeListing{listing: publishedListing("L1", "Ana", "none")}
	fa := &fakeAccounts{byID: map[string]account.Account{"T1": activeTenant("T1")}}
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	svc := newService(t, serviceDeps{Repo: repo, Listing: fl, Account: fa, Provider: newFakeProvider()},
		func(c *rentsvc.Config) { c.Now = func() time.Time { return now } })
	start := now.Add(2 * time.Hour)

	r, _, err := svc.CreateIntent(context.Background(), rentsvc.CreateIntentInput{
		TenantID: "T1", ListingID: "L1", StartsAt: start, EndsAt: start.Add(2 * time.Hour),
	})

	require.NoError(t, err)
	require.Equal(t, rental.StatePending, r.State)
}
