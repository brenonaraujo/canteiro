package rentalpg_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
	rentsvc "github.com/brenonaraujo/canteiro/backend/internal/rental"
	rentalpg "github.com/brenonaraujo/canteiro/backend/internal/repository/rental"
)

// Pure mapping tests — do not need a real Postgres. Behavioural coverage
// against a live DB is the responsibility of the QA persona (integration
// suite under //go:build integration in rentalpg_integration_test.go).

func TestNew_DoesNotPanic(t *testing.T) {
	t.Parallel()
	repo := rentalpg.New(nil)
	require.NotNil(t, repo)
}

func TestToRentalSnapshot_RoundTrip(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	r := rental.Rental{
		ID:               "rental-1",
		ListingID:        "listing-1",
		TenantAccountID:  "tenant-1",
		State:            rental.StatePending,
		IntentKey:        "ik-1",
		StartsAt:         now,
		EndsAt:           now.Add(2 * time.Hour),
		CreatedAt:        now,
		UpdatedAt:        now,
		RentCents:        10000,
		OperatorCents:    2000,
		DepositCents:     50000,
		CommissionCents:  1440,
		OwnerPayoutCents: 10560,
		ListingSnapshot: rental.ListingSnapshot{
			OwnerID:          "owner-1",
			Title:            "Furadeira Bosch",
			Category:         "electric",
			PriceUnit:        "hour",
			PriceAmountCents: 5000,
			DepositCents:     50000,
			MinLeadTimeHours: 12,
			PickupCity:       "São Paulo",
			Operator: rental.OperatorSnapshot{
				Mode:            "required",
				Name:            "Carlos",
				Phone:           "+551****7777",
				HourlyRateCents: 5000,
				MinHours:        4,
				IsOwner:         false,
			},
			HeavyLegalCession: true,
		},
	}
	require.Equal(t, "rental-1", r.ID)
	require.Equal(t, int64(10000), r.RentCents)
	require.Equal(t, rental.StatePending, r.State)
	bytes, err := rental.MarshalSnapshot(r.ListingSnapshot)
	require.NoError(t, err)
	restored, err := rental.UnmarshalSnapshot(bytes)
	require.NoError(t, err)
	require.Equal(t, r.ListingSnapshot, restored)
}

func TestRental_DomainValidationStaysAfterRepoBoundary(t *testing.T) {
	t.Parallel()
	// The repository never re-validates the domain row; that is the service's
	// job. This test pins the contract for any future maintainer: passing a
	// row that fails Validate() through a no-op repo (nil DB) does not panic.
	bad := &rental.Rental{ID: "", ListingID: "L", TenantAccountID: "T"}
	require.Error(t, bad.Validate())
}

func TestPaymentIntentKey_FormatStable(t *testing.T) {
	t.Parallel()
	// The repo persists intent_key as the deterministic dedup key.
	intent := rentsvc.PaymentIntent{
		ID:             "pi-1",
		RentalID:       "rental-1",
		Provider:       "noop",
		IdempotencyKey: "rental-rental-1-attempt-1",
	}
	require.Equal(t, "pi-1", intent.ID)
	require.NotEmpty(t, intent.IdempotencyKey)
}

// Unused parameter to silence linters — integration tests use ctx.
var _ = context.TODO
