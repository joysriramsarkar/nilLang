package uii18n

import (
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/nilui/ir"
)

func TestLocalizeTree(t *testing.T) {
	n := ir.New("Page").At("a.nil", 1, 1).WithProp("title", "i18n:pos.title")
	n.Add(ir.New("Text").At("a.nil", 2, 1).WithProp("value", "i18n:cart.empty"))
	LocalizeTree(n, LocaleBnBD)
	if n.Props["title"] == "i18n:pos.title" {
		t.Fatal("title not localized")
	}
	if n.Children[0].Props["value"] == "i18n:cart.empty" {
		t.Fatal("child not localized")
	}
	// Unknown key falls back to the key itself (never empty).
	m := ir.New("Text").At("a.nil", 1, 1).WithProp("value", "i18n:no.such.key")
	LocalizeTree(m, LocaleBnBD)
	if m.Props["value"] == "" {
		t.Fatal("fallback must not be empty")
	}
}

func TestDropdown(t *testing.T) {
	opts := DropdownOptions()
	if len(opts) != 4 || opts[0][0] != LocaleBnBD {
		t.Fatalf("dropdown broken: %v", opts)
	}
}

func TestBanglaLocale(t *testing.T) {
	if FormatNumberBN(123456) != "১,২৩,৪৫৬" {
		t.Fatalf("number: %q", FormatNumberBN(123456))
	}
	m := FormatMoneyBN(150000)
	if len(m) == 0 {
		t.Fatal("money empty")
	}
	for _, r := range m {
		if r >= '0' && r <= '9' {
			t.Fatalf("money must use Bengali digits: %q", m)
		}
	}
	d := FormatDateBN(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if d == "" {
		t.Fatal("date empty")
	}
}

func TestRTL(t *testing.T) {
	if IsRTL(LocaleBnBD) || IsRTL(LocaleEnUS) {
		t.Fatal("bn/en must be LTR")
	}
	if !IsRTL("ar") {
		t.Fatal("ar must be RTL")
	}
}

func TestPlatformLocale(t *testing.T) {
	var p PlatformLocale = SystemLocale{Tag: LocaleBnBD}
	if p.Locale() != LocaleBnBD {
		t.Fatal("platform locale broken")
	}
}
