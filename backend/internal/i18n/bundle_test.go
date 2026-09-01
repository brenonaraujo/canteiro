package i18n

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad_EmbedsRequiredLocales(t *testing.T) {
	t.Parallel()
	b, err := Load()
	require.NoError(t, err)
	require.NotNil(t, b)
	require.NotNil(t, Bundle)
}
