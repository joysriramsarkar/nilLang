package tests

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
)

// Optional access (`?.`) and null coalescing (`??`).
//
// # What is being tested
//
// Both operators lower to `if`/`else` before any backend sees the program, so
// they reach the two engines through exactly the path `match` and `for` take.
// Every behavioural case therefore runs in both, and a divergence would mean the
// lowering does not do what it claims.
//
// # The two rules that are easy to get wrong
//
// The receiver is evaluated **once** (`a?.b.c` on an expensive `a` must not
// compute `a` three times), and `?.` guards the **whole chain** rather than the
// single step it appears on (`o?.i.v` is null when `o` is, not an attempt to
// read `.v` off a null). Both have their own test below.

// TestOptionalAccess covers both operators in the shapes they are written in.
func TestOptionalAccess(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "coalesce takes the fallback when the value is null",
			source: `let a = null; let result = a ?? "d";`,
			want:   "d",
		},
		{
			name:   "coalesce keeps a present value",
			source: `let a = "v"; let result = a ?? "d";`,
			want:   "v",
		},
		{
			name:   "coalesce keeps a falsy but present value",
			source: `let a = 0; let result = a ?? 9;`,
			want:   "0",
		},
		{
			name:   "coalesce keeps an empty string",
			source: `let a = ""; let result = a ?? "d";`,
			want:   "",
		},
		{
			name:   "coalesce chains left to right",
			source: `let a = null; let b = null; let c = "x"; let result = a ?? b ?? c;`,
			want:   "x",
		},
		{
			name:   "coalesce does not evaluate the fallback when it is not needed",
			source: `fn boom() -> String { return "boom"; } let a = "v"; let result = a ?? boom();`,
			want:   "v",
		},
		{
			name:   "coalesce evaluates its left side once",
			source: `let n = 0; fn once() { n = n + 1; return null; } let r = once() ?? "d"; let result = n;`,
			want:   "1",
		},
		{
			name:   "coalesce on both integers and strings",
			source: `let a = 5; let result = "\(a * 2) \(0 ?? 9)";`,
			want:   "10 0",
		},
		{
			name:   "safe access reads through a present value",
			source: `struct P { name: String } let p = P { name: "Rex" }; let result = p?.name;`,
			want:   "Rex",
		},
		{
			name:   "safe access on null is null, not an error",
			source: `let a = null; let result = "\(a?.name)";`,
			want:   "null",
		},
		{
			name:   "safe access then coalesce",
			source: `struct P { name: String } let nothing = null; let result = nothing?.name ?? "none";`,
			want:   "none",
		},
		{
			name:   "isNull is available to programs, not only to the lowering",
			source: `let a = null; let result = "\(isNull(a)) \(isNull(1))";`,
			want:   "true false",
		},
		{
			name:   "the ternary still parses",
			source: `let n = 5; let result = n > 3 ? "big" : "small";`,
			want:   "big",
		},
		{
			name:   "the optional type annotation still parses",
			source: `let username: ?String = null; let result = username ?? "guest";`,
			want:   "guest",
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

// TestOptionalAccessGuardsTheWholeChain pins the rule that `?.` is not a
// per-step operator.
//
// `o?.i.v` reads as "reach through `o` for `i.v`, or nothing at all". A lowering
// that guards only `o.i` reads `.v` off a null, which is the failure this test
// exists to catch.
func TestOptionalAccessGuardsTheWholeChain(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "the chain resolves when the receiver is present",
			source: `struct Inner { v: Int } struct Outer { i: Inner } let o = Outer { i: Inner { v: 7 } }; let result = o?.i.v;`,
			want:   "7",
		},
		{
			name:   "the whole chain is null when the receiver is null",
			source: `struct Inner { v: Int } struct Outer { i: Inner } let nothing = null; let result = "\(nothing?.i.v)";`,
			want:   "null",
		},
		{
			name:   "a chain after a call is guarded too",
			source: `struct Inner { v: Int } let nothing = null; let result = "\(nothing?.inner.v)";`,
			want:   "null",
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

// TestOptionalAccessOnATraitMethod checks that `?.` resolves a trait method
// rather than falling back to a field access on a hash.
//
// It fell back to exactly that at one point: the checker recorded method
// resolutions against the member-access node, and a `?.` callee is a different
// node, so the lowering looked up a name that was never recorded and produced a
// plain hash read. The failure surfaced as "unknown method" at run time.
func TestOptionalAccessOnATraitMethod(t *testing.T) {
	source := `trait Greeter { fn greet(self) -> String }
		struct Dog { name: String }
		impl Greeter for Dog { fn greet(self) -> String { return "Woof"; } }
		let d = Dog { name: "Rex" };
		let nothing = null;
		let result = "\(d?.greet()) \(isNull(nothing?.greet()))";`

	res := evalSource(t, source)
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}
	for label, out := range map[string]string{
		"evaluator": mustEvaluatorEval(t, res.Program, "result"),
		"VM":        mustVMEval(t, res.Program, "result"),
	} {
		if out != "Woof true" {
			t.Errorf("%s produced %q, want %q", label, out, "Woof true")
		}
	}
}

// TestOptionalFieldsAcceptTheirBaseType covers the rule that makes `?.` usable
// at all.
//
// `?Address` is sugar for `Address | Null`, so an `Address` value belongs in it.
// It did not: the checker left the optional's base as a placeholder name with no
// fields, so the placeholder and the declared struct were unequal and every
// assignment was rejected. A `?Address` field was a type nothing could be put
// into — and an optional field is the ordinary reason a receiver can be null,
// which is the only thing `?.` has to guard.
func TestOptionalFieldsAcceptTheirBaseType(t *testing.T) {
	source := `struct Address { city: String }
		struct User { name: String, address: ?Address }
		let missing = "-";
		let full = User { name: "Ali", address: Address { city: "Dhaka" } };
		let bare = User { name: "Bo", address: null };
		let result = "\(full?.address?.city ?? missing) \(bare?.address?.city ?? missing)";`

	res := evalSource(t, source)
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}
	for label, out := range map[string]string{
		"evaluator": mustEvaluatorEval(t, res.Program, "result"),
		"VM":        mustVMEval(t, res.Program, "result"),
	} {
		if out != "Dhaka -" {
			t.Errorf("%s produced %q, want %q", label, out, "Dhaka -")
		}
	}
}

