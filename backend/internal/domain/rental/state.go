// Package rental owns the F3 reservation lifecycle: the state machine, the
// commercial snapshot, pricing/split math (deposit OUTSIDE the commissionable
// base, commission 12% only over rent + operator hours), and idempotency
// primitives.
//
// This file holds pure types + state-machine helpers. No IO, no DB. The
// rental.Service (in package rental at internal/rental) wires these to the
// repository and to the listing + payment collaborators.
package rental

// State is the discrete lifecycle of a rental. Each transition is enforced
// in service.CanTransition (this file) and in the SQL CHECK constraint on
// the rentals.state column (see migration 000004 / 000006).
//
// Lifecycle:
//
//	pending → authorized → confirmed      (happy path: tenant paid, owner accepted)
//	pending → authorized → declined       (owner refused)
//	pending → authorized → expired        (12h window passed without response)
//	pending → cancelled                   (tenant cancelled pre-authorization; F3)
//	authorized/confirmed → cancellation_in_progress → cancelled (F4 cancellation flow)
//	authorized/confirmed → refunded       (post-capture refund; F4 owns the policy)
type State string

// Rental lifecycle states. StatePending is the initial state right after
// CreateIntent persists a reservation; StateAuthorized follows a successful
// payment authorization; StateConfirmed is the post-acceptance terminal
// state of the happy path; StateDeclined/StateExpired cover the
// owner-refused and 12h-window-timeout branches; StateCancellationInProgress
// is F4's serialisable intermediate that locks the rental against a
// concurrent capture of the deposit (R2/EC-6 anti-double-penalty);
// StateCancelled and StateRefunded cover the post-authorization negative paths.
const (
	StatePending                State = "pending"
	StateAuthorized             State = "authorized"
	StateConfirmed              State = "confirmed"
	StateDeclined               State = "declined"
	StateExpired                State = "expired"
	StateCancellationInProgress State = "cancellation_in_progress"
	StateCancelled              State = "cancelled"
	StateRefunded               State = "refunded"
)

// OccupiesCalendar reports whether a rental in this state should block the
// listing's calendar (and therefore prevent new rentals from starting in
// the same window). EC-1 / R1 mitigation: the DB EXCLUDE constraint is on
// exactly these states.
//
// F4 nuance: a rental in cancellation_in_progress still occupies the
// calendar (the cancellation hasn't settled yet; another tenant cannot
// reserve the same interval). Once it moves to cancelled, the calendar
// frees up (EC-7).
func (s State) OccupiesCalendar() bool {
	return s == StateAuthorized || s == StateConfirmed || s == StateCancellationInProgress
}

// Terminal reports whether the state is terminal (no further transitions).
// Used to short-circuit idempotent re-applies of transitions.
func (s State) Terminal() bool {
	switch s {
	case StateConfirmed, StateDeclined, StateExpired, StateCancelled, StateRefunded:
		return true
	}
	return false
}

// CanTransition reports whether moving from `from` to `to` is valid.
// Any transition not listed here is rejected. The service layer wraps this
// in errors for the handler.
//
// F4 additions:
//   - authorized/confirmed → cancellation_in_progress (lock before persist)
//   - cancellation_in_progress → cancelled (commit)
//   - confirmed → refunded (PSP refund webhook)
func CanTransition(from, to State) bool {
	switch from {
	case StatePending:
		return to == StateAuthorized || to == StateCancelled
	case StateAuthorized:
		return to == StateConfirmed || to == StateDeclined || to == StateExpired ||
			to == StateCancellationInProgress || to == StateRefunded
	case StateConfirmed:
		return to == StateCancellationInProgress || to == StateRefunded
	case StateCancellationInProgress:
		return to == StateCancelled
	}
	return false
}
