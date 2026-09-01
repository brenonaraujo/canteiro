package i18n

import (
	"context"
	"net/http"
	"strings"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type localeKey struct{}

const defaultLocale = "pt-BR"

// WithLocale stores the resolved locale on ctx.
func WithLocale(ctx context.Context, tag string) context.Context {
	return context.WithValue(ctx, localeKey{}, tag)
}

func localeFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(localeKey{}).(string); ok && v != "" {
		return v
	}
	return defaultLocale
}

// T localizes key using the locale on ctx. Missing keys return the key.
func T(ctx context.Context, key string) string {
	if Bundle == nil {
		return key
	}
	loc := goi18n.NewLocalizer(Bundle, localeFromCtx(ctx))
	s, err := loc.Localize(&goi18n.LocalizeConfig{MessageID: key})
	if err != nil {
		return key
	}
	return s
}

// LocaleFromRequest picks en, pt-BR or es from Accept-Language.
func LocaleFromRequest(r *http.Request) string {
	accept := r.Header.Get("Accept-Language")
	if accept == "" {
		return defaultLocale
	}
	lower := strings.ToLower(accept)
	switch {
	case strings.Contains(lower, "pt"):
		return "pt-BR"
	case strings.Contains(lower, "en"):
		return "en"
	case strings.Contains(lower, "es"):
		return "es"
	default:
		return defaultLocale
	}
}
