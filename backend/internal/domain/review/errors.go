// Package review owns the F6 avaliação domain. It contains the Review
// and ReviewAggregate types, sentinel errors consumed by the HTTP
// adapter, and a small pure-function library (no IO, no DB). The
// review.Service lives in package review (this same package) and is
// consumed by the F6 HTTP adapter (internal/handler/review.go).
//
// F6 only registers reviews from participants of a F5-terminal rental.
// The service is the gate that enforces the "sem locação paga, sem
// review" invariant (Pilar 1 do DoD) — it derives eligibility from
// the rental state plus a TerminalCheck callable (production: SQL
// query against devolucoes / avaria_pedidos; tests: canned result).
package review

import "errors"

// Sentinel errors consumed by the HTTP adapter; each maps 1:1 to an i18n
// key in the `review.*` namespace.
var (
	// ErrInvalidInput covers malformed input that the service rejects
	// without consulting state: score out of 1..5, empty rental_id,
	// missing rater_user_id, comment over the size limit.
	ErrInvalidInput = errors.New("review invalid input")

	// ErrNotFound is the rental id does not exist.
	ErrNotFound = errors.New("review rental not found")

	// ErrNotParticipant is the rater is neither the tenant nor the
	// listing owner recorded on the rental snapshot.
	ErrNotParticipant = errors.New("review not a rental participant")

	// ErrRentalNotTerminal is the rental is not in F5-terminal state
	// yet (no closed devolução, no resolved damage). Pilar 1: the
	// "sem locação paga, sem review" invariant — derived from state,
	// not from a flag.
	ErrRentalNotTerminal = errors.New("review rental not terminal")

	// ErrAlreadyReviewed is the rater already submitted a review for
	// this (rental, scope) pair. UNIQUE(rental_id, rater_user_id,
	// scope) backstops the check.
	ErrAlreadyReviewed = errors.New("review already exists")

	// ErrSelfReview is the rater == ratee. The tenant cannot review
	// themselves; the owner cannot review themselves; the operator
	// (when distinct from owner) cannot review themselves.
	ErrSelfReview = errors.New("review self-review forbidden")

	// ErrScopeInvalid is the requested scope is not one of {listing,
	// owner, operator, renter}. The handler is responsible for
	// parsing scope from the URL; the service is the gate.
	ErrScopeInvalid = errors.New("review scope invalid")
)
