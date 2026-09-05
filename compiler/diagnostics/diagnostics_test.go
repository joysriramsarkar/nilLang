package diagnostics

import (
	"strings"
	"testing"
)

func TestDiagnosticFormat(t *testing.T) {
	d := &Diagnostic{
		Code:     "E0101",
		Severity: SeverityError,
		Message:  "mismatched types: expected Int, got String",
		Span: Span{
			Filename:  "main.nil",
			StartLine: 12,
			StartCol:  5,
			EndLine:   12,
			EndCol:    10,
		},
		ContextLine:   "let a: Int = \"hello\";",
		Suggestion:    "cast using int(...) or change variable type to String",
		AIExplanation: ExplainCode("E0101"),
	}

	formatted := d.Format(false)
	if !strings.Contains(formatted, "[E0101]") {
		t.Errorf("Formatted string should contain code [E0101]")
	}
	if !strings.Contains(formatted, "^^^^^") {
		t.Errorf("Formatted string should contain caret pointers: %s", formatted)
	}
	if !strings.Contains(formatted, "ai truth") {
		t.Errorf("Formatted string should contain ai truth explanation")
	}
}
