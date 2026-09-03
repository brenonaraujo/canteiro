package f5_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
	f5svc "github.com/brenonaraujo/canteiro/backend/internal/rental/f5"
)

func TestDamage_ExpireStale_TransitionsOpenToExpired(t *testing.T) {
	opened := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	now := opened.Add(50 * time.Hour) // past the 48h defense window
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	rentals := newFakeRentalLookup()
	rentals.put(rentalFixture("r-1", "owner-1", "renter-1", ends, 50000))
	dmg := newFakeDamageRepo()
	claim, err := svcOpenAt(t, rentals, dmg, opened, ends, rental.DamageFunctional, 30000)
	require.NoError(t, err)
	_ = claim

	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, rentals, fakeReturnRepo2{}, dmg, fakeDebtRepo2{})
	n, err := svc.ExpireStale(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, n)

	// Verify the claim is now Expired
	got, err := dmg.GetByID(context.Background(), claim.ID)
	require.NoError(t, err)
	require.Equal(t, rental.DamageExpired, got.State)
}

func TestDamage_ExpireStale_SkipsClaimsStillWithinWindow(t *testing.T) {
	opened := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	now := opened.Add(10 * time.Hour) // still inside 48h
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	rentals := newFakeRentalLookup()
	rentals.put(rentalFixture("r-1", "owner-1", "renter-1", ends, 50000))
	dmg := newFakeDamageRepo()
	_, err := svcOpenAt(t, rentals, dmg, opened, ends, rental.DamageFunctional, 30000)
	require.NoError(t, err)

	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, rentals, fakeReturnRepo2{}, dmg, fakeDebtRepo2{})
	n, err := svc.ExpireStale(context.Background())
	require.NoError(t, err)
	require.Equal(t, 0, n)
}

func TestDamage_StaffResolve_HappyPath(t *testing.T) {
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	rentals := newFakeRentalLookup()
	rentals.put(rentalFixture("r-1", "owner-1", "renter-1", ends, 50000))
	dmg := newFakeDamageRepo()
	claim, err := svcOpen(t, rentals, dmg, now, ends, rental.DamageFunctional, 30000)
	require.NoError(t, err)
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, rentals, fakeReturnRepo2{}, dmg, fakeDebtRepo2{})

	_, err = svc.RenterRespond(context.Background(), f5svc.RenterRespondInput{
		ClaimID: claim.ID, RenterID: "renter-1", Response: f5svc.ResponseContest, Note: "dispute",
	})
	require.NoError(t, err)

	resolved, err := svc.StaffResolve(context.Background(), f5svc.StaffResolveInput{
		ClaimID: claim.ID, AgreedCents: 15000, Note: "evidence supports owner",
	})
	require.NoError(t, err)
	require.Equal(t, rental.DamageStaffResolved, resolved.State)
	require.Equal(t, int64(15000), resolved.AgreedCents)
	require.NotNil(t, resolved.DecidedAt)
	require.NotNil(t, resolved.ResolvedAt)
}

func TestDamage_StaffResolve_RejectsAmountOverCap(t *testing.T) {
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	rentals := newFakeRentalLookup()
	rentals.put(rentalFixture("r-1", "owner-1", "renter-1", ends, 50000))
	dmg := newFakeDamageRepo()
	claim, err := svcOpen(t, rentals, dmg, now, ends, rental.DamageFunctional, 30000)
	require.NoError(t, err)
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, rentals, fakeReturnRepo2{}, dmg, fakeDebtRepo2{})

	_, err = svc.RenterRespond(context.Background(), f5svc.RenterRespondInput{
		ClaimID: claim.ID, RenterID: "renter-1", Response: f5svc.ResponseContest, Note: "dispute",
	})
	require.NoError(t, err)
	_, err = svc.StaffResolve(context.Background(), f5svc.StaffResolveInput{
		ClaimID: claim.ID, AgreedCents: 60000, Note: "way over",
	})
	require.ErrorIs(t, err, rental.ErrF5DamageAmountInvalid)
}

func TestDamage_RenterRespond_RejectsWrongCaller(t *testing.T) {
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	rentals := newFakeRentalLookup()
	rentals.put(rentalFixture("r-1", "owner-1", "renter-1", ends, 50000))
	dmg := newFakeDamageRepo()
	claim, err := svcOpen(t, rentals, dmg, now, ends, rental.DamageFunctional, 30000)
	require.NoError(t, err)
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, rentals, fakeReturnRepo2{}, dmg, fakeDebtRepo2{})

	_, err = svc.RenterRespond(context.Background(), f5svc.RenterRespondInput{
		ClaimID: claim.ID, RenterID: "intruder", Response: f5svc.ResponseAgree, AgreedCents: 30000,
	})
	require.ErrorIs(t, err, rental.ErrForbidden)
}

func TestDamage_RenterRespond_RejectsDoubleAgree(t *testing.T) {
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	ends := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	rentals := newFakeRentalLookup()
	rentals.put(rentalFixture("r-1", "owner-1", "renter-1", ends, 50000))
	dmg := newFakeDamageRepo()
	claim, err := svcOpen(t, rentals, dmg, now, ends, rental.DamageFunctional, 30000)
	require.NoError(t, err)
	svc := f5svc.NewService(f5svc.Config{Now: fakeClock2{t: now}}, rentals, fakeReturnRepo2{}, dmg, fakeDebtRepo2{})

	_, err = svc.RenterRespond(context.Background(), f5svc.RenterRespondInput{
		ClaimID: claim.ID, RenterID: "renter-1", Response: f5svc.ResponseAgree, AgreedCents: 30000,
	})
	require.NoError(t, err)
	_, err = svc.RenterRespond(context.Background(), f5svc.RenterRespondInput{
		ClaimID: claim.ID, RenterID: "renter-1", Response: f5svc.ResponseAgree, AgreedCents: 30000,
	})
	require.ErrorIs(t, err, rental.ErrF5DamageAlreadyAgreed)
}
