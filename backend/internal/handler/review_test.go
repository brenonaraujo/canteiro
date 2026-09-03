package handler_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/api"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/review"
	"github.com/brenonaraujo/canteiro/backend/internal/handler"
)

// --- minimal fakes for the review handler ------------------------------

type fakeReviewSvc struct {
	create       review.Review
	createErr    error
	listOut      []review.Review
	listErr      error
	aggregateOut review.ReviewAggregate
	aggregateErr error
}

func (f *fakeReviewSvc) CreateReview(_ context.Context, _ review.CreateReviewInput) (review.Review, error) {
	return f.create, f.createErr
}
func (f *fakeReviewSvc) ListReceivedReviews(_ context.Context, _ review.ListReceivedReviewsInput) ([]review.Review, error) {
	return f.listOut, f.listErr
}
func (f *fakeReviewSvc) GetAggregate(_ context.Context, _ string, _ review.Scope) (review.ReviewAggregate, error) {
	return f.aggregateOut, f.aggregateErr
}

// --- harness -----------------------------------------------------------

func newReviewHarness(t *testing.T, svc *fakeReviewSvc, session bool) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	current := func(*gin.Context) (string, bool) {
		if !session {
			return "", false
		}
		return "actor-1", true
	}
	h := handler.NewReviewAPI(svc, current)
	r := gin.New()
	r.POST("/rentals/:id/review", func(c *gin.Context) {
		h.CreateRentalReview(c, uuidFromParam(c, "id"))
	})
	r.GET("/users/:id/reviews", func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		require.NoError(t, err)
		h.ListUserReviews(c, id, api.ListUserReviewsParams{})
	})
	r.GET("/users/:id/review-aggregate", func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		require.NoError(t, err)
		scope := api.ReviewScope(c.Query("scope"))
		h.GetUserReviewAggregate(c, id, api.GetUserReviewAggregateParams{Scope: scope})
	})
	return r
}

// --- CreateRentalReview ------------------------------------------------

func TestReviewHandler_Create_HappyPath(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	svc := &fakeReviewSvc{create: review.Review{
		ID: uuid.NewString(), RentalID: uuid.NewString(),
		RaterUserID: "actor-1", RateeUserID: "owner-1",
		Scope: review.ScopeOwner, Score: 5, Comment: "great",
		CreatedAt: now,
	}}
	h := newReviewHarness(t, svc, true)
	w := doReq(t, h, "POST", "/rentals/"+uuid.NewString()+"/review", `{"scope":"owner","score":5,"comment":"great"}`)
	require.Equal(t, http.StatusCreated, w.Code)
	body := mustJSON(t, w)
	require.Equal(t, "owner", body["scope"])
	require.InDelta(t, 5, body["score"], 0.001)
}

func TestReviewHandler_Create_RequiresSession(t *testing.T) {
	h := newReviewHarness(t, &fakeReviewSvc{}, false)
	w := doReq(t, h, "POST", "/rentals/"+uuid.NewString()+"/review", `{"scope":"owner","score":5}`)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	body := mustJSON(t, w)
	require.Equal(t, "unauthorized", body["code"])
}

func TestReviewHandler_Create_NotParticipant(t *testing.T) {
	svc := &fakeReviewSvc{createErr: review.ErrNotParticipant}
	h := newReviewHarness(t, svc, true)
	w := doReq(t, h, "POST", "/rentals/"+uuid.NewString()+"/review", `{"scope":"owner","score":5}`)
	require.Equal(t, http.StatusForbidden, w.Code)
	body := mustJSON(t, w)
	require.Equal(t, "not_participant", body["code"])
}

func TestReviewHandler_Create_RentalNotTerminal(t *testing.T) {
	svc := &fakeReviewSvc{createErr: review.ErrRentalNotTerminal}
	h := newReviewHarness(t, svc, true)
	w := doReq(t, h, "POST", "/rentals/"+uuid.NewString()+"/review", `{"scope":"owner","score":5}`)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	body := mustJSON(t, w)
	require.Equal(t, "rental_not_terminal", body["code"])
}

func TestReviewHandler_Create_AlreadyReviewed(t *testing.T) {
	svc := &fakeReviewSvc{createErr: review.ErrAlreadyReviewed}
	h := newReviewHarness(t, svc, true)
	w := doReq(t, h, "POST", "/rentals/"+uuid.NewString()+"/review", `{"scope":"owner","score":5}`)
	require.Equal(t, http.StatusConflict, w.Code)
	body := mustJSON(t, w)
	require.Equal(t, "already_reviewed", body["code"])
}

func TestReviewHandler_Create_SelfReviewForbidden(t *testing.T) {
	svc := &fakeReviewSvc{createErr: review.ErrSelfReview}
	h := newReviewHarness(t, svc, true)
	w := doReq(t, h, "POST", "/rentals/"+uuid.NewString()+"/review", `{"scope":"owner","score":5}`)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	body := mustJSON(t, w)
	require.Equal(t, "self_review_forbidden", body["code"])
}

func TestReviewHandler_Create_InvalidInput_BadScore(t *testing.T) {
	svc := &fakeReviewSvc{createErr: review.ErrInvalidInput}
	h := newReviewHarness(t, svc, true)
	w := doReq(t, h, "POST", "/rentals/"+uuid.NewString()+"/review", `{"scope":"owner","score":7}`)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	body := mustJSON(t, w)
	require.Equal(t, "invalid_input", body["code"])
}

