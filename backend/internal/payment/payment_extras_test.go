package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	rental "github.com/brenonaraujo/canteiro/backend/internal/rental"
)

func TestNoop_VerifyWebhookSignature_AlwaysAccepts(t *testing.T) {
	t.Parallel()
	n := NewNoop()
	got, err := n.VerifyWebhookSignature(context.TODO(), []byte("any"), "any")
	require.NoError(t, err)
	require.Equal(t, rental.ProviderWebhookEvent{}, got)
}

func TestNoop_TriggerAuthorized_Shape(t *testing.T) {
	t.Parallel()
	n := NewNoop()
	ev := n.TriggerAuthorized("rental-x", 1000, 500)
	require.Equal(t, "noop", ev.Provider)
	require.Equal(t, "rental-x", ev.RentalID)
	require.Equal(t, int64(1000), ev.AmountCents)
	require.Equal(t, int64(500), ev.DepositCents)
	require.True(t, ev.Authorized)
	require.Equal(t, "payment.authorized", ev.EventType)
	require.Contains(t, ev.ProviderEventID, "rental-x")
}

func TestNoop_TriggerFailed_Shape(t *testing.T) {
	t.Parallel()
	n := NewNoop()
	ev := n.TriggerFailed("rental-y", "card_declined", "Bem-vindo de volta, cliente.")
	require.Equal(t, "noop", ev.Provider)
	require.Equal(t, "rental-y", ev.RentalID)
	require.Equal(t, "card_declined", ev.FailureCode)
	require.Equal(t, "Bem-vindo de volta, cliente.", ev.FailureMessage)
	require.Equal(t, "payment.failed", ev.EventType)
	require.False(t, ev.Authorized)
}

func TestSanitizeKey_KeepsAllowedChars(t *testing.T) {
	t.Parallel()
	got := sanitizeKey("aB1-_-x")
	require.Equal(t, "aB1-_-x", got)
}

func TestSanitizeKey_ReplacesSpecialChars(t *testing.T) {
	t.Parallel()
	got := sanitizeKey("a/b c.d")
	require.Equal(t, "a_b_c_d", got)
}

func TestSanitizeKey_HandlesUnicodeAndPunctuation(t *testing.T) {
	t.Parallel()
	// Each non-ASCII rune (é, á) and each punctuation char (/,!,?) collapses
	// to a single underscore, so we expect 5 underscores total here.
	got := sanitizeKey("café/olá!?")
	require.Equal(t, "caf__ol___", got)
}

func TestSanitizeKey_Empty(t *testing.T) {
	t.Parallel()
	require.Empty(t, sanitizeKey(""))
}

func TestStripe_NewStripe_StoresFields(t *testing.T) {
	t.Parallel()
	s := NewStripe("sk_test", "whsec_test")
	require.Equal(t, "sk_test", s.APIKey)
	require.Equal(t, "whsec_test", s.WebhookSecret)
}

func TestStripe_CreateIntent_RequiresAPIKey(t *testing.T) {
	t.Parallel()
	s := &Stripe{}
	_, err := s.CreateIntent(context.TODO(), rental.CreateIntentRequest{IdempotencyKey: "k"})
	require.ErrorIs(t, err, ErrNotConfigured)
}

func TestStripe_CreateIntent_ReturnsRequiresAction(t *testing.T) {
	t.Parallel()
	s := NewStripe("sk_test", "whsec_test")
	resp, err := s.CreateIntent(context.TODO(), rental.CreateIntentRequest{IdempotencyKey: "ik-1"})
	require.NoError(t, err)
	require.Equal(t, "stripe", resp.Provider)
	require.Equal(t, "requires_action", resp.Status)
	require.Contains(t, resp.ProviderPaymentID, "pi_stub_ik-1")
}

func TestStripe_VerifyWebhookSignature_RequiresSecret(t *testing.T) {
	t.Parallel()
	s := &Stripe{APIKey: "sk_test"}
	_, err := s.VerifyWebhookSignature(context.TODO(), []byte("body"), "t=1,v1=deadbeef")
	require.ErrorIs(t, err, ErrNotConfigured)
}

func TestStripe_VerifyWebhookSignature_RejectsBadSignature(t *testing.T) {
	t.Parallel()
	s := NewStripe("sk_test", "whsec_test")
	_, err := s.VerifyWebhookSignature(context.TODO(), []byte("body"), "t=1,v1=deadbeef")
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrNotConfigured)
}

func TestStripe_VerifyWebhookSignature_AcceptsWellFormed(t *testing.T) {
	t.Parallel()
	secret := "whsec_test"
	ts := "1700000000"
	body := []byte(`{"id":"evt_abc"}`)
	mac := hmacForTest(secret, ts, body)
	s := NewStripe("sk_test", secret)
	_, err := s.VerifyWebhookSignature(context.TODO(), body, "t="+ts+",v1="+mac)
	require.NoError(t, err)
}

func hmacForTest(secret, ts string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
