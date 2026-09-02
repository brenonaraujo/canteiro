package listing

import "errors"

// Sentinel errors consumed by F2 handlers and asserted via errors.Is in
// services. Each error maps 1:1 to an i18n message key consumed by the
// HTTP adapter so that the user-facing copy stays in sync with the
// domain.
var (
	// ErrInvalidInput is a generic 422 for malformed domain input
	// (empty title, oversized description, negative cents, etc.).
	ErrInvalidInput = errors.New("listing invalid input")
	// ErrNotFound is an unknown listing id.
	ErrNotFound = errors.New("listing not found")
	// ErrForbidden is the caller's account is not the listing owner.
	ErrForbidden = errors.New("listing forbidden")
	// ErrAlreadyPublished is returned when PATCH is called on a published
	// listing; the owner must pause first.
	ErrAlreadyPublished = errors.New("listing already published")
	// ErrNotPublished is returned when pause is called on a non-published
	// listing.
	ErrNotPublished = errors.New("listing not published")
	// ErrPublishGates is the aggregate of failing publish checks. The
	// service attaches the list of missing keys to the message; the
	// handler maps this to a 422 with the same code.
	ErrPublishGates = errors.New("listing publish gates unsatisfied")
	// ErrDeactivated is the account is deactivated. Returns 403.
	ErrDeactivated = errors.New("account deactivated")
	// ErrProfileIncomplete is the account is active but lacks visible
	// name and phone. Returns 403.
	ErrProfileIncomplete = errors.New("profile incomplete")
	// ErrOwnerOnboardingRequired is payout details + owner terms are not
	// accepted yet. Returns 422 when client attempts publish, 200 when
	// persisted as draft (drafts are allowed without onboarding).
	ErrOwnerOnboardingRequired = errors.New("owner onboarding required")
	// ErrBlockOverlap is the new block intersects an existing block for
	// the same listing. Returns 409.
	ErrBlockOverlap = errors.New("block overlaps existing")
	// ErrBlockWindow is the block end is not after the start. Returns 422.
	ErrBlockWindow = errors.New("block end must be after start")
)

// PublishMissing is the structured list of publish-gate failures. The
// handler translates this into a 422 with a stable `missing` array so
// the UI can highlight each unsatisfied field.
type PublishMissing []string

// Missing returns the canonical list of unsatisfied gates.
func (p PublishMissing) Missing() []string {
	out := make([]string, len(p))
	copy(out, p)
	return out
}
