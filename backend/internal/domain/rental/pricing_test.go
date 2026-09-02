package rental

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPriceQuote_AC5_NoOperator(t *testing.T) {
	t.Parallel()
	snap := ListingSnapshot{
		Title: "Furadeira", Category: "electric",
		PriceUnit: "hour", PriceAmountCents: 5000, DepositCents: 20000,
		Operator: OperatorSnapshot{Mode: "none"},
	}
	start := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	q, err := PriceQuote(QuoteInput{Snapshot: snap, StartsAt: start, EndsAt: end})
	require.NoError(t, err)
	require.Equal(t, int64(10000), q.RentCents)
	require.Equal(t, int64(0), q.OperatorCents)
	require.Equal(t, int64(20000), q.DepositCents)
	require.Equal(t, int64(30000), q.TotalCents)
	require.Equal(t, int64(10000), q.CommissionBaseCents)
	require.Equal(t, int64(1200), q.CommissionCents)
}

func TestPriceQuote_AC5_OwnerIsOperator(t *testing.T) {
	t.Parallel()
	snap := ListingSnapshot{
		PriceUnit: "hour", PriceAmountCents: 5000, DepositCents: 20000,
		Operator: OperatorSnapshot{Mode: "optional", HourlyRateCents: 3000, IsOwner: true},
	}
	start := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(3 * time.Hour)
	q, err := PriceQuote(QuoteInput{Snapshot: snap, StartsAt: start, EndsAt: end, WithOperator: true})
	require.NoError(t, err)
	require.Equal(t, int64(15000), q.RentCents)
	require.Equal(t, int64(9000), q.OperatorCents)
	require.Equal(t, int64(20000), q.DepositCents)
	require.Equal(t, int64(24000), q.CommissionBaseCents)
	require.Equal(t, int64(2880), q.CommissionCents)
}

func TestPriceQuote_AC5_ThirdPartyOperator(t *testing.T) {
	t.Parallel()
	snap := ListingSnapshot{
		PriceUnit: "hour", PriceAmountCents: 5000, DepositCents: 20000,
		Operator: OperatorSnapshot{Mode: "required", HourlyRateCents: 3000, IsOwner: false},
	}
	start := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	q, err := PriceQuote(QuoteInput{Snapshot: snap, StartsAt: start, EndsAt: end, WithOperator: true})
	require.NoError(t, err)
	require.Equal(t, int64(10000), q.RentCents)
	require.Equal(t, int64(6000), q.OperatorCents)
	require.Equal(t, int64(16000), q.CommissionBaseCents)
	require.Equal(t, int64(1920), q.CommissionCents)
	require.Equal(t, int64(8800), q.OwnerPayoutCents)
	require.Equal(t, int64(5280), q.OperatorPayoutCents)
}

func TestPriceQuote_AC7_DepositNeverInCommission(t *testing.T) {
	t.Parallel()
	snap := ListingSnapshot{
		PriceUnit: "hour", PriceAmountCents: 100, DepositCents: 999999,
		Operator: OperatorSnapshot{Mode: "none"},
	}
	start := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	q, err := PriceQuote(QuoteInput{Snapshot: snap, StartsAt: start, EndsAt: end})
	require.NoError(t, err)
	require.Equal(t, int64(100), q.CommissionBaseCents, "deposit MUST NOT enter base")
	require.Equal(t, int64(12), q.CommissionCents)
	require.Equal(t, int64(999999), q.DepositCents)
}

func TestPriceQuote_RejectsOperatorNoneWhenRequested(t *testing.T) {
	t.Parallel()
	snap := ListingSnapshot{
		PriceUnit: "hour", PriceAmountCents: 5000, DepositCents: 20000,
		Operator: OperatorSnapshot{Mode: "none"},
	}
	start := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	_, err := PriceQuote(QuoteInput{Snapshot: snap, StartsAt: start, EndsAt: end, WithOperator: true})
	require.ErrorIs(t, err, ErrOperatorNotAvailable)
}

func TestPriceQuote_RejectsInvalidWindow(t *testing.T) {
	t.Parallel()
	snap := ListingSnapshot{
		PriceUnit: "hour", PriceAmountCents: 5000, DepositCents: 20000,
		Operator: OperatorSnapshot{Mode: "none"},
	}
	start := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	_, err := PriceQuote(QuoteInput{Snapshot: snap, StartsAt: start, EndsAt: start})
	require.ErrorIs(t, err, ErrInvalidInput)
}
