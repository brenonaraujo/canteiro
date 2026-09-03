// Package handler: F5 endpoints (return, damage, debt). Implemented as
// separate API types so the F3 RentalAPI (handler/rental.go) stays
// untouched. F5 wiring happens via apiMux in internal/app/server.go.
package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/brenonaraujo/canteiro/backend/internal/api"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
	"github.com/brenonaraujo/canteiro/backend/internal/i18n"
	f5svc "github.com/brenonaraujo/canteiro/backend/internal/rental/f5"
)

// ReturnService is the slice of f5.Service the return handlers need.
type ReturnService interface {
	RegisterPickup(ctx context.Context, rentalID string, ev rental.EvidencePayload) (rental.Return, error)
	RegisterReturn(ctx context.Context, rentalID string, ev rental.EvidencePayload) (rental.Return, error)
}

// DamageService is the slice of f5.Service the damage handlers need.
type DamageService interface {
	OpenDamageClaim(ctx context.Context, in f5svc.OpenDamageClaimInput) (rental.DamageClaim, error)
	RenterRespond(ctx context.Context, in f5svc.RenterRespondInput) (rental.DamageClaim, error)
	StaffResolve(ctx context.Context, in f5svc.StaffResolveInput) (rental.DamageClaim, error)
}

// DebtService is the slice of f5.Service the debt handlers need.
type DebtService interface {
	SettleDebt(ctx context.Context, in f5svc.SettleDebtInput) (rental.Debt, error)
	ForgiveDebt(ctx context.Context, in f5svc.ForgiveDebtInput) (rental.Debt, error)
}

// F5API bundles the F5 HTTP endpoints. Wired into apiMux as F5API.
type F5API struct {
	returns ReturnService
	damage  DamageService
	debts   DebtService
	current CurrentAccountFn
}

// NewF5API builds the F5 adapter. current is the session lookup (nil = no
// session, useful for tests).
func NewF5API(returns ReturnService, damage DamageService, debts DebtService, current CurrentAccountFn) *F5API {
	if current == nil {
		current = noSession
	}
	return &F5API{returns: returns, damage: damage, debts: debts, current: current}
}

// --- helpers -------------------------------------------------------------

func (h *F5API) requireSession(c *gin.Context) (string, bool) {
	id, ok := h.current(c)
	if !ok {
		h.writeErr(c, http.StatusUnauthorized, "unauthorized", "f5.unauthorized")
		return "", false
	}
	return id, true
}

func (h *F5API) writeErr(c *gin.Context, status int, code, key string) {
	c.JSON(status, api.Error{Code: code, Message: i18n.T(c.Request.Context(), key), MessageKey: key})
}

