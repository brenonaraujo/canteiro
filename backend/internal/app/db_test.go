package app

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewDB_InvalidDSNFails(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := NewDB(ctx, "postgres://app:app@127.0.0.1:1/app?sslmode=disable&connect_timeout=1")
	require.Error(t, err)
}

func TestNewDB_EmptyDSNFails(t *testing.T) {
	t.Parallel()
	_, err := NewDB(context.Background(), "")
	require.Error(t, err)
}
