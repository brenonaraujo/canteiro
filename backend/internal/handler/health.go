package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/brenonaraujo/canteiro/backend/internal/api"
)

// Healthz is liveness: always 200, no backing-service checks.
func (s *Server) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, api.HealthResponse{
		Status:  "ok",
		Service: &s.Service,
	})
}
