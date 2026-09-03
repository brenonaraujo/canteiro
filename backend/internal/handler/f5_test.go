package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
	"github.com/brenonaraujo/canteiro/backend/internal/handler"
	f5svc "github.com/brenonaraujo/canteiro/backend/internal/rental/f5"
)

// --- minimal fakes for the F5 handler ---

type f5ReturnSvc struct {
	created rental.Return
	err     error
}

func (f *f5ReturnSvc) RegisterPickup(_ context.Context, _ string, _ rental.EvidencePayload) (rental.Return, error) {
	return f.created, f.err
}
func (f *f5ReturnSvc) RegisterReturn(_ context.Context, _ string, _ rental.EvidencePayload) (rental.Return, error) {
	return f.created, f.err
}

type f5DamageSvc struct {
	opened rental.DamageClaim
	responded rental.DamageClaim
	resolved rental.DamageClaim
	err     error
}

func (f *f5DamageSvc) OpenDamageClaim(_ context.Context, _ f5svc.OpenDamageClaimInput) (rental.DamageClaim, error) {
	return f.opened, f.err
}
func (f *f5DamageSvc) RenterRespond(_ context.Context, _ f5svc.RenterRespondInput) (rental.DamageClaim, error) {
	return f.responded, f.err
}
func (f *f5DamageSvc) StaffResolve(_ context.Context, _ f5svc.StaffResolveInput) (rental.DamageClaim, error) {
	return f.resolved, f.err
}

type f5DebtSvc struct {
	settled  rental.Debt
	forgiven rental.Debt
	err      error
}

func (f *f5DebtSvc) SettleDebt(_ context.Context, _ f5svc.SettleDebtInput) (rental.Debt, error) {
	return f.settled, f.err
}
func (f *f5DebtSvc) ForgiveDebt(_ context.Context, _ f5svc.ForgiveDebtInput) (rental.Debt, error) {
	return f.forgiven, f.err
}

// --- helpers ---

func newF5Harness(t *testing.T, returns *f5ReturnSvc, damage *f5DamageSvc, debts *f5DebtSvc) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	current := func(*gin.Context) (string, bool) { return "actor-1", true }
	api := handler.NewF5API(returns, damage, debts, current)
	r := gin.New()
	// We mount the handlers directly to avoid pulling in the full router.
	r.POST("/rentals/:id/pickup", func(c *gin.Context) {
		api.RegisterRentalPickup(c, uuidFromParam(c, "id"))
	})
	r.POST("/rentals/:id/return", func(c *gin.Context) {
		api.RegisterRentalReturn(c, uuidFromParam(c, "id"))
	})
	r.POST("/rentals/:id/damage", func(c *gin.Context) {
		api.OpenDamageClaim(c, uuidFromParam(c, "id"))
	})
	r.POST("/damage/:id/respond", func(c *gin.Context) {
		api.RespondDamageClaim(c, uuidFromParam(c, "id"))
	})
	r.POST("/damage/:id/resolve", func(c *gin.Context) {
		api.StaffResolveDamage(c, uuidFromParam(c, "id"))
	})
	r.POST("/debt/:id/settle", func(c *gin.Context) {
		api.SettleDebt(c, uuidFromParam(c, "id"))
	})
	r.POST("/debt/:id/forgive", func(c *gin.Context) {
		api.ForgiveDebt(c, uuidFromParam(c, "id"))
	})
	return r
}

func uuidFromParam(c *gin.Context, name string) (out [16]byte) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		return out
	}
	return id
}

func doReq(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func mustJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
	return m
}

// --- tests ---------------------------------------------------------------

func TestF5Handler_RegisterRentalPickup_HappyPath(t *testing.T) {
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	returns := &f5ReturnSvc{created: rental.Return{ID: "id-1", RentalID: "rid-1", State: rental.ReturnInProgress, CreatedAt: now, UpdatedAt: now}}
	h := newF5Harness(t, returns, &f5DamageSvc{}, &f5DebtSvc{})

	w := doReq(t, h, "POST", "/rentals/"+uuid.NewString()+"/pickup", `{"photos":["p1"]}`)
	require.Equal(t, http.StatusOK, w.Code)
	body := mustJSON(t, w)
	require.Equal(t, "in_progress", body["state"])
}

func TestF5Handler_RegisterRentalPickup_ServiceError_InvalidInput(t *testing.T) {
	returns := &f5ReturnSvc{err: rental.ErrInvalidInput}
	h := newF5Harness(t, returns, &f5DamageSvc{}, &f5DebtSvc{})
	w := doReq(t, h, "POST", "/rentals/"+uuid.NewString()+"/pickup", `{}`)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	body := mustJSON(t, w)
	require.Equal(t, "invalid_input", body["code"])
}

func TestF5Handler_RegisterRentalReturn_ServiceError_WindowOpen(t *testing.T) {
	returns := &f5ReturnSvc{err: rental.ErrF5ReturnWindowOpen}
	h := newF5Harness(t, returns, &f5DamageSvc{}, &f5DebtSvc{})
	w := doReq(t, h, "POST", "/rentals/"+uuid.NewString()+"/return", `{}`)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	body := mustJSON(t, w)
	require.Equal(t, "return_window_open", body["code"])
}

func TestF5Handler_OpenDamageClaim_HappyPath(t *testing.T) {
	opened := rental.DamageClaim{ID: "id-1", RentalID: "rid-1", State: rental.DamageOpen, Nature: rental.DamageCosmetic, OpenedAt: time.Now()}
	damage := &f5DamageSvc{opened: opened}
	h := newF5Harness(t, &f5ReturnSvc{}, damage, &f5DebtSvc{})
	w := doReq(t, h, "POST", "/rentals/"+uuid.NewString()+"/damage", `{"nature":"cosmetic","proposed_cents":1000,"description":"scratch"}`)
	require.Equal(t, http.StatusCreated, w.Code)
}

