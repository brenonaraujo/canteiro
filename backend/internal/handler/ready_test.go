package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/i18n"
	"github.com/brenonaraujo/canteiro/backend/internal/repository"
)

type stubChecker struct {
	err error
}

func (s stubChecker) Name() string { return "db" }

func (s stubChecker) Check(context.Context) error { return s.err }

func TestReadyz_OK_EN(t *testing.T) {
	_, err := i18n.Load()
	require.NoError(t, err)
	w := hitReady(t, stubChecker{}, "en")
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ready", body["status"])
	assert.Equal(t, "ready.ok", body["message_key"])
	assert.Equal(t, "Service is ready", body["message"])
}

func TestReadyz_NotReady_PTBR(t *testing.T) {
	_, err := i18n.Load()
	require.NoError(t, err)
	w := hitReady(t, stubChecker{err: errors.New("dial tcp")}, "pt-BR")
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "not_ready")
	assert.Contains(t, w.Body.String(), "Serviço não está pronto")
	assert.NotContains(t, w.Body.String(), "dial tcp")
}

func hitReady(t *testing.T, stub stubChecker, lang string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	s := NewServer("canteiro", []repository.Checker{stub})
	r := gin.New()
	r.GET("/readyz", func(c *gin.Context) {
		c.Request = c.Request.WithContext(i18n.WithLocale(c.Request.Context(), lang))
		s.Readyz(c)
	})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
