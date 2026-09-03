// Package rental also owns the F5 sentinel errors. Each maps 1:1 to an
// i18n key. F3 errors keep their original names; F5 errors are namespaced
// with "F5" suffix to avoid collision.
package rental

import "errors"

// Sentinel errors consumed by the HTTP adapter; each maps 1:1 to an i18n key.
var (
	// ErrInvalidInput is a generic 422 (malformed window, negative cents, etc).
	ErrInvalidInput = errors.New("rental invalid input")
	// ErrNotFound is an unknown rental id.
	ErrNotFound = errors.New("rental not found")
	// ErrForbidden is the caller is neither the tenant nor the listing owner.
	ErrForbidden = errors.New("rental forbidden")
	// ErrListingUnavailable is the listing is paused/draft/deleted (EC-6).
	ErrListingUnavailable = errors.New("listing unavailable")
	// ErrCalendarOverlap is the window overlaps an existing authorized/confirmed
	// rental OR an owner-declared block (EC-1).
	ErrCalendarOverlap = errors.New("calendar overlap")
	// ErrOperatorTermsRequired is the listing requires operator and the tenant
	// did not accept terms (AC-5).
	ErrOperatorTermsRequired = errors.New("operator terms required")
	// ErrOperatorNotAvailable is the listing's mode is `none` but the tenant
	// asked for the operator.
	ErrOperatorNotAvailable = errors.New("operator not available")
	// ErrTenantHasDebt is the tenant carries an unpaid avaria from F5 (AC-12).
	// F5 owns the writes; F3 reads.
	ErrTenantHasDebt = errors.New("tenant has unpaid avaria")
	// ErrOpenDebt is the F5 Pilar 5 gate on CreateIntent: the renter has at
	// least one open (unpaid, unforgiven) avaria debt, so no new reservation
	// intent may be opened. F5 owns the debt lifecycle; F3 only reads the
	// aggregate answer via the DebtGate port.
	ErrOpenDebt = errors.New("renter has an open debt")
	// ErrInvalidTransition is the requested state change is not allowed.
	ErrInvalidTransition = errors.New("invalid state transition")
	// ErrAcceptanceExpired is the owner tried to accept after the 12h deadline (EC-5).
	ErrAcceptanceExpired = errors.New("acceptance window expired")
	// ErrPaymentTotalMismatch is EC-4 — the PSP-amount doesn't match the
	// server-computed total (manipulation local).
	ErrPaymentTotalMismatch = errors.New("payment total mismatch")
	// ErrIdempotencyConflict is the same idempotency key resolved to a
	// different intent (someone is replaying our keys). Hard error.
	ErrIdempotencyConflict = errors.New("idempotency conflict")
	// ErrAccountDeactivated is the tenant or owner account was deactivated.
	ErrAccountDeactivated = errors.New("account deactivated")
	// ErrProfileIncomplete is the tenant account lacks visible name + phone (F1 gate).
	ErrProfileIncomplete = errors.New("profile incomplete")
	// ErrOwnerOnboardingRequired is the listing owner hasn't set payout + terms.
	ErrOwnerOnboardingRequired = errors.New("owner onboarding required")
	// ErrReceiptAlreadyExists is returned when SaveReceipt is called twice for
	// the same rental (EC-2 idempotency surface).
	ErrReceiptAlreadyExists = errors.New("receipt already exists")

	// --- F5: devolução / avaria / dívida ---

	// ErrF5ReturnNotFound is the F5 return row does not exist for the rental.
	ErrF5ReturnNotFound = errors.New("F5 return not found")
	// ErrF5ReturnAlreadyExists is a return row already exists for the rental.
	ErrF5ReturnAlreadyExists = errors.New("F5 return already exists")
	// ErrF5ReturnInvalidState is the return state transition is not allowed.
	ErrF5ReturnInvalidState = errors.New("F5 return invalid state")
	// ErrF5RentalNotConfirmed is the rental is not in StateConfirmed (AC-1
	// requires a confirmed rental before pickup can be registered).
	ErrF5RentalNotConfirmed = errors.New("F5 rental not confirmed")
	// ErrF5PickupAlreadyRegistered is a pickup state has already been recorded.
	ErrF5PickupAlreadyRegistered = errors.New("F5 pickup already registered")
	// ErrF5ReturnWindowOpen is the rental is still inside the pickup window
	// (return cannot be registered before ends_at unless a grace period
	// is configured; default is to require ends_at to have passed).
	ErrF5ReturnWindowOpen = errors.New("F5 return window not yet open")
	// ErrF5ReturnAlreadyClosed is the return is already terminal.
	ErrF5ReturnAlreadyClosed = errors.New("F5 return already closed")

	// ErrF5DamageNotFound is the damage claim does not exist.
	ErrF5DamageNotFound = errors.New("F5 damage not found")
	// ErrF5DamageAlreadyExists is the owner already opened a claim for this rental.
	ErrF5DamageAlreadyExists = errors.New("F5 damage already exists")
	// ErrF5DamageWindowExpired is the 48h owner claim window (or 48h renter
	// defense window) has passed.
	ErrF5DamageWindowExpired = errors.New("F5 damage window expired")
	// ErrF5DamageInvalidNature is the nature is not one of the recognized values.
	ErrF5DamageInvalidNature = errors.New("F5 damage invalid nature")
	// ErrF5DamageAmountInvalid is the proposed amount is <= 0 or exceeds the cap.
	ErrF5DamageAmountInvalid = errors.New("F5 damage amount invalid")
	// ErrF5DamageEvidenceRequired is the open-claim request lacks photos / description.
	ErrF5DamageEvidenceRequired = errors.New("F5 damage evidence required")
	// ErrF5DamageInvalidState is the damage state transition is not allowed.
	ErrF5DamageInvalidState = errors.New("F5 damage invalid state")
	// ErrF5DamageAlreadyContested is the renter already responded with a contest.
	ErrF5DamageAlreadyContested = errors.New("F5 damage already contested")
	// ErrF5DamageAlreadyAgreed is the renter already agreed to the claim.
	ErrF5DamageAlreadyAgreed = errors.New("F5 damage already agreed")

	// ErrF5DebtNotFound is the debt does not exist.
	ErrF5DebtNotFound = errors.New("F5 debt not found")
	// ErrF5DebtAlreadyExists is a debt row already exists for the damage.
	ErrF5DebtAlreadyExists = errors.New("F5 debt already exists")
	// ErrF5DebtAlreadySettled is the debt was already settled or forgiven.
	ErrF5DebtAlreadySettled = errors.New("F5 debt already settled")
	// ErrF5DebtInvalidState is the debt state transition is not allowed.
	ErrF5DebtInvalidState = errors.New("F5 debt invalid state")
	// ErrF5DebtForgiveRequiresReason is staff must justify any forgiveness.
	ErrF5DebtForgiveRequiresReason = errors.New("F5 debt forgive requires reason")
	// ErrF5DebtAmountInvalid is the debt amount is <= 0 or overflows.
	ErrF5DebtAmountInvalid = errors.New("F5 debt amount invalid")
	// ErrF5DebtCapExceeded is the attempted cap is below the captured
	// deposit portion (cannot record a debt with negative residual).
	ErrF5DebtCapExceeded = errors.New("F5 debt cap exceeded")
)
