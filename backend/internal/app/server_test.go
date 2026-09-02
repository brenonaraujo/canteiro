package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/i18n"
	"github.com/brenonaraujo/canteiro/backend/internal/repository"
)

func perform(r http.Handler, method, path, accept string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if accept != "" {
		req.Header.Set("Accept-Language", accept)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestNewRouter_Healthz(t *testing.T) {
	require.NoError(t, loadI18n(t))
	r := NewRouter(ServerOpts{ServiceName: "canteiro"})
	w := perform(r, http.MethodGet, "/healthz", "")
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
	assert.Equal(t, "canteiro", body["service"])
}

func TestNewRouter_ReadyzOK(t *testing.T) {
	require.NoError(t, loadI18n(t))
	r := NewRouter(ServerOpts{
		ServiceName: "canteiro",
		Checkers:    []repository.Checker{stubChecker{ok: true}},
	})
	w := perform(r, http.MethodGet, "/readyz", "en")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"ready"`)
	assert.Contains(t, w.Body.String(), "Service is ready")
}

func TestNewRouter_ReadyzFailHasNoDSN(t *testing.T) {
	require.NoError(t, loadI18n(t))
	r := NewRouter(ServerOpts{
		Checkers: []repository.Checker{stubChecker{ok: false}},
	})
	w := perform(r, http.MethodGet, "/readyz", "pt-BR")
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"not_ready"`)
	assert.NotContains(t, strings.ToLower(w.Body.String()), "postgres://")
}

func TestNewRouter_Metrics(t *testing.T) {
	r := NewRouter(ServerOpts{})
	w := perform(r, http.MethodGet, "/metrics", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "http_requests_total")
}

func TestNewRouter_AuthGoogleUnconfigured(t *testing.T) {
	require.NoError(t, loadI18n(t))
	r := NewRouter(ServerOpts{ServiceName: "canteiro"})
	w := perform(r, http.MethodGet, "/auth/google", "pt-BR")
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "auth.not_configured")
}

func TestNewRouter_CORSPreflight(t *testing.T) {
	r := NewRouter(ServerOpts{CORSOrigin: "http://localhost:3000"})
	req := httptest.NewRequest(http.MethodOptions, "/account", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
}

func loadI18n(t *testing.T) error {
	t.Helper()
	_, err := i18n.Load()
	return err
}

type stubChecker struct{ ok bool }

func (s stubChecker) Name() string { return "db" }

func (s stubChecker) Check(_ context.Context) error {
	if s.ok {
		return nil
	}
	return errors.New("db down")
}