// TestOptionalsAreLowered pins that neither node reaches a backend. A backend
// that received one would either refuse it — which the guards turn into a clear
// internal error — or, before the guards existed, silently emit nothing and
// produce a wrong answer.
func TestOptionalsAreLowered(t *testing.T) {
	res := evalSource(t, `struct P { name: String }
		let nothing = null;
		let a = nothing?.name ?? "d";
		let result = a;`)
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}
	if res.Lowered == nil {
		t.Fatal("no lowered program")
	}

	ast.Inspect(res.Lowered, func(node ast.Node) bool {
		switch node.(type) {
		case *ast.CoalesceExpression:
			t.Error("a `??` survived lowering")
		case *ast.SafeAccessExpression:
			t.Error("a `?.` survived lowering")
		}
		return true
	})
}

// TestUnloweredOptionalsAreRefused checks the backends' guards directly.
//
// Without these the compiler emitted nothing for the operator and the VM
// underflowed its stack while the evaluator carried on and produced a wrong
// answer. Two engines disagreeing about the same program is the worst outcome
// available, and the guard turns it into one clear message.
func TestUnloweredOptionalsAreRefused(t *testing.T) {
	res := frontend.Check(`let a = null; let r = a ?? "d";`, frontend.Options{
		Filename:     "x.nil",
		SkipLowering: false,
	})
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}

	// Compile the *source* program, bypassing lowering the way a bug in the
	// lowering pass would.
	err := compileWithoutLowering(res.Program)
	if err == nil {
		t.Fatal("the compiler accepted an unlowered `??`; it must refuse")
	}
	if !strings.Contains(err.Error(), "without being lowered") {
		t.Fatalf("error does not say what went wrong: %v", err)
	}
}
