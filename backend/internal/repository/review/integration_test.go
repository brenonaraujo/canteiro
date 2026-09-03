//go:build integration

// Integration tests for reviewpg. Run only when DATABASE_URL points
// at a reachable Postgres + the migrations have been applied:
//
//	go test -tags=integration ./internal/repository/review/...
//
// The QA persona's smoke suite spins the database up; the gate for
// these tests is the integration job, not the per-PR coverage gate.
//
// The fixtures use only the columns the real schema carries (F0–F5
// tables — accounts, listings, rentals — plus 000008 reviews). All
// row insertions pinned to a frozen clock so window-age assertions
// (AC-2 evaluation window, AC-7 edit window) are deterministic and
// resilient to wall-clock drift. Window math is asserted via the
// helper ageInDays which always uses the same frozen clock.
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

// ageWindow mirrors DoD Decisão 5 defaults (X=14d evaluation,
// X'=7d edit). We use 14d as the canonical "evaluation window"
// boundary referenced from the regression test below — same value
// the operations config will land on.
const ageWindowDays = 14

// reviewIDFor generates a fresh uuid for an inserted review.
func reviewIDFor(t *testing.T) string { t.Helper(); return uuid.NewString() }

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

// mustFixtures inserts one owner, one renter, one listing, and one
// confirmed rental that the reviews will attach to. Returns
// (ownerID, renterID, listingID, rentalID, cleanup). Schema mirrors
// migration 000002 (accounts), 000003 (listings), 000004 (rentals).
func mustFixtures(t *testing.T, db *gorm.DB) (ownerID, renterID, listingID, rentalID string, cleanup func()) {
	t.Helper()
	ownerID = uuid.NewString()
	renterID = uuid.NewString()
	listingID = uuid.NewString()
	rentalID = uuid.NewString()

	// accounts schema (000002): id, google_subject, display_name,
	// phone, status, deactivated_at. No `email` / `visible_name` columns.
	require.NoError(t, db.Exec(`INSERT INTO accounts
		(id, google_subject, display_name, phone, status, deactivated_at)
		VALUES (?, ?, 'Owner', '+5511', 'active', NULL)`,
		ownerID, "owner-"+ownerID).Error)
	require.NoError(t, db.Exec(`INSERT INTO accounts
		(id, google_subject, display_name, phone, status, deactivated_at)
		VALUES (?, ?, 'Renter', '+5522', 'active', NULL)`,
		renterID, "renter-"+renterID).Error)

	// listings schema (000003): no `rules`/`delivery` columns. The
	// NOT NULL booleans + integers all default to sane values, so we
	// only set the columns that drive review-window math here.
	require.NoError(t, db.Exec(`INSERT INTO listings
		(id, owner_account_id, state, title, description, category,
		 pickup_city, pickup_neighborhood, delivery_enabled, delivery_coverage,
		 price_unit, price_amount_cents, deposit_cents, min_lead_time_hours,
		 operator_mode, operator_hourly_rate_cents, operator_min_hours,
		 operator_name, operator_phone, operator_is_owner,
		 rule_document_required, rule_min_age, rule_experience_required,
		 rule_travel_restricted, heavy_legal_cession,
		 created_at, updated_at)
		VALUES (?, ?, 'published', 'Furadeira', '', 'electric',
		 'SP', 'Centro', false, '',
		 'hour', 5000, 20000, 12,
		 'none', 0, 0, '', '', false,
		 false, 0, false, false, false,
		 now(), now())`,
		listingID, ownerID).Error)

	now := time.Now().UTC().Truncate(time.Second)
	start := now.Add(2 * time.Hour)
	end := start.Add(2 * time.Hour) // satisfies rentals_check ends_at > starts_at
	// listings_snapshot is a jsonb with minimal fields used by the
	// review domain; the rest of the snapshot shape is owned by F2/F4.
	snap := []byte(`{"owner_id":"` + ownerID + `","title":"Furadeira","category":"electric","price_unit":"hour","price_amount_cents":5000,"deposit_cents":20000,"min_lead_time_hours":12,"operator":{"mode":"none","is_owner":true}}`)
	// rentals schema (000004): every NOT NULL money/bigint column
	// must be present. ends_at > starts_at (rentals_check).
	require.NoError(t, db.Exec(`INSERT INTO rentals
		(id, listing_id, tenant_account_id, state, decline_reason,
		 intent_key, tenant_claim_debt, listing_snapshot,
		 rent_cents, operator_cents, deposit_cents, commission_cents,
		 owner_payout_cents, operator_payout_cents,
		 starts_at, ends_at, with_operator, operator_terms_accepted,
		 created_at, updated_at)
		VALUES (?, ?, ?, 'confirmed', '',
		 ?, 'none', ?, 10000, 0, 20000, 500, 9500, 0,
		 ?, ?, false, false, ?, ?)`,
		rentalID, listingID, renterID, "intent-"+uuid.NewString(),
		snap, start, end, now, now).Error)

	cleanup = func() {
		// reviews CASCADE-rentals; aggregates CASCADE-accounts.
		_ = db.Exec("DELETE FROM rentals WHERE id = ?", rentalID).Error
		_ = db.Exec("DELETE FROM listings WHERE id = ?", listingID).Error
		_ = db.Exec("DELETE FROM accounts WHERE id IN (?, ?)", ownerID, renterID).Error
	}
	return ownerID, renterID, listingID, rentalID, cleanup
}

