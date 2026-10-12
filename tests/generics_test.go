package tests

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/frontend"
)

// Generic functions (`L2.2`).
//
// # What is implemented
//
// A function may declare type parameters. Inside its declaration every mention of
// one is a real type, so the body is checked rather than skipped. At a call site
// the checker matches each parameter type against the argument, binds the
// variables it can, and then checks the argument against the *instantiated*
// signature.
//
// # What is not
//
// Inference is structural, not unification. It sees through a generic type, an
// optional, and a union; it does not reason through a function type's parameter
// list. A variable it cannot pin down becomes `Any` rather than an error, because
// a value the checker has not read can be anything.
//
// Every test below is written against the implemented behaviour, including the
// case that is accepted and should not be — that one is the boundary, and a test
// suite that only checked what works would let the boundary move unnoticed.

// TestGenericFunction covers the shapes a generic function is written in.
func TestGenericFunction(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "identity",
			source: `fn id<T>(x: T) -> T { return x; } let result = id(5);`,
			want:   "5",
		},
		{
			name:   "identity on a string",
			source: `fn id<T>(x: T) -> T { return x; } let result = id("s");`,
			want:   "s",
		},
		{
			name:   "a container parameter",
			source: `fn size<T>(items: Array<T>) -> Int { return len(items); } let result = size([1, 2, 3]);`,
			want:   "3",
		},
		{
			name:   "the same generic over a different type",
			source: `fn size<T>(items: Array<T>) -> Int { return len(items); } let result = size(["a", "b"]);`,
			want:   "2",
		},
		{
			name:   "nested containers",
			source: `fn depth<T>(xs: Array<Array<T>>) -> Int { return len(xs); } let result = depth([[1], [2]]);`,
			want:   "2",
		},
		{
			name:   "two type parameters",
			source: `fn pair<A, B>(a: A, b: B) -> String { return "\(a)-\(b)"; } let result = pair(1, "x");`,
			want:   "1-x",
		},
		{
			name:   "an optional return",
			source: `fn head<T>(items: Array<T>) -> ?T { return items[0]; } let result = head([7, 8]);`,
			want:   "7",
		},
		{
			name:   "a generic calling a non-generic",
			source: `fn twice<T>(x: T) -> Int { return len([x, x]); } let result = twice("a");`,
			want:   "2",
		},
		{
			name:   "a generic function passed as a value",
			source: `fn id<T>(x: T) -> T { return x; } let f = id; let result = f(3);`,
			want:   "3",
		},
		{
			name:   "a declared type parameter that is never used",
			source: `fn tagged<T>(x: Int) -> Int { return x; } let result = tagged(4);`,
			want:   "4",
		},
	}

	for _, tc := range cases {
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
}

// TestGenericInferenceNarrowsTheReturnType checks that inference reaches the
// call's *own* type, not just its arguments.
//
// Without substitution the call `id(5)` would return the bare `T`, and every use
// of the result would be unchecked. That is the difference between a generic
// function and an unchecked one.
func TestGenericInferenceNarrowsTheReturnType(t *testing.T) {
	t.Run("the return type is instantiated", func(t *testing.T) {
		res := evalSource(t, `fn id<T>(x: T) -> T { return x; }
			let xs = id([1, 2]);
			let bad: Int = xs;`)
		if res.OK {
			t.Fatal("an array assigned to an Int must be rejected")
		}
	})

	t.Run("a variable the call pins down is respected", func(t *testing.T) {
		res := evalSource(t, `fn id<T>(x: T) -> T { return x; }
			let s = id("text");
			let bad: Int = s;`)
		if res.OK {
			t.Fatal("a string assigned to an Int must be rejected")
		}
	})
}

// TestGenericStructuralMismatch covers the argument a generic signature cannot
// accept at all, whatever the variable.
func TestGenericStructuralMismatch(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{
			name:   "an integer where an array is expected",
			source: `fn size<T>(items: Array<T>) -> Int { return len(items); } let result = size(5);`,
		},
		{
			name:   "an integer where nested arrays are expected",
			source: `fn depth<T>(xs: Array<Array<T>>) -> Int { return len(xs); } let result = depth(5);`,
		},
		{
			name:   "too few arguments",
			source: `fn pair<A, B>(a: A, b: B) -> Int { return 1; } let result = pair(1);`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := frontend.Check(tc.source, frontend.Options{Filename: "generics.nil"})
			if res.OK {
				t.Fatal("expected the program to be rejected")
			}
		})
	}
}

// TestGenericSyntaxErrors pins the parse failures.
func TestGenericSyntaxErrors(t *testing.T) {
	cases := []struct {
		name   string
		source string
		inText string
	}{
		{
			name:   "a type parameter list must hold names",
			source: `fn id<>(x: Int) -> Int { return x; } let result = id(1);`,
			inText: "type parameter list holds names",
		},
		{
			name:   "a type parameter declared twice",
			source: `fn id<T, T>(x: T) -> T { return x; } let result = id(1);`,
			inText: "declared twice",
		},
		{
			name:   "the list must close",
			source: `fn id<T(x: T) -> T { return x; } let result = id(1);`,
			inText: "must close",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := frontend.Check(tc.source, frontend.Options{Filename: "generics.nil"})
			if len(res.ParseErrors) == 0 && res.OK {
				t.Fatal("expected the source to be rejected")
			}
			joined := strings.Join(res.ParseErrors, "\n")
			for _, d := range res.Diagnostics {
				joined += "\n" + d.Message
			}
			if !strings.Contains(joined, tc.inText) {
				t.Fatalf("expected a message mentioning %q, got:\n%s", tc.inText, joined)
			}
		})
	}
}

// TestGenericBoundaryIsWhereItIs documents the limit, so it cannot drift.
//
// A type parameter that appears in no parameter cannot be pinned down by any
// argument, so it stays `Any` and the call is accepted for every instantiation.
// `empty()` returning `Array<Any>` is what makes `let xs: Array<Int> = empty()`
// compile — which is wrong, and is asserted here rather than left to be
// discovered. A suite that only checked what works would let the boundary move
// without anyone noticing.
func TestGenericBoundaryIsWhereItIs(t *testing.T) {
	res := evalSource(t, `fn empty<T>() -> Array<T> { return []; }
		let xs: Array<Int> = empty();
		let result = len(xs);`)
	if !res.OK {
		// Rejecting it would also be defensible; what is not defensible is the
		// answer depending on which day the checker was run.
		t.Fatalf("the boundary moved: %s", res.ErrSummary())
	}
	for label, out := range map[string]string{
		"evaluator": mustEvaluatorEval(t, res.Program, "result"),
		"VM":        mustVMEval(t, res.Program, "result"),
	} {
		if out != "0" {
			t.Errorf("%s produced %q, want %q", label, out, "0")
		}
	}
}
