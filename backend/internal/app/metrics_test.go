package app

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRegisterAppInfo(t *testing.T) {
	t.Parallel()
	RegisterAppInfo("dev", "abc123")
	assert.NotNil(t, HTTPRequestsTotal)
	assert.NotNil(t, HTTPRequestDuration)
	assert.NotNil(t, DBQueriesTotal)
	assert.NotNil(t, DBQueryDuration)
	assert.NotNil(t, AppInfo)
}

func TestRouterHelpers(t *testing.T) {
	t.Parallel()
	assert.Equal(t, gin.ReleaseMode, ginMode(""))
	assert.Equal(t, gin.DebugMode, ginMode(gin.DebugMode))
	assert.Equal(t, "canteiro", serviceName(""))
	assert.Equal(t, "api", serviceName("api"))
	assert.Equal(t, "/metrics", metricsPath(""))
	assert.Equal(t, "/m", metricsPath("/m"))
}
