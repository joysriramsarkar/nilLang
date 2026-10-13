package tests

// TestScopeCorpus — P0.4 (docs/CORE_STABILIZATION.md): one scope corpus
// through checker, evaluator, and VM.
//
// Rules pinned here (see docs/spec/SEMANTICS.md §3.2 and §4.2):
//   - Blocks scope: bindings declared in a block are invisible outside it.
//   - `let` shadows: re-declaring an outer name inside a block leaves the
//     outer binding untouched (this was a write-through bug: both engines
//     returned 2 for the shadow case below).
//   - Same-scope redeclaration still reuses one binding.
//   - Loop bodies re-bind every pass: a closure created in the body captures
//     that pass's value, in functions and at top level alike.
//   - `app` bodies are deliberately transparent: their bindings are
//     program-level (see TestAppStatementExecution).
//   - Components scope state to handlers/render; handlers assign through.

import (
	"testing"
)

func TestScopeCorpusAgreesAcrossEngines(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "block bindings do not leak",
			source: `if (true) { let q = 41; } let result = 0;`,
			want:   "0",
		},
		{
			name:   "let shadows outer binding",
			source: `let x = 1; if (true) { let x = 2; } let result = x;`,
			want:   "1",
		},
		{
			name:   "shadow may change type",
			source: `let x = 1; if (true) { let x = "s"; } let result = x;`,
			want:   "1",
		},
		{
			name:   "function params shadow globals",
			source: `let x = 1; fn f() { let x = 2; return x; } let result = f() + x;`,
			want:   "3",
		},
		{
			name:   "same-scope redeclaration reuses binding",
			source: `let x = 1; let x = 2; let result = x;`,
			want:   "2",
		},
		{
			name:   "assignment reaches outer binding",
			source: `let i = 0; while (i < 3) { let t = i * 2; i = i + 1; } let result = i;`,
			want:   "3",
		},
		{
			name:   "nested functions write nearest enclosing",
			source: `let g = 5; fn outer() { let g = 6; fn inner() { g = g + 1; return g; } inner(); return g; } outer(); let result = g;`,
			want:   "5",
		},
		{
			name: "loop body rebinds every pass in functions",
			source: `
fn collect() {
  let fns = [];
  for i in [1, 2] { fns = push(fns, fn() { return i; }); }
  return fns[0]() + fns[1]();
}
let result = collect();
`,
			want: "3",
		},
		{
			name: "loop body rebinds every pass at top level",
			source: `
let fns = [];
for i in [1, 2] { fns = push(fns, fn() { return i; }); }
let result = fns[0]() + fns[1]();
`,
			want: "3",
		},
		{
			name:   "app body bindings are program-level",
			source: `app H { let ax = 9; } let result = ax;`,
			want:   "9",
		},
		{
			name:   "component state visible to render",
			source: `component C { state n: Int = 5; render { return n; } } let result = 1;`,
			want:   "1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := evalSource(t, tc.source)
			if !res.OK {
				t.Fatalf("source was rejected: %s", res.ErrSummary())
			}
			evalOut, err := evaluatorEval(res.Program, "result")
			if err != nil {
				t.Fatalf("evaluator: %v", err)
			}
			if evalOut != tc.want {
				t.Errorf("evaluator produced %q, want %q", evalOut, tc.want)
			}
			vmOut, err := vmEval(res.Program, "result")
			if err != nil {
				t.Fatalf("VM: %v", err)
			}
			if vmOut != tc.want {
				t.Errorf("VM produced %q, want %q", vmOut, tc.want)
			}
		})
	}
}

// TestBlockBindingsAreInvisibleOutside pins the checker leg of block scoping:
// reading a block local after the block is E0102.
func TestBlockBindingsAreInvisibleOutside(t *testing.T) {
	res := evalSource(t, `if (true) { let q = 41; } let result = q;`)
	if res.OK {
		t.Fatal("expected the checker to reject a leaked block binding")
	}
	found := false
	for _, d := range res.Diagnostics {
		if d.Code == "E0102" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected E0102, diagnostics=%v", res.Diagnostics)
	}
}
