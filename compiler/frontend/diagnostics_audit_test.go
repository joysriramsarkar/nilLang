package frontend

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/diagnostics"
)

// TestDiagnosticCatalogCoversKnownCodes ratchets the catalog against the
// ERROR_MODEL.md registry: every documented code resolves to an explanation.
// A code with no entry renders "Compiler diagnostic.", which hides the one
// thing a diagnostic is for.
func TestDiagnosticCatalogCoversKnownCodes(t *testing.T) {
	codes := []string{
		"E0000", "E0001", "E0002", "E0003", "E0004", "E0005",
		"E0101", "E0102", "E0103", "E0104", "E0105", "E0106",
		"E0107", "E0108", "E0110", "E0111", "E0112", "E0113",
		"E0201", "E0202", "E0203", "E0204",
		"E0301", "E0302", "E0303",
		"E0401", "E0402", "E0403", "E0404", "E0405", "E0406",
		"E0501", "E0502", "E0503", "E0504", "E0505", "E0506",
		"E0507", "E0508", "E0509", "E0510", "E0511",
		"E0601", "E0602", "E0603",
	}
	for _, code := range codes {
		text, ok := diagnostics.DiagnosticCatalog[code]
		if !ok || text == "" {
			t.Errorf("code %s missing from DiagnosticCatalog", code)
		}
	}
}

// TestDiagnosticsAreStructurallyComplete is the P0.7 ratchet: every diagnostic
// the frontend can produce carries a catalogued code, a severity, a positioned
// span with a filename, and the source line for context.
//
// End columns and suggestions are explicitly NOT required here: a point is
// honest where a range would be fabricated, and suggestions are a separate
// feature. If either becomes required, this is the test that says so.
func TestDiagnosticsAreStructurallyComplete(t *testing.T) {
	sources := []string{
		`let x = (1 + 2;`,
		`let x = 1; }`,
		`struct A{`,
		`enum B{`,
		`y = 1;`,
		`break;`,
		`continue;`,
		`let x: Int = "s";`,
		`fn f() -> Int { return "s"; }`,
		`f(1, 2, 3);`,
		`5();`,
		`let a = [1]; a[10];`,
		`import "nonexistent/mod";`,
		`let x: Missing = 1;`,
		`while (1) { x; }`,
		`match x { 1 => 2 };`,
		`extern "C" fn s(x: Float) -> Float;`,
		`component C { state s: Int = 0; render { return s; } }`,
		`let u = User { id: 1 };`,
		`trait T { fn m(); }`,
		`type ID = Int; let v: ID = 1;`,
		`let a = b ?? c;`,
		`for i in [1, 2] { x; }`,
	}

	for _, src := range sources {
		res := Check(src, Options{Filename: "audit.nil"})
		if res.OK {
			continue
		}
		if len(res.Diagnostics) == 0 && len(res.ParseErrors) == 0 {
			t.Errorf("source %q failed with neither diagnostics nor parse errors", src)
			continue
		}
		for _, d := range res.Diagnostics {
			if d == nil {
				t.Errorf("source %q produced a nil diagnostic", src)
				continue
			}
			if d.Code == "" {
				t.Errorf("source %q: diagnostic without code: %q", src, d.Message)
			} else if _, ok := diagnostics.DiagnosticCatalog[d.Code]; !ok {
				t.Errorf("source %q: code %q not in the catalog", src, d.Code)
			}
			if d.Severity == "" {
				t.Errorf("source %q: diagnostic without severity: %q", src, d.Message)
			}
			if d.Span.StartLine <= 0 {
				t.Errorf("source %q: diagnostic without a positioned span: [%s] %q",
					src, d.Code, d.Message)
			}
			if d.Span.Filename != "audit.nil" {
				t.Errorf("source %q: span filename = %q, want audit.nil", src, d.Span.Filename)
			}
			if d.ContextLine == "" {
				t.Errorf("source %q: diagnostic without source context: [%s] %q",
					src, d.Code, d.Message)
			}
		}
		// Legacy string errors travel alongside for compatibility callers; a
		// failure with no channel at all is the one shape that must not exist.
		if len(res.Diagnostics) == 0 && len(res.ParseErrors) == 0 {
			t.Errorf("source %q failed with no diagnostics and no parse errors", src)
		}
	}
}
