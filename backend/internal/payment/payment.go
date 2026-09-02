package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/brenonaraujo/canteiro/backend/internal/rental"
)

// Noop is the in-memory provider.
type Noop struct {
	intents map[string]rental.CreateIntentResponse
	mu      sync.Mutex
}

// NewNoop builds a fresh noop provider.
func NewNoop() *Noop { return &Noop{intents: map[string]rental.CreateIntentResponse{}} }

// CreateIntent returns a deterministic response keyed by IdempotencyKey.
func (n *Noop) CreateIntent(_ context.Context, req rental.CreateIntentRequest) (rental.CreateIntentResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if existing, ok := n.intents[req.IdempotencyKey]; ok {
		return existing, nil
	}
	resp := rental.CreateIntentResponse{
		Provider:          "noop",
		ProviderPaymentID: "pi_" + sanitizeKey(req.IdempotencyKey),
		Status:            "requires_action",
	}
	n.intents[req.IdempotencyKey] = resp
	return resp, nil
}

// VerifyWebhookSignature accepts every signature in the noop provider.
func (n *Noop) VerifyWebhookSignature(_ context.Context, _ []byte, _ string) (rental.ProviderWebhookEvent, error) {
	return rental.ProviderWebhookEvent{}, nil
}

// TriggerAuthorized is a test helper.
func (n *Noop) TriggerAuthorized(rentalID string, amountCents, depositCents int64) rental.ProviderWebhookEvent {
	return rental.ProviderWebhookEvent{
		Provider:        "noop",
		ProviderEventID: "evt_noop_" + rentalID + "_" + time.Now().UTC().Format(time.RFC3339Nano),
		EventType:       "payment.authorized",
		RentalID:        rentalID,
		AmountCents:     amountCents,
		DepositCents:    depositCents,
		Authorized:      true,
	}
}

// TriggerFailed is a test helper.
func (n *Noop) TriggerFailed(rentalID, code, msg string) rental.ProviderWebhookEvent {
	return rental.ProviderWebhookEvent{
		Provider:        "noop",
		ProviderEventID: "evt_noop_fail_" + rentalID + "_" + time.Now().UTC().Format(time.RFC3339Nano),
		EventType:       "payment.failed",
		RentalID:        rentalID,
		FailureCode:     code,
		FailureMessage:  msg,
	}
}

// sanitizeKey trims invalid characters.
func sanitizeKey(k string) string {
	out := strings.Builder{}
	for _, c := range k {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
			out.WriteRune(c)
		default:
			out.WriteRune('_')
		}
	}
	return out.String()
}

// Stripe is a stub Stripe provider.
type Stripe struct {
	WebhookSecret string
	APIKey        string
}

// NewStripe builds the Stripe provider.
func NewStripe(apiKey, webhookSecret string) *Stripe {
	return &Stripe{APIKey: apiKey, WebhookSecret: webhookSecret}
}

// ErrNotConfigured is returned when Stripe is built without secrets.
var ErrNotConfigured = errors.New("payment: stripe provider not configured")

// CreateIntent is the stub.
func (s *Stripe) CreateIntent(_ context.Context, req rental.CreateIntentRequest) (rental.CreateIntentResponse, error) {
	if s.APIKey == "" {
		return rental.CreateIntentResponse{}, ErrNotConfigured
	}
	return rental.CreateIntentResponse{
		Provider:          "stripe",
		ProviderPaymentID: "pi_stub_" + sanitizeKey(req.IdempotencyKey),
		Status:            "requires_action",
	}, nil
}

// VerifyWebhookSignature verifies a Stripe-Signature header using HMAC-SHA256.
func (s *Stripe) VerifyWebhookSignature(_ context.Context, rawBody []byte, signature string) (rental.ProviderWebhookEvent, error) {
	if s.WebhookSecret == "" {
		return rental.ProviderWebhookEvent{}, ErrNotConfigured
	}
	if !verifyStripeSignature(s.WebhookSecret, rawBody, signature) {
		return rental.ProviderWebhookEvent{}, errors.New("payment: signature mismatch")
	}
	return rental.ProviderWebhookEvent{}, nil
}

func verifyStripeSignature(secret string, body []byte, header string) bool {
	parts := strings.Split(header, ",")
	var ts, v1 string
	for _, p := range parts {
		if strings.HasPrefix(p, "t=") {
			ts = strings.TrimPrefix(p, "t=")
		}
		if strings.HasPrefix(p, "v1=") {
			v1 = strings.TrimPrefix(p, "v1=")
		}
	}
	if ts == "" || v1 == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(v1))
}
