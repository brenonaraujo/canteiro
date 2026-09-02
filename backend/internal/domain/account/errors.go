package account

import "errors"

var (
	// ErrNotFound is an unknown account id or Google subject.
	ErrNotFound = errors.New("account not found")
	// ErrProfileIncomplete blocks reserve and publish until name and phone exist.
	ErrProfileIncomplete = errors.New("profile incomplete")
	// ErrDeactivated blocks new reserve and publish; in-progress rentals stay.
	ErrDeactivated = errors.New("account deactivated")
	// ErrOwnerOnboardingRequired is F2: payout details and owner terms.
	ErrOwnerOnboardingRequired = errors.New("owner onboarding required")
	// ErrInvalidProfile is empty or oversized name/phone.
	ErrInvalidProfile = errors.New("invalid profile")
	// ErrDuplicateGoogle is a unique-subject race; caller should reload.
	ErrDuplicateGoogle = errors.New("google subject already linked")
)
