package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewToken_HashesAndDiffers(t *testing.T) {
	t.Parallel()
	a, ha, err := NewToken()
	require.NoError(t, err)
	b, hb, err := NewToken()
	require.NoError(t, err)
	require.NotEqual(t, a, b)
	require.Equal(t, HashToken(a), ha)
	require.NotEqual(t, ha, hb)
	require.Len(t, ha, 64)
}

func TestRawCookie_Missing(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := rawCookie(req, CookieSettings{})
	require.ErrorIs(t, err, ErrNoSession)
}

func TestCookieSecureFor_HTTPSForwarded(t *testing.T) {
	t.Parallel()
	cfg := CookieSettings{}
	require.False(t, cfg.secureFor(nil))
	httpReq := httptest.NewRequest(http.MethodGet, "/", nil)
	require.False(t, cfg.secureFor(httpReq))
	httpsReq := httptest.NewRequest(http.MethodGet, "/", nil)
	httpsReq.Header.Set("X-Forwarded-Proto", "https")
	require.True(t, cfg.secureFor(httpsReq))
	forced := CookieSettings{Secure: true}
	require.True(t, forced.secureFor(httpReq))
}

func TestSetSessionCookie_SecureOnPublicHost(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	setSessionCookie(w, CookieSettings{Name: "canteiro_session", TTL: time.Hour}, "tok", req)
	got := cookieByName(t, w, "canteiro_session")
	require.True(t, got.Secure)
	require.True(t, got.HttpOnly)
	require.Equal(t, "/", got.Path)
	require.Equal(t, http.SameSiteLaxMode, got.SameSite)
}

func cookieByName(t *testing.T, w *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("missing cookie %s", name)
	return nil
}
