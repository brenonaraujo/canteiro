package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/i18n"
)

func TestGoogleCallback_InvalidStateAndBadCode(t *testing.T) {
	require.NoError(t, loadI18n(t))
	a, st := testAPI()
	r := routerFor(a)
	badState := do(r, httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=ok-code&state=nope", nil))
	require.Equal(t, http.StatusFound, badState.Code)
	assert.Contains(t, badState.Header().Get("Location"), "auth=error")

	state, err := st.Issue()
	require.NoError(t, err)
	badCode := do(r, httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=nope&state="+url.QueryEscape(state), nil))
	require.Equal(t, http.StatusFound, badCode.Code)
	assert.Contains(t, badCode.Header().Get("Location"), "auth=error")
}

func TestLogout_Unauthorized(t *testing.T) {
	require.NoError(t, loadI18n(t))
	r := routerFor(mustAPI())
	w := do(r, httptest.NewRequest(http.MethodPost, "/auth/logout", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRedirect_EmptyWebApp(t *testing.T) {
	require.NoError(t, loadI18n(t))
	a, _ := testAPI()
	a.deps.WebAppURL = ""
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	a.redirect(c, "ok")
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/?auth=ok", w.Header().Get("Location"))
}

func TestRedirect_SameHostStaysRelative(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "https://canteiro.brenon.cloud/auth/google/callback", nil)
	got := postAuthLocation("https://canteiro.brenon.cloud", req, "ok")
	require.Equal(t, "/?auth=ok", got)
}

func TestRedirect_DifferentHostUsesWebApp(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/auth/google/callback", nil)
	got := postAuthLocation("http://localhost:3000", req, "denied")
	require.Equal(t, "http://localhost:3000/?auth=denied", got)
}

func TestWriteAccountErr_Internal(t *testing.T) {
	require.NoError(t, loadI18n(t))
	a, _ := testAPI()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request = c.Request.WithContext(i18n.WithLocale(c.Request.Context(), "en"))
	require.True(t, a.writeAccountErr(c, errors.New("db")))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGoogle_DefaultAuthURLAndHTTPClient(t *testing.T) {
	t.Parallel()
	tok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(tok.Close)
	g := &Google{
		ClientID: "cid", ClientSecret: "sec", RedirectURL: "http://localhost/cb",
		TokenURL: tok.URL,
	}
	assert.Contains(t, g.AuthCodeURL("s"), "accounts.google.com")
	_, err := g.Exchange(context.Background(), "x")
	require.Error(t, err)
}

func TestGoogle_TokenInfoRejectsEmptySub(t *testing.T) {
	t.Parallel()
	info := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"aud": "cid", "sub": ""})
	}))
	t.Cleanup(info.Close)
	tok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": "a.b.c"})
	}))
	t.Cleanup(tok.Close)
	g := &Google{
		ClientID: "cid", ClientSecret: "sec", RedirectURL: "http://localhost/cb",
		TokenURL: tok.URL, InfoURL: info.URL, HTTP: tok.Client(),
	}
	_, err := g.Exchange(context.Background(), "x")
	require.Error(t, err)
}

func TestCookieTTLDefault(t *testing.T) {
	t.Parallel()
	require.Equal(t, int64((7*24)*3600), int64(CookieSettings{}.ttl().Seconds()))
}

func TestRowTableNames(t *testing.T) {
	t.Parallel()
	require.Equal(t, "accounts", accountRow{}.TableName())
	require.Equal(t, "sessions", sessionRow{}.TableName())
}

func TestIsUnique_DuplicateKeyString(t *testing.T) {
	t.Parallel()
	require.True(t, isUnique(errors.New("ERROR: duplicate key value violates unique constraint")))
}

func TestMapAccount_OtherError(t *testing.T) {
	t.Parallel()
	_, err := mapAccount(accountRow{}, errors.New("dial"))
	require.Error(t, err)
}
