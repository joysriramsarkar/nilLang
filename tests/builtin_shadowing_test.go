package tests

import (
	"strings"
	"testing"
)

// A `let` that shadows a builtin is ordinary, legal code. It used to be broken
// in the bytecode VM: the symbol table's Define returned the existing builtin
// symbol instead of creating a new binding, so the declaration wrote to a local
// slot while every read went to the builtin. The program then failed at run time
// with a type error about BUILTIN and INTEGER, which points nowhere near the
// real cause.

// TestInterpolationCannotNestStringLiterals records a real limitation.
//
// The lexer terminates a string at the first unescaped quote, so a string
// literal inside an interpolation closes the outer literal:
//
//	"\(join(["a", "b"], "-"))"     ← rejected
//	let parts = ["a", "b"];
//	"\(join(parts, "-"))"           ← accepted
//
// This is written as a test that *expects* rejection so that the day the
// limitation is lifted, the change forces a decision: either update this test or
// document the new behaviour deliberately, rather than having it change silently.
func TestInterpolationCannotNestStringLiterals(t *testing.T) {
	res := evalSource(t, `let result = "\(join(["a", "b"], "-"))";`)
	if res.OK {
		t.Error("a nested string literal inside an interpolation was accepted; " +
			"if that is now supported, update this test and LANGUAGE_SPEC.md §2.4")
	}
}

// TestShadowingABuiltinWorks covers the common names a developer reaches for
// when naming a local.
func TestShadowingABuiltinWorks(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "shadow len",
			source: `let len = 99; let sum = len + 1; let result = sum;`,
			want:   "100",
		},
		{
			name:   "shadow first with a string",
			source: `let first = "hello"; let result = first;`,
			want:   "hello",
		},
		{
			name:   "shadow type",
			source: `let type = "widget"; let result = type;`,
			want:   "widget",
		},
		{
			name:   "shadow push",
			source: `let push = 5; let result = push * 2;`,
			want:   "10",
		},
		{
			name:   "shadow set",
			source: `let set = [1, 2]; let result = len(set);`,
			want:   "2",
		},
		{
			name: "shadow then call a real builtin",
			source: `
let first = "abc";
let result = len(first);
`,
			want: "3",
		},
		{
			name: "shadowed name inside a function",
			source: `
fn f() {
  let len = 5;
  return len + 1;
}
let result = f();
`,
			want: "6",
		},
		{
			name: "shadow const",
			source: `
const limit = 3;
let result = limit;
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

			if out := mustEvaluatorEval(t, res.Program, "result"); out != tc.want {
				t.Errorf("evaluator produced %q, want %q", out, tc.want)
			}
			if out := mustVMEval(t, res.Program, "result"); out != tc.want {
				t.Errorf("VM produced %q, want %q", out, tc.want)
			}
		})
	}
}

// TestShadowedNameIsStillMutable confirms the shadow created a real binding
// rather than a constant, which would have masked the original bug differently.
func TestShadowedNameIsStillMutable(t *testing.T) {
	source := `
let count = 0;
count = count + 7;
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
		if out != "7" {
			t.Errorf("%s produced %q, want 7", label, out)
		}
	}
}

// TestConstStillRejectsMutation guards the other direction: the shadow fix must
// not have weakened const enforcement, since DefineConst shares the same
// lookup path.
func TestConstStillRejectsMutation(t *testing.T) {
	res := evalSource(t, `const limit = 3; limit = 4;`)
	if res.OK {
		t.Fatal("assigning to a const must be rejected")
	}
	if summary := res.ErrSummary(); !strings.Contains(summary, "E0104") {
		t.Errorf("expected E0104 Const Mutation, got: %s", summary)
	}
}

// TestBuiltinStillReachableWhenNotShadowed is the negative control: the shadow
// fix must not have broken the builtins themselves.
func TestBuiltinStillReachableWhenNotShadowed(t *testing.T) {
	cases := []struct {
		source string
		want   string
	}{
		{`let xs = [3, 1, 2]; let result = len(xs);`, "3"},
		{`let result = upper("abc");`, "ABC"},
		// A nested string literal inside an interpolation is not supported by
		// the lexer yet, so the arguments come from variables here. See
		// TestInterpolationCannotNestStringLiterals for that limitation.
		{`let parts = ["a", "b"]; let sep = "-"; let result = join(parts, sep);`, "a-b"},
		{`let xs = [9, 8]; let result = first(xs);`, "9"},
	}

	for _, tc := range cases {
		res := evalSource(t, tc.source)
		if !res.OK {
			t.Fatalf("source %q was rejected: %s", tc.source, res.ErrSummary())
		}
		for label, out := range map[string]string{
			"evaluator": mustEvaluatorEval(t, res.Program, "result"),
			"VM":        mustVMEval(t, res.Program, "result"),
		} {
			if out != tc.want {
				t.Errorf("%s produced %q for %q, want %q", label, out, tc.source, tc.want)
			}
		}
	}
}
