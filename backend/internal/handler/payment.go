package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/brenonaraujo/canteiro/backend/internal/api"
	"github.com/brenonaraujo/canteiro/backend/internal/domain/rental"
	"github.com/brenonaraujo/canteiro/backend/internal/i18n"
	rentsvc "github.com/brenonaraujo/canteiro/backend/internal/rental"
)

// PaymentAPI wires the F3 payment webhook. The endpoint is intentionally
// unauthenticated — the PSP signature is the auth boundary. The handler
// is idempotent on (provider, provider_event_id) and on rental state.
type PaymentAPI struct {
	svc    RentalWebhookService
	verify RentalSignatureVerifier
}

// RentalWebhookService is the slice of rental.Service the handler needs.
type RentalWebhookService interface {
	HandleWebhookEvent(ctx context.Context, ev rentsvc.ProviderWebhookEvent) error
}

// RentalSignatureVerifier is the slice of payment.Stripe the handler needs.
type RentalSignatureVerifier interface {
	VerifyWebhookSignature(ctx context.Context, rawBody []byte, signature string) (rentsvc.ProviderWebhookEvent, error)
}

// NewPaymentAPI builds the adapter.
func NewPaymentAPI(svc RentalWebhookService, verify RentalSignatureVerifier) *PaymentAPI {
	return &PaymentAPI{svc: svc, verify: verify}
}

// PaymentWebhook implements api.PaymentWebhook.
func (h *PaymentAPI) PaymentWebhook(c *gin.Context, params api.PaymentWebhookParams) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.writeErr(c, http.StatusBadRequest, "invalid_body", "payment.invalid_body")
		return
	}
	var ev api.PaymentWebhookEvent
	if err = json.Unmarshal(body, &ev); err != nil {
		h.writeErr(c, http.StatusBadRequest, "invalid_json", "payment.invalid_json")
		return
	}
	sig := ""
	if params.StripeSignature != nil {
		sig = *params.StripeSignature
	}
	verified, err := h.verify.VerifyWebhookSignature(c.Request.Context(), body, sig)
	if err != nil {
		h.writeErr(c, http.StatusUnauthorized, "signature_invalid", "payment.signature_invalid")
		return
	}
	verified.ProviderEventID = ev.Id
	verified.EventType = string(ev.Type)
	if ev.Data.RentalId.String() != "00000000-0000-0000-0000-000000000000" {
		verified.RentalID = ev.Data.RentalId.String()
	}
	if ev.Data.PaymentIntentId != nil {
		verified.PaymentIntentID = ev.Data.PaymentIntentId.String()
	}
	verified.AmountCents = int64(ev.Data.AmountCents)
	if ev.Data.DepositCents != nil {
		verified.DepositCents = int64(*ev.Data.DepositCents)
	}
	if ev.Data.FailureCode != nil {
		verified.FailureCode = *ev.Data.FailureCode
	}
	if ev.Data.FailureMessage != nil {
		verified.FailureMessage = *ev.Data.FailureMessage
	}
	if err := h.svc.HandleWebhookEvent(c.Request.Context(), verified); err != nil {
		h.writeWebhookErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"received": true})
}

func (h *PaymentAPI) writeErr(c *gin.Context, status int, code, key string) {
	c.JSON(status, api.Error{Code: code, Message: i18n.T(c.Request.Context(), key), MessageKey: key})
}

func (h *PaymentAPI) writeWebhookErr(c *gin.Context, err error) {
	if errors.Is(err, rental.ErrPaymentTotalMismatch) {
		h.writeErr(c, http.StatusUnprocessableEntity, "payment_mismatch", "payment.mismatch")
		return
	}
	if errors.Is(err, rental.ErrNotFound) {
		h.writeErr(c, http.StatusNotFound, "not_found", "rental.not_found")
		return
	}
	h.writeErr(c, http.StatusInternalServerError, "internal_error", "error.internal")
}
