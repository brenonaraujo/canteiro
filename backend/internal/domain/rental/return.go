// F5 domain: pure types for the devolução (return) flow. No IO, no DB.
package rental

import "time"

// Return is the F5 lifecycle row attached to a single rental. Exactly one
// Return exists per rental (UNIQUE on rental_id is enforced in the schema).
// The ReturnState reflects the post-confirmation portion of the rental
// (pickup registered → in progress → return registered → closed/contested).
type Return struct {
	ReturnedAt *time.Time `json:"returned_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	ID       string      `json:"id"`
	RentalID string      `json:"rental_id"`
	State    ReturnState `json:"state"`

	// Pickup evidence — the parties' registration of the asset state at
	// pickup. Stored as JSONB; the service layer is responsible for the
	// canonical encoding (see MarshalReturnEvidence).
	PickupEvidence []byte `json:"pickup_evidence,omitempty"`
	// ReturnEvidence — the parties' registration of the asset state at
	// return. Empty until the renter / owner registers the return.
	ReturnEvidence []byte `json:"return_evidence,omitempty"`

	// DepositReleasedCents is the amount of the deposit that was released
	// back to the renter when the return closed (happy path: full deposit;
	// damage path: deposit - captured damage).
	DepositReleasedCents int64 `json:"deposit_released_cents"`
	// DepositCapturedCents is the amount of the deposit that was captured
	// against a damage claim. Always <= rental.DepositCents.
	DepositCapturedCents int64 `json:"deposit_captured_cents"`
}

// EvidencePayload is the structured payload the parties send when they
// register the pickup or return state. Photos are stored as opaque refs
// (object storage keys); the API layer is responsible for uploading them
// before invoking the F5 service. Description and checklist are kept here
// for audit and redaction in logs (see LGPD: never log in cleartext).
type EvidencePayload struct {
	Photos      []string `json:"photos"`
	Description string   `json:"description"`
	Checklist   []string `json:"checklist"`
}

// MarshalReturnEvidence encodes the payload for JSONB storage.
func MarshalReturnEvidence(p EvidencePayload) ([]byte, error) {
	return marshalEvidence(p)
}

// UnmarshalReturnEvidence decodes a JSONB column.
func UnmarshalReturnEvidence(b []byte) (EvidencePayload, error) {
	if len(b) == 0 {
		return EvidencePayload{}, nil
	}
	return unmarshalEvidence(b)
}