func (h *F5API) writeServiceErr(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, rental.ErrNotFound):
		h.writeErr(c, http.StatusNotFound, "not_found", "f5.not_found")
	case errors.Is(err, rental.ErrForbidden):
		h.writeErr(c, http.StatusForbidden, "forbidden", "f5.forbidden")
	case errors.Is(err, rental.ErrInvalidInput):
		h.writeErr(c, http.StatusUnprocessableEntity, "invalid_input", "f5.invalid_input")
	case errors.Is(err, rental.ErrF5RentalNotConfirmed):
		h.writeErr(c, http.StatusConflict, "rental_not_confirmed", "f5.rental_not_confirmed")
	case errors.Is(err, rental.ErrF5ReturnNotFound):
		h.writeErr(c, http.StatusNotFound, "return_not_found", "f5.return_not_found")
	case errors.Is(err, rental.ErrF5ReturnAlreadyExists):
		h.writeErr(c, http.StatusConflict, "return_already_exists", "f5.return_already_exists")
	case errors.Is(err, rental.ErrF5ReturnInvalidState):
		h.writeErr(c, http.StatusConflict, "return_invalid_state", "f5.return_invalid_state")
	case errors.Is(err, rental.ErrF5ReturnWindowOpen):
		h.writeErr(c, http.StatusUnprocessableEntity, "return_window_open", "f5.return_window_open")
	case errors.Is(err, rental.ErrF5ReturnAlreadyClosed):
		h.writeErr(c, http.StatusConflict, "return_already_closed", "f5.return_already_closed")
	case errors.Is(err, rental.ErrF5DamageNotFound):
		h.writeErr(c, http.StatusNotFound, "damage_not_found", "f5.damage_not_found")
	case errors.Is(err, rental.ErrF5DamageAlreadyExists):
		h.writeErr(c, http.StatusConflict, "damage_already_exists", "f5.damage_already_exists")
	case errors.Is(err, rental.ErrF5DamageWindowExpired):
		h.writeErr(c, http.StatusUnprocessableEntity, "damage_window_expired", "f5.damage_window_expired")
	case errors.Is(err, rental.ErrF5DamageInvalidNature):
		h.writeErr(c, http.StatusUnprocessableEntity, "damage_invalid_nature", "f5.damage_invalid_nature")
	case errors.Is(err, rental.ErrF5DamageAmountInvalid):
		h.writeErr(c, http.StatusUnprocessableEntity, "damage_amount_invalid", "f5.damage_amount_invalid")
	case errors.Is(err, rental.ErrF5DamageEvidenceRequired):
		h.writeErr(c, http.StatusUnprocessableEntity, "damage_evidence_required", "f5.damage_evidence_required")
	case errors.Is(err, rental.ErrF5DamageInvalidState):
		h.writeErr(c, http.StatusConflict, "damage_invalid_state", "f5.damage_invalid_state")
	case errors.Is(err, rental.ErrF5DebtNotFound):
		h.writeErr(c, http.StatusNotFound, "debt_not_found", "f5.debt_not_found")
	case errors.Is(err, rental.ErrF5DebtAlreadySettled):
		h.writeErr(c, http.StatusConflict, "debt_already_settled", "f5.debt_already_settled")
	case errors.Is(err, rental.ErrF5DebtInvalidState):
		h.writeErr(c, http.StatusConflict, "debt_invalid_state", "f5.debt_invalid_state")
	case errors.Is(err, rental.ErrF5DebtForgiveRequiresReason):
		h.writeErr(c, http.StatusUnprocessableEntity, "debt_forgive_requires_reason", "f5.debt_forgive_requires_reason")
	case errors.Is(err, rental.ErrF5DebtAmountInvalid):
		h.writeErr(c, http.StatusUnprocessableEntity, "debt_amount_invalid", "f5.debt_amount_invalid")
	case errors.Is(err, rental.ErrF5DebtCapExceeded):
		h.writeErr(c, http.StatusUnprocessableEntity, "debt_cap_exceeded", "f5.debt_cap_exceeded")
	default:
		h.writeErr(c, http.StatusInternalServerError, "internal_error", "error.internal")
	}
	return true
}

// --- conversions ---------------------------------------------------------

func returnToAPI(r rental.Return) api.Return {
	out := api.Return{
		Id:       toUUID(r.ID),
		RentalId: toUUID(r.RentalID),
		State:    api.ReturnState(r.State),
	}
	if r.DepositReleasedCents != 0 {
		v := int(r.DepositReleasedCents)
		out.DepositReleasedCents = &v
	}
	if r.DepositCapturedCents != 0 {
		v := int(r.DepositCapturedCents)
		out.DepositCapturedCents = &v
	}
	if !r.CreatedAt.IsZero() {
		t := r.CreatedAt
		out.CreatedAt = &t
	}
	if !r.UpdatedAt.IsZero() {
		t := r.UpdatedAt
		out.UpdatedAt = &t
	}
	if r.ReturnedAt != nil {
		t := *r.ReturnedAt
		out.ReturnedAt = &t
	}
	return out
}