func TestReviewHandler_Create_InvalidInput_BadJSON(t *testing.T) {
	svc := &fakeReviewSvc{}
	h := newReviewHarness(t, svc, true)
	w := doReq(t, h, "POST", "/rentals/"+uuid.NewString()+"/review", `{not json`)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestReviewHandler_Create_NotFound(t *testing.T) {
	svc := &fakeReviewSvc{createErr: review.ErrNotFound}
	h := newReviewHarness(t, svc, true)
	w := doReq(t, h, "POST", "/rentals/"+uuid.NewString()+"/review", `{"scope":"owner","score":5}`)
	require.Equal(t, http.StatusNotFound, w.Code)
	body := mustJSON(t, w)
	require.Equal(t, "not_found", body["code"])
}

func TestReviewHandler_Create_UnknownError_MapsInternal(t *testing.T) {
	svc := &fakeReviewSvc{createErr: errors.New("kaboom")}
	h := newReviewHarness(t, svc, true)
	w := doReq(t, h, "POST", "/rentals/"+uuid.NewString()+"/review", `{"scope":"owner","score":5}`)
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- ListUserReviews ---------------------------------------------------

func TestReviewHandler_List_Public_NoSessionRequired(t *testing.T) {
	svc := &fakeReviewSvc{listOut: []review.Review{
		{ID: uuid.NewString(), RaterUserID: "ren-1", RateeUserID: "owner-1",
			Scope: review.ScopeOwner, Score: 5, CreatedAt: time.Now().UTC()},
	}}
	h := newReviewHarness(t, svc, false) // no session
	w := doReq(t, h, "GET", "/users/"+uuid.NewString()+"/reviews", "")
	require.Equal(t, http.StatusOK, w.Code)
	body := mustJSON(t, w)
	reviews, ok := body["reviews"].([]any)
	require.True(t, ok)
	require.Len(t, reviews, 1)
}

func TestReviewHandler_List_EmptyPage(t *testing.T) {
	svc := &fakeReviewSvc{listOut: nil}
	h := newReviewHarness(t, svc, false)
	w := doReq(t, h, "GET", "/users/"+uuid.NewString()+"/reviews", "")
	require.Equal(t, http.StatusOK, w.Code)
	body := mustJSON(t, w)
	reviews, ok := body["reviews"].([]any)
	require.True(t, ok)
	require.Empty(t, reviews)
}

func TestReviewHandler_List_ServiceError_MapsInternal(t *testing.T) {
	svc := &fakeReviewSvc{listErr: errors.New("db oops")}
	h := newReviewHarness(t, svc, false)
	w := doReq(t, h, "GET", "/users/"+uuid.NewString()+"/reviews", "")
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- GetUserReviewAggregate --------------------------------------------

func TestReviewHandler_Aggregate_Public(t *testing.T) {
	svc := &fakeReviewSvc{aggregateOut: review.NewAggregate("owner-1", review.ScopeOwner, 5, 23)}
	h := newReviewHarness(t, svc, false)
	w := doReq(t, h, "GET", "/users/"+uuid.NewString()+"/review-aggregate?scope=owner", "")
	require.Equal(t, http.StatusOK, w.Code)
	body := mustJSON(t, w)
	require.Equal(t, "owner", body["scope"])
	require.InDelta(t, 5, body["count"], 0.001)
	require.InDelta(t, 23, body["sum"], 0.001)
	require.InDelta(t, 4.6, body["avg"], 0.01)
}

func TestReviewHandler_Aggregate_ZeroWhenNoneInserted(t *testing.T) {
	svc := &fakeReviewSvc{aggregateOut: review.ReviewAggregate{
		RateeUserID: "owner-1", Scope: review.ScopeOwner, Count: 0, Sum: 0, Avg: 0,
	}}
	h := newReviewHarness(t, svc, false)
	w := doReq(t, h, "GET", "/users/"+uuid.NewString()+"/review-aggregate?scope=owner", "")
	require.Equal(t, http.StatusOK, w.Code)
	body := mustJSON(t, w)
	require.InDelta(t, 0, body["count"], 0.001)
	require.InDelta(t, 0, body["avg"], 0.001)
}

func TestReviewHandler_Aggregate_InvalidScopeReturns422(t *testing.T) {
	svc := &fakeReviewSvc{}
	h := newReviewHarness(t, svc, false)
	w := doReq(t, h, "GET", "/users/"+uuid.NewString()+"/review-aggregate?scope=bogus", "")
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	body := mustJSON(t, w)
	require.Equal(t, "scope_invalid", body["code"])
}

// --- Localization keys -------------------------------------------------

func TestReviewHandler_LocalizationKeys_FollowDomainPattern(t *testing.T) {
	svc := &fakeReviewSvc{createErr: review.ErrInvalidInput}
	h := newReviewHarness(t, svc, true)
	w := doReq(t, h, "POST", "/rentals/"+uuid.NewString()+"/review", `{"scope":"owner","score":5}`)
	body := mustJSON(t, w)
	require.NotEmpty(t, body["message_key"])
	require.Equal(t, "review.invalid_input", body["message_key"])
}
