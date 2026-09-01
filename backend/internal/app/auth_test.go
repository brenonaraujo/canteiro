package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewAuthAPI_NilConfig(t *testing.T) {
	t.Parallel()
	require.NotNil(t, NewAuthAPI(nil, nil))
}

func TestNewAuthAPI_WithoutProvider(t *testing.T) {
	t.Parallel()
	got := NewAuthAPI(&Config{
		SessionSecret: "0123456789abcdef0123456789abcdef",
		WebAppURL:     "http://localhost:3000",
	}, nil)
	require.NotNil(t, got)
}

func TestNewAuthAPI_WithProvider(t *testing.T) {
	t.Parallel()
	got := NewAuthAPI(&Config{
		GoogleClientID:     "cid",
		GoogleClientSecret: "csec",
		GoogleRedirectURL:  "http://localhost:8080/auth/google/callback",
		SessionSecret:      "0123456789abcdef0123456789abcdef",
		WebAppURL:          "http://localhost:3000",
	}, nil)
	require.NotNil(t, got)
}
