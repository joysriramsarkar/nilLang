package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/alap/ui"
)

// U16.3: every portfolio example must load + render (runnable gate).
// U16.4: ui-counter is the counter→production path with state restoration.
func TestPortfolioExamplesRunnable(t *testing.T) {
	examples := []string{
		"../../examples/ui-counter/src/main.nil",
		"../../examples/alap-pos/src/main.nil",
		"../../examples/alap-music/src/main.nil",
		"../../examples/alap-typing/src/main.nil",
		"../../examples/alap-settings/src/main.nil",
		"../../examples/alap-search/src/main.nil",
		"../../examples/alap-master-detail/src/main.nil",
	}
	for _, ex := range examples {
		ex := ex
		t.Run(ex, func(t *testing.T) {
			code, err := os.ReadFile(filepath.FromSlash(ex))
			if err != nil {
				t.Skipf("example missing: %v", err)
			}
			app, err := loadDeclarativeUI(string(code), filepath.Dir(ex))
			if err != nil {
				t.Fatalf("load %s: %v", ex, err)
			}
			page, err := app.Render()
			if err != nil {
				t.Fatalf("render %s: %v", ex, err)
			}
			if html := page.RenderHTML(ui.OnuronTheme()); html == "" {
				t.Fatalf("empty render %s", ex)
			}
		})
	}
}
