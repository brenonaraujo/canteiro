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
	rentsvc "github.com/brenonaraujo/canteiro/backend/internal/rental"
)

// RentalAPI wires the F3 rental endpoints. Mirrors the ListingAPI surface
// (requireSession + writeServiceErr) so errors map 1:1 to i18n keys.
type RentalAPI struct {
	svc     RentalService
	current CurrentAccountFn
}

// RentalService is the slice of rental.Service the handler depends on.
// Keeping the surface narrow lets tests inject fakes without standing up
// the full service graph.
type RentalService interface {
	CreateIntent(ctx context.Context, in rentsvc.CreateIntentInput) (rental.Rental, rental.MoneyBreakdown, error)
	ListForTenant(ctx context.Context, tenantID string) ([]rental.Rental, error)
	Get(ctx context.Context, id string) (rental.Rental, error)
	AuthorizeIntent(ctx context.Context, in rentsvc.AuthorizeIntentInput) (rentsvc.PaymentIntent, error)
	Accept(ctx context.Context, in rentsvc.AcceptInput) (rental.Rental, error)
	Decline(ctx context.Context, in rentsvc.DeclineInput) (rental.Rental, error)
	CancelPreAuth(ctx context.Context, in rentsvc.CancelPreAuthInput) (rental.Rental, error)
	GetReceipt(ctx context.Context, rentalID, tenantID string) (rental.Receipt, error)
	Cancel(ctx context.Context, in rentsvc.CancelInput) (rentsvc.CancellationResult, error)
	GetCancellation(ctx context.Context, rentalID, callerAccountID string) (rentsvc.CancellationRecord, error)
}

// NewRentalAPI builds the rental adapter.
func NewRentalAPI(svc RentalService, current CurrentAccountFn) *RentalAPI {
	if current == nil {
		current = noSession
	}
	return &RentalAPI{svc: svc, current: current}
}

func (h *RentalAPI) requireSession(c *gin.Context) (string, bool) {
	id, ok := h.current(c)
	if !ok {
		h.writeErr(c, http.StatusUnauthorized, "unauthorized", "rental.unauthorized")
		return "", false
	}
	return id, true
}

func (h *RentalAPI) writeErr(c *gin.Context, status int, code, key string) {
	c.JSON(status, api.Error{Code: code, Message: i18n.T(c.Request.Context(), key), MessageKey: key})
}

func (h *RentalAPI) writeServiceErr(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, rental.ErrNotFound):
		h.writeErr(c, http.StatusNotFound, "not_found", "rental.not_found")
	case errors.Is(err, rental.ErrForbidden):
		h.writeErr(c, http.StatusForbidden, "forbidden", "rental.forbidden")
	case errors.Is(err, rental.ErrInvalidInput):
		h.writeErr(c, http.StatusUnprocessableEntity, "invalid_input", "rental.invalid_input")
	case errors.Is(err, rental.ErrCalendarOverlap):
		h.writeErr(c, http.StatusConflict, "calendar_overlap", "rental.calendar_overlap")
	case errors.Is(err, rental.ErrOperatorTermsRequired):
		h.writeErr(c, http.StatusUnprocessableEntity, "operator_terms_required", "rental.operator_terms_required")
	case errors.Is(err, rental.ErrOperatorNotAvailable):
		h.writeErr(c, http.StatusUnprocessableEntity, "operator_not_available", "rental.operator_not_available")
	case errors.Is(err, rental.ErrAccountDeactivated):
		h.writeErr(c, http.StatusForbidden, "account_deactivated", "rental.account_deactivated")
	case errors.Is(err, rental.ErrProfileIncomplete):
		h.writeErr(c, http.StatusForbidden, "profile_incomplete", "rental.profile_incomplete")
	case errors.Is(err, rental.ErrInvalidTransition):
		h.writeErr(c, http.StatusConflict, "invalid_transition", "rental.invalid_transition")
	case errors.Is(err, rental.ErrAcceptanceExpired):
		h.writeErr(c, http.StatusConflict, "acceptance_expired", "rental.acceptance_expired")
	case errors.Is(err, rental.ErrPaymentTotalMismatch):
		h.writeErr(c, http.StatusUnprocessableEntity, "payment_mismatch", "rental.payment_mismatch")
	case errors.Is(err, rental.ErrTenantHasDebt):
		h.writeErr(c, http.StatusForbidden, "tenant_has_debt", "rental.tenant_has_debt")
	case errors.Is(err, rental.ErrOpenDebt):
		h.writeErr(c, http.StatusUnprocessableEntity, "open_debt", "rental.debt.open_blocks_intent")
	case errors.Is(err, rental.ErrListingUnavailable):
		h.writeErr(c, http.StatusConflict, "listing_unavailable", "rental.listing_unavailable")
	default:
		h.writeErr(c, http.StatusInternalServerError, "internal_error", "error.internal")
	}
	return true
}

