package payment

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	rental "github.com/brenonaraujo/canteiro/backend/internal/rental"
)

func TestVerifyStripeSignature_RejectsMissingParts(t *testing.T) {
	t.Parallel()
	require.False(t, verifyStripeSignature("whsec_test", []byte(`{"id":"evt_1"}`), ""))
	require.False(t, verifyStripeSignature("whsec_test", []byte(`{"id":"evt_1"}`), "t=12345"))
	require.False(t, verifyStripeSignature("whsec_test", []byte(`{"id":"evt_1"}`), "v1=deadbeef"))
}

func TestVerifyStripeSignature_AcceptsValidHMAC(t *testing.T) {
	t.Parallel()
	secret := "whsec_test"
	ts := "1700000000"
	body := []byte(`{"id":"evt_1"}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	v1 := hex.EncodeToString(mac.Sum(nil))
	header := "t=" + ts + ",v1=" + v1
	require.True(t, verifyStripeSignature(secret, body, header))
}

func TestVerifyStripeSignature_RejectsTamperedBody(t *testing.T) {
	t.Parallel()
	secret := "whsec_test"
	ts := "1700000000"
	body := []byte(`{"id":"evt_1"}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	v1 := hex.EncodeToString(mac.Sum(nil))
	header := "t=" + ts + ",v1=" + v1
	tampered := []byte(`{"id":"evt_other"}`)
	require.False(t, verifyStripeSignature(secret, tampered, header))
}

func TestVerifyStripeSignature_RejectsEmptySecret(t *testing.T) {
	t.Parallel()
	require.False(t, verifyStripeSignature("", []byte("body"), "t=1,v1=anything"))
}

func TestNoop_CreateIntentIsIdempotent(t *testing.T) {
	t.Parallel()
	n := NewNoop()
	req := rental.CreateIntentRequest{IdempotencyKey: "key-1"}
	r1, err := n.CreateIntent(nil, req)
	require.NoError(t, err)
	r2, err := n.CreateIntent(nil, req)
	require.NoError(t, err)
	require.Equal(t, r1.ProviderPaymentID, r2.ProviderPaymentID)
}
