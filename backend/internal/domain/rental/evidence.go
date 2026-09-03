// F5 domain helpers shared by the return / damage / debt types.
package rental

import "encoding/json"

// marshalEvidence is the canonical encoder for EvidencePayload, used by the
// repository layer. Centralizing it here keeps JSONB round-trips consistent
// (see MarshalSnapshot for the F3 analog).
func marshalEvidence(p EvidencePayload) ([]byte, error) {
	return json.Marshal(p)
}

// unmarshalEvidence is the canonical decoder.
func unmarshalEvidence(b []byte) (EvidencePayload, error) {
	var p EvidencePayload
	if err := json.Unmarshal(b, &p); err != nil {
		return EvidencePayload{}, err
	}
	return p, nil
}
