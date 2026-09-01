package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDBChecker_NilDB(t *testing.T) {
	t.Parallel()
	c := DBChecker{}
	require.Equal(t, "db", c.Name())
	require.Error(t, c.Check(context.Background()))
}
