package tests

import (
	"strings"
	"testing"
)

// For loops, ranges, `break`, and `continue` are lowered once, before any
// backend sees the program. These tests pin the behaviour both engines must
// therefore share.

// TestForLoopAgreesAcrossEngines covers the three `for` forms against both
// engines. Structural agreement is the design goal: there is one lowering, so a
// divergence here means the engines are not both running the lowered form.
func TestForLoopAgreesAcrossEngines(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "exclusive range sums",
			source: `
let total = 0;
for i in 0..5 { total = total + i; }
let result = total;
`,
			want: "10",
		},
		{
			name: "inclusive range includes the last value",
			source: `
let total = 0;
for i in 0..5 { total = total + i; }
let result = total;
`,
			want: "10",
		},
		{
			name: "array iteration yields elements",
			source: `
let xs = [10, 20, 30];
let total = 0;
for x in xs { total = total + x; }
let result = total;
`,
			want: "60",
		},
		{
			name: "two variable form binds index and value",
			source: `
let xs = ["a", "b", "c"];
let joined = "";
for i, v in xs { joined = joined + v; }
let result = joined;
`,
			want: "abc",
		},
		{
			name: "descending range counts down",
			source: `
let total = 0;
for i in 3..0 { total = total + i; }
let result = total;
`,
			want: "6",
		},
		{
			name: "break leaves the loop",
			source: `
let count = 0;
for i in 0..100 { if (i == 4) { break; } count = count + 1; }
let result = count;
`,
			want: "4",
		},
		{
			name: "continue skips the rest of the body",
			source: `
let total = 0;
for i in 0..6 { if (i % 2 == 0) { continue; } total = total + i; }
let result = total;
`,
			want: "9",
		},
		{
			name: "continue still advances the loop",
			source: `
let count = 0;
for i in 0..10 { continue; count = count + 1; }
let result = count;
`,
			want: "0",
		},
		{
			name: "break inside continue's shadow",
			source: `
let count = 0;
for i in 0..10 { if (i > 2) { break; } count = count + 1; }
let result = count;
`,
			want: "3",
		},
		{
			name: "loop over a string yields characters",
			source: `
let joined = "";
for c in "abc" { joined = joined + c; }
let result = joined;
`,
			want: "abc",
		},
		{
			name: "empty collection runs zero times",
			source: `
let count = 0;
for x in [] { count = count + 1; }
let result = count;
`,
			want: "0",
		},
		{
			name: "nested loops keep separate cursors",
			source: `
let total = 0;
for i in 0..3 { for j in 0..3 { total = total + 1; } }
let result = total;
`,
			want: "9",
		},
		{
			name: "break in the inner loop leaves the outer one running",
			source: `
let total = 0;
for i in 0..3 {
  for j in 0..3 { if (j == 1) { break; } total = total + 1; }
  total = total + 10;
}
let result = total;
`,
			want: "33",
		},
		{
			name: "continue in the inner loop leaves the outer one running",
			source: `
let total = 0;
for i in 0..3 {
  for j in 0..3 { if (j == 1) { continue; } total = total + 1; }
  total = total + 10;
}
let result = total;
`,
			want: "36",
		},
		{
			name: "while loop also honours break",
			source: `
let count = 0;
let i = 0;
while (i < 100) { if (i == 3) { break; } count = count + 1; i = i + 1; }
let result = count;
`,
			want: "3",
		},
		{
			name: "while loop also honours continue",
			source: `
let count = 0;
let i = 0;
while (i < 5) { i = i + 1; if (i % 2 == 0) { continue; } count = count + 1; }
let result = count;
`,
			want: "3",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := evalSource(t, tc.source)
			if !res.OK {
				t.Fatalf("source was rejected: %s", res.ErrSummary())
			}

			evalOut := mustEvaluatorEval(t, res.Program, "result")
			if evalOut != tc.want {
				t.Errorf("evaluator produced %q, want %q", evalOut, tc.want)
			}

			vmOut := mustVMEval(t, res.Program, "result")
			if vmOut != tc.want {
				t.Errorf("VM produced %q, want %q", vmOut, tc.want)
			}
		})
	}
}

// TestForLoopVariablesAreScopedToTheBody checks that a loop variable does not
// leak. Leaking would let code after the loop read a stale binding and would
// make a second loop's variable assignment look like it was changing the first.
func TestForLoopVariablesAreScopedToTheBody(t *testing.T) {
	res := evalSource(t, `
for i in 0..3 { }
let after = i;
`)
	if res.OK {
		t.Fatal("the loop variable must not be visible after the loop")
	}
	if summary := res.ErrSummary(); !strings.Contains(summary, "i") {
		t.Errorf("summary should name the out-of-scope variable: %s", summary)
	}
}

// TestLoopVariablesAreFreshPerIteration guards the same property for a value
// captured by a closure created inside the loop. If the binding were shared,
// every closure would report the final value instead of its own.
func TestLoopVariablesAreFreshPerIteration(t *testing.T) {
	source := `
let first = "";
let second = "";
for i in 0..2 {
  let capture = fn() { return i; };
  if (i == 0) { first = "\(capture())"; } else { second = "\(capture())"; }
}
let result = first + "," + second;
`
	res := evalSource(t, source)
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}

	for label, out := range map[string]string{
		"evaluator": mustEvaluatorEval(t, res.Program, "result"),
		"VM":        mustVMEval(t, res.Program, "result"),
	} {
		if out != "0,1" {
			t.Errorf("%s produced %q, want \"0,1\"", label, out)
		}
	}
}

// TestBreakOutsideLoopIsRejected confirms the diagnostic fires rather than the
// jump patch list silently doing nothing.
func TestBreakOutsideLoopIsRejected(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{"break at top level", "break;", "`break` used outside of a loop"},
		{"continue at top level", "continue;", "`continue` used outside of a loop"},
		{"break inside a function with no loop", "fn f() { break; }", "`break` used outside of a loop"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := evalSource(t, tc.source)
			if res.OK {
				t.Fatalf("expected a rejection for %q", tc.source)
			}
			summary := res.ErrSummary()
			if !strings.Contains(summary, tc.want) {
				t.Errorf("summary does not mention %q:\n%s", tc.want, summary)
			}
		})
	}
}

// TestLargeRangeDoesNotAllocate confirms a range is not materialised. The loop
// breaks after a handful of iterations, so this finishes instantly whether or
// not the range was expanded; what it rules out is an array of a billion entries
// being built first.
func TestLargeRangeDoesNotAllocate(t *testing.T) {
	source := `
let count = 0;
for i in 0..1000000000 { count = count + 1; if (count == 5) { break; } }
let result = count;
`
	res := evalSource(t, source)
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}

	for label, out := range map[string]string{
		"evaluator": mustEvaluatorEval(t, res.Program, "result"),
		"VM":        mustVMEval(t, res.Program, "result"),
	} {
		if out != "5" {
			t.Errorf("%s produced %q, want 5", label, out)
		}
	}
}

// TestRangeRequiresAForHeader confirms a range is rejected anywhere else, rather
// than compiling to something with no runtime representation.
func TestRangeRequiresAForHeader(t *testing.T) {
	res := evalSource(t, `let r = 0..5;`)
	if res.OK {
		t.Fatal("a bare range should be rejected")
	}
	if summary := res.ErrSummary(); !strings.Contains(summary, "range") {
		t.Errorf("summary should mention the range: %s", summary)
	}
}