// --- conversions ----------------------------------------------------------

func rentalToAPI(r rental.Rental) api.Rental {
	out := api.Rental{
		Id:                    toUUID(r.ID),
		ListingId:             toUUID(r.ListingID),
		TenantAccountId:       toUUID(r.TenantAccountID),
		State:                 api.RentalState(r.State),
		StartsAt:              r.StartsAt,
		EndsAt:                r.EndsAt,
		WithOperator:          r.WithOperator,
		OperatorTermsAccepted: r.OperatorTermsAccepted,
		IntentKey:             r.IntentKey,
		RentCents:             int(r.RentCents),
		OperatorCents:         int(r.OperatorCents),
		DepositCents:          int(r.DepositCents),
		CommissionCents:       int(r.CommissionCents),
		OwnerPayoutCents:      int(r.OwnerPayoutCents),
		OperatorPayoutCents:   int(r.OperatorPayoutCents),
		ListingSnapshot:       snapshotToAPI(r.ListingSnapshot),
		DeclineReason:         &r.DeclineReason,
	}
	if r.AcceptanceDeadlineAt != nil {
		t := *r.AcceptanceDeadlineAt
		out.AcceptanceDeadlineAt = &t
	}
	if r.ConfirmedAt != nil {
		t := *r.ConfirmedAt
		out.ConfirmedAt = &t
	}
	if r.DeclinedAt != nil {
		t := *r.DeclinedAt
		out.DeclinedAt = &t
	}
	out.CreatedAt = r.CreatedAt
	out.UpdatedAt = r.UpdatedAt
	return out
}

func snapshotToAPI(s rental.ListingSnapshot) api.ListingSnapshot {
	mode := api.OperatorMode(s.Operator.Mode)
	out := api.ListingSnapshot{
		OwnerId:          toUUID(s.OwnerID),
		Title:            s.Title,
		Category:         api.ListingCategory(s.Category),
		PriceUnit:        api.PriceUnit(s.PriceUnit),
		PriceAmountCents: int(s.PriceAmountCents),
		DepositCents:     int(s.DepositCents),
		MinLeadTimeHours: &s.MinLeadTimeHours,
		PickupCity:       &s.PickupCity,
		Operator: api.SnapshotOperator{
			Mode:            mode,
			HourlyRateCents: int(s.Operator.HourlyRateCents),
			MinHours:        s.Operator.MinHours,
			Name:            &s.Operator.Name,
			Phone:           &s.Operator.Phone,
			IsOwner:         s.Operator.IsOwner,
		},
	}
	return out
}

func quoteToAPI(r rental.Rental, b rental.MoneyBreakdown) api.RentalQuoteOut {
	return api.RentalQuoteOut{
		RentalId:            toUUID(r.ID),
		RentCents:           int(b.RentCents),
		OperatorCents:       int(b.OperatorCents),
		DepositCents:        int(b.DepositCents),
		TotalCents:          int(b.TotalCents),
		CommissionBaseCents: int(b.CommissionBaseCents),
		CommissionCents:     int(b.CommissionCents),
		OwnerPayoutCents:    int(b.OwnerPayoutCents),
		OperatorPayoutCents: int(b.OperatorPayoutCents),
	}
}

func receiptToAPI(r rental.Receipt) api.RentalReceipt {
	return api.RentalReceipt{
		RentalId:            toUUID(r.RentalID),
		TenantAccountId:     toUUID(r.TenantAccountID),
		RentCents:           int(r.RentCents),
		OperatorCents:       int(r.OperatorCents),
		DepositCents:        int(r.DepositCents),
		TotalCents:          int(r.TotalCents),
		CommissionBaseCents: int(r.CommissionBaseCents),
		CommissionCents:     int(r.CommissionCents),
		OwnerPayoutCents:    int(r.OwnerPayoutCents),
		OperatorPayoutCents: int(r.OperatorPayoutCents),
		ListingSnapshot:     snapshotToAPI(r.ListingSnapshot),
		WindowStartsAt:      r.WindowStartsAt,
		WindowEndsAt:        r.WindowEndsAt,
		IssuedAt:            r.IssuedAt,
	}
}

