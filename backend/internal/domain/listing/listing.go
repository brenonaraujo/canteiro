package listing

import (
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxTitle                = 120
	minTitle                = 4
	maxDescription          = 4000
	minDescription          = 12
	maxCity                 = 80
	maxNeighborhood         = 80
	maxCoverage             = 240
	maxReason               = 240
	maxOperatorName         = 80
	maxOperatorPhone        = 32
	maxPhotos               = 12
	maxHourlyRateCents      = 1_000_000_00
	maxPriceCents           = 1_000_000_00
	maxDepositCents         = 1_000_000_00
	defaultTermsVersion     = "v1"
	defaultMinOperatorHours = 4
	defaultMinLeadHours     = 12
)

// State is the listing lifecycle exposed in the API.
type State string

const (
	StateDraft     State = "draft"
	StatePublished State = "published"
	StatePaused    State = "paused"
)

// Size is the publication group: light (manual/electric/light_construction/
// agricultural) vs heavy (trator/guindaste and equivalents).
type Size string

const (
	SizeLight Size = "light"
	SizeHeavy Size = "heavy"
)

// Category is the publication category. The set is closed and configured
// in the database; we mirror it here as constants for compile-time checks.
type Category string

const (
	CategoryManual            Category = "manual"
	CategoryElectric          Category = "electric"
	CategoryLightConstruction Category = "light_construction"
	CategoryAgricultural      Category = "agricultural"
	CategoryHeavy             Category = "heavy"
)

// Valid reports whether the category is a known listing category.
func (c Category) Valid() bool {
	switch c {
	case CategoryManual, CategoryElectric, CategoryLightConstruction, CategoryAgricultural, CategoryHeavy:
		return true
	}
	return false
}

// Size returns the publication size for a category.
func (c Category) Size() Size {
	if c == CategoryHeavy {
		return SizeHeavy
	}
	return SizeLight
}

// PriceUnit is hour or day.
type PriceUnit string

const (
	PriceHour PriceUnit = "hour"
	PriceDay  PriceUnit = "day"
)

// Valid reports whether the price unit is hour or day.
func (p PriceUnit) Valid() bool {
	return p == PriceHour || p == PriceDay
}

// OperatorMode is none / optional / required per SPEC §4.9.
type OperatorMode string

const (
	OperatorNone     OperatorMode = "none"
	OperatorOptional OperatorMode = "optional"
	OperatorRequired OperatorMode = "required"
)

// Valid reports whether the mode value is in the closed set.
func (o OperatorMode) Valid() bool {
	switch o {
	case OperatorNone, OperatorOptional, OperatorRequired:
		return true
	}
	return false
}

// OperatorIdentity is the operator pointer declared on the listing.
// `IsOwner` distinguishes the "owner's own operator service" from a
// named third party.
type OperatorIdentity struct {
	Name     string
	Phone    string
	IsOwner  bool
}

// Rules is the owner-declared eligibility + behaviour rules.
type Rules struct {
	DocumentRequired   bool
	MinAge             int
	ExperienceRequired bool
	TravelRestricted   bool
}

// Delivery captures the optional delivery area declaration. A listing
// may declare delivery (`Enabled=true`) only when `Coverage` is a
// non-empty geographic descriptor; an empty coverage with `Enabled=true`
// is rejected at publish time (EC-8).
type Delivery struct {
	Enabled  bool
	Coverage string
}

// Block is an owner-defined availability block. F3 will introduce
// booking-derived blocks but the calendar invariant (no overlapping
// blocks for the same listing) lives in F2.
type Block struct {
	ID        string
	ListingID string
	StartsAt  time.Time
	EndsAt    time.Time
	Reason    string
	CreatedAt time.Time
}

