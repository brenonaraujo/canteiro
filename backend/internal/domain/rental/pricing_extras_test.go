package rental

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRentHours_DayUnitAndRemainder(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		unit     string
		dur      time.Duration
		expected int64
	}{
		{name: "exact day", unit: "day", dur: 24 * time.Hour, expected: 1},
		{name: "two days", unit: "day", dur: 48 * time.Hour, expected: 2},
		{name: "day plus one hour rounds up", unit: "day", dur: 25 * time.Hour, expected: 2},
		{name: "hour remainder rounds up", unit: "hour", dur: 90 * time.Minute, expected: 2},
		{name: "zero duration", unit: "hour", dur: 0, expected: 0},
		{name: "negative duration", unit: "hour", dur: -time.Hour, expected: 0},
		{name: "unknown unit", unit: "garbage", dur: 3 * time.Hour, expected: 0},
		{name: "empty unit defaults hour", unit: "", dur: 90 * time.Minute, expected: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := rentHours(start, start.Add(tc.dur), tc.unit)
			require.Equal(t, tc.expected, got)
		})
	}
}

func TestRentCents_HandlesNegatives(t *testing.T) {
	t.Parallel()
	require.Equal(t, int64(0), rentCents(-1, 10))
	require.Equal(t, int64(0), rentCents(100, -1))
	require.Equal(t, int64(200), rentCents(100, 2))
}

func TestOperatorCents_MinHoursApplied(t *testing.T) {
	t.Parallel()
	snap := ListingSnapshot{Operator: OperatorSnapshot{HourlyRateCents: 5000, MinHours: 5}}
	// 2 hours requested but MinHours=5 → operator billed for 5 hours.
	got := operatorCents(snap, 2, true)
	require.Equal(t, int64(25000), got)
}

func TestOperatorCents_NoRateMeansZero(t *testing.T) {
	t.Parallel()
	snap := ListingSnapshot{Operator: OperatorSnapshot{HourlyRateCents: 0, MinHours: 5}}
	got := operatorCents(snap, 10, true)
	require.Equal(t, int64(0), got)
}

func TestApplyCommission_RejectsZeroBase(t *testing.T) {
	t.Parallel()
	require.Equal(t, int64(0), applyCommission(0, 1200))
	require.Equal(t, int64(0), applyCommission(100, 0))
	require.Equal(t, int64(0), applyCommission(-10, 1200))
}

func TestEffectiveBPS_DefaultsOnZero(t *testing.T) {
	t.Parallel()
	require.Equal(t, DefaultCommissionBPS, effectiveBPS(0))
	require.Equal(t, DefaultCommissionBPS, effectiveBPS(-1))
	require.Equal(t, int64(900), effectiveBPS(900))
}

func TestSplitLiquids_RejectsNegatives(t *testing.T) {
	t.Parallel()
	owner, op := splitLiquids(-1, 0, ListingSnapshot{}, 100)
	require.Equal(t, int64(0), owner)
	require.Equal(t, int64(0), op)
}

func TestSplitLiquids_ZeroBase(t *testing.T) {
	t.Parallel()
	owner, op := splitLiquids(0, 0, ListingSnapshot{}, 0)
	require.Equal(t, int64(0), owner)
	require.Equal(t, int64(0), op)
}

func TestMoneyBreakdown_ApplyToRentalCopiesFields(t *testing.T) {
	t.Parallel()
	r := &Rental{}
	b := MoneyBreakdown{
		RentCents:           100,
		OperatorCents:       200,
		DepositCents:        300,
		CommissionCents:     40,
		OwnerPayoutCents:    50,
		OperatorPayoutCents: 60,
	}
	b.ApplyToRental(r)
	require.Equal(t, int64(100), r.RentCents)
	require.Equal(t, int64(200), r.OperatorCents)
	require.Equal(t, int64(300), r.DepositCents)
	require.Equal(t, int64(40), r.CommissionCents)
	require.Equal(t, int64(50), r.OwnerPayoutCents)
	require.Equal(t, int64(60), r.OperatorPayoutCents)
}

func TestReceiptFromRental_PropagatesSnapshotAndWindow(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	r := Rental{
		ID:              "rental-r",
		TenantAccountID: "tenant-r",
		ListingSnapshot: ListingSnapshot{OwnerID: "owner-r", Title: "Betoneira"},
		StartsAt:        start,
		EndsAt:          start.Add(4 * time.Hour),
	}
	b := MoneyBreakdown{
		RentCents:           1000,
		OperatorCents:       500,
		DepositCents:        5000,
		TotalCents:          6500,
		CommissionBaseCents: 1500,
		CommissionCents:     180,
		OwnerPayoutCents:    900,
		OperatorPayoutCents: 420,
	}
	rec := ReceiptFromRental(r, b)
	require.Equal(t, r.ID, rec.RentalID)
	require.Equal(t, r.TenantAccountID, rec.TenantAccountID)
	require.Equal(t, r.ListingSnapshot, rec.ListingSnapshot)
	require.True(t, rec.WindowStartsAt.Equal(r.StartsAt))
	require.True(t, rec.WindowEndsAt.Equal(r.EndsAt))
	require.Equal(t, b.TotalCents, rec.TotalCents)
	require.Equal(t, b.CommissionCents, rec.CommissionCents)
}