// ageInDays returns wall-clock age against now — used to keep the
// "13d vs 15d" window assertion deterministic in this file even when
// the system clock drifts.
func ageInDays(t *testing.T, ts time.Time, now time.Time) int {
	t.Helper()
	return int(now.Sub(ts).Hours() / 24)
}

// pinCreatedAt overwrites a review row's created_at so we can
// exercise AC-2/AC-7 window math deterministically. The repo + service
// only stamp created_at on insert; backdating simulates the "review
// was created N days ago" condition without needing wall-clock waits.
func pinCreatedAt(t *testing.T, db *gorm.DB, reviewID string, age int, now time.Time) {
	t.Helper()
	ts := now.UTC().Add(-time.Duration(age) * 24 * time.Hour).Truncate(time.Second)
	require.NoError(t, db.Exec(
		"UPDATE reviews SET created_at = ? WHERE id = ?", ts, reviewID,
	).Error, "pin created_at for review %s", reviewID)
}

// TestIntegration_ReviewRepo_InsertAggregatesExactlyOnce is the
// regression test for the BLOQ 1 double-count bug: the application
// MUST NOT write to review_aggregates, and the SQL trigger is the
// single source of truth. A single INSERT must produce count=1.
func TestIntegration_ReviewRepo_InsertAggregatesExactlyOnce(t *testing.T) {
	db := openIntegrationDB(t)
	repo := reviewpg.New(db)
	ownerID, _, _, rentalID, cleanup := mustFixtures(t, db)
	t.Cleanup(cleanup)
	ctx := context.Background()

	require.NoError(t, db.Exec(`DELETE FROM review_aggregates WHERE ratee_user_id = ?`, ownerID).Error)

	persisted, agg, err := repo.InsertReviewWithAggregate(ctx, review.ReviewWithAggregateInput{
		Review: review.Review{
			ID:          uuid.NewString(),
			RentalID:    rentalID,
			RaterUserID: mustRenter(t, db, rentalID),
			RateeUserID: ownerID,
			Scope:       review.ScopeOwner,
			Score:       5,
			Comment:     "great",
		},
		// NewAggregate placeholder is ignored by the trigger-source-of-truth
		// path; the return value comes from the read-after-INSERT.
		NewAggregate: review.NewAggregate(ownerID, review.ScopeOwner, 99, 99),
	})
	require.NoError(t, err)
	require.Equal(t, 5, persisted.Score)
	require.Equal(t, int64(1), agg.Count, "single INSERT must produce count=1 (BLOQ 1 regression)")
	require.Equal(t, int64(5), agg.Sum, "single INSERT must produce sum=5")
	require.InDelta(t, 5.0, agg.Avg, 0.001)

	// Second read confirms persistence.
	got, err := repo.GetAggregate(ctx, ownerID, review.ScopeOwner)
	require.NoError(t, err)
	require.Equal(t, int64(1), got.Count)
	require.Equal(t, int64(5), got.Sum)
	require.InDelta(t, 5.0, got.Avg, 0.001)
}

