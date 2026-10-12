package codegen

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/nilui/theme"
)

func TestGenerateNilang(t *testing.T) {
	out := GenerateNilang([]ComponentSpec{
		{Name: "Pay", Type: "Button", Props: map[string]string{"label": "Pay", "variant": "primary"}},
	})
	if !strings.Contains(out, `"type": "Button"`) || !strings.Contains(out, `"label": "Pay"`) {
		t.Fatalf("codegen broken:\n%s", out)
	}
	// Deterministic: same input twice → identical output.
	if out2 := GenerateNilang([]ComponentSpec{{Name: "Pay", Type: "Button", Props: map[string]string{"variant": "primary", "label": "Pay"}}}); out != out2 {
		t.Fatal("codegen must be deterministic")
	}
}

func TestGenerateThemeGo(t *testing.T) {
	if got := GenerateThemeGo(theme.DefaultTokens(), true); !strings.Contains(got, "DarkRoles") {
		t.Fatalf("theme codegen broken: %s", got)
	}
}
