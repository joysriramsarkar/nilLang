package tests

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/frontend"
)

// Numeric width checking (`L2.5`).
//
// # What changed
//
// A width used to resolve to `Int`, so `let x: i8 = 300;` compiled and nothing
// was checked. Each width is now its own type, a literal takes a width when it
// fits one, and arithmetic keeps the width. The warning that stood in for all of
// this is gone, because there is nothing left to warn about.
//
// # What is not checked
//
// The run-time representation is still 64-bit. `let a: u8 = 200; let b: u8 = a + a;`
// is 400 and stays 400. Catching that needs a narrowing instruction in both
// engines, and a check that fires for some values and not others is worse than
// one documented as compile-time.

// TestNumericWidthIsChecked covers what widths permit and what they refuse.
func TestNumericWidthIsChecked(t *testing.T) {
	accepted := []struct {
		name   string
		source string
		want   string
	}{
		{"a literal inside its range", `let x: u8 = 5; let result = x;`, "5"},
		{"the maximum of an unsigned width", `let x: u8 = 255; let result = x;`, "255"},
		{"the maximum of a signed width", `let x: i8 = 127; let result = x;`, "127"},
		{"the minimum of a signed width", `let x: i8 = -128; let result = x;`, "-128"},
		{"a large unsigned width", `let x: u32 = 4294967295; let result = x;`, "4294967295"},
		{"a width survives arithmetic", `let x: u8 = 5; let y: u8 = x + 1; let result = y;`, "6"},
		{"a width survives multiplication", `let x: i8 = 7; let y: i8 = x * 2; let result = y;`, "14"},
		{"state declares a width", `state count: i32 = 3; let result = count;`, "3"},
	}

	for _, tc := range accepted {
		t.Run(tc.name, func(t *testing.T) {
			res := evalSource(t, tc.source)
			if !res.OK {
				t.Fatalf("source was rejected: %s", res.ErrSummary())
			}
			for label, out := range map[string]string{
				"evaluator": mustEvaluatorEval(t, res.Program, "result"),
				"VM":        mustVMEval(t, res.Program, "result"),
			} {
				if out != tc.want {
					t.Errorf("%s produced %q, want %q", label, out, tc.want)
				}
			}
		})
	}

	rejected := []struct {
		name     string
		source   string
		wantCode string
	}{
		{
			name:     "above the maximum of an unsigned width",
			source:   `let x: u8 = 300;`,
			wantCode: "E0110",
		},
		{
			name:     "one past the maximum of an unsigned width",
			source:   `let x: u8 = 256;`,
			wantCode: "E0110",
		},
		{
			name:     "above the maximum of a signed width",
			source:   `let x: i8 = 128;`,
			wantCode: "E0110",
		},
		{
			name:     "below the minimum of a signed width",
			source:   `let x: i8 = -129;`,
			wantCode: "E0110",
		},
		{
			name:     "a negative into an unsigned width",
			source:   `let x: u8 = -1;`,
			wantCode: "E0110",
		},
		{
			name:     "Int does not become a width",
			source:   `let a: Int = 5; let x: u8 = a;`,
			wantCode: "E0101",
		},
		{
			name:     "a width does not become Int",
			source:   `let a: u8 = 5; let x: Int = a;`,
			wantCode: "E0101",
		},
		{
			name:     "one width does not become another",
			source:   `let a: u8 = 5; let x: u16 = a;`,
			wantCode: "E0101",
		},
	}

	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			res := frontend.Check(tc.source, frontend.Options{Filename: "width.nil"})
			if res.OK {
				t.Fatal("expected the program to be rejected")
			}
			joined := ""
			for _, d := range res.Diagnostics {
				joined += d.Code + " " + d.Message + "\n"
			}
			if !strings.Contains(joined, tc.wantCode) {
				t.Fatalf("expected %s, got:\n%s", tc.wantCode, joined)
			}
		})
	}
}

// TestNumericWidthsDoNotWarn checks that the old W0201 is gone.
//
// It is worth a test of its own: the warning existed because widths were a claim
// the compiler could not back up. Now that it can, the warning would be noise on
// every width annotation, and a diagnostic that fires on correct code is worse
// than no diagnostic at all.
func TestNumericWidthsDoNotWarn(t *testing.T) {
	res := frontend.Check(`let x: u8 = 5; let y = x + 1;`, frontend.Options{Filename: "width.nil"})
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}
	for _, d := range res.Diagnostics {
		if d.Code == "W0201" {
			t.Fatalf("widths are checked now; the warning is obsolete: %s", d.Message)
		}
	}
}
