// Package reviewpg is the Postgres-backed repository for the F6
// avaliação domain. It is consumed by review.Service; fakes in
// tests live next to the service.
//
// The review + aggregate writes happen in a single SQL transaction:
// INSERT INTO reviews (drives the UNIQUE constraint that surfaces
// ErrAlreadyReviewed) → INSERT … ON CONFLICT … UPDATE
// review_aggregates. The migration also installs a trigger that
// keeps the aggregate consistent — belt-and-braces for any future
// writer that forgets the upsert.
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
	DB *gorm.DB
}

// New returns the review repository.
func New(db *gorm.DB) *Repo { return &Repo{DB: db} }

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

// InsertReviewWithAggregate implements review.Repository. It runs the
// insert and the aggregate upsert inside a single transaction.
//
// The repo reads the current aggregate inside the tx (for the
// updated-at / count bookkeeping the API will surface), upserts the
// new values, and returns the persisted review + the new aggregate
// state. The trigger on reviews (migration 000008) is a redundant
// backstop — application writes are the primary path.
func (r *Repo) InsertReviewWithAggregate(ctx context.Context, in review.ReviewWithAggregateInput) (review.Review, review.ReviewAggregate, error) {
	if in.Review.RateeUserID == "" && in.Review.Scope != review.ScopeListing {
		return review.Review{}, review.ReviewAggregate{}, errors.New("reviewpg: ratee_user_id required for non-listing scope")
	}
	var out review.Review
	var newAgg review.ReviewAggregate
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		persisted, err := insertReviewRow(tx, in.Review)
		if err != nil {
			return err
		}
		out = persisted
		agg, err := upsertAggregate(tx, in.Review)
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
// stamped with the DB-side created_at.
func insertReviewRow(tx *gorm.DB, in review.Review) (review.Review, error) {
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
		row.CreatedAt = time.Now().UTC()
	}
	return review.Review{
		ID: row.ID, RentalID: row.RentalID, RaterUserID: row.RaterID,
		RateeUserID: deref(row.RateeID), Scope: review.Scope(row.Scope),
		Score: row.Score, Comment: row.Comment, CreatedAt: row.CreatedAt,
	}, nil
}

// upsertAggregate reads the current (ratee, scope) aggregate,
// merges the new score, and writes the result. Listing scope has
// no user aggregate; we return a zero aggregate to keep the API
// contract uniform.
func upsertAggregate(tx *gorm.DB, in review.Review) (review.ReviewAggregate, error) {
	if in.Scope == review.ScopeListing || in.RateeUserID == "" {
		return review.ReviewAggregate{Scope: review.ScopeListing}, nil
	}
	var prev aggregateRow
	txErr := tx.Where("ratee_user_id = ? AND scope = ?", in.RateeUserID, in.Scope).
		Take(&prev).Error
	if txErr != nil && !errors.Is(txErr, gorm.ErrRecordNotFound) {
		return review.ReviewAggregate{}, txErr
	}
	newCount := prev.Count + 1
	newSum := prev.Sum + int64(in.Score)
	upsert := aggregateRow{
		RateeID: in.RateeUserID, Scope: string(in.Scope),
		Count: newCount, Sum: newSum,
		Avg: review.Compute(newCount, newSum), UpdatedAt: time.Now().UTC(),
	}
	// GORM Save on a multi-column PK uses INSERT … ON CONFLICT —
	// covers fresh and existing aggregate rows in one statement.
	if err := tx.Save(&upsert).Error; err != nil {
		return review.ReviewAggregate{}, err
	}
	return review.NewAggregate(upsert.RateeID, review.Scope(upsert.Scope), upsert.Count, upsert.Sum), nil
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
