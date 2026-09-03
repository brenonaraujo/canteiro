package rental

import (
	"encoding/json"
	"fmt"
	"time"
)

// ListingSnapshot is the immutable commercial snapshot of a listing at the
// moment a rental is authorized. Mirrors the listing fields that affect
// pricing and rules; subsequent edits to the listing never propagate
// (AC-10).
//
// We persist it as JSONB in the DB so we can extend the shape without
// migrations for new fields. Marshalling/Unmarshalling here is the
// canonical encoding — the repository MUST round-trip through these
// helpers, not json.Marshal directly.
type ListingSnapshot struct {
	OwnerID           string           `json:"owner_id"`
	Title             string           `json:"title"`
	Category          string           `json:"category"`
	PriceUnit         string           `json:"price_unit"`
	PickupCity        string           `json:"pickup_city"`
	Operator          OperatorSnapshot `json:"operator"`
	PriceAmountCents  int64            `json:"price_amount_cents"`
	DepositCents      int64            `json:"deposit_cents"`
	MinLeadTimeHours  int              `json:"min_lead_time_hours"`
	HeavyLegalCession bool             `json:"heavy_legal_cession,omitempty"`
}

// OperatorSnapshot mirrors the F2 listing.Operator shape.
type OperatorSnapshot struct {
	Mode            string `json:"mode"`
	Name            string `json:"name,omitempty"`
	Phone           string `json:"phone,omitempty"`
	HourlyRateCents int64  `json:"hourly_rate_cents"`
	MinHours        int    `json:"min_hours"`
	IsOwner         bool   `json:"is_owner"`
}

// MoneyBreakdown is the canonical pricing output for a rental.
type MoneyBreakdown struct {
	RentCents           int64 `json:"rent_cents"`
	OperatorCents       int64 `json:"operator_cents"`
	DepositCents        int64 `json:"deposit_cents"`
	TotalCents          int64 `json:"total_cents"`
	CommissionBaseCents int64 `json:"commission_base_cents"`
	CommissionCents     int64 `json:"commission_cents"`
	OwnerPayoutCents    int64 `json:"owner_payout_cents"`
	OperatorPayoutCents int64 `json:"operator_payout_cents"`
}

// Rental is the domain entity.
type Rental struct {
	AcceptanceDeadlineAt *time.Time `json:"acceptance_deadline_at,omitempty"`
	ConfirmedAt          *time.Time `json:"confirmed_at,omitempty"`
	DeclinedAt           *time.Time `json:"declined_at,omitempty"`

	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	ID              string `json:"id"`
	ListingID       string `json:"listing_id"`
	TenantAccountID string `json:"tenant_account_id"`
	State           State  `json:"state"`
	DeclineReason   string `json:"decline_reason,omitempty"`
	IntentKey       string `json:"intent_key"`
	TenantClaimDebt string `json:"tenant_claim_debt"`

	ListingSnapshot ListingSnapshot `json:"listing_snapshot"`

	RentCents           int64 `json:"rent_cents"`
	OperatorCents       int64 `json:"operator_cents"`
	DepositCents        int64 `json:"deposit_cents"`
	CommissionCents     int64 `json:"commission_cents"`
	OwnerPayoutCents    int64 `json:"owner_payout_cents"`
	OperatorPayoutCents int64 `json:"operator_payout_cents"`

	WithOperator          bool `json:"with_operator"`
	OperatorTermsAccepted bool `json:"operator_terms_accepted"`
}

// HasOverlap reports whether [aStart, aEnd) intersects [bStart, bEnd).
func HasOverlap(aStart, aEnd, bStart, bEnd time.Time) bool {
	if !aEnd.After(aStart) || !bEnd.After(bStart) {
		return false
	}
	return aStart.Before(bEnd) && bStart.Before(aEnd)
}

// ValidateWindow enforces the basic invariants of a reservation window.
func ValidateWindow(startsAt, endsAt time.Time, now time.Time, minLead time.Duration) error {
	if startsAt.IsZero() || endsAt.IsZero() {
		return fmt.Errorf("%w: window timestamps required", ErrInvalidInput)
	}
	if !endsAt.After(startsAt) {
		return fmt.Errorf("%w: ends_at must be after starts_at", ErrInvalidInput)
	}
	if !now.IsZero() && startsAt.Before(now.Add(minLead)) {
		return fmt.Errorf("%w: starts_at violates lead time", ErrInvalidInput)
	}
	return nil
}

// Validate enforces row-level invariants.
func (r *Rental) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("%w: id required", ErrInvalidInput)
	}
	if r.ListingID == "" {
		return fmt.Errorf("%w: listing_id required", ErrInvalidInput)
	}
	if r.TenantAccountID == "" {
		return fmt.Errorf("%w: tenant_account_id required", ErrInvalidInput)
	}
	if r.RentCents < 0 || r.OperatorCents < 0 || r.DepositCents < 0 {
		return fmt.Errorf("%w: money must be non-negative", ErrInvalidInput)
	}
	if r.RentCents+r.OperatorCents+r.DepositCents == 0 {
		return fmt.Errorf("%w: total must be > 0", ErrInvalidInput)
	}
	if err := ValidateWindow(r.StartsAt, r.EndsAt, time.Time{}, 0); err != nil {
		return err
	}
	if !CanTransition(StatePending, r.State) && r.State != StatePending {
		return fmt.Errorf("%w: state %q is not pending", ErrInvalidInput, r.State)
	}
	if r.ListingSnapshot.Operator.Mode == "required" && !r.OperatorTermsAccepted {
		return fmt.Errorf("%w: operator terms required", ErrOperatorTermsRequired)
	}
	if r.WithOperator && r.ListingSnapshot.Operator.Mode == "none" {
		return fmt.Errorf("%w: operator not available", ErrOperatorNotAvailable)
	}
	return nil
}

// MarshalSnapshot encodes the snapshot for JSONB storage.
func MarshalSnapshot(s ListingSnapshot) ([]byte, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("%w: snapshot marshal: %v", ErrInvalidInput, err)
	}
	return b, nil
}

// UnmarshalSnapshot decodes the JSONB column.
func UnmarshalSnapshot(b []byte) (ListingSnapshot, error) {
	var s ListingSnapshot
	if len(b) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("%w: snapshot unmarshal: %v", ErrInvalidInput, err)
	}
	return s, nil
}

// IsOwner reports whether ownerID is the listing's owner at snapshot time.
func (r *Rental) IsOwner(ownerID string) bool {
	return r.ListingSnapshot.OwnerID != "" && r.ListingSnapshot.OwnerID == ownerID
}

// IsTenant reports whether tenantID is the tenant account on this rental.
func (r *Rental) IsTenant(tenantID string) bool {
	return r.TenantAccountID == tenantID
}
