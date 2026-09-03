// Package review: service layer for the F6 avaliação domain.
//
// The service is the gate that enforces the F6 SPEC §4.8 invariant
// "sem locação paga, sem review" (Pilar 1 do DoD). It derives
// eligibility from the rental state plus a TerminalCheck callable:
//
//   - the rental must exist (ErrNotFound)
//   - the rater must be a participant (ErrNotParticipant)
//   - the rater must be allowed to rate the requested scope on this
//     rental (ErrNotParticipant) — collapses the operator==owner
//     case (EC-5)
//   - the rental must be F5-terminal: devolução closed OR avaria
//     resolved (ErrRentalNotTerminal)
//   - the rater must not have already rated the (rental, scope) pair
//     (ErrAlreadyReviewed) — UNIQUE backstop is in the SQL schema
//   - the ratee is derived from the scope; the self-review check
//     happens as a final guard before insert
//
// On success the service delegates the row insert + aggregate
// upsert to the repository in a single transaction
// (Decisão 1 do DoD: agregados materializados por evento).
package review

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Clock returns the current UTC time. Injected for deterministic
// tests.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

// RentalLookup is the slice of rental.Service the review service
// needs. Implemented by rental.Service in production (we already
// expose Get); fakes in tests provide both the participant and the
// F5-terminal probe.
//
// The service does NOT import the rental package directly to keep
// the review domain self-contained — only the public surface the
// rental package already exposes is in use.
type RentalLookup interface {
	// GetParticipant returns the rental's participants (renter, owner,
	// optional operator) and a flag indicating whether the operator
	// role is filled by the owner (EC-5). Returns ErrNotFound when
	// the rental id is unknown.
	GetParticipant(ctx context.Context, rentalID string) (RentalParticipant, error)
	// IsTerminal reports whether the rental has reached F5-terminal
	// state: devolução closed OR avaria resolved (any of
	// renter_agreed / staff_resolved). Returns ErrNotFound when the
	// rental id is unknown.
	IsTerminal(ctx context.Context, rentalID string) (bool, error)
}

// RentalParticipant mirrors the slice of rental.Rental the service
// needs. Owned by this package to avoid an import cycle on the
// rental package (the review domain is meant to be standalone — see
// service.go header).
type RentalParticipant struct {
	RentalID        string
	RenterID        string
	OwnerID         string
	OperatorID      string // empty when no operator or operator == owner
	OperatorIsOwner bool
}

// Repository is the persistence contract. Implemented by the
// reviewpg subpackage in production; fakes in tests. The insert and
// the aggregate upsert happen in the same SQL transaction; the
// service treats them as one operation.
type Repository interface {
	// InsertReviewWithAggregate inserts the review and updates (or
	// creates) the (ratee, scope) aggregate atomically. Returns
	// ErrAlreadyReviewed on UNIQUE conflict. Returns the persisted
	// review (with created_at stamped by the repo) and the new
	// aggregate state after the insert.
	InsertReviewWithAggregate(ctx context.Context, in ReviewWithAggregateInput) (Review, ReviewAggregate, error)
	// ListByRatee returns reviews addressed to the given user,
	// optionally filtered by scope. limit/offset are enforced
	// server-side. Scope=="" means any scope.
	ListByRatee(ctx context.Context, rateeUserID string, scope Scope, limit, offset int) ([]Review, error)
	// GetAggregate returns the materialized aggregate for (ratee,
	// scope), or the zero aggregate when none exists.
	GetAggregate(ctx context.Context, rateeUserID string, scope Scope) (ReviewAggregate, error)
}

// ReviewWithAggregateInput is the payload the service hands to the
// repository. The repository must apply the (rental_id,
// rater_user_id, scope) UNIQUE constraint and bump the (ratee,
// scope) aggregate counters in the same transaction.
//
//nolint:revive // stutter: matches the ReviewAggregate naming (see aggregate.go).
type ReviewWithAggregateInput struct {
	Review       Review
	NewAggregate ReviewAggregate
}