func intentToAPI(i rentsvc.PaymentIntent) api.PaymentIntent {
	out := api.PaymentIntent{
		Id:                 toUUID(i.ID),
		RentalId:           toUUID(i.RentalID),
		Provider:           api.PaymentIntentProvider(i.Provider),
		IdempotencyKey:     i.IdempotencyKey,
		Attempt:            i.Attempt,
		AmountCents:        int(i.AmountCents),
		DepositCents:       int(i.DepositCents),
		ExpectedTotalCents: int(i.ExpectedTotalCents),
		Status:             api.PaymentIntentStatus(i.Status),
		FailureCode:        &i.FailureCode,
		FailureMessage:     &i.FailureMessage,
	}
	if i.ProviderPaymentID != "" {
		v := i.ProviderPaymentID
		out.ProviderPaymentId = &v
	}
	if !i.CreatedAt.IsZero() {
		t := i.CreatedAt
		out.CreatedAt = &t
	}
	if !i.UpdatedAt.IsZero() {
		t := i.UpdatedAt
		out.UpdatedAt = &t
	}
	return out
}

// --- handlers -------------------------------------------------------------

// CreateRental implements api.CreateRental.
func (h *RentalAPI) CreateRental(c *gin.Context) {
	tenantID, ok := h.requireSession(c)
	if !ok {
		return
	}
	var req api.CreateRentalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeErr(c, http.StatusBadRequest, "invalid_request", "rental.invalid_payload")
		return
	}
	r, b, err := h.svc.CreateIntent(c.Request.Context(), rentsvc.CreateIntentInput{
		TenantID:              tenantID,
		ListingID:             req.ListingId.String(),
		StartsAt:              req.StartsAt,
		EndsAt:                req.EndsAt,
		WithOperator:          req.WithOperator,
		OperatorTermsAccepted: req.OperatorTermsAccepted,
	})
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, quoteToAPI(r, b))
}

// ListMyRentals implements api.ListMyRentals.
func (h *RentalAPI) ListMyRentals(c *gin.Context) {
	tenantID, ok := h.requireSession(c)
	if !ok {
		return
	}
	items, err := h.svc.ListForTenant(c.Request.Context(), tenantID)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	out := make([]api.Rental, len(items))
	for i, item := range items {
		out[i] = rentalToAPI(item)
	}
	c.JSON(http.StatusOK, out)
}

// GetRental implements api.GetRental.
func (h *RentalAPI) GetRental(c *gin.Context, id openapi_types.UUID) {
	tenantID, ok := h.requireSession(c)
	if !ok {
		return
	}
	r, err := h.svc.Get(c.Request.Context(), id.String())
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	if !r.IsTenant(tenantID) && !r.IsOwner(tenantID) {
		h.writeErr(c, http.StatusForbidden, "forbidden", "rental.forbidden")
		return
	}
	c.JSON(http.StatusOK, rentalToAPI(r))
}

// AuthorizeRentalPayment implements api.AuthorizeRentalPayment.
func (h *RentalAPI) AuthorizeRentalPayment(c *gin.Context, id openapi_types.UUID) {
	tenantID, ok := h.requireSession(c)
	if !ok {
		return
	}
	intent, err := h.svc.AuthorizeIntent(c.Request.Context(), rentsvc.AuthorizeIntentInput{
		TenantID: tenantID,
		RentalID: id.String(),
	})
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusOK, intentToAPI(intent))
}

// AcceptRental implements api.AcceptRental.
func (h *RentalAPI) AcceptRental(c *gin.Context, id openapi_types.UUID) {
	ownerID, ok := h.requireSession(c)
	if !ok {
		return
	}
	r, err := h.svc.Accept(c.Request.Context(), rentsvc.AcceptInput{
		OwnerID:  ownerID,
		RentalID: id.String(),
	})
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusOK, rentalToAPI(r))
}

// DeclineRental implements api.DeclineRental.
func (h *RentalAPI) DeclineRental(c *gin.Context, id openapi_types.UUID) {
	ownerID, ok := h.requireSession(c)
	if !ok {
		return
	}
	var body api.DeclineRentalRequest
	_ = c.ShouldBindJSON(&body) // body is optional; we accept empty
	reason := ""
	if body.Reason != nil {
		reason = string(*body.Reason)
	}
	r, err := h.svc.Decline(c.Request.Context(), rentsvc.DeclineInput{
		OwnerID:       ownerID,
		RentalID:      id.String(),
		DeclineReason: reason,
	})
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusOK, rentalToAPI(r))
}