// Listing is the F2 aggregate root. Photos live on a child slice for
// persistence but are part of the entity model.
type Listing struct {
	ID                  string
	OwnerAccountID      string
	State               State
	Title               string
	Description         string
	Category            Category
	PickupCity          string
	PickupNeighborhood  string
	Delivery            Delivery
	PriceUnit           PriceUnit
	PriceAmountCents    int64
	DepositCents        int64
	MinLeadTimeHours    int
	Photos              []string
	Rules               Rules
	Operator            Operator
	HeavyLegalCession   bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// Operator is the operator declaration (mode + optional service details).
type Operator struct {
	Mode             OperatorMode
	HourlyRateCents  int64
	MinHours         int
	Identity         OperatorIdentity
}

// OwnerOnboarding captures the owner-side pre-publish onboarding state.
// Both `PayoutSet` and `TermsAccepted` must be true before the listing
// can be published (AC-2).
type OwnerOnboarding struct {
	AccountID       string
	PayoutKind      string
	PayoutLast4     string
	TermsAcceptedAt time.Time
	TermsVersion    string
}

// PayoutSet reports whether the owner has stored valid payout details.
func (o OwnerOnboarding) PayoutSet() bool {
	return strings.TrimSpace(o.PayoutKind) != "" && strings.TrimSpace(o.PayoutLast4) != ""
}

// TermsAccepted reports whether the owner has accepted the current
// terms version.
func (o OwnerOnboarding) TermsAccepted(currentVersion string) bool {
	if o.TermsAcceptedAt.IsZero() {
		return false
	}
	return strings.TrimSpace(o.TermsVersion) == currentVersion
}

// CategoryConfig is the deposit-minimum config for a category, mirrored
// from the `listing_categories` table.
type CategoryConfig struct {
	Category         Category
	Size             Size
	DepositMinCents  int64
}

// Validate normalises (trim) and checks basic invariants. It does NOT
// evaluate publish gates (those are in service.go).
func (l Listing) Validate() error {
	l.Title = strings.TrimSpace(l.Title)
	l.Description = strings.TrimSpace(l.Description)
	l.PickupCity = strings.TrimSpace(l.PickupCity)
	l.PickupNeighborhood = strings.TrimSpace(l.PickupNeighborhood)
	l.Delivery.Coverage = strings.TrimSpace(l.Delivery.Coverage)
	if utf8.RuneCountInString(l.Title) < minTitle || utf8.RuneCountInString(l.Title) > maxTitle {
		return ErrInvalidInput
	}
	if utf8.RuneCountInString(l.Description) < minDescription || utf8.RuneCountInString(l.Description) > maxDescription {
		return ErrInvalidInput
	}
	if !l.Category.Valid() {
		return ErrInvalidInput
	}
	if utf8.RuneCountInString(l.PickupCity) < 1 || utf8.RuneCountInString(l.PickupCity) > maxCity {
		return ErrInvalidInput
	}
	if utf8.RuneCountInString(l.PickupNeighborhood) > maxNeighborhood {
		return ErrInvalidInput
	}
	if l.Delivery.Enabled && utf8.RuneCountInString(l.Delivery.Coverage) == 0 {
		return ErrInvalidInput
	}
	if !l.PriceUnit.Valid() || l.PriceAmountCents <= 0 || l.PriceAmountCents > maxPriceCents {
		return ErrInvalidInput
	}
	if l.DepositCents < 0 || l.DepositCents > maxDepositCents {
		return ErrInvalidInput
	}
	if l.MinLeadTimeHours < 0 {
		return ErrInvalidInput
	}
	if l.Operator.HourlyRateCents < 0 || l.Operator.HourlyRateCents > maxHourlyRateCents {
		return ErrInvalidInput
	}
	if l.Operator.MinHours < 0 {
		return ErrInvalidInput
	}
	if utf8.RuneCountInString(l.Operator.Identity.Name) > maxOperatorName {
		return ErrInvalidInput
	}
	if utf8.RuneCountInString(l.Operator.Identity.Phone) > maxOperatorPhone {
		return ErrInvalidInput
	}
	return nil
}

// IsHeavy reports whether the listing category is heavy (trator,
// guindaste, etc.) — the SPEC §4.2 requires extra publish gates.
func (l Listing) IsHeavy() bool {
	return l.Category.Size() == SizeHeavy
}
