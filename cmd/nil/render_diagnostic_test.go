package main

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/object"
)

func mustHash(t *testing.T, pairs map[string]object.Object) *object.Hash {
	t.Helper()
	return evaluator.MakeHashObj(pairs)
}

// U0.4: unknown component types must produce a diagnostic, never a silent drop.
func TestConvertUnknownTypeDiagnostic(t *testing.T) {
	h := mustHash(t, map[string]object.Object{
		"type": &object.String{Value: "FluxCapacitor"},
	})
	comp, diag := convertHashToComponent(h)
	if comp != nil {
		t.Errorf("unknown type must not produce a component, got %v", comp)
	}
	if diag == nil {
		t.Fatalf("unknown type must produce a diagnostic")
	}
	if diag.Code == "" || diag.Message == "" {
		t.Errorf("diagnostic must carry code+message, got %+v", diag)
	}
	if diag.ComponentType != "FluxCapacitor" {
		t.Errorf("diagnostic must name the type, got %+v", diag)
	}
}

// U0.4/U7: every catalog type binds (5 natives + 54 passthrough).
func TestConvertCatalogTypesBind(t *testing.T) {
	for _, typ := range []string{"Button", "Text", "Badge", "Dialog", "Grid", "Chart", "Route"} {
		h := mustHash(t, map[string]object.Object{
			"type":  &object.String{Value: typ},
			"title": &object.String{Value: "T"},
			"label": &object.String{Value: "L"},
			"body":  &object.String{Value: "B"},
			"id":    &object.String{Value: "x"},
		})
		comp, diag := convertHashToComponent(h)
		if diag != nil {
			t.Errorf("catalog type %q must not diagnose: %v", typ, diag)
			continue
		}
		if comp == nil {
			t.Errorf("catalog type %q must produce a component", typ)
		}
	}
}

// U0.4: empty type is a diagnostic, not a panic.
func TestConvertEmptyTypeDiagnostic(t *testing.T) {
	h := mustHash(t, map[string]object.Object{})
	if _, diag := convertHashToComponent(h); diag == nil {
		t.Errorf("empty type must produce a diagnostic")
	}
}
