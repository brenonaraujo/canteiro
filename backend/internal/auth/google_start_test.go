package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStartGoogleAuth_NotConfigured_ProviderAbsent(t *testing.T) {
	require.NoError(t, loadI18n(t))
	a, _ := testAPI()
	a.deps.Google = nil
	w := do(routerFor(a), httptest.NewRequest(http.MethodGet, "/auth/google", nil))
	assertNotConfigured503(t, w)
}

func TestStartGoogleAuth_NotConfigured_StateNotReady(t *testing.T) {
	require.NoError(t, loadI18n(t))
	a, _ := testAPI()
	a.deps.State = NewState("short")
	w := do(routerFor(a), httptest.NewRequest(http.MethodGet, "/auth/google", nil))
	assertNotConfigured503(t, w)
}

func TestStartGoogleAuth_RedirectsWhenConfigured(t *testing.T) {
	require.NoError(t, loadI18n(t))
	a, _ := testAPI()
	a.deps.Google = &Google{
		ClientID:     "cid",
		ClientSecret: "super-secret-value",
		RedirectURL:  "http://localhost/auth/google/callback",
	}
	w := do(routerFor(a), httptest.NewRequest(http.MethodGet, "/auth/google", nil))
	require.Equal(t, http.StatusFound, w.Code)
	loc := w.Header().Get("Location")
	require.Contains(t, loc, "accounts.google.com")
	require.Contains(t, loc, "client_id=cid")
	require.NotContains(t, loc, "super-secret-value")
	require.NotContains(t, w.Body.String(), "not_configured")
}