func TestF5Handler_OpenDamageClaim_RejectsNonOwner(t *testing.T) {
	damage := &f5DamageSvc{err: rental.ErrForbidden}
	h := newF5Harness(t, &f5ReturnSvc{}, damage, &f5DebtSvc{})
	w := doReq(t, h, "POST", "/rentals/"+uuid.NewString()+"/damage", `{"nature":"cosmetic","proposed_cents":1000,"description":"x"}`)
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestF5Handler_OpenDamageClaim_RejectsWindowExpired(t *testing.T) {
	damage := &f5DamageSvc{err: rental.ErrF5DamageWindowExpired}
	h := newF5Harness(t, &f5ReturnSvc{}, damage, &f5DebtSvc{})
	w := doReq(t, h, "POST", "/rentals/"+uuid.NewString()+"/damage", `{"nature":"cosmetic","proposed_cents":1000,"description":"x"}`)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestF5Handler_OpenDamageClaim_RejectsMissingEvidence(t *testing.T) {
	damage := &f5DamageSvc{err: rental.ErrF5DamageEvidenceRequired}
	h := newF5Harness(t, &f5ReturnSvc{}, damage, &f5DebtSvc{})
	w := doReq(t, h, "POST", "/rentals/"+uuid.NewString()+"/damage", `{"nature":"cosmetic","proposed_cents":1000,"description":""}`)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestF5Handler_OpenDamageClaim_RejectsInvalidPayload(t *testing.T) {
	h := newF5Harness(t, &f5ReturnSvc{}, &f5DamageSvc{}, &f5DebtSvc{})
	w := doReq(t, h, "POST", "/rentals/"+uuid.NewString()+"/damage", `{not-json`)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestF5Handler_RespondDamageClaim_HappyPath(t *testing.T) {
	responded := rental.DamageClaim{ID: "id-1", State: rental.DamageContested, Nature: rental.DamageCosmetic}
	damage := &f5DamageSvc{responded: responded}
	h := newF5Harness(t, &f5ReturnSvc{}, damage, &f5DebtSvc{})
	w := doReq(t, h, "POST", "/damage/"+uuid.NewString()+"/respond", `{"response":"contest","note":"i didn't"}`)
	require.Equal(t, http.StatusOK, w.Code)
	body := mustJSON(t, w)
	require.Equal(t, "contested", body["state"])
}

func TestF5Handler_StaffResolveDamage_HappyPath(t *testing.T) {
	resolved := rental.DamageClaim{ID: "id-1", State: rental.DamageStaffResolved, Nature: rental.DamageCosmetic, AgreedCents: 5000}
	damage := &f5DamageSvc{resolved: resolved}
	h := newF5Harness(t, &f5ReturnSvc{}, damage, &f5DebtSvc{})
	w := doReq(t, h, "POST", "/damage/"+uuid.NewString()+"/resolve", `{"agreed_cents":5000,"note":"evidence"}`)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestF5Handler_StaffResolveDamage_RejectsMissingNote(t *testing.T) {
	damage := &f5DamageSvc{err: rental.ErrF5DamageEvidenceRequired}
	h := newF5Harness(t, &f5ReturnSvc{}, damage, &f5DebtSvc{})
	w := doReq(t, h, "POST", "/damage/"+uuid.NewString()+"/resolve", `{"agreed_cents":5000,"note":""}`)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestF5Handler_SettleDebt_HappyPath(t *testing.T) {
	debts := &f5DebtSvc{settled: rental.Debt{ID: "id-1", State: rental.DebtSettled, OriginalCents: 5000, SettledCents: 5000}}
	h := newF5Harness(t, &f5ReturnSvc{}, &f5DamageSvc{}, debts)
	w := doReq(t, h, "POST", "/debt/"+uuid.NewString()+"/settle", `{}`)
	require.Equal(t, http.StatusOK, w.Code)
	body := mustJSON(t, w)
	require.Equal(t, "settled", body["state"])
}

func TestF5Handler_ForgiveDebt_HappyPath(t *testing.T) {
	debts := &f5DebtSvc{forgiven: rental.Debt{ID: "id-1", State: rental.DebtForgiven, OriginalCents: 5000, ForgivenCents: 5000}}
	h := newF5Harness(t, &f5ReturnSvc{}, &f5DamageSvc{}, debts)
	w := doReq(t, h, "POST", "/debt/"+uuid.NewString()+"/forgive", `{"cents":5000,"reason":"owner withdrew"}`)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestF5Handler_ForgiveDebt_RejectsMissingReason(t *testing.T) {
	debts := &f5DebtSvc{err: rental.ErrF5DebtForgiveRequiresReason}
	h := newF5Harness(t, &f5ReturnSvc{}, &f5DamageSvc{}, debts)
	w := doReq(t, h, "POST", "/debt/"+uuid.NewString()+"/forgive", `{"cents":1000,"reason":""}`)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestF5Handler_GenericInternalError(t *testing.T) {
	returns := &f5ReturnSvc{err: errors.New("db exploded")}
	h := newF5Harness(t, returns, &f5DamageSvc{}, &f5DebtSvc{})
	w := doReq(t, h, "POST", "/rentals/"+uuid.NewString()+"/pickup", `{}`)
	require.Equal(t, http.StatusInternalServerError, w.Code)
}
