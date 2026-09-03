package f5pg

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// B1 RED-first: assert that defaultJSON returns "{}" for nil/empty bytes
// (the schema DEFAULT that GORM fails to apply for `[]byte`), and that a
// non-empty payload is passed through untouched.
func TestDefaultJSON_NilReturnsEmptyObject(t *testing.T) {
	got := defaultJSON(nil)
	require.Equal(t, []byte("{}"), got)
}

func TestDefaultJSON_EmptyReturnsEmptyObject(t *testing.T) {
	got := defaultJSON([]byte{})
	require.Equal(t, []byte("{}"), got)
}

func TestDefaultJSON_NonEmptyPassesThrough(t *testing.T) {
	in := []byte(`{"photos":["p1"],"description":"x"}`)
	got := defaultJSON(in)
	require.Equal(t, in, got)
}
