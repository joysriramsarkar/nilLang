// Package codegen implements U16.1: design tokens → code generation.
// Tokens become Nilang component source + Go catalog stubs deterministically.
package codegen

import (
	"fmt"
	"sort"
	"strings"

	"github.com/joysriramsarkar/nilLang/pkg/nilui/theme"
)

// ComponentSpec is one generated component.
type ComponentSpec struct {
	Name  string
	Type  string
	Props map[string]string
}

// GenerateNilang emits a declarative component hash source from tokens.
func GenerateNilang(specs []ComponentSpec) string {
	var sb strings.Builder
	sb.WriteString("// GENERATED from design tokens — do not edit by hand.\n")
	for _, s := range specs {
		sb.WriteString(fmt.Sprintf("{\n  \"type\": %q,\n  \"title\": %q,\n", s.Type, s.Name))
		keys := make([]string, 0, len(s.Props))
		for k := range s.Props {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("  %q: %q,\n", k, s.Props[k]))
		}
		sb.WriteString("}\n")
	}
	return sb.String()
}

// GenerateThemeGo emits Go role initialization from tokens.
func GenerateThemeGo(t theme.Tokens, dark bool) string {
	var sb strings.Builder
	sb.WriteString("// GENERATED from design tokens.\npackage theme\n\nfunc GeneratedRoles() Roles {\n")
	mode := "LightRoles"
	if dark {
		mode = "DarkRoles"
	}
	sb.WriteString(fmt.Sprintf("\treturn %s(DefaultTokens())\n}\n", mode))
	return sb.String()
}
