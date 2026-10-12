package components

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/nilui/theme"
)

func TestCatalogCount54(t *testing.T) {
	if len(Registry) != 54 {
		t.Fatalf("catalog must hold 54, got %d: %v", len(Registry), Names())
	}
	counts := map[Category]int{}
	for _, d := range Registry {
		counts[d.Contract.Category]++
	}
	want := map[Category]int{CatFoundation: 6, CatLayout: 11, CatInput: 11, CatDisplay: 9, CatFeedback: 6, CatNavigation: 6, CatAlapSpecial: 5}
	for c, n := range want {
		if counts[c] != n {
			t.Fatalf("category %s: want %d got %d", c, n, counts[c])
		}
	}
}

func TestContractsComplete(t *testing.T) {
	for _, name := range Names() {
		d := Registry[name]
		if d.Contract.Role == "" {
			t.Errorf("%s: missing a11y role", name)
		}
		if len(d.Contract.States) == 0 {
			t.Errorf("%s: missing states", name)
		}
		if d.Contract.Category == "" {
			t.Errorf("%s: missing category", name)
		}
	}
}

func TestValidation(t *testing.T) {
	b := Registry["Button"]
	if err := b.Validate(map[string]string{"label": "x"}); err != nil {
		t.Fatalf("valid button rejected: %v", err)
	}
	if err := b.Validate(map[string]string{}); err == nil {
		t.Fatal("button without label must fail")
	}
}

func TestLifecycle(t *testing.T) {
	d := Registry["Dialog"]
	d.Mount()
	d.Mount()
	d.Unmount()
	if d.Mounted() != 1 {
		t.Fatalf("lifecycle broken: %d", d.Mounted())
	}
	d.Unmount()
	d.Unmount() // never negative
	if d.Mounted() != 0 {
		t.Fatalf("unmount floor broken: %d", d.Mounted())
	}
}

func TestThemeAware(t *testing.T) {
	tk := theme.DefaultTokens()
	light, dark := theme.LightRoles(tk), theme.DarkRoles(tk)
	b := Registry["Badge"]
	a := b.RenderHTML(map[string]string{"label": "new"}, light)
	c := b.RenderHTML(map[string]string{"label": "new"}, dark)
	if a == c {
		t.Fatal("light/dark render must differ")
	}
	if !strings.Contains(a, `role="status"`) || !strings.Contains(a, "new") {
		t.Fatalf("badge html broken: %s", a)
	}
}

func TestA11yGateAllComponents(t *testing.T) {
	// U11.9 mandatory gate: every component exposes role + label.
	for _, name := range Names() {
		d := Registry[name]
		props := map[string]string{}
		for _, p := range d.Contract.Props {
			props[strings.TrimSuffix(p, "?")] = "x"
		}
		label := d.A11yLabel(props)
		if label == "" || d.Contract.Role == "" {
			t.Errorf("%s: a11y gate failed (role=%q label=%q)", name, d.Contract.Role, label)
		}
	}
}
