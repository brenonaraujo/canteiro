package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
