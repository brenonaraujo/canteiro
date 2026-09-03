// F5 domain: pure types for the avaria (damage) flow. No IO, no DB.
package rental

import "time"

// DamageClaim is the F5 row for an avaria. Created by the owner within 48h
// of the return; defended by the renter within 48h of opening; resolved by
// agreement or by staff mediation. The cap (deposit / declared value) is
// enforced at the service layer; the row itself only records the agreed
// values.
type DamageClaim struct {
	DecidedAt *time.Time `json:"decided_at,omitempty"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	OpenedAt  time.Time  `json:"opened_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	RespondedAt *time.Time `json:"responded_at,omitempty"`

	ID       string      `json:"id"`
	RentalID string      `json:"rental_id"`
	State    DamageState `json:"state"`
	Nature   DamageNature `json:"nature"`

	// OwnerID is denormalized from the rental's listing snapshot so the
	// authz check does not need to load the rental.
	OwnerID string `json:"owner_id"`
	// RenterID is the renter's account id, denormalized for the same reason.
	RenterID string `json:"renter_id"`

	// Description is the owner's text describing the damage. PII — never
	// logged in cleartext (LGPD).
	Description string `json:"description"`
	// Evidence is the JSONB payload of photo refs + checklist.
	Evidence []byte `json:"evidence,omitempty"`

	// ProposedCents is the owner's initial proposed value in cents.
	ProposedCents int64 `json:"proposed_cents"`
	// AgreedCents is the final value both parties (or the staff) agreed on.
	// Set on transition to DamageRenterAgreed or DamageStaffResolved.
	AgreedCents int64 `json:"agreed_cents"`
	// RenterResponseKind is "agree" / "contest" / "counter" — recorded
	// for audit (Pilar 4).
	RenterResponseKind string `json:"renter_response_kind,omitempty"`
	// RenterResponseNote is the renter's optional note on their response.
	RenterResponseNote string `json:"renter_response_note,omitempty"`
	// StaffDecisionNote is set on DamageStaffResolved (Pilar 4: staff
	// decisions are final in v1).
	StaffDecisionNote string `json:"staff_decision_note,omitempty"`
}

// Debt is the F5 row for an avaria divida ativa (active debt). One per
// avaria that exceeds the deposit, or one per failed auto-charge attempt.
// The lifecycle is Open → Settled or Open → Forgiven.
type Debt struct {
	SettledAt *time.Time `json:"settled_at,omitempty"`
	ForgivenAt *time.Time `json:"forgiven_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DueAt      time.Time  `json:"due_at"`

	ID       string    `json:"id"`
	RentalID string    `json:"rental_id"`
	DamageID string    `json:"damage_id"`
	RenterID string    `json:"renter_id"`
	State    DebtState `json:"state"`

	// OriginalCents is the initial amount when the debt was created.
	// ForgivenCents is the amount that was forgiven by staff (can be
	// partial). SettledCents is the amount that was paid by the renter.
	// Invariant: OriginalCents == ForgivenCents + SettledCents + Remaining
	// (where Remaining is original - forgiven - settled). For v1 we keep
	// only the gross values; the live remaining is derived.
	OriginalCents int64 `json:"original_cents"`
	ForgivenCents int64 `json:"forgiven_cents"`
	SettledCents  int64 `json:"settled_cents"`

	// ForgivenReason is mandatory when ForgivenCents > 0 (Pilar 5).
	ForgivenReason string `json:"forgiven_reason,omitempty"`
}