// Config groups the service dependencies and the optional knobs.
type Config struct {
	// Now is injectable for tests; zero → realClock.
	Now Clock
	// IDGen is injectable for tests; zero → defaultIDGen.
	IDGen IDGenerator
	// MaxCommentBytes overrides the package-level default when set.
	MaxCommentBytes int
}

// Service orchestrates the F6 evaluation flow.
type Service struct {
	rentals RentalLookup
	repo    Repository
	now     Clock
	idgen   IDGenerator
}

// NewService wires the review service. The caller is responsible for
// passing a rentals lookup that implements both GetParticipant and
// IsTerminal — production wires this to rental.Service + a small
// SQL probe; tests use fakes.
func NewService(cfg Config, rentals RentalLookup, repo Repository) *Service {
	if cfg.Now == nil {
		cfg.Now = realClock{}
	}
	if cfg.IDGen == nil {
		cfg.IDGen = defaultIDGen{}
	}
	return &Service{
		rentals: rentals,
		repo:    repo,
		now:     cfg.Now,
		idgen:   cfg.IDGen,
	}
}

// CreateReviewInput is the payload the handler hands to the service.
type CreateReviewInput struct {
	RentalID    string
	RaterUserID string
	Scope       Scope
	Score       int
	Comment     string
}

// ListReceivedReviewsInput is the payload for the read side.
type ListReceivedReviewsInput struct {
	RateeUserID string
	Scope       Scope
	Limit       int
	Offset      int
}

// limit clamps the read side. The handler enforces this defensively
// too; the service is the gate.
const (
	defaultListLimit = 20
	maxListLimit     = 100
)

// CreateReview applies the F6 invariants in order and persists the
// review + aggregate atomically.
//
// Order of checks (DoD Pilar 1 — "sem locação paga, sem review"):
//  1. Input shape (Validate).
//  2. Rental exists.
//  3. Rater is a participant AND the scope is one of the rater's
//     allowed scopes on this rental.
//  4. Rental is F5-terminal.
//  5. Self-review guard.
//  6. Ratee resolution from scope.
//  7. Repository insert (UNIQUE race + aggregate update).
//
// Reasons for the order: cheap shape checks first; existence checks
// before expensive aggregate writes; the ratee resolution needs the
// rental row, so it runs after step 2.
func (s *Service) CreateReview(ctx context.Context, in CreateReviewInput) (Review, error) {
	if err := validateCreateReviewInput(in); err != nil {
		return Review{}, err
	}
	parts, err := s.rentals.GetParticipant(ctx, in.RentalID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Review{}, ErrNotFound
		}
		return Review{}, err
	}
	if !raterScopeAllowed(in.RaterUserID, in.Scope, parts) {
		return Review{}, ErrNotParticipant
	}
	terminal, err := s.rentals.IsTerminal(ctx, in.RentalID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Review{}, ErrNotFound
		}
		return Review{}, err
	}
	if !terminal {
		return Review{}, ErrRentalNotTerminal
	}
	rateeID := resolveRatee(in.Scope, parts)
	if in.RaterUserID == rateeID && rateeID != "" {
		return Review{}, ErrSelfReview
	}
	review := buildReview(in, rateeID, s.idgen.String())
	return s.persist(ctx, review, rateeID)
}

// validateCreateReviewInput checks the cheap shape constraints
// before consulting state. Errors are sentinel and surface as
// 422 / scope_invalid in the handler.
func validateCreateReviewInput(in CreateReviewInput) error {
	if in.RentalID == "" || in.RaterUserID == "" {
		return fmt.Errorf("%w: rental_id and rater_user_id required", ErrInvalidInput)
	}
	if !in.Scope.IsValid() {
		return fmt.Errorf("%w: scope=%s", ErrScopeInvalid, in.Scope)
	}
	if in.Score < 1 || in.Score > 5 {
		return fmt.Errorf("%w: score=%d", ErrInvalidInput, in.Score)
	}
	if len(in.Comment) > MaxCommentBytes {
		return fmt.Errorf("%w: comment exceeds %d bytes", ErrInvalidInput, MaxCommentBytes)
	}
	return nil
}