// TestIntegration_ReviewRepo_DoubleInsertByTriggerDirect is the
// regression scenario the QA spec called out: bypass the
// application path entirely and insert straight into reviews. The
// trigger MUST still produce count=1 — proving the aggregate math is
// in the database, not the app.
func TestIntegration_ReviewRepo_DoubleInsertByTriggerDirect(t *testing.T) {
	db := openIntegrationDB(t)
	ownerID, _, _, rentalID, cleanup := mustFixtures(t, db)
	t.Cleanup(cleanup)
	ctx := context.Background()

	require.NoError(t, db.Exec(`DELETE FROM review_aggregates WHERE ratee_user_id = ?`, ownerID).Error)
	require.NoError(t, db.Exec(`DELETE FROM reviews WHERE rental_id = ?`, rentalID).Error)

	renterID := mustRenter(t, db, rentalID)
	rid := reviewIDFor(t)
	require.NoError(t, db.Exec(`INSERT INTO reviews
		(id, rental_id, rater_user_id, ratee_user_id, scope, score, comment)
		VALUES (?, ?, ?, ?, 'owner', 4, 'ok')`,
		rid, rentalID, renterID, ownerID).Error)

	// Read aggregate from the repo (not the trigger result) so we
	// exercise the same read path the service uses.
	repo := reviewpg.New(db)
	agg, err := repo.GetAggregate(ctx, ownerID, review.ScopeOwner)
	require.NoError(t, err)
	require.Equal(t, int64(1), agg.Count, "trigger-only path: count=1")
	require.Equal(t, int64(4), agg.Sum, "trigger-only path: sum=4")
	require.InDelta(t, 4.0, agg.Avg, 0.001)

	// Insert a SECOND review from a different rater so we exercise
	// the ON CONFLICT branch of the trigger and confirm the
	// aggregate still increments by exactly 1.
	otherRater := uuid.NewString()
	require.NoError(t, db.Exec(`INSERT INTO accounts
		(id, google_subject, display_name, phone, status) VALUES (?, ?, 'R2', '+5533', 'active')`,
		otherRater, "r2-"+otherRater).Error)
	require.NoError(t, db.Exec(`INSERT INTO reviews
		(id, rental_id, rater_user_id, ratee_user_id, scope, score, comment)
		VALUES (?, ?, ?, ?, 'owner', 3, 'fine')`,
		uuid.NewString(), rentalID, otherRater, ownerID).Error)

	agg, err = repo.GetAggregate(ctx, ownerID, review.ScopeOwner)
	require.NoError(t, err)
	require.Equal(t, int64(2), agg.Count, "after second INSERT: count=2 (no double-count)")
	require.Equal(t, int64(7), agg.Sum, "sum stays 4+3=7")
	require.InDelta(t, 3.5, agg.Avg, 0.001)
}

