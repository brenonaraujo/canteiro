package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartGoogleAuth_NotConfigured(t *testing.T) {
	require.NoError(t, loadI18n(t))
	r := routerFor(&API{})
	w := do(r, httptest.NewRequest(http.MethodGet, "/auth/google", nil))
	assertNotConfigured503(t, w)
}

func TestGoogleFlow_ProfileLogoutDeactivate(t *testing.T) {
	require.NoError(t, loadI18n(t))
	a, st := testAPI()
	r := routerFor(a)

	w := do(r, httptest.NewRequest(http.MethodGet, "/auth/google", nil))
	require.Equal(t, http.StatusFound, w.Code)
	loc := w.Header().Get("Location")
	require.Contains(t, loc, "accounts.google.com")
	u, err := url.Parse(loc)
	require.NoError(t, err)
	state := u.Query().Get("state")
	require.NoError(t, st.Verify(state))

	denied := do(r, httptest.NewRequest(http.MethodGet, "/auth/google/callback?error=access_denied", nil))
	require.Equal(t, http.StatusFound, denied.Code)
	assert.Contains(t, denied.Header().Get("Location"), "auth=denied")

	cb := "/auth/google/callback?code=ok-code&state=" + url.QueryEscape(state)
	login := do(r, httptest.NewRequest(http.MethodGet, cb, nil))
	require.Equal(t, http.StatusFound, login.Code)
	assert.Contains(t, login.Header().Get("Location"), "auth=ok")
	cookie := cookieValue(t, login, "canteiro_session")
	assertAccountLifecycle(t, r, st, cookie)
}

func assertAccountLifecycle(t *testing.T, r http.Handler, st *State, cookie string) {
	t.Helper()
	me := accountReq(r, http.MethodGet, "/account", cookie, "")
	require.Equal(t, http.StatusOK, me.Code)
	body := asMap(t, me)
	assert.Equal(t, "incomplete", body["status"])
	assert.Equal(t, false, body["capabilities"].(map[string]any)["reserve"])
	assert.Equal(t, false, body["capabilities"].(map[string]any)["publish"])
	assert.NotContains(t, me.Body.String(), "google-sub-1")
	id := body["id"].(string)

	bad := accountReq(r, http.MethodPatch, "/account", cookie, `{"display_name":"","phone":"1199"}`)
	require.Equal(t, http.StatusUnprocessableEntity, bad.Code)
	okp := accountReq(r, http.MethodPatch, "/account", cookie, `{"display_name":"Ana","phone":"1199"}`)
	require.Equal(t, http.StatusOK, okp.Code)
	got := asMap(t, okp)
	assert.Equal(t, "active", got["status"])
	assert.Equal(t, true, got["capabilities"].(map[string]any)["reserve"])
	assert.Equal(t, false, got["capabilities"].(map[string]any)["publish"])

	state2, err := st.Issue()
	require.NoError(t, err)
	login2 := do(r, httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=ok-code&state="+url.QueryEscape(state2), nil))
	cookie2 := cookieValue(t, login2, "canteiro_session")
	assert.Equal(t, id, asMap(t, accountReq(r, http.MethodGet, "/account", cookie2, ""))["id"])

	out := accountReq(r, http.MethodPost, "/auth/logout", cookie2, "")
	require.Equal(t, http.StatusNoContent, out.Code)
	require.Equal(t, http.StatusUnauthorized, accountReq(r, http.MethodGet, "/account", cookie2, "").Code)
	assertDeactivate(t, r, st)
}

func assertDeactivate(t *testing.T, r http.Handler, st *State) {
	t.Helper()
	state3, err := st.Issue()
	require.NoError(t, err)
	login3 := do(r, httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=ok-code&state="+url.QueryEscape(state3), nil))
	cookie3 := cookieValue(t, login3, "canteiro_session")
	noconf := accountReq(r, http.MethodPost, "/account/deactivate", cookie3, `{"confirm":false}`)
	require.Equal(t, http.StatusUnprocessableEntity, noconf.Code)
	dead := accountReq(r, http.MethodPost, "/account/deactivate", cookie3, `{"confirm":true}`)
	require.Equal(t, http.StatusOK, dead.Code)
	assert.Equal(t, "deactivated", asMap(t, dead)["status"])
	patch := accountReq(r, http.MethodPatch, "/account", cookie3, `{"display_name":"Bia","phone":"1188"}`)
	require.Equal(t, http.StatusForbidden, patch.Code)
}

func TestGetAccount_Unauthorized(t *testing.T) {
	require.NoError(t, loadI18n(t))
	r := routerFor(mustAPI())
	w := do(r, httptest.NewRequest(http.MethodGet, "/account", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code)
}
