package review

import (
	"fmt"
	"strings"
	"time"
)

// Scope is the discrete target of an evaluation. Each evaluation
// registers against exactly one scope; per the F6 SPEC §4.8:
//   - listing: rate of the asset (only the renter can rate the listing)
//   - owner:   rate of the listing owner (only the renter)
//   - operator: rate of the operator when distinct from the owner
//     (only the renter; EC-5 — operator=owner collapses into
//     the owner scope and the operator scope does not exist)
//   - renter:  rate of the renter (only the owner)
//
// When the operator is the owner (OperatorIsOwner == true on the
// snapshot), the renter rates owner only; operator scope is invalid.
type Scope string

// The four valid scopes. The receiver above documents the semantics;
// these constants stay one-liners so revive's exported-const rule
// stays satisfied via the type's doc-comment.
const (
	ScopeListing  Scope = "listing"
	ScopeOwner    Scope = "owner"
	ScopeOperator Scope = "operator"
	ScopeRenter   Scope = "renter"
)

// IsValid reports whether s is one of the four known scopes.
func (s Scope) IsValid() bool {
	switch s {
	case ScopeListing, ScopeOwner, ScopeOperator, ScopeRenter:
		return true
	}
	return false
}

// ParticipantRoles is the set of participants in a single rental.
// Used to compute who is allowed to rate which scope (DoD Pilar 1).
type ParticipantRoles struct {
	RenterID   string
	OwnerID    string
	OperatorID string // empty when no operator or operator == owner
}

// AllowedRaterScopes returns the scopes the given user is allowed to
// rate on this rental.
//
//   - Renter rates: listing, owner, (operator when operator != owner).
//   - Owner rates:  renter.
//   - Operator (when distinct from owner): no scope in v1 — the
//     operator does not receive reviews from the owner (DoD does not
//     require that path; out of scope).
//
// When operatorID == ownerID (OperatorIsOwner case), the operator
// scope is collapsed into the owner scope and never surfaces.
func (p ParticipantRoles) AllowedRaterScopes(userID string) []Scope {
	if userID == "" {
		return nil
	}
	switch userID {
	case p.RenterID:
		scopes := []Scope{ScopeListing, ScopeOwner}
		if p.OperatorID != "" && p.OperatorID != p.OwnerID {
			scopes = append(scopes, ScopeOperator)
		}
		return scopes
	case p.OwnerID:
		return []Scope{ScopeRenter}
	}
	return nil
}

// RateeID returns the account id that receives the evaluation for the
// given scope, or "" if the scope does not apply (e.g. listing scope
// rates the listing, not a user).
//
// Mirrors the AggregateKeyFor contract: only the user-scoped scopes
// contribute to a user aggregate; the listing scope contributes to
// the listing aggregate (out of scope of this turn).
func (p ParticipantRoles) RateeID(s Scope) string {
	switch s {
	case ScopeOwner:
		return p.OwnerID
	case ScopeRenter:
		return p.RenterID
	case ScopeOperator:
		return p.OperatorID // empty when no operator
	}
	return ""
}

// MaxCommentBytes is the hard ceiling on the size of the free-text
// comment. The handler may be more lenient; the service is strict
// (DoD Pilar 2: defence in depth on PII). 4 KiB.
const MaxCommentBytes = 4096

// Review is the domain entity. Persisted in the reviews table; one row
// per (rental_id, rater_user_id, scope). The scope is part of the
// unique key so the renter can rate listing+owner+operator on the
// same rental without collision.
type Review struct {
	CreatedAt time.Time `json:"created_at"`

	ID          string `json:"id"`
	RentalID    string `json:"rental_id"`
	RaterUserID string `json:"rater_user_id"`
	RateeUserID string `json:"ratee_user_id,omitempty"` // empty for scope=listing
	Scope       Scope  `json:"scope"`
	Score       int    `json:"score"` // 1..5 inclusive
	Comment     string `json:"comment,omitempty"`
}

// Validate enforces the row-level invariants the service can check
// without consulting state. The repository enforces the rest (FK,
// UNIQUE).
func (r *Review) Validate() error {
	if r.RentalID == "" {
		return fmt.Errorf("%w: rental_id required", ErrInvalidInput)
	}
	if r.RaterUserID == "" {
		return fmt.Errorf("%w: rater_user_id required", ErrInvalidInput)
	}
	if !r.Scope.IsValid() {
		return fmt.Errorf("%w: scope=%s", ErrScopeInvalid, r.Scope)
	}
	if r.Score < 1 || r.Score > 5 {
		return fmt.Errorf("%w: score=%d", ErrInvalidInput, r.Score)
	}
	if len(r.Comment) > MaxCommentBytes {
		return fmt.Errorf("%w: comment exceeds %d bytes", ErrInvalidInput, MaxCommentBytes)
	}
	if r.Scope != ScopeListing && r.RateeUserID == "" {
		return fmt.Errorf("%w: ratee_user_id required for scope=%s", ErrInvalidInput, r.Scope)
	}
	if r.RaterUserID != "" && r.RateeUserID != "" && r.RaterUserID == r.RateeUserID {
		return fmt.Errorf("%w: rater==ratee", ErrSelfReview)
	}
	// Strip control characters from the comment (DoD: defence on
	// free-text input). Newlines (\n, \r) are allowed; everything
	// below 0x20 except tab is dropped.
	if hasControl(r.Comment) {
		r.Comment = stripControl(r.Comment)
	}
	return nil
}

func hasControl(s string) bool {
	for _, r := range s {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return true
		}
	}
	return false
}

func stripControl(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 0x20 || r == '\t' || r == '\n' || r == '\r' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