func evidenceFromAPI(e api.ReturnEvidence) rental.EvidencePayload {
	out := rental.EvidencePayload{}
	if e.Description != nil {
		out.Description = *e.Description
	}
	if e.Photos != nil {
		out.Photos = *e.Photos
	}
	if e.Checklist != nil {
		out.Checklist = *e.Checklist
	}
	return out
}

func damageToAPI(c rental.DamageClaim) api.DamageClaim {
	out := api.DamageClaim{
		Id:       toUUID(c.ID),
		RentalId: toUUID(c.RentalID),
		State:    api.DamageState(c.State),
		Nature:   api.DamageNature(c.Nature),
	}
	if c.Description != "" {
		s := c.Description
		out.Description = &s
	}
	if c.ProposedCents != 0 {
		v := int(c.ProposedCents)
		out.ProposedCents = &v
	}
	if c.AgreedCents != 0 {
		v := int(c.AgreedCents)
		out.AgreedCents = &v
	}
	if !c.OpenedAt.IsZero() {
		t := c.OpenedAt
		out.OpenedAt = &t
	}
	if c.RespondedAt != nil {
		t := *c.RespondedAt
		out.RespondedAt = &t
	}
	if c.DecidedAt != nil {
		t := *c.DecidedAt
		out.DecidedAt = &t
	}
	if c.ResolvedAt != nil {
		t := *c.ResolvedAt
		out.ResolvedAt = &t
	}
	return out
}

func debtToAPI(d rental.Debt) api.Debt {
	out := api.Debt{
		Id:       toUUID(d.ID),
		RentalId: toUUID(d.RentalID),
		State:    api.DebtState(d.State),
	}
	if d.DamageID != "" {
		v := toUUID(d.DamageID)
		out.DamageId = &v
	}
	if !d.DueAt.IsZero() {
		t := d.DueAt
		out.DueAt = &t
	}
	if d.OriginalCents != 0 {
		out.OriginalCents = int(d.OriginalCents)
	}
	if d.ForgivenCents != 0 {
		v := int(d.ForgivenCents)
		out.ForgivenCents = &v
	}
	if d.SettledCents != 0 {
		v := int(d.SettledCents)
		out.SettledCents = &v
	}
	if d.ForgivenReason != "" {
		s := d.ForgivenReason
		out.ForgivenReason = &s
	}
	if d.SettledAt != nil {
		t := *d.SettledAt
		out.SettledAt = &t
	}
	if d.ForgivenAt != nil {
		t := *d.ForgivenAt
		out.ForgivenAt = &t
	}
	return out
}

// --- return handlers -----------------------------------------------------

// RegisterRentalPickup implements api.RegisterRentalPickup.
func (h *F5API) RegisterRentalPickup(c *gin.Context, id openapi_types.UUID) {
	_, ok := h.requireSession(c)
	if !ok {
		return
	}
	var req api.ReturnEvidence
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeErr(c, http.StatusBadRequest, "invalid_request", "f5.invalid_evidence")
		return
	}
	got, err := h.returns.RegisterPickup(c.Request.Context(), id.String(), evidenceFromAPI(req))
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusOK, returnToAPI(got))
}

// RegisterRentalReturn implements api.RegisterRentalReturn.
func (h *F5API) RegisterRentalReturn(c *gin.Context, id openapi_types.UUID) {
	_, ok := h.requireSession(c)
	if !ok {
		return
	}
	var req api.ReturnEvidence
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeErr(c, http.StatusBadRequest, "invalid_request", "f5.invalid_evidence")
		return
	}
	got, err := h.returns.RegisterReturn(c.Request.Context(), id.String(), evidenceFromAPI(req))
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusOK, returnToAPI(got))
}

// --- damage handlers -----------------------------------------------------

