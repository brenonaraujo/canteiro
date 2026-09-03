// F5 lifecycle: the post-confirmation portion of a rental — pickup, return,
// damage claim, debt collection. Lives alongside the F3 state machine but is
// a separate concern: the SQL CHECK on rentals.state only knows about the
// F3 states; the F5 states are persisted in dedicated tables (devolucao,
// avaria_pedido, divida) and reference the rental by id.
//
// This file owns pure types + state machine helpers. No IO, no DB.
package rental

// ReturnState is the discrete lifecycle of the F5 return flow. A rental in
// rental.StateConfirmed can carry one of these for the *current* return
// attempt. The terminal values overlap with "rental closed" but a return is
// per-attempt; the rental itself can be re-confirmed in a future iteration
// of the platform.
type ReturnState string

const (
	// ReturnAwaitingPickup is the initial state: rental confirmed but neither
	// party has registered the pickup state yet. This is the only state in
	// which the F5 deposit capture can be authorized (legal: the deposit
	// was authorized at checkout; F5 only *captures* it when damage
	// occurs).
	ReturnAwaitingPickup ReturnState = "awaiting_pickup"
	// ReturnInProgress is set once at least one party has registered the
	// pickup state. The window is now active.
	ReturnInProgress ReturnState = "in_progress"
	// ReturnAwaitingConfirmation is set when the rental reached ends_at
	// but the return state has not been registered by both parties.
	ReturnAwaitingConfirmation ReturnState = "awaiting_confirmation"
	// ReturnClosed is the happy-path terminal state: deposit released (or
	// partially captured with no debt).
	ReturnClosed ReturnState = "closed"
	// ReturnContested means the renter opened a dispute that the staff
	// must mediate. Deposit stays held (not released, not captured) until
	// the staff decides.
	ReturnContested ReturnState = "contested"
)

// DamageNature is the type of damage claimed by the owner. Drives the cap:
// cosmetic/functional are capped at the deposit; loss is capped at the
// declared value of the asset.
type DamageNature string

const (
	// DamageCosmetic is superficial damage that does not affect function.
	DamageCosmetic DamageNature = "cosmetic"
	// DamageFunctional is damage where function is affected but the asset
	// is repairable.
	DamageFunctional DamageNature = "functional"
	// DamageLoss is the asset is unusable or missing (perda total).
	DamageLoss DamageNature = "loss"
)

// AllDamageNatures is the canonical, ordered set of allowed damage natures.
// Iteration order matches the cap priority (cosmetic < functional < loss).
var AllDamageNatures = []DamageNature{DamageCosmetic, DamageFunctional, DamageLoss}

// IsValid reports whether n is one of the recognized natures.
func (n DamageNature) IsValid() bool {
	switch n {
	case DamageCosmetic, DamageFunctional, DamageLoss:
		return true
	}
	return false
}

// DamageState is the discrete lifecycle of an avaria (damage claim).
type DamageState string

const (
	// DamageOpen is the initial state right after the owner opens the
	// claim. The 48h defense window for the renter has not yet elapsed
	// and the deposit has not been captured.
	DamageOpen DamageState = "open"
	// DamageRenterAgreed means the renter accepted the proposed value.
	// The deposit is captured (partially or fully) and any residual
	// becomes a debt (Pilar 3).
	DamageRenterAgreed DamageState = "renter_agreed"
	// DamageContested means the renter opened a defense. The deposit is
	// held; the staff must decide.
	DamageContested DamageState = "contested"
	// DamageStaffResolved means the staff resolved the dispute. The
	// deposit is captured per the decision; any residual becomes a debt.
	DamageStaffResolved DamageState = "staff_resolved"
	// DamageExpired means the 48h owner window passed without a claim
	// (release full deposit) or the 48h renter defense window passed
	// without a response (mediation, not silent agreement — D1).
	DamageExpired DamageState = "expired"
	// DamageCancelled is set when the owner withdraws the claim (e.g.
	// after staff mediation). The deposit is fully released.
	DamageCancelled DamageState = "cancelled"
)

// DebtState is the lifecycle of a divida ativa.
type DebtState string

const (
	// DebtOpen is the initial state: charge to the original PSP method
	// failed or the debt was created above the deposit cap.
	DebtOpen DebtState = "open"
	// DebtSettled means the renter paid the full amount.
	DebtSettled DebtState = "settled"
	// DebtForgiven means the staff forgave the debt (full or partial
	// recorded in the history). The remaining amount is zero.
	DebtForgiven DebtState = "forgiven"
)

// CanReturnTransition reports whether moving a return from one state to
// another is allowed. The matrix intentionally narrows the surface: only
// the operations exposed by the F5 service can move the return forward.
func CanReturnTransition(from, to ReturnState) bool {
	switch from {
	case ReturnAwaitingPickup:
		return to == ReturnInProgress
	case ReturnInProgress:
		return to == ReturnAwaitingConfirmation || to == ReturnClosed || to == ReturnContested
	case ReturnAwaitingConfirmation:
		return to == ReturnClosed || to == ReturnContested
	case ReturnContested:
		return to == ReturnClosed
	}
	return false
}

// CanDamageTransition reports whether moving a damage claim from one state
// to another is allowed. Note: DamageOpen → DamageRenterAgreed is a happy
// path; DamageOpen → DamageContested is the defense path; DamageContested
// → DamageStaffResolved is the staff decision.
func CanDamageTransition(from, to DamageState) bool {
	switch from {
	case DamageOpen:
		return to == DamageRenterAgreed || to == DamageContested || to == DamageExpired || to == DamageCancelled
	case DamageContested:
		return to == DamageStaffResolved || to == DamageRenterAgreed || to == DamageCancelled
	}
	return false
}

// CanDebtTransition reports whether a debt can move between states.
func CanDebtTransition(from, to DebtState) bool {
	if from == DebtOpen {
		return to == DebtSettled || to == DebtForgiven
	}
	return false
}
