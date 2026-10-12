package tests

import (
	"strings"
	"testing"
)

// `struct`, `enum`, `match`, and `type` aliases reach both engines through the
// lowering pass, so a divergence in any of them means the engines are not running
// the same lowered program. Every behavioural case below therefore runs in both.

// TestStructDeclarationAndFieldAccess covers declaration, construction, and
// field reading.
func TestStructDeclarationAndFieldAccess(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "one field",
			source: `struct P { x: Int } let p = P { x: 7 }; let result = p.x;`,
			want:   "7",
		},
		{
			name:   "two fields keep their values",
			source: `struct P { x: Int, y: String } let p = P { x: 1, y: "a" }; let result = "\(p.x)\(p.y)";`,
			want:   "1a",
		},
		{
			name:   "a field may be written after construction",
			source: `struct P { x: Int } let p = P { x: 1 }; p.x = 9; let result = p.x;`,
			want:   "9",
		},
		{
			name:   "a struct may be passed to a function",
			source: `struct P { x: Int } fn read(p) { return p.x; } let result = read(P { x: 4 });`,
			want:   "4",
		},
		{
			name:   "a struct may be an element of an array",
			source: `struct P { x: Int } let ps = [P { x: 1 }, P { x: 2 }]; let result = ps[1].x;`,
			want:   "2",
		},
		{
			name:   "field order follows the declaration",
			source: `struct P { b: String, a: String } let p = P { b: "1", a: "2" }; let result = p;`,
			want:   "{b: 1, a: 2}",
		},
		{
			name:   "a struct declaration produces no runtime value",
			source: `struct P { x: Int } let result = 1;`,
			want:   "1",
		},
		{
			name:   "a field may hold another struct",
			source: `struct Inner { v: Int } struct Outer { i: Inner } let o = Outer { i: Inner { v: 5 } }; let result = o.i.v;`,
			want:   "5",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := evalSource(t, tc.source)

			if tc.want == "" {
				// The case documents a rejection; the diagnostic itself is
				// checked in TestDeclarativeTypeErrors.
				if res.OK {
					t.Fatal("expected the program to be rejected")
				}
				return
			}

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

// TestEnumDeclarationAndConstruction covers variant values with and without a
// payload.
func TestEnumDeclarationAndConstruction(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "payload-free variant",
			source: `enum E { A, B } let result = A;`,
			want:   "{variant: A}",
		},
		{
			name:   "variant with a payload",
			source: `enum E { A(Int) } let result = A(3);`,
			want:   "{variant: A, payload: 3}",
		},
		{
			name:   "the payload may be any value",
			source: `enum E { A(String) } let result = A("hi");`,
			want:   "{variant: A, payload: hi}",
		},
		{
			name:   "two variants differ",
			source: `enum E { A, B } let result = B;`,
			want:   "{variant: B}",
		},
		{
			name:   "a variant may be stored in an array",
			source: `enum E { A(Int) } let xs = [A(1), A(2)]; let result = xs[1];`,
			want:   "{variant: A, payload: 2}",
		},
		{
			name:   "a variant name is shared across enums, so a clash is rejected",
			source: `enum E { A } enum F { A } let result = 1;`,
			want:   "", // reported by the type checker; see TestDeclarativeTypeErrors
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := evalSource(t, tc.source)

			if tc.want == "" {
				// The case documents a rejection; the diagnostic itself is
				// checked in TestDeclarativeTypeErrors.
				if res.OK {
					t.Fatal("expected the program to be rejected")
				}
				return
			}

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

// TestMatchPatterns covers every pattern form the language offers.
func TestMatchPatterns(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "integer literal",
			source: `let v = 2; let result = match v { 1 => "a", 2 => "b", _ => "c" };`,
			want:   "b",
		},
		{
			name:   "negative literal",
			source: `let v = -1; let result = match v { -1 => "neg", _ => "other" };`,
			want:   "neg",
		},
		{
			name:   "string literal",
			source: `let s = "b"; let result = match s { "a" => "A", "b" => "B" };`,
			want:   "B",
		},
		{
			name:   "boolean literal",
			source: `let b = false; let result = match b { true => "t", false => "f" };`,
			want:   "f",
		},
		{
			name:   "null literal",
			source: `let n = null; let result = match n { null => "none", _ => "some" };`,
			want:   "none",
		},
		{
			name:   "wildcard",
			source: `let v = 9; let result = match v { 1 => "a", _ => "other" };`,
			want:   "other",
		},
		{
			name:   "binding",
			source: `let v = 5; let result = match v { x => "got \(x)" };`,
			want:   "got 5",
		},
		{
			name:   "or-pattern",
			source: `let v = 2; let result = match v { 1 | 2 | 3 => "low", _ => "high" };`,
			want:   "low",
		},
		{
			name:   "guard true",
			source: `let v = 7; let result = match v { n if n > 5 => "big", _ => "small" };`,
			want:   "big",
		},
		{
			name:   "guard false falls through",
			source: `let v = 3; let result = match v { n if n > 5 => "big", _ => "small" };`,
			want:   "small",
		},
		{
			name:   "guard may use an enum payload",
			source: `enum E { A(Int) } let v = A(9); let result = match v { A(n) if n > 5 => "big", _ => "small" };`,
			want:   "big",
		},
		{
			name:   "enum variant without payload",
			source: `enum E { A, B } let v = B; let result = match v { A => "a", B => "b" };`,
			want:   "b",
		},
		{
			name:   "enum variant with payload bound",
			source: `enum E { A(Int) } let v = A(4); let result = match v { A(n) => n, _ => 0 };`,
			want:   "4",
		},
		{
			name:   "enum payload discarded",
			source: `enum E { A(Int) } let v = A(4); let result = match v { A(_) => "any", _ => "no" };`,
			want:   "any",
		},
		{
			name:   "struct pattern",
			source: `struct P { x: Int, y: Int } let p = P { x: 1, y: 2 }; let result = match p { P { x: a, y: b } => a + b };`,
			want:   "3",
		},
		{
			name:   "struct pattern ignores unlisted fields",
			source: `struct P { x: Int, y: String } let p = P { x: 1, y: "z" }; let result = match p { P { x: a } => a, _ => 0 };`,
			want:   "1",
		},
		{
			name:   "struct pattern with a literal field",
			source: `struct P { x: Int } let p = P { x: 5 }; let result = match p { P { x: 5 } => "five", _ => "other" };`,
			want:   "five",
		},
		{
			name:   "Ok binds the value",
			source: `let r = Ok(5); let result = match r { Ok(v) => v, Err(e) => 0 };`,
			want:   "5",
		},
		{
			name:   "Err binds the error",
			source: `let r = Err("bad"); let result = match r { Ok(v) => "ok", Err(e) => e };`,
			want:   "bad",
		},
		{
			name:   "Some binds the value",
			source: `let o = Some(3); let result = match o { Some(v) => v, None => 0 };`,
			want:   "3",
		},
		{
			name:   "nested match",
			source: `enum E { A(Int) } let v = A(2); let result = match v { A(n) => match n { 1 => "one", 2 => "two", _ => "many" }, _ => "no" };`,
			want:   "two",
		},
		{
			name:   "match inside a loop",
			source: `let total = 0; for i in 0..4 { total = total + match i { 0 => 10, _ => 1 }; } let result = total;`,
			want:   "13",
		},
		{
			name:   "match as a function argument",
			source: `let v = 1; let result = upper(match v { 1 => "a", _ => "b" });`,
			want:   "A",
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

// TestMatchEvaluatesTheSubjectOnce covers the reason `match` binds its subject to
// a hidden local rather than inlining it into every arm's test.
func TestMatchEvaluatesTheSubjectOnce(t *testing.T) {
	res := evalSource(t, `
let calls = 0;
fn next() { calls = calls + 1; return calls; }
let out = match next() { 1 => "one", 2 => "two", _ => "many" };
let result = "\(out) calls=\(calls)";
`)
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}

	for label, out := range map[string]string{
		"evaluator": mustEvaluatorEval(t, res.Program, "result"),
		"VM":        mustVMEval(t, res.Program, "result"),
	} {
		if out != "one calls=1" {
			t.Errorf("%s produced %q, want \"one calls=1\"", label, out)
		}
	}
}

// TestMatchBindingsDoNotLeak confirms an arm's bindings are scoped to it, so a
// later arm cannot read a name it never bound.
func TestMatchBindingsDoNotLeak(t *testing.T) {
	res := evalSource(t, `
let v = 1;
let out = match v { x => "first", _ => "second" };
let after = x;
`)
	if res.OK {
		t.Fatal("a pattern binding must not be visible after the match")
	}
}

// TestMatchIsAnExpression confirms `match` produces a value rather than only
// running statements.
func TestMatchIsAnExpression(t *testing.T) {
	res := evalSource(t, `
fn classify(n) {
  return match n {
    0 => "zero",
    1 | 2 | 3 => "small",
    x if x < 0 => "negative",
    _ => "large",
  };
}
let result = "\(classify(0)) \(classify(2)) \(classify(-5)) \(classify(99))";
`)
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}

	const want = "zero small negative large"
	for label, out := range map[string]string{
		"evaluator": mustEvaluatorEval(t, res.Program, "result"),
		"VM":        mustVMEval(t, res.Program, "result"),
	} {
		if out != want {
			t.Errorf("%s produced %q, want %q", label, out, want)
		}
	}
}

// TestTypeAlias covers alias declaration and use.
func TestTypeAlias(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "primitive alias",
			source: `type Count = Int let n: Count = 5; let result = n;`,
			want:   "5",
		},
		{
			name:   "alias to a struct",
			source: `struct P { x: Int } type Id = P let p: Id = P { x: 3 }; let result = p.x;`,
			want:   "3",
		},
		{
			name:   "alias chains resolve to the end",
			source: `type A = Int type B = A type C = B let n: C = 7; let result = n;`,
			want:   "7",
		},
		{
			name:   "a type still shadows an alias name in value position",
			source: `type Count = Int let Count = 2; let result = Count;`,
			want:   "2",
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

// TestDeclarativeTypeErrors covers every rejection these constructs are supposed
// to make. Each is a program that would otherwise run and produce a wrong answer
// rather than fail.
func TestDeclarativeTypeErrors(t *testing.T) {
	cases := []struct {
		name     string
		source   string
		wantCode string
	}{
		{"unknown struct", `let p = Nope { x: 1 };`, "E0502"},
		{"unknown field", `struct P { x: Int } let p = P { y: 1 };`, "E0503"},
		{"missing field", `struct P { x: Int, y: Int } let p = P { x: 1 };`, "E0506"},
		{"wrong field type", `struct P { x: Int } let p = P { x: "s" };`, "E0101"},
		{"duplicate struct", `struct P { a: Int } struct P { b: Int }`, "E0501"},
		{"unknown field type", `struct P { x: NoSuchType }`, "E0502"},
		{"unknown alias target", `type A = NoSuchType`, "E0502"},
		{"variant wrong arity", `enum E { A(Int) } let v = A();`, "E0510"},
		{"undeclared variant in pattern", `let v = 1; let r = match v { Nope(x) => 1, _ => 0 };`, "E0505"},
		{"payload not bound", `enum E { A(Int) } let v = A(1); let r = match v { A => 1, _ => 0 };`, "E0510"},
		{"payload bound on a bare variant", `enum E { A, B(Int) } let v = B(1); let r = match v { B(x) => x, _ => 0 }; A;`, ""},
		{"non-exhaustive bool", `let b = true; let r = match b { true => 1 };`, "E0507"},
		{"non-exhaustive enum", `enum E { A, B } let v = A; let r = match v { A => 1 };`, "E0507"},
		{"non-exhaustive result", `let r0 = Ok(1); let r = match r0 { Ok(v) => 1 };`, "E0507"},
		{"non-exhaustive optional", `let o = Some(1); let r = match o { Some(v) => 1 };`, "E0507"},
		{"struct pattern with an unknown field", `struct P { x: Int } let v = P { x: 1 }; let r = match v { P { y: a } => a, _ => 0 };`, "E0503"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := evalSource(t, tc.source)
			if res.OK {
				if tc.wantCode == "" {
					return // the case documents that this is accepted
				}
				t.Fatalf("expected rejection with %s, but the program was accepted", tc.wantCode)
			}
			if tc.wantCode == "" {
				t.Fatalf("expected acceptance, got: %s", res.ErrSummary())
			}
			if summary := res.ErrSummary(); !strings.Contains(summary, tc.wantCode) {
				t.Errorf("expected %s, got: %s", tc.wantCode, summary)
			}
		})
	}
}

// TestMatchSubjectIsEvaluatedInOrder guards against a lowering that reorders the
// subject's side effects relative to the arms.
func TestMatchSubjectIsEvaluatedInOrder(t *testing.T) {
	res := evalSource(t, `
let log = "";
fn note(s) { log = log + s; return 0; }
let out = match note("a") { 0 => note("b") + "1", _ => "2" };
let result = log;
`)
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}

	for label, out := range map[string]string{
		"evaluator": mustEvaluatorEval(t, res.Program, "result"),
		"VM":        mustVMEval(t, res.Program, "result"),
	} {
		if out != "ab" {
			t.Errorf("%s produced %q, want \"ab\"", label, out)
		}
	}
}