// OpenDamageClaim implements api.OpenDamageClaim.
func (h *F5API) OpenDamageClaim(c *gin.Context, id openapi_types.UUID) {
	actorID, ok := h.requireSession(c)
	if !ok {
		return
	}
	var req api.OpenDamageClaimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeErr(c, http.StatusBadRequest, "invalid_request", "f5.invalid_payload")
		return
	}
	ev := rental.EvidencePayload{}
	if req.Evidence != nil {
		ev = evidenceFromAPI(*req.Evidence)
	}
	got, err := h.damage.OpenDamageClaim(c.Request.Context(), f5svc.OpenDamageClaimInput{
		RentalID:      id.String(),
		OwnerID:       actorID,
		Nature:        rental.DamageNature(req.Nature),
		ProposedCents: int64(req.ProposedCents),
		Description:   req.Description,
		Evidence:      ev,
	})
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, damageToAPI(got))
}

// RespondDamageClaim implements api.RespondDamageClaim.
func (h *F5API) RespondDamageClaim(c *gin.Context, id openapi_types.UUID) {
	actorID, ok := h.requireSession(c)
	if !ok {
		return
	}
	var req api.RespondDamageClaimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeErr(c, http.StatusBadRequest, "invalid_request", "f5.invalid_payload")
		return
	}
	note := ""
	if req.Note != nil {
		note = *req.Note
	}
	agreed := int64(0)
	if req.AgreedCents != nil {
		agreed = int64(*req.AgreedCents)
	}
	got, err := h.damage.RenterRespond(c.Request.Context(), f5svc.RenterRespondInput{
		ClaimID:     id.String(),
		RenterID:    actorID,
		Response:    f5svc.ResponseKind(req.Response),
		AgreedCents: agreed,
		Note:        note,
	})
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusOK, damageToAPI(got))
}

// StaffResolveDamage implements api.StaffResolveDamage.
func (h *F5API) StaffResolveDamage(c *gin.Context, id openapi_types.UUID) {
	_, ok := h.requireSession(c)
	if !ok {
		return
	}
	var req api.StaffResolveDamageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeErr(c, http.StatusBadRequest, "invalid_request", "f5.invalid_payload")
		return
	}
	got, err := h.damage.StaffResolve(c.Request.Context(), f5svc.StaffResolveInput{
		ClaimID:     id.String(),
		AgreedCents: int64(req.AgreedCents),
		Note:        req.Note,
	})
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusOK, damageToAPI(got))
}

// --- debt handlers -------------------------------------------------------

// SettleDebt implements api.SettleDebt.
func (h *F5API) SettleDebt(c *gin.Context, id openapi_types.UUID) {
	_, ok := h.requireSession(c)
	if !ok {
		return
	}
	var req api.SettleDebtRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeErr(c, http.StatusBadRequest, "invalid_request", "f5.invalid_payload")
		return
	}
	if req.SettledCents <= 0 {
		h.writeErr(c, http.StatusUnprocessableEntity, "debt_amount_invalid", "f5.debt_amount_invalid")
		return
	}
	// The service enforces SettledCents == remaining
	// (original - forgiven - already settled) and returns ErrF5DebtAmountInvalid
	// otherwise, which the error switch maps to 422 debt_amount_invalid.
	got, err := h.debts.SettleDebt(c.Request.Context(), f5svc.SettleDebtInput{
		DebtID:       id.String(),
		SettledCents: req.SettledCents,
	})
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusOK, debtToAPI(got))
}

// ForgiveDebt implements api.ForgiveDebt.
func (h *F5API) ForgiveDebt(c *gin.Context, id openapi_types.UUID) {
	actorID, ok := h.requireSession(c)
	if !ok {
		return
	}
	var req api.ForgiveDebtRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeErr(c, http.StatusBadRequest, "invalid_request", "f5.invalid_payload")
		return
	}
	got, err := h.debts.ForgiveDebt(c.Request.Context(), f5svc.ForgiveDebtInput{
		DebtID:  id.String(),
		Cents:   int64(req.Cents),
		Reason:  req.Reason,
		StaffID: actorID,
	})
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusOK, debtToAPI(got))
}
