package typecheck

import (
	"strings"
	"testing"
	"time"
)

// TestNarrowingDoesNotReinferExponentially pins the fix for the exponential
// blowup in nested narrowing builtins: each level used to re-infer its
// arguments, doubling the work (and the diagnostics) per level.
func TestNarrowingDoesNotReinferExponentially(t *testing.T) {
	depth := 16
	// A 16-deep array narrowed once per level stays well-typed all the way
	// down to Int, so any diagnostic or second spent is pure re-inference.
	expr := "1"
	for i := 0; i < depth; i++ {
		expr = "[" + expr + "]"
	}
	for i := 0; i < depth; i++ {
		expr = "first(" + expr + ")"
	}
	src := "let x = " + expr + ";"

	start := time.Now()
	c, ok := checkSource(t, src)
	elapsed := time.Since(start)
	if !ok {
		t.Fatalf("nested narrowing was refused: %v", c.Diagnostics)
	}
	// Old behaviour: 2^16 diagnostics and seconds of inference. New
	// behaviour: a handful of diagnostics at most, in milliseconds.
	if len(c.Diagnostics) > depth*2 {
		t.Fatalf("%d diagnostics for %d nesting levels (want at most %d)", len(c.Diagnostics), depth, depth*2)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("inference took %v for %d nesting levels", elapsed, depth)
	}
}

// TestGateRefusesNamesInsideSkippedForms pins the traversal holes:
// comparison and logical operands, await operands and range bounds must all
// be inferred, so an undefined name inside any of them fails the gate.
// Task bodies are NOT in this list on purpose: the concurrency spec pins
// runtime faults for task bodies (§4.2 typed fault), so the gate must let
// them through and the scheduler reports TaskFaulted at run time.
func TestGateRefusesNamesInsideSkippedForms(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"equality hides undefined", `let x = (nosuch == 1);`},
		{"comparison hides undefined", `let x = (nosuch < 1);`},
		{"logical hides undefined call", `let x = (a && undefinedFn());`},
		{"await hides undefined", `let y = await nosuch;`},
		{"range hides undefined bounds", "for i in nosuchStart..nosuchEnd { }"},
		{"range rejects string bounds", `for i in "a".."z" { }`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, ok := checkSource(t, tc.src)
			if ok {
				t.Fatalf("gate passed for %q", tc.src)
			}
			if len(c.Diagnostics) == 0 {
				t.Fatalf("gate failed for %q with no diagnostics", tc.src)
			}
		})
	}
}

// TestGateStillAcceptsHealthyForms guards against overcorrection: valid
// comparisons, tasks, awaits and ranges (including width-typed bounds) pass.
func TestGateStillAcceptsHealthyForms(t *testing.T) {
	cases := []string{
		`let x = (1 == 1);`,
		`let x = (1 < 2 && 3 > 1);`,
		`let t = task { 1 };`,
		`for i in 0..10 { }`,
		`for i in 10..0 { }`,
		`let n: i8 = 5; for i in 0..n { }`,
	}
	for _, src := range cases {
		c, ok := checkSource(t, src)
		if !ok {
			msgs := make([]string, 0, len(c.Diagnostics))
			for _, d := range c.Diagnostics {
				msgs = append(msgs, d.Message)
			}
			t.Fatalf("gate refused %q: %s", src, strings.Join(msgs, "; "))
		}
	}
}
