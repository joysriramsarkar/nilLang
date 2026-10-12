package tests

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/frontend"
)

// Function types and their variance (`L2.3`).
//
// # What this file is for
//
// The rules were already implemented — this file is what keeps them implemented.
// Variance is the easiest thing in a type system to break without noticing,
// because a change that makes it *stricter* rejects working programs and a change
// that makes it *looser* accepts broken ones, and neither shows up until much
// later.
//
// # The two directions, and why they are opposite
//
// Parameters are **contravariant**: `fn(Any) -> R` may stand where
// `fn(String) -> R` is expected, because a function that accepts anything can be
// used where one that accepts a String is wanted. The reverse is refused.
//
// A return type is **covariant**: `fn(A) -> Any` may stand where `fn(A) -> Int`
// is expected, because anything that reads the result is prepared for the wider
// type. The reverse is refused.
//
// Both halves are asserted here, in both directions, because asserting only the
// accepting direction would pass just as well with an `Any` that matches
// everything.

// TestFunctionTypeVarianceAccepts covers what the rules permit.
func TestFunctionTypeVarianceAccepts(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{
			name: "an exact match",
			source: `fn takes(f: fn(Int) -> Int) -> Int { return f(1); }
				fn one(x: Int) -> Int { return x; }
				let result = takes(one);`,
		},
		{
			name: "a wider parameter is accepted where a narrower one is wanted",
			source: `fn takes(f: fn(String) -> Int) -> Int { return 1; }
				fn wide(x: Any) -> Int { return 1; }
				let result = takes(wide);`,
		},
		{
			name: "a narrower return is accepted where a wider one is wanted",
			source: `fn takes(f: fn(Int) -> Any) -> Int { return 1; }
				fn narrow(x: Int) -> Int { return x; }
				let result = takes(narrow);`,
		},
		{
			name: "both at once",
			source: `fn takes(f: fn(String) -> Any) -> Int { return 1; }
				fn both(x: Any) -> Int { return 1; }
				let result = takes(both);`,
		},
		{
			name: "several parameters",
			source: `fn add(a: Int, b: Int) -> Int { return a + b; }
				fn takes(f: fn(Int, Int) -> Int) -> Int { return f(1, 2); }
				let result = takes(add);`,
		},
		{
			name: "a generic is accepted where a function type is wanted",
			source: `fn apply(f: fn(Int) -> Int, x: Int) -> Int { return f(x); }
				fn id<T>(v: T) -> T { return v; }
				let result = apply(id, 5);`,
		},
		{
			name: "a function-typed binding",
			source: `fn one(x: Int) -> Int { return x; }
				let f: fn(Int) -> Int = one;
				let result = f(5);`,
		},
		{
			name: "a function-typed binding taking another",
			source: `fn twice(f: fn(Int) -> Int) -> Int { return f(f(1)); }
				let g = fn(x: Int) -> Int { return x * 2; };
				let result = twice(g);`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := evalSource(t, tc.source)
			if !res.OK {
				t.Fatalf("source was rejected: %s", res.ErrSummary())
			}
			// The point is that it was accepted; running it as well would only
			// repeat what the call tests already cover.
			_ = mustEvaluatorEval(t, res.Program, "result")
		})
	}
}

// TestFunctionTypeVarianceRejects covers what the rules refuse.
//
// Each case is the *mirror* of one in the test above. Pairing them is what makes
// the pair meaningful: a check that accepts everything would pass the accepting
// test alone, and one that refuses everything would pass this one alone.
func TestFunctionTypeVarianceRejects(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{
			name: "a narrower parameter is refused where a wider one is wanted",
			source: `fn takes(f: fn(Any) -> Int) -> Int { return 1; }
				fn narrow(x: String) -> String { return x; }
				let result = takes(narrow);`,
		},
		{
			name: "a wider return is refused where a narrower one is wanted",
			source: `fn takes(f: fn(Int) -> String) -> Int { return 1; }
				fn wide(x: Int) -> Int { return x; }
				let result = takes(wide);`,
		},
		{
			name: "a mismatched parameter type",
			source: `fn takes(f: fn(String) -> Int) -> Int { return 1; }
				fn one(x: Int) -> Int { return x; }
				let result = takes(one);`,
		},
		{
			name: "a mismatched return type",
			source: `fn takes(f: fn(Int) -> String) -> Int { return 1; }
				fn one(x: Int) -> Int { return x; }
				let result = takes(one);`,
		},
		{
			name: "the wrong number of parameters",
			source: `fn takes(f: fn(Int, Int) -> Int) -> Int { return 1; }
				fn one(x: Int) -> Int { return x; }
				let result = takes(one);`,
		},
		{
			name: "too few parameters",
			source: `fn takes(f: fn(Int) -> Int) -> Int { return 1; }
				fn none() -> Int { return 1; }
				let result = takes(none);`,
		},
		{
			name: "a value that is not a function",
			source: `fn takes(f: fn(Int) -> Int) -> Int { return 1; }
				let result = takes(5);`,
		},
		{
			name:   "a function type annotation given a non-function",
			source: `let f: fn(Int) -> Int = 5; let result = 1;`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := frontend.Check(tc.source, frontend.Options{Filename: "fntypes.nil"})
			if res.OK {
				t.Fatal("expected the program to be rejected")
			}
		})
	}
}

// TestFunctionVarianceIsNotPlainEquality is the test that would catch the rule
// being replaced by an identity check.
//
// Every other case would still pass if the two directions were simply `Equals`:
// an identity rule accepts and rejects almost the same programs as variance
// does, and the difference only shows up where a type sits between `Any` and
// something concrete. These two sources are that case, and they go opposite ways.
func TestFunctionVarianceIsNotPlainEquality(t *testing.T) {
	accepted := `fn takes(f: fn(String) -> Any) -> Int { return 1; }
		fn wide(x: Any) -> Int { return 1; }
		let result = takes(wide);`
	if res := frontend.Check(accepted, frontend.Options{Filename: "variance.nil"}); !res.OK {
		t.Fatalf("a wider function must stand where a narrower one is wanted: %s", res.ErrSummary())
	}

	rejected := `fn takes(f: fn(Any) -> Int) -> Int { return 1; }
		fn narrow(x: String) -> String { return x; }
		let result = takes(narrow);`
	if res := frontend.Check(rejected, frontend.Options{Filename: "variance.nil"}); res.OK {
		t.Fatal("a narrower function must not stand where a wider one is wanted")
	}
}
