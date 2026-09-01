package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"sync"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.json
var localesFS embed.FS

// Bundle is loaded once at process start.
var Bundle *goi18n.Bundle

var (
	loadOnce sync.Once
	loadErr  error
)

// Load embeds en, pt-BR and es. Default language is Brazilian Portuguese.
func Load() (*goi18n.Bundle, error) {
	loadOnce.Do(func() {
		b := goi18n.NewBundle(language.BrazilianPortuguese)
		b.RegisterUnmarshalFunc("json", json.Unmarshal)
		for _, name := range []string{"pt-BR.json", "en.json", "es.json"} {
			if _, err := b.LoadMessageFileFS(localesFS, "locales/"+name); err != nil {
				loadErr = fmt.Errorf("load locale %s: %w", name, err)
				return
			}
		}
		Bundle = b
	})
	return Bundle, loadErr
}
