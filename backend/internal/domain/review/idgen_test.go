package review_test

import (
	"regexp"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/review"
)

// v4Layout matches the canonical UUID v4 string layout. defaultIDGen
// flips bits to set the version (4) and variant (RFC 4122) nibbles,
// so every emitted id must satisfy this regex.
var v4Layout = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// TestDefaultIDGen_String_ReturnsValidV4UUID pins the contract the
// service relies on: every id is a parseable UUID v4. Covers
// idgen.go:19 String (was 0%).
func TestDefaultIDGen_String_ReturnsValidV4UUID(t *testing.T) {
	gen := review.DefaultIDGen()
	for i := 0; i < 64; i++ {
		id := gen.String()
		require.Regexp(t, v4Layout, id, "iteration %d: id=%q", i, id)
		parsed, err := uuid.Parse(id)
		require.NoError(t, err)
		require.Equal(t, uuid.Version(4), parsed.Version(),
			"iteration %d: id=%q is not v4", i, id)
		require.Equal(t, uuid.RFC4122, parsed.Variant(),
			"iteration %d: id=%q is not RFC 4122", i, id)
	}
}

// TestDefaultIDGen_String_IsUniqueAcrossCalls verifies the underlying
// crypto/rand source delivers non-colliding ids in a small sample
// (1k draws). Catches a regression where someone replaces the
// implementation with a constant or a counter by mistake.
func TestDefaultIDGen_String_IsUniqueAcrossCalls(t *testing.T) {
	gen := review.DefaultIDGen()
	const draws = 1024
	seen := make(map[string]struct{}, draws)
	for i := 0; i < draws; i++ {
		id := gen.String()
		_, dup := seen[id]
		require.False(t, dup, "duplicate id %q after %d draws", id, i)
		seen[id] = struct{}{}
	}
	require.Len(t, seen, draws)
}
