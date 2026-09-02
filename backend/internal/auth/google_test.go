package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoogle_AuthCodeURL(t *testing.T) {
	t.Parallel()
	g := &Google{
		ClientID:     "cid",
		ClientSecret: "sec",
		RedirectURL:  "http://localhost:8080/auth/google/callback",
		AuthURL:      "https://example.test/auth",
	}
	u := g.AuthCodeURL("abc")
	assert.Contains(t, u, "client_id=cid")
	assert.Contains(t, u, "scope=openid")
	assert.NotContains(t, u, "sec")
}

func TestGoogle_Exchange_VerifiesAudAndSub(t *testing.T) {
	t.Parallel()
	info := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"aud": "cid", "sub": "sub-9"})
	}))
	t.Cleanup(info.Close)
	tok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		assert.Equal(t, "good", r.Form.Get("code"))
		assert.Equal(t, "sec", r.Form.Get("client_secret"))
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": "header.payload.sig"})
	}))
	t.Cleanup(tok.Close)
	g := &Google{
		ClientID: "cid", ClientSecret: "sec", RedirectURL: "http://localhost/cb",
		TokenURL: tok.URL, InfoURL: info.URL, HTTP: info.Client(),
	}
	id, err := g.Exchange(t.Context(), "good")
	require.NoError(t, err)
	assert.Equal(t, "sub-9", id.Subject)
}

func TestGoogle_Exchange_RejectsBadAud(t *testing.T) {
	t.Parallel()
	info := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"aud": "other", "sub": "sub-9"})
	}))
	t.Cleanup(info.Close)
	tok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": "x.y.z"})
	}))
	t.Cleanup(tok.Close)
	g := &Google{
		ClientID: "cid", ClientSecret: "sec", RedirectURL: "http://localhost/cb",
		TokenURL: tok.URL, InfoURL: info.URL, HTTP: tok.Client(),
	}
	_, err := g.Exchange(t.Context(), "good")
	require.Error(t, err)
}

func TestGoogle_Exchange_EmptyCode(t *testing.T) {
	t.Parallel()
	g := &Google{ClientID: "c", ClientSecret: "s", RedirectURL: "http://x"}
	_, err := g.Exchange(t.Context(), "")
	require.Error(t, err)
}
