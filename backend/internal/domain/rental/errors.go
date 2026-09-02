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
)
