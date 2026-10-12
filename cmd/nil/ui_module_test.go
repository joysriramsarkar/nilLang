package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/alap/ui"
)

// TestDeclarativeUIResolvesRelativeModuleImports guards a real defect: the
// render and dev paths evaluated the entry program without telling the
// evaluator where the entry file lives, so a component importing a sibling
// module only worked when the process was started from that directory.
//
// A developer running `nil render examples/app/src/main.nil` from the
// repository root must get the same page as running it from examples/app.
func TestDeclarativeUIResolvesRelativeModuleImports(t *testing.T) {
	root := t.TempDir()
	srcDir := filepath.Join(root, "src")
	libDir := filepath.Join(root, "lib")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatalf("mkdir lib: %v", err)
	}

	moduleSource := `
let titles = ["প্রথম", "দ্বিতীয়", "তৃতীয়"];
let getAll = fn() { return titles; };
`
	entrySource := `
import "../lib/Greetings.nil" as Greetings;

component List {
    render {
        let all = Greetings.getAll();
        let cards = [];
        let i = 0;
        while (i < len(all)) {
            cards = append(cards, {"type": "Card", "title": all[i], "body": "row \(i)"});
            i = i + 1;
        }
        return {
            "type": "Page",
            "title": "Greetings",
            "content": cards,
            "footer": "count: \(len(all))"
        };
    }
}
`
	if err := os.WriteFile(filepath.Join(libDir, "Greetings.nil"), []byte(moduleSource), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}
	entryPath := filepath.Join(srcDir, "main.nil")
	if err := os.WriteFile(entryPath, []byte(entrySource), 0o644); err != nil {
		t.Fatalf("write entry: %v", err)
	}

	source, err := os.ReadFile(entryPath)
	if err != nil {
		t.Fatalf("read entry: %v", err)
	}

	// Pass the entry directory, exactly as cmdRender and cmdDev now do.
	app, err := loadDeclarativeUI(string(source), srcDir)
	if err != nil {
		t.Fatalf("loadDeclarativeUI with an entry directory failed: %v", err)
	}

	page, err := app.Render()
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	html := page.RenderHTML(ui.DefaultTheme())
	for _, want := range []string{"প্রথম", "দ্বিতীয়", "তৃতীয়", "Greetings"} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered page is missing %q:\n%s", want, html)
		}
	}
}

// TestDeclarativeUIReportsUnresolvableImport confirms a genuinely missing
// module still fails loudly rather than silently rendering an empty page.
func TestDeclarativeUIReportsUnresolvableImport(t *testing.T) {
	root := t.TempDir()
	entry := `
import "../nowhere/Missing.nil" as Missing;
component Page { render { return {"type": "Page", "title": "x", "content": []}; } }
`
	_, err := loadDeclarativeUI(entry, root)
	if err == nil {
		t.Fatal("expected an error for a missing module, got none")
	}
	if !strings.Contains(err.Error(), "Missing.nil") {
		t.Errorf("error should name the missing module, got: %v", err)
	}
}
