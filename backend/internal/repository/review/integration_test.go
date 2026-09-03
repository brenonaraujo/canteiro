//go:build integration

// Integration tests for reviewpg. Run only when DATABASE_URL points
// at a reachable Postgres + the migrations have been applied:
//
//	go test -tags=integration ./internal/repository/review/...
//
// The QA persona's smoke suite spins the database up; the gate for
// these tests is the integration job, not the per-PR coverage gate.
package reviewpg_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/review"
	reviewpg "github.com/brenonaraujo/canteiro/backend/internal/repository/review"
)

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

func mustFixtures(t *testing.T, db *gorm.DB) (ownerID, renterID, listingID, rentalID string, cleanup func()) {
	t.Helper()
	ownerID = uuid.NewString()
	renterID = uuid.NewString()
	listingID = uuid.NewString()
	rentalID = uuid.NewString()

	require.NoError(t, db.Exec(`INSERT INTO accounts (id, email, visible_name, phone, deactivated_at)
		VALUES (?, ?, 'Owner', '+5511', NULL)`,
		ownerID, ownerID+"@x").Error)
	require.NoError(t, db.Exec(`INSERT INTO accounts (id, email, visible_name, phone, deactivated_at)
		VALUES (?, ?, 'Renter', '+5522', NULL)`,
		renterID, renterID+"@x").Error)
	require.NoError(t, db.Exec(`INSERT INTO listings (id, owner_account_id, state, title, category, price_unit, price_amount_cents, deposit_cents, pickup_city, pickup_neighborhood, rules, delivery, created_at, updated_at)
		VALUES (?, ?, 'published', 'Furadeira', 'electric', 'hour', 5000, 20000, 'SP', 'Centro', '{}'::jsonb, '{}'::jsonb, now(), now())`,
		listingID, ownerID).Error)

	now := time.Now().UTC().Truncate(time.Second)
	start := now.Add(2 * time.Hour)
	end := start.Add(2 * time.Hour)
	snap := []byte(`{"owner_id":"` + ownerID + `","title":"Furadeira","category":"electric","price_unit":"hour","price_amount_cents":5000,"deposit_cents":20000,"min_lead_time_hours":12,"operator":{"mode":"none","is_owner":true}}`)
	require.NoError(t, db.Exec(`INSERT INTO rentals (id, listing_id, tenant_account_id, state, starts_at, ends_at, intent_key, listing_snapshot, rent_cents, deposit_cents, created_at, updated_at)
		VALUES (?, ?, ?, 'confirmed', ?, ?, ?, ?, 10000, 20000, ?, ?)`,
		rentalID, listingID, renterID, start, end, "intent-"+uuid.NewString(), snap, now, now).Error)

	cleanup = func() {
		_ = db.Exec("DELETE FROM rentals WHERE id = ?", rentalID).Error
		_ = db.Exec("DELETE FROM listings WHERE id = ?", listingID).Error
		_ = db.Exec("DELETE FROM accounts WHERE id IN (?, ?)", ownerID, renterID).Error
	}
	return ownerID, renterID, listingID, rentalID, cleanup
}

func TestIntegration_ReviewRepo_InsertAggregatesAndUniqueness(t *testing.T) {
	db := openIntegrationDB(t)
	repo := reviewpg.New(db)
	ownerID, _, _, rentalID, cleanup := mustFixtures(t, db)
	t.Cleanup(cleanup)
	ctx := context.Background()

	// Pre-flight: clean any aggregate row that might exist from a prior run.
	require.NoError(t, db.Exec(`DELETE FROM review_aggregates WHERE ratee_user_id = ? AND scope = 'owner'`, ownerID).Error)

	r1 := review.Review{
		ID: uuid.NewString(), RentalID: rentalID, RaterUserID: mustRenter(t, db, rentalID),
		RateeUserID: ownerID, Scope: review.ScopeOwner, Score: 5, Comment: "great",
	}
	persisted, agg, err := repo.InsertReviewWithAggregate(ctx, review.ReviewWithAggregateInput{
		Review: r1, NewAggregate: review.NewAggregate(ownerID, review.ScopeOwner, 1, 5),
	})
	require.NoError(t, err)
	require.Equal(t, 5, persisted.Score)
	require.Equal(t, int64(1), agg.Count)
	require.Equal(t, int64(5), agg.Sum)
	require.InDelta(t, 5.0, agg.Avg, 0.001)

	// Second insert from same rater: UNIQUE → ErrAlreadyReviewed.
	_, _, err = repo.InsertReviewWithAggregate(ctx, review.ReviewWithAggregateInput{
		Review:       r1,
		NewAggregate: review.NewAggregate(ownerID, review.ScopeOwner, 2, 9),
	})
	require.True(t, errors.Is(err, review.ErrAlreadyReviewed))

	// Aggregate was read back from the trigger-updated row.
	got, err := repo.GetAggregate(ctx, ownerID, review.ScopeOwner)
	require.NoError(t, err)
	require.Equal(t, int64(1), got.Count)
	require.Equal(t, int64(5), got.Sum)
	require.InDelta(t, 5.0, got.Avg, 0.001)
}

func TestIntegration_ReviewRepo_ListByRateeFiltersByScope(t *testing.T) {
	db := openIntegrationDB(t)
	repo := reviewpg.New(db)
	ownerID, _, _, rentalID, cleanup := mustFixtures(t, db)
	t.Cleanup(cleanup)
	ctx := context.Background()
	renterID := mustRenter(t, db, rentalID)

	require.NoError(t, db.Exec(`DELETE FROM reviews WHERE rental_id = ?`, rentalID).Error)
	require.NoError(t, db.Exec(`DELETE FROM review_aggregates WHERE ratee_user_id = ?`, ownerID).Error)

	_, _, err := repo.InsertReviewWithAggregate(ctx, review.ReviewWithAggregateInput{
		Review: review.Review{
			ID: uuid.NewString(), RentalID: rentalID, RaterUserID: renterID, RateeUserID: ownerID,
			Scope: review.ScopeOwner, Score: 4,
		},
		NewAggregate: review.NewAggregate(ownerID, review.ScopeOwner, 1, 4),
	})
	require.NoError(t, err)

	all, err := repo.ListByRatee(ctx, ownerID, "", 10, 0)
	require.NoError(t, err)
	require.Len(t, all, 1)

	ownerScoped, err := repo.ListByRatee(ctx, ownerID, review.ScopeOwner, 10, 0)
	require.NoError(t, err)
	require.Len(t, ownerScoped, 1)

	renterScoped, err := repo.ListByRatee(ctx, ownerID, review.ScopeRenter, 10, 0)
	require.NoError(t, err)
	require.Empty(t, renterScoped, "owner has no renter-scoped reviews")
}

func mustRenter(t *testing.T, db *gorm.DB, rentalID string) string {
	t.Helper()
	var id string
	require.NoError(t, db.Raw("SELECT tenant_account_id FROM rentals WHERE id = ?", rentalID).Scan(&id).Error)
	return id
}
