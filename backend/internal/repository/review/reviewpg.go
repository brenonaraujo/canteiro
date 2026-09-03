// Package reviewpg is the Postgres-backed repository for the F6
// avaliação domain. It is consumed by review.Service; fakes in
// tests live next to the service.
//
// Single source of truth for review_aggregates is the SQL trigger
// reviews_after_insert_aggregate_sync (migration 000008). The
// application code does NOT upsert the aggregate on the write path:
// any such upsert would race the trigger and double-count
// (RCA: 2026-09-03 QA reprovou). The repo inserts the review and
// reads back the trigger-maintained aggregate in the same tx.
package reviewpg

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/review"
)

// Repo is the Postgres-backed review repository.
type Repo struct {
	DB  *gorm.DB
	Now func() time.Time // optional; nil → real clock (testable via WithClock).
}

// New returns the review repository with a real clock.
func New(db *gorm.DB) *Repo {
	return &Repo{DB: db, Now: func() time.Time { return time.Now().UTC() }}
}

// WithClock returns a copy of the receiver whose clock is replaced.
// The repo is otherwise unchanged — useful in integration tests so
// the CreatedAt fallback can be pinned without touching production
// code (see InsertReviewWithAggregate).
func (r *Repo) WithClock(now func() time.Time) *Repo {
	cp := *r
	if now != nil {
		cp.Now = now
	}
	return &cp
}

// reviewRow mirrors the reviews table (migration 000008).
type reviewRow struct {
	CreatedAt time.Time `gorm:"column:created_at"`
	ID        string    `gorm:"column:id;primaryKey"`
	RentalID  string    `gorm:"column:rental_id"`
	RaterID   string    `gorm:"column:rater_user_id"`
	RateeID   *string   `gorm:"column:ratee_user_id"`
	Scope     string    `gorm:"column:scope"`
	Score     int       `gorm:"column:score"`
	Comment   string    `gorm:"column:comment"`
}

func (reviewRow) TableName() string { return "reviews" }

// aggregateRow mirrors the review_aggregates table.
type aggregateRow struct {
	UpdatedAt time.Time `gorm:"column:updated_at"`
	RateeID   string    `gorm:"column:ratee_user_id;primaryKey"`
	Scope     string    `gorm:"column:scope;primaryKey"`
	Count     int64     `gorm:"column:count"`
	Sum       int64     `gorm:"column:sum"`
	Avg       float64   `gorm:"column:avg"`
}

func (aggregateRow) TableName() string { return "review_aggregates" }

// isUniqueViolation reports whether err is a Postgres UNIQUE
// constraint violation. We only care about the reviews UNIQUE — the
// aggregates PK conflict is normal (ON CONFLICT handles it).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// GORM wraps the underlying pgx error in gorm.ErrDuplicatedKey.
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "reviews_unique_per_rater_scope") ||
		strings.Contains(msg, "duplicate key value violates unique constraint")
}

// InsertReviewWithAggregate implements review.Repository. The review
// is inserted inside a single transaction; the SQL trigger
// reviews_after_insert_aggregate_sync (migration 000008) is the
// single source of truth for the (ratee, scope) aggregate, so this
// function only inserts the review and reads the post-trigger
// aggregate back. It deliberately does NOT upsert the aggregate
// itself — any such write races the trigger (AFTER INSERT, same tx)
// and double-counts. The QA round of 2026-09-03 caught exactly that
// bug. See issue #8, BLOQ 1.
//
// in.NewAggregate is preserved on the signature for caller-side
// backward compatibility; its fields are ignored — the returned
// ReviewAggregate is the trigger's authoritative value.
func (r *Repo) InsertReviewWithAggregate(ctx context.Context, in review.ReviewWithAggregateInput) (review.Review, review.ReviewAggregate, error) {
	if in.Review.RateeUserID == "" && in.Review.Scope != review.ScopeListing {
		return review.Review{}, review.ReviewAggregate{}, errors.New("reviewpg: ratee_user_id required for non-listing scope")
	}
	var out review.Review
	var newAgg review.ReviewAggregate
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		persisted, err := insertReviewRow(tx, in.Review, r.Now)
		if err != nil {
			return err
		}
		out = persisted
		agg, err := readAggregate(tx, in.Review.RateeUserID, in.Review.Scope)
		if err != nil {
			return err
		}
		newAgg = agg
		return nil
	})
	if err != nil {
		return review.Review{}, review.ReviewAggregate{}, err
	}
	return out, newAgg, nil
}

