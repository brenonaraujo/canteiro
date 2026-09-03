package review

import "fmt"

// ReviewAggregate is the materialized score+count for a single
// (ratee_user_id, scope) pair. Persisted in the review_aggregates
// table; updated in the same transaction as the reviews insert
// (Pilar 3 + Decisão 1 do DoD: agregados materializados por evento).
//
// The avg column is rounded to two decimal places at write time so
// the API surface never has to think about precision; the underlying
// sum is kept intact so the avg can be re-derived without drift.
//
// hide the entity from callers (matches the F5 DamageClaim pattern).
//
//nolint:revive // stutter: ReviewAggregate is the public domain name; renaming would
type ReviewAggregate struct {
	RateeUserID string  `json:"ratee_user_id"`
	Scope       Scope   `json:"scope"`
	Count       int64   `json:"count"`
	Sum         int64   `json:"sum"`
	Avg         float64 `json:"avg"`
}

// Zero reports whether the aggregate has any rows behind it. Empty
// aggregates are stored as Count=0, Sum=0, Avg=0 — the API layer
// must surface them as "no reviews yet" rather than as 0.0 stars.
func (a ReviewAggregate) Zero() bool { return a.Count == 0 }

// Compute returns the aggregate value implied by (count, sum). Round
// to 2 decimal places using banker-friendly half-up semantics (Go's
// math/big would be overkill here; 2 decimals is a display
// constraint, not a financial one).
func Compute(count, sum int64) (avg float64) {
	if count <= 0 {
		return 0
	}
	avg = float64(sum) / float64(count)
	// round to 2 decimals (half-up).
	return float64(int64(avg*100+0.5)) / 100
}

// NewAggregate builds the canonical aggregate struct after applying
// the delta. The repository writes the result; this helper exists so
// the service can compute it without a round-trip through the DB.
func NewAggregate(rateeUserID string, scope Scope, count, sum int64) ReviewAggregate {
	return ReviewAggregate{
		RateeUserID: rateeUserID,
		Scope:       scope,
		Count:       count,
		Sum:         sum,
		Avg:         Compute(count, sum),
	}
}

// AggregateKey is the canonical (ratee, scope) lookup key. The
// repository uses it for inserts and updates; the handler uses it
// for path params.
func AggregateKey(rateeUserID string, scope Scope) string {
	return fmt.Sprintf("%s|%s", rateeUserID, scope)
}
