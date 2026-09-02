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
// the rentals.state column (see migration 000004).
//
// Lifecycle:
//
//	pending → authorized → confirmed      (happy path: tenant paid, owner accepted)
//	pending → authorized → declined       (owner refused)
//	pending → authorized → expired        (12h window passed without response)
//	pending → cancelled                   (tenant cancelled pre-authorization; F4 may extend)
//	authorized/confirmed → refunded       (post-capture refund; F4 owns the policy)
type State string

// Rental lifecycle states. StatePending is the initial state right after
// CreateIntent persists a reservation; StateAuthorized follows a successful
// payment authorization; StateConfirmed is the post-acceptance terminal
// state of the happy path; StateDeclined/StateExpired cover the
// owner-refused and 12h-window-timeout branches; StateCancelled and
// StateRefunded cover the post-authorization negative paths (F4 may extend
// the policy for partial refunds).
const (
	StatePending    State = "pending"
	StateAuthorized State = "authorized"
	StateConfirmed  State = "confirmed"
	StateDeclined   State = "declined"
	StateExpired    State = "expired"
	StateCancelled  State = "cancelled"
	StateRefunded   State = "refunded"
)

// OccupiesCalendar reports whether a rental in this state should block the
// listing's calendar (and therefore prevent new rentals from starting in
// the same window). EC-1 / R1 mitigation: the DB EXCLUDE constraint is on
// exactly these states.
func (s State) OccupiesCalendar() bool {
	return s == StateAuthorized || s == StateConfirmed
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
func CanTransition(from, to State) bool {
	switch from {
	case StatePending:
		return to == StateAuthorized || to == StateCancelled
	case StateAuthorized:
		return to == StateConfirmed || to == StateDeclined || to == StateExpired || to == StateRefunded
	case StateConfirmed:
		return to == StateRefunded
	}
	return false
}
