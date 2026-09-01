package i18n

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestT_UsesLocaleAndFallback(t *testing.T) {
	_, err := Load()
	require.NoError(t, err)

	tests := []struct {
		name   string
		locale string
		key    string
		want   string
	}{
		{"pt-BR ready", "pt-BR", "ready.ok", "Serviço está pronto"},
		{"en ready", "en", "ready.ok", "Service is ready"},
		{"es not ready", "es", "ready.not_ready", "El servicio no está listo"},
		{"missing key", "en", "no.such.key", "no.such.key"},
		{"empty locale defaults pt-BR", "", "health.ok", "Serviço está no ar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.locale != "" {
				ctx = WithLocale(ctx, tt.locale)
			}
			assert.Equal(t, tt.want, T(ctx, tt.key))
		})
	}
}

func TestLocaleFromRequest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		header string
		want   string
	}{
		{"", "pt-BR"},
		{"pt-BR,pt;q=0.9", "pt-BR"},
		{"en-US,en;q=0.8", "en"},
		{"es-MX,es;q=0.9", "es"},
		{"fr-FR", "pt-BR"},
	}
	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			t.Parallel()
			r, err := http.NewRequest(http.MethodGet, "/", nil)
			require.NoError(t, err)
			if tt.header != "" {
				r.Header.Set("Accept-Language", tt.header)
			}
			assert.Equal(t, tt.want, LocaleFromRequest(r))
		})
	}
}
