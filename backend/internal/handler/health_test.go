package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthz_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	s := NewServer("canteiro", nil)
	r.GET("/healthz", s.Healthz)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
	assert.Equal(t, "canteiro", body["service"])
}

func TestNewServer_DefaultName(t *testing.T) {
	t.Parallel()
	s := NewServer("", nil)
	assert.Equal(t, "canteiro", s.Service)
}
