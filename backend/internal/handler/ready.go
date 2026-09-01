package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/brenonaraujo/canteiro/backend/internal/api"
	"github.com/brenonaraujo/canteiro/backend/internal/i18n"
	"github.com/brenonaraujo/canteiro/backend/internal/repository"
)

const readyTimeout = 2 * time.Second

// Readyz is readiness: 200 if every checker passes, 503 otherwise.
// Checker errors are not echoed (no DSN / PII).
func (s *Server) Readyz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), readyTimeout)
	defer cancel()
	status, key, code := evaluateReady(ctx, s.Checkers)
	c.JSON(code, api.ReadyResponse{
		Status:     api.ReadyResponseStatus(status),
		Message:    i18n.T(c.Request.Context(), key),
		MessageKey: key,
		Service:    &s.Service,
	})
}

func evaluateReady(ctx context.Context, checkers []repository.Checker) (string, string, int) {
	for _, ch := range checkers {
		if err := ch.Check(ctx); err != nil {
			return "not_ready", "ready.not_ready", http.StatusServiceUnavailable
		}
	}
	return "ready", "ready.ok", http.StatusOK
}
