package tests

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/frontend"
)

// Member chains (`L2.6`), union annotations (`L2.4`), and numeric width
// annotations (`L2.5`).
//
// # The shape of what went wrong
//
// All three are the same mistake in the type system: something was written down
// and then not acted on. A member chain inferred as `Any` whatever it was asked
// about; a union annotation resolved to placeholders that compare equal to
// nothing, so it accepted every value; a width spelling resolved to `Int` and
// enforced nothing.
//
// Each is easy to miss in review because the code that refuses to check them
// still runs, still type-checks, and still compiles. What it produces is a
// wrong answer or a failure three phases later.

// TestMemberChainIsChecked covers `a.b.c` reaching the declared field type.
func TestMemberChainIsChecked(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "one level",
			source: `struct P { v: Int } let p = P { v: 7 }; let result = p.v;`,
			want:   "7",
		},
		{
			name:   "two levels",
			source: `struct Inner { v: Int } struct Outer { i: Inner } let o = Outer { i: Inner { v: 7 } }; let result = o.i.v;`,
			want:   "7",
		},
		{
			name: "three levels",
			source: `struct A { v: Int } struct B { a: A } struct C { b: B }
				let c = C { b: B { a: A { v: 7 } } }; let result = c.b.a.v;`,
			want: "7",
		},
		{
			name:   "a field's type is checked, not assumed",
			source: `struct P { v: Int } let p = P { v: 1 }; let result = p.v;`,
			want:   "1",
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

// TestMemberChainErrors covers `E0111`.
//
// Before it, every one of these reached the backend and failed at run time with
// "unknown method X on object" — which reads like a missing method rather than a
// misspelled field, and sends the reader looking in the wrong file.
func TestMemberChainErrors(t *testing.T) {
	cases := []struct {
		name   string
		source string
		inText string
	}{
		{
			name:   "a field that does not exist",
			source: `struct P { v: Int } let p = P { v: 1 }; let result = p.nosuch;`,
			inText: "nosuch",
		},
		{
			name:   "a field missing at the end of a chain",
			source: `struct Inner { v: Int } struct Outer { i: Inner } let o = Outer { i: Inner { v: 1 } }; let result = o.i.nosuch;`,
			inText: "nosuch",
		},
		{
			name:   "a field missing in the middle of a chain",
			source: `struct Inner { v: Int } struct Outer { i: Inner } let o = Outer { i: Inner { v: 1 } }; let result = o.nosuch.v;`,
			inText: "nosuch",
		},
		{
			name:   "a field that belongs to another struct",
			source: `struct P { a: Int } struct Q { b: Int } let p = P { a: 1 }; let result = p.b;`,
			inText: "b",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := frontend.Check(tc.source, frontend.Options{Filename: "members.nil"})
			if res.OK {
				t.Fatal("expected the program to be rejected")
			}
			joined := ""
			for _, d := range res.Diagnostics {
				joined += d.Code + " " + d.Message + "\n"
			}
			if !strings.Contains(joined, "E0111") {
				t.Fatalf("expected E0111, got:\n%s", joined)
			}
			if !strings.Contains(joined, tc.inText) {
				t.Fatalf("message does not name the member: %s", joined)
			}
		})
	}
}

// TestMemberAccessThatCannotBeChecked covers the receivers the checker must
// leave alone.
//
// A hash has whatever keys were put in it, and a receiver whose type was lost is
// not evidence of a mistake. Refusing an unknown key in either case would reject
// correct programs, and the errors would be invented rather than found.
func TestMemberAccessThatCannotBeChecked(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{"a hash key is not a struct field", `let h = {"a": 1}; let result = h.a;`, "1"},
		{"an undeclared name is not a member error", `let result = 1;`, "1"},
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

// TestUnionAnnotation covers `A | B` actually being checked.
func TestUnionAnnotation(t *testing.T) {
	accepted := []struct {
		name   string
		source string
		want   string
	}{
		{"a primitive member", `let x: Int | String = 5; let result = x;`, "5"},
		{"the other primitive member", `let x: Int | String = "s"; let result = x;`, "s"},
		{"a struct member", `struct P { a: Int } struct Q { b: Int } let x: P | Q = P { a: 3 }; let result = x.a;`, "3"},
		{"the other struct member", `struct P { a: Int } struct Q { b: Int } let x: P | Q = Q { b: 4 }; let result = x.b;`, "4"},
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
		name   string
		source string
		inText string
	}{
		{
			name:   "a value in neither member",
			source: `let x: Int | String = true; let result = x;`,
			inText: "Bool",
		},
		{
			name:   "a member that is not a type",
			source: `let x: Int | Nope = 5; let result = x;`,
			inText: "Nope",
		},
		{
			name:   "a whole annotation that is not a type",
			source: `let x: Nope = 5; let result = x;`,
			inText: "Nope",
		},
		{
			// Before the fix this was accepted: the union held a placeholder, and
			// a placeholder compares equal to nothing, so the one real member was
			// all that was ever checked — and here it did not match either.
			name:   "a struct value outside the union",
			source: `struct P { a: Int } struct R { c: Int } let x: P | Q = R { c: 1 }; let result = x;`,
			inText: "",
		},
	}

	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			res := frontend.Check(tc.source, frontend.Options{Filename: "union.nil"})
			if res.OK {
				t.Fatal("expected the program to be rejected")
			}
			joined := ""
			for _, d := range res.Diagnostics {
				joined += d.Code + " " + d.Message + "\n"
			}
			if tc.inText != "" && !strings.Contains(joined, tc.inText) {
				t.Fatalf("message does not mention %q: %s", tc.inText, joined)
			}
		})
	}
}