// CancelRental implements api.CancelRental (F3 legacy — pre-authorisation cancel).
func (h *RentalAPI) CancelRental(c *gin.Context, id openapi_types.UUID) {
	tenantID, ok := h.requireSession(c)
	if !ok {
		return
	}
	r, err := h.svc.CancelPreAuth(c.Request.Context(), rentsvc.CancelPreAuthInput{
		TenantID: tenantID,
		RentalID: id.String(),
	})
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusOK, rentalToAPI(r))
}

// CreateRentalCancellation implements api.CreateRentalCancellation (F4).
// POST /rentals/{id}/cancellations — tenant, owner, or platform.
func (h *RentalAPI) CreateRentalCancellation(c *gin.Context, id openapi_types.UUID) {
	callerID, ok := h.requireSession(c)
	if !ok {
		return
	}
	var req api.CreateRentalCancellationRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		h.writeErr(c, http.StatusBadRequest, "invalid_request", "rental.invalid_payload")
		return
	}
	actorKind := rental.ActorKind(string(req.ActorKind))
	res, err := h.svc.Cancel(c.Request.Context(), rentsvc.CancelInput{
		CallerAccountID: callerID,
		RentalID:        id.String(),
		ActorKind:       actorKind,
		Reason:          reasonFromPtr(req.Reason),
	})
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, cancellationToAPI(res.Cancellation, res.Rental))
}

// GetRentalCancellation implements api.GetRentalCancellation (F4).
// GET /rentals/{id}/cancellations — both tenant and owner; 404 when
// no cancellation exists yet.
func (h *RentalAPI) GetRentalCancellation(c *gin.Context, id openapi_types.UUID) {
	callerID, ok := h.requireSession(c)
	if !ok {
		return
	}
	rec, err := h.svc.GetCancellation(c.Request.Context(), id.String(), callerID)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	r, err := h.svc.Get(c.Request.Context(), id.String())
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusOK, cancellationToAPI(rec, r))
}

// reasonFromPtr turns the nullable request reason into a plain string.
func reasonFromPtr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// cancellationToAPI maps the service-layer record + rental to the
// openapi.RentalCancellation shape. ADR-lite #2: the record is
// immutable once written; corrections are new events referencing the
// original.
func cancellationToAPI(rec rentsvc.CancellationRecord, _ rental.Rental) api.RentalCancellation {
	out := api.RentalCancellation{
		Id:                                   toUUID(rec.ID),
		RentalId:                             toUUID(rec.RentalID),
		ActorAccountId:                       toUUID(rec.ActorID),
		ActorKind:                            api.RentalCancellationActorKind(rec.ActorKind),
		WindowApplied:                        string(rec.WindowCode),
		CancellationFeeCents:                 int(rec.CancellationFeeCents),
		TenantRefundCents:                    int(rec.TenantRefundCents),
		OwnerPayoutCentsAfterCancellation:    int(rec.OwnerPayoutCentsAfterCancellation),
		OperatorPayoutCentsAfterCancellation: int(rec.OperatorPayoutCentsAfterCancellation),
		DepositCaptureCents:                  int(rec.DepositCaptureCents),
		DepositReleaseCents:                  int(rec.DepositReleaseCents),
		DepositPartialCaptureCents:           int(rec.DepositPartialCaptureCents),
		StateDeposit:                         api.RentalCancellationStateDeposit(rec.DepositState),
		IssuedAt:                             rec.IssuedAt,
	}
	if rec.ProcessorOperationID != "" {
		v := rec.ProcessorOperationID
		out.ProcessorOperationId = &v
	}
	if rec.ReversalReason != "" {
		v := rec.ReversalReason
		out.ReversalReason = &v
	}
	return out
}

// GetRentalReceipt implements api.GetRentalReceipt.
func (h *RentalAPI) GetRentalReceipt(c *gin.Context, id openapi_types.UUID) {
	tenantID, ok := h.requireSession(c)
	if !ok {
		return
	}
	rec, err := h.svc.GetReceipt(c.Request.Context(), id.String(), tenantID)
	if err != nil {
		h.writeServiceErr(c, err)
		return
	}
	c.JSON(http.StatusOK, receiptToAPI(rec))
}
