package app

import (
	"log/slog"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/i18n"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

func TestLocaleMiddleware_SetsPTBRDefault(t *testing.T) {
	require.NoError(t, loadI18n(t))
	r := gin.New()
	r.Use(localeMiddleware())
	r.GET("/x", func(c *gin.Context) {
		assert.Equal(t, "Serviço está no ar", i18n.T(c.Request.Context(), "health.ok"))
		c.Status(http.StatusNoContent)
	})
	w := perform(r, http.MethodGet, "/x", "")
	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestMetricsMiddleware_Records(t *testing.T) {
	r := gin.New()
	r.Use(metricsMiddleware())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := perform(r, http.MethodGet, "/x", "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestLogMiddleware_NilLogger(t *testing.T) {
	r := gin.New()
	r.Use(requestLogMiddleware(nil))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := perform(r, http.MethodGet, "/x", "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestLogMiddleware_WithLogger(t *testing.T) {
	r := gin.New()
	r.Use(requestLogMiddleware(slog.Default()))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := perform(r, http.MethodGet, "/x", "")
	assert.Equal(t, http.StatusOK, w.Code)
}