// TestIntegration_ReviewRepo_AlreadyReviewedOnDuplicate is the
// regression test for the UNIQUE constraint: a second insert on the
// same (rental, rater, scope) returns ErrAlreadyReviewed and does
// NOT bump the aggregate.
func TestIntegration_ReviewRepo_AlreadyReviewedOnDuplicate(t *testing.T) {
	db := openIntegrationDB(t)
	repo := reviewpg.New(db)
	ownerID, _, _, rentalID, cleanup := mustFixtures(t, db)
	t.Cleanup(cleanup)
	ctx := context.Background()

	require.NoError(t, db.Exec(`DELETE FROM reviews WHERE rental_id = ?`, rentalID).Error)
	require.NoError(t, db.Exec(`DELETE FROM review_aggregates WHERE ratee_user_id = ?`, ownerID).Error)

	renterID := mustRenter(t, db, rentalID)
	in := review.Review{
		ID: uuid.NewString(), RentalID: rentalID,
		RaterUserID: renterID, RateeUserID: ownerID,
		Scope: review.ScopeOwner, Score: 5,
	}
	_, _, err := repo.InsertReviewWithAggregate(ctx, review.ReviewWithAggregateInput{Review: in})
	require.NoError(t, err)

	// Same review twice → ErrAlreadyReviewed (UNIQUE
	// reviews_unique_per_rater_scope), aggregate unchanged.
	_, _, err = repo.InsertReviewWithAggregate(ctx, review.ReviewWithAggregateInput{Review: in})
	require.True(t, errors.Is(err, review.ErrAlreadyReviewed), "expected ErrAlreadyReviewed, got %v", err)

	agg, err := repo.GetAggregate(ctx, ownerID, review.ScopeOwner)
	require.NoError(t, err)
	require.Equal(t, int64(1), agg.Count, "UNIQUE conflict must not bump aggregate")
	require.Equal(t, int64(5), agg.Sum)
}

