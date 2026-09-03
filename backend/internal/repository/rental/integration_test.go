//go:build integration

package rentalpg_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
	rentalpg "github.com/brenonaraujo/canteiro/backend/internal/repository/rental"
)

// Integration tests for rentalpg. Run only when DATABASE_URL points at a
// reachable Postgres + the migrations have been applied:
//
//	go test -tags=integration ./internal/repository/rental/...
//
// The QA persona's smoke suite spins the database up; the gate for these
// tests is the integration job, not the per-PR coverage gate.

func openIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	db, err := gorm.Open(postgres.Open(url), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func TestIntegration_CreateAndQueryRental(t *testing.T) {
	db := openIntegrationDB(t)
	repo := rentalpg.New(db)
	ctx := context.Background()

	// Find any existing tenant/listing pair the test DB has — we just want
	// a valid FK target, we are not asserting ownership here.
	var tenantID, listingID string
	require.NoError(t, db.Raw("SELECT id FROM accounts LIMIT 1").Scan(&tenantID).Error)
	require.NoError(t, db.Raw("SELECT id FROM listings LIMIT 1").Scan(&listingID).Error)
	if tenantID == "" || listingID == "" {
		t.Skip("no accounts/listings rows present; run migrations + seed before this test")
	}

	now := time.Now().UTC().Truncate(time.Second)
	start := now.Add(2 * time.Hour)
	end := start.Add(2 * time.Hour)
	r := rental.Rental{
		ID:              uuid.NewString(),
		ListingID:       listingID,
		TenantAccountID: tenantID,
		StartsAt:        start,
		EndsAt:          end,
		State:           rental.StatePending,
		IntentKey:       "int-" + uuid.NewString(),
		RentCents:       10000,
		DepositCents:    20000,
		CreatedAt:       now,
		UpdatedAt:       now,
		ListingSnapshot: rental.ListingSnapshot{
			OwnerID:          "test-owner",
			Title:            "Furadeira",
			Category:         "electric",
			PriceUnit:        "hour",
			PriceAmountCents: 5000,
			DepositCents:     20000,
			MinLeadTimeHours: 12,
		},
	}
	persisted, err := repo.CreateIntent(ctx, r, nil)
	require.NoError(t, err)
	require.Equal(t, r.ID, persisted.ID)

	loaded, err := repo.GetByID(ctx, r.ID)
	require.NoError(t, err)
	require.Equal(t, r.State, loaded.State)

	// Cleanup best-effort.
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM rentals WHERE id = ?", r.ID).Error
	})
}