// insertReviewRow persists the review row and returns the value
// stamped with the DB-side created_at. When GORM does not hydrate
// created_at from the DB-side default (older GORM versions,
// pre-INSERT RETURNING), the now clock is used as a deterministic
// fallback. nil now is allowed and degrades to time.Now().
func insertReviewRow(tx *gorm.DB, in review.Review, now func() time.Time) (review.Review, error) {
	row := reviewRow{
		ID: in.ID, RentalID: in.RentalID, RaterID: in.RaterUserID,
		Scope: string(in.Scope), Score: in.Score, Comment: in.Comment,
	}
	if in.RateeUserID != "" {
		id := in.RateeUserID
		row.RateeID = &id
	}
	if err := tx.Create(&row).Error; err != nil {
		if isUniqueViolation(err) {
			return review.Review{}, review.ErrAlreadyReviewed
		}
		return review.Review{}, err
	}
	if row.CreatedAt.IsZero() {
		if now != nil {
			row.CreatedAt = now()
		} else {
			row.CreatedAt = time.Now().UTC()
		}
	}
	return review.Review{
		ID: row.ID, RentalID: row.RentalID, RaterUserID: row.RaterID,
		RateeUserID: deref(row.RateeID), Scope: review.Scope(row.Scope),
		Score: row.Score, Comment: row.Comment, CreatedAt: row.CreatedAt,
	}, nil
}

// readAggregate fetches the (ratee, scope) aggregate as left by the
// reviews_after_insert_aggregate_sync trigger. Listing scope has no
// user aggregate; we return a zero aggregate so the API surface stays
// uniform. Reads always observe the post-trigger value because
// AFTER INSERT fires before the surrounding transaction commits.
func readAggregate(tx *gorm.DB, rateeUserID string, scope review.Scope) (review.ReviewAggregate, error) {
	if scope == review.ScopeListing || rateeUserID == "" {
		return review.ReviewAggregate{RateeUserID: rateeUserID, Scope: scope}, nil
	}
	var row aggregateRow
	txErr := tx.Where("ratee_user_id = ? AND scope = ?", rateeUserID, string(scope)).
		Take(&row).Error
	if errors.Is(txErr, gorm.ErrRecordNotFound) {
		// Trigger runs ON EVERY ROW (incl. listing→ no-op). For a
		// user scope this branch is unreachable in the create
		// path, but we keep a graceful miss in case future writers
		// delete the row out from under us.
		return review.ReviewAggregate{RateeUserID: rateeUserID, Scope: scope}, nil
	}
	if txErr != nil {
		return review.ReviewAggregate{}, txErr
	}
	return review.NewAggregate(row.RateeID, review.Scope(row.Scope), row.Count, row.Sum), nil
}

// ListByRatee implements review.Repository. limit/offset are enforced
// by the SQL LIMIT/OFFSET clauses; the service is responsible for
// clamping them.
func (r *Repo) ListByRatee(ctx context.Context, rateeUserID string, scope review.Scope, limit, offset int) ([]review.Review, error) {
	if _, err := uuid.Parse(rateeUserID); err != nil {
		return nil, errors.New("reviewpg: ratee_user_id must be uuid")
	}
	q := r.DB.WithContext(ctx).Where("ratee_user_id = ?", rateeUserID)
	if scope != "" {
		q = q.Where("scope = ?", string(scope))
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	var rows []reviewRow
	if err := q.Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]review.Review, 0, len(rows))
	for _, row := range rows {
		out = append(out, review.Review{
			ID:          row.ID,
			RentalID:    row.RentalID,
			RaterUserID: row.RaterID,
			RateeUserID: deref(row.RateeID),
			Scope:       review.Scope(row.Scope),
			Score:       row.Score,
			Comment:     row.Comment,
			CreatedAt:   row.CreatedAt,
		})
	}
	return out, nil
}

// GetAggregate implements review.Repository.
func (r *Repo) GetAggregate(ctx context.Context, rateeUserID string, scope review.Scope) (review.ReviewAggregate, error) {
	if _, err := uuid.Parse(rateeUserID); err != nil {
		return review.ReviewAggregate{}, errors.New("reviewpg: ratee_user_id must be uuid")
	}
	var row aggregateRow
	err := r.DB.WithContext(ctx).
		Where("ratee_user_id = ? AND scope = ?", rateeUserID, string(scope)).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return review.ReviewAggregate{RateeUserID: rateeUserID, Scope: scope}, nil
	}
	if err != nil {
		return review.ReviewAggregate{}, err
	}
	return review.NewAggregate(row.RateeID, review.Scope(row.Scope), row.Count, row.Sum), nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