// raterScopeAllowed reports whether userID may rate scope on the
// rental described by parts. EC-5 collapses operator=owner into
// owner; only the renter sees the operator scope, and only when
// the operator slot is filled by a non-owner.
func raterScopeAllowed(userID string, scope Scope, parts RentalParticipant) bool {
	if userID == "" {
		return false
	}
	allowed := ParticipantRoles{
		RenterID: parts.RenterID, OwnerID: parts.OwnerID,
	}.AllowedRaterScopes(userID)
	if userID == parts.RenterID && parts.OperatorID != "" && !parts.OperatorIsOwner {
		allowed = append(allowed, ScopeOperator)
	}
	for _, sc := range allowed {
		if sc == scope {
			return true
		}
	}
	return false
}

// resolveRatee picks the ratee account id for the given scope on
// the rental described by parts. Listing scope has no user ratee.
func resolveRatee(scope Scope, parts RentalParticipant) string {
	return ParticipantRoles{
		RenterID: parts.RenterID, OwnerID: parts.OwnerID,
		OperatorID: parts.OperatorID,
	}.RateeID(scope)
}

// buildReview builds the persistent Review entity. The id is the
// caller-supplied value (the idgen result) so the persistence call
// is idempotent on retries.
func buildReview(in CreateReviewInput, rateeID, id string) Review {
	return Review{
		ID:          id,
		RentalID:    in.RentalID,
		RaterUserID: in.RaterUserID,
		RateeUserID: rateeID,
		Scope:       in.Scope,
		Score:       in.Score,
		Comment:     stripControl(in.Comment),
	}
}

// persist hands the review + a placeholder aggregate shape to the
// repository. The repository merges with the existing aggregate in
// a single transaction (UNIQUE race + aggregate update).
func (s *Service) persist(ctx context.Context, review Review, rateeID string) (Review, error) {
	placeholder := NewAggregate(rateeID, review.Scope, 1, int64(review.Score))
	persisted, _, err := s.repo.InsertReviewWithAggregate(ctx, ReviewWithAggregateInput{
		Review: review, NewAggregate: placeholder,
	})
	if err != nil {
		return Review{}, err
	}
	return persisted, nil
}

// ListReceivedReviews returns reviews addressed to the given user,
// optionally filtered by scope, paginated.
func (s *Service) ListReceivedReviews(ctx context.Context, in ListReceivedReviewsInput) ([]Review, error) {
	if in.RateeUserID == "" {
		return nil, fmt.Errorf("%w: ratee_user_id required", ErrInvalidInput)
	}
	if in.Scope != "" && !in.Scope.IsValid() {
		return nil, fmt.Errorf("%w: scope=%s", ErrScopeInvalid, in.Scope)
	}
	if in.Limit <= 0 {
		in.Limit = defaultListLimit
	}
	if in.Limit > maxListLimit {
		in.Limit = maxListLimit
	}
	if in.Offset < 0 {
		in.Offset = 0
	}
	return s.repo.ListByRatee(ctx, in.RateeUserID, in.Scope, in.Limit, in.Offset)
}

// GetAggregate returns the materialized (ratee, scope) aggregate.
func (s *Service) GetAggregate(ctx context.Context, rateeUserID string, scope Scope) (ReviewAggregate, error) {
	if rateeUserID == "" {
		return ReviewAggregate{}, fmt.Errorf("%w: ratee_user_id required", ErrInvalidInput)
	}
	if !scope.IsValid() {
		return ReviewAggregate{}, fmt.Errorf("%w: scope=%s", ErrScopeInvalid, scope)
	}
	return s.repo.GetAggregate(ctx, rateeUserID, scope)
}