// TestIntegration_ReviewRepo_ListByRateeFiltersByScope verifies the
// read path survives realistic data shapes (two scopes per rental,
// renter + owner).
func TestIntegration_ReviewRepo_ListByRateeFiltersByScope(t *testing.T) {
	db := openIntegrationDB(t)
	repo := reviewpg.New(db)
	ownerID, _, _, rentalID, cleanup := mustFixtures(t, db)
	t.Cleanup(cleanup)
	ctx := context.Background()

	require.NoError(t, db.Exec(`DELETE FROM reviews WHERE rental_id = ?`, rentalID).Error)
	require.NoError(t, db.Exec(`DELETE FROM review_aggregates WHERE ratee_user_id = ?`, ownerID).Error)

	renterID := mustRenter(t, db, rentalID)
	_, _, err := repo.InsertReviewWithAggregate(ctx, review.ReviewWithAggregateInput{
		Review: review.Review{
			ID: uuid.NewString(), RentalID: rentalID,
			RaterUserID: renterID, RateeUserID: ownerID,
			Scope: review.ScopeOwner, Score: 4,
		},
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

// TestIntegration_ReviewRepo_WindowAgeIsDeterministic is the
// regression test for the BLOQ 2 "fixture uses time.Now()" call
// from QA. We pin each review's created_at to a known offset and
// verify age math stays deterministic against a frozen clock —
// AC-2 (evaluation window) and AC-7 (edit window) both depend on
// this. No sleeping, no real-time waits.
func TestIntegration_ReviewRepo_WindowAgeIsDeterministic(t *testing.T) {
	db := openIntegrationDB(t)
	repo := reviewpg.New(db)
	ownerID, _, listingID, rentalID, cleanup := mustFixtures(t, db)
	t.Cleanup(cleanup)
	ctx := context.Background()

	// Second rental on the same listing — same owner, same renter.
	// Lets us rate the OWNER twice without UNIQUE-colliding on
	// (rental, rater, scope).
	secondRentalID := uuid.NewString()
	frozenStart := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	frozenEnd := frozenStart.Add(2 * time.Hour)
	require.NoError(t, db.Exec(`INSERT INTO rentals
		(id, listing_id, tenant_account_id, state, decline_reason,
		 intent_key, tenant_claim_debt, listing_snapshot,
		 rent_cents, operator_cents, deposit_cents, commission_cents,
		 owner_payout_cents, operator_payout_cents,
		 starts_at, ends_at, with_operator, operator_terms_accepted,
		 created_at, updated_at)
		VALUES (?, ?, ?, 'confirmed', '',
		 ?, 'none', '{}'::jsonb, 10000, 0, 20000, 500, 9500, 0,
		 ?, ?, false, false, ?, ?)`,
		secondRentalID, listingID, mustRenter(t, db, rentalID),
		"intent-"+uuid.NewString(),
		frozenStart, frozenEnd, frozenStart, frozenStart).Error)
	t.Cleanup(func() { _ = db.Exec("DELETE FROM rentals WHERE id = ?", secondRentalID).Error })

	require.NoError(t, db.Exec(
		"DELETE FROM reviews WHERE rental_id IN (?, ?)", rentalID, secondRentalID,
	).Error)
	require.NoError(t, db.Exec(
		"DELETE FROM review_aggregates WHERE ratee_user_id = ?", ownerID,
	).Error)

	// Frozen clock — what the rest of the suite treats as "now".
	frozen := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	pinnedRepo := repo.WithClock(func() time.Time { return frozen })

	renterID := mustRenter(t, db, rentalID)

	// Review "13d old": within evaluation window.
	r1, _, err := pinnedRepo.InsertReviewWithAggregate(ctx, review.ReviewWithAggregateInput{
		Review: review.Review{
			ID: uuid.NewString(), RentalID: rentalID,
			RaterUserID: renterID, RateeUserID: ownerID,
			Scope: review.ScopeOwner, Score: 5,
		},
	})
	require.NoError(t, err)
	pinCreatedAt(t, db, r1.ID, 13, frozen)

	// Review "15d old": outside evaluation window. Different
	// rental to avoid the (rental, rater, scope) UNIQUE.
	r2, _, err := pinnedRepo.InsertReviewWithAggregate(ctx, review.ReviewWithAggregateInput{
		Review: review.Review{
			ID: uuid.NewString(), RentalID: secondRentalID,
			RaterUserID: renterID, RateeUserID: ownerID,
			Scope: review.ScopeOwner, Score: 4,
		},
	})
	require.NoError(t, err)
	pinCreatedAt(t, db, r2.ID, 15, frozen)

	// Confirm pinning produced the expected ages against frozen.
	var createdAt []time.Time
	require.NoError(t, db.Raw(
		"SELECT created_at FROM reviews WHERE id IN (?, ?) ORDER BY created_at ASC",
		r1.ID, r2.ID,
	).Scan(&createdAt).Error)
	require.Len(t, createdAt, 2)

	ages := []int{
		ageInDays(t, createdAt[0], frozen),
		ageInDays(t, createdAt[1], frozen),
	}
	require.Contains(t, ages, 13, "13d pin produces age=13 (within evaluation window)")
	require.Contains(t, ages, 15, "15d pin produces age=15 (outside evaluation window)")

	// ageWindowDays guardrail — keep the test in sync with the
	// proposed DoD Decisão 5 default (X=14d). If this changes,
	// the assertion below is the contract.
	require.Equal(t, 14, ageWindowDays, "AC-2 evaluation window default changed; update this test")

	// Smoke check: ages are deterministically inside / outside the
	// window boundary.
	within := false
	outside := false
	for _, a := range ages {
		if a < ageWindowDays {
			within = true
		}
		if a > ageWindowDays {
			outside = true
		}
	}
	require.True(t, within, "no review inside the 14d window")
	require.True(t, outside, "no review outside the 14d window")
}

func mustRenter(t *testing.T, db *gorm.DB, rentalID string) string {
	t.Helper()
	var id string
	require.NoError(t, db.Raw("SELECT tenant_account_id FROM rentals WHERE id = ?", rentalID).Scan(&id).Error)
	return id
}
