// Package handler: F6 review endpoints (Create, List, Aggregate).
// Implemented as a separate API type so the F5 handler (F5API) and
// the F3 rental handler (RentalAPI) stay untouched. F6 wiring
// happens via apiMux in internal/app/server.go.
package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/brenonaraujo/canteiro/backend/internal/api"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/review"
	"github.com/brenonaraujo/canteiro/backend/internal/i18n"
)

// ReviewService is the slice of review.Service the handlers need.
// Production wires the full *review.Service; tests can substitute a
// fake without bringing in the Postgres repository.
type ReviewService interface {
	CreateReview(ctx context.Context, in review.CreateReviewInput) (review.Review, error)
	ListReceivedReviews(ctx context.Context, in review.ListReceivedReviewsInput) ([]review.Review, error)
	GetAggregate(ctx context.Context, rateeUserID string, scope review.Scope) (review.ReviewAggregate, error)
}

// ReviewAPI bundles the F6 HTTP endpoints.
type ReviewAPI struct {
	svc     ReviewService
	current CurrentAccountFn
}

// NewReviewAPI builds the F6 adapter. current is the session lookup;
// nil falls back to noSession (useful in tests).
func NewReviewAPI(svc ReviewService, current CurrentAccountFn) *ReviewAPI {
	if current == nil {
		current = noSession
	}
	return &ReviewAPI{svc: svc, current: current}
}

// --- helpers ------------------------------------------------------------

func (h *ReviewAPI) requireSession(c *gin.Context) (string, bool) {
	id, ok := h.current(c)
	if !ok {
		h.writeErr(c, http.StatusUnauthorized, "unauthorized", "review.unauthorized")
		return "", false
	}
	return id, true
}

func (h *ReviewAPI) writeErr(c *gin.Context, status int, code, key string) {
	c.JSON(status, api.Error{
		Code:       code,
		Message:    i18n.T(c.Request.Context(), key),
		MessageKey: key,
	})
}

// writeServiceErr translates the F6 sentinel errors to HTTP responses.
// Returns true when the error was handled (caller should return).
func (h *ReviewAPI) writeServiceErr(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, review.ErrInvalidInput):
		h.writeErr(c, http.StatusUnprocessableEntity, "invalid_input", "review.invalid_input")
	case errors.Is(err, review.ErrScopeInvalid):
		h.writeErr(c, http.StatusUnprocessableEntity, "scope_invalid", "review.scope_invalid")
	case errors.Is(err, review.ErrNotFound):
		h.writeErr(c, http.StatusNotFound, "not_found", "review.not_found")
	case errors.Is(err, review.ErrNotParticipant):
		h.writeErr(c, http.StatusForbidden, "not_participant", "review.not_participant")
	case errors.Is(err, review.ErrRentalNotTerminal):
		h.writeErr(c, http.StatusUnprocessableEntity, "rental_not_terminal", "review.rental_not_terminal")
	case errors.Is(err, review.ErrAlreadyReviewed):
		h.writeErr(c, http.StatusConflict, "already_reviewed", "review.already_reviewed")
	case errors.Is(err, review.ErrSelfReview):
		h.writeErr(c, http.StatusUnprocessableEntity, "self_review_forbidden", "review.self_review_forbidden")
	default:
		h.writeErr(c, http.StatusInternalServerError, "internal_error", "error.internal")
	}
	return true
}

// --- conversions --------------------------------------------------------

func reviewToAPI(r review.Review) api.Review {
	out := api.Review{
		Id:          toUUID(r.ID),
		RentalId:    toUUID(r.RentalID),
		RaterUserId: toUUID(r.RaterUserID),
		Scope:       api.ReviewScope(r.Scope),
		Score:       r.Score,
		CreatedAt:   r.CreatedAt,
	}
	if r.RateeUserID != "" {
		v := toUUID(r.RateeUserID)
		out.RateeUserId = &v
	}
	if r.Comment != "" {
		s := r.Comment
		out.Comment = &s
	}
	return out
}

func aggregateToAPI(a review.ReviewAggregate) api.ReviewAggregate {
	return api.ReviewAggregate{
		RateeUserId: toUUID(a.RateeUserID),
		Scope:       api.ReviewScope(a.Scope),
		Count:       a.Count,
		Sum:         a.Sum,
		Avg:         float32(a.Avg),
	}
}

func parseScopeParam(s string) (review.Scope, bool) {
	sc := review.Scope(s)
	if !sc.IsValid() {
		return "", false
	}
	return sc, true
}

// --- handlers -----------------------------------------------------------

// CreateRentalReview implements api.CreateRentalReview.
func (h *ReviewAPI) CreateRentalReview(c *gin.Context, id openapi_types.UUID) {
	actorID, ok := h.requireSession(c)
	if !ok {
		return
	}
	var req api.CreateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeErr(c, http.StatusBadRequest, "invalid_request", "review.invalid_input")
		return
	}
	in := review.CreateReviewInput{
		RentalID:    id.String(),
		RaterUserID: actorID,
		Scope:       review.Scope(req.Scope),
		Score:       req.Score,
	}
	if req.Comment != nil {
		in.Comment = *req.Comment
	}
	got, err := h.svc.CreateReview(c.Request.Context(), in)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, reviewToAPI(got))
}

// ListUserReviews implements api.ListUserReviews.
func (h *ReviewAPI) ListUserReviews(c *gin.Context, id openapi_types.UUID, params api.ListUserReviewsParams) {
	if _, err := uuid.Parse(id.String()); err != nil {
		h.writeErr(c, http.StatusBadRequest, "invalid_id", "review.invalid_input")
		return
	}
	in := review.ListReceivedReviewsInput{
		RateeUserID: id.String(),
	}
	if params.Scope != nil {
		sc, ok := parseScopeParam(string(*params.Scope))
		if !ok {
			h.writeErr(c, http.StatusUnprocessableEntity, "scope_invalid", "review.scope_invalid")
			return
		}
		in.Scope = sc
	}
	if params.Limit != nil {
		in.Limit = *params.Limit
	}
	if params.Offset != nil {
		in.Offset = *params.Offset
	}
	out, err := h.svc.ListReceivedReviews(c.Request.Context(), in)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	items := make([]api.Review, 0, len(out))
	for _, r := range out {
		items = append(items, reviewToAPI(r))
	}
	c.JSON(http.StatusOK, api.ReviewPage{Reviews: items})
}

// GetUserReviewAggregate implements api.GetUserReviewAggregate.
func (h *ReviewAPI) GetUserReviewAggregate(c *gin.Context, id openapi_types.UUID, params api.GetUserReviewAggregateParams) {
	if _, err := uuid.Parse(id.String()); err != nil {
		h.writeErr(c, http.StatusBadRequest, "invalid_id", "review.invalid_input")
		return
	}
	sc, ok := parseScopeParam(string(params.Scope))
	if !ok {
		h.writeErr(c, http.StatusUnprocessableEntity, "scope_invalid", "review.scope_invalid")
		return
	}
	agg, err := h.svc.GetAggregate(c.Request.Context(), id.String(), sc)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusOK, aggregateToAPI(agg))
}
