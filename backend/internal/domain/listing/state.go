package listing

import "time"

// CanEdit reports whether the listing can be modified through the PATCH
// endpoint. A published listing must be paused first (AC-3).
func (l Listing) CanEdit() bool {
	return l.State == StateDraft || l.State == StatePaused
}

// CanPause reports whether the pause transition is valid. Pausing is
// only valid from `published`.
func (l Listing) CanPause() bool {
	return l.State == StatePublished
}

// CanPublishFrom reports whether the listing can move to `published` from
// the supplied state. The function does NOT evaluate content gates; it
// only checks the source state.
func (l Listing) CanPublishFrom(s State) bool {
	return s == StateDraft || s == StatePaused
}

// PublishGates are the content gates that must all pass for publication.
// Returns nil if every gate is satisfied, otherwise a `PublishMissing`
// slice listing the missing keys in stable order.
//
// Gates (derived from SPEC §4.2, §4.3, §4.9 and domain-expert ACs):
//
//   - account_active: caller is active (handled by service before call).
//   - profile_complete: caller has visible name and phone.
//   - payout_set: owner has stored payout details.
//   - terms_accepted: owner accepted current terms version.
//   - has_photo: at least one photo URL.
//   - title_set / description_set: already enforced by Validate().
//   - deposit_min: deposit >= category minimum.
//   - cession_declared: heavy category requires HeavyLegalCession=true.
//   - operator_identified: mode=required requires a non-empty
//     operator name (and phone when not is_owner).
//   - delivery_covered: enabled delivery requires non-empty coverage.
func (l Listing) PublishGates(cfg CategoryConfig) PublishMissing {
	var miss PublishMissing
	if len(l.Photos) == 0 {
		miss = append(miss, "has_photo")
	}
	if cfg.DepositMinCents > 0 && l.DepositCents < cfg.DepositMinCents {
		miss = append(miss, "deposit_min")
	}
	if l.IsHeavy() && !l.HeavyLegalCession {
		miss = append(miss, "cession_declared")
	}
	if l.Operator.Mode == OperatorRequired && strings_TrimIsEmpty(l.Operator.Identity.Name) {
		miss = append(miss, "operator_identified")
	}
	if l.Delivery.Enabled && strings_TrimIsEmpty(l.Delivery.Coverage) {
		miss = append(miss, "delivery_covered")
	}
	return miss
}

// HasOverlappingBlock reports whether the candidate [start,end) overlaps
// any block in the supplied slice. The caller is responsible for
// truncating the slice to the candidate's listing and ordering.
func HasOverlappingBlock(blocks []Block, start, end time.Time) bool {
	for _, b := range blocks {
		if end.Equal(b.StartsAt) || start.Equal(b.EndsAt) {
			continue
		}
		if start.Before(b.EndsAt) && b.StartsAt.Before(end) {
			return true
		}
	}
	return false
}

func strings_TrimIsEmpty(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
	}
	return true
}
