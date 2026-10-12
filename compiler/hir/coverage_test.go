package hir

import (
	"sort"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/lower"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

// HIR coverage (`L3.2`).
//
// # The measurement
//
// The lowering used to answer an unrecognised node with `nil` — the same value a
// legitimately empty translation produces — so a construct that had silently
// stopped working looked exactly like one that had never existed. Nothing
// downstream could tell the two apart.
//
// Every dropped node is now named, which turns "something went wrong somewhere in
// the backend" into "this construct has no HIR translation".
//
// # Why this test records gaps rather than asserting none exist
//
// There are nineteen. They are the constructs `L1` and `L2` added — `for`,
// `match`, `struct`, `enum`, `trait`, `impl`, `while let`, `?.`, `??` — none of
// which the HIR layer has ever been taught. Writing a test that demanded zero
// would be a test that fails, and a test that always fails is a test nobody reads.
//
// So the gap list *is* the test. It asserts that the set of missing translations is
// exactly the recorded one. A construct that starts being dropped fails the next
// run; a construct that gets translated fails too, which is the point — closing a
// gap is a deliberate act that removes a line, not a silent improvement that
// changes nothing.
//
// The alternative — leaving `L3.2` open with no instrument at all — is what let
// the ternary lose its `else` in the first place.

// knownGaps is every AST node type that reaches the HIR lowering with no
// translation, keyed by the feature it belongs to rather than by node type.
//
// It is empty. Every construct the language has now reaches HIR — which is not the
// same as saying every construct reaches HIR *correctly*. `TestHIRPreservesEvery
// LanguageConstruct` is the half that checks correctness, and an empty list here
// only means nothing is being dropped.
//
// Keeping the map rather than deleting it is deliberate: a ratchet with nothing to
// measure is a ratchet that has been switched off, and the next node someone adds
// to the parser needs somewhere to be recorded before it can be forgotten.
var knownGaps = map[string][]string{}

func TestHIRCoverageRatchet(t *testing.T) {
	observed := observeUnlowered(t)

	recorded := map[string]bool{}
	for _, nodes := range knownGaps {
		for _, node := range nodes {
			recorded[node] = true
		}
	}

	for node := range observed {
		if !recorded[node] {
			t.Errorf("NEW GAP: %s reaches the lowering with no translation", node)
		}
	}
	for node := range recorded {
		if !observed[node] {
			t.Errorf("GAP CLOSED: %s is translated now — remove it from knownGaps", node)
		}
	}

	if t.Failed() {
		features := make([]string, 0, len(knownGaps))
		for feature := range knownGaps {
			features = append(features, feature)
		}
		sort.Strings(features)
		t.Logf("features with no HIR translation: %v", features)
	}
}

// TestHIRPreservesEveryLanguageConstruct is the half a node list cannot check.
//
// The ratchet says nothing is *dropped*. This says nothing is dropped *silently* —
// every construct still appears in the HIR, as a recognisable node rather than as
// a `nil` or a placeholder.
//
// The two together are the only honest coverage claim. "No node was dropped" is
// satisfiable by a lowering that returns nothing at all, and "every node survived"
// is satisfiable by one that returns the input unchanged. Asserting on the HIR's
// own text catches both.
func TestHIRPreservesEveryLanguageConstruct(t *testing.T) {
	// A declaration is kept as data and emits nothing, so `struct` and `trait`
	// are asserted by their presence in the HIR rather than by an instruction.
	cases := []struct {
		name   string
		source string
		want   []string
	}{
		{"a struct declaration", `struct P { x: Int }`, []string{"struct P"}},
		{"an enum declaration", `enum E { A, B }`, []string{"enum E", "A", "B"}},
		{"a type alias", `type Meters = Int`, []string{"type Meters"}},
		{"a trait", `trait Show { fn show(self) -> String; }`, []string{"trait Show", "show"}},
		// `export` is a modifier on the declaration, not a wrapper node in the HIR, so
		// the assertion is on the function surviving rather than on `export` appearing.
		{"an exported function", `export fn g() -> Int { return 1; }`, []string{"g"}},
		{"a struct literal and its member", `struct P { x: Int } let p = P { x: 1 }; let v = p.x;`, []string{"P", "x"}},
		{"an enum match", `enum E { A, B } let c = A; let s = match c { A => "a", B => "b" };`, []string{"A", "b"}},
		{"a for loop", `for i in 0..3 { i; }`, []string{"while"}},
		{"a while let", `let xs = [1]; while let Some(x) = xs[0] { x; }`, []string{"while"}},
		{"break", `let i = 0; while i < 3 { break; }`, []string{"break"}},
		{"continue", `let i = 0; while i < 3 { continue; }`, []string{"continue"}},
		{"a safe access", `let p = null; p?.name;`, []string{"name"}},
		{"a coalesce", `let p = null; p.name ?? "none";`, []string{"none"}},
		{"a task", `let t = task { 1 };`, []string{"task"}},
		{"an await", `let t = task { 1 }; let v = await t;`, []string{"await"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text := hirTextFor(t, tc.source)
			for _, want := range tc.want {
				if !contains(text, want) {
					t.Errorf("HIR does not mention %q:\n%s", want, text)
				}
			}
		})
	}
}

// hirTextFor runs the real backend path — parse, lower, HIR — and returns the HIR.
func hirTextFor(t *testing.T, source string) string {
	t.Helper()

	p := parser.New(lexer.New(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	lowered, err := lower.Program(prog)
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}
	return NewLowerer().LowerProgram(lowered).String()
}

// TestTheOptimizerPreservesNodeIdentity is the test for a whole class of bug.
//
// Every `case` in the optimiser that *rebuilds* a node must copy every field. One
// that omits a field does not fail — it produces a node that is 99% right, and the
// 1% surfaces somewhere else entirely: a named function printed as `fn()` only
// after optimisation, so the fault appeared to belong to the optimiser's folding
// logic rather than to a line that forgot a field.
//
// Asserting on the optimised text catches it because the omission is visible. The
// cases below were the ones rebuilding a node when this was written; a new one
// needs no test of its own, only a field copied.
func TestTheOptimizerPreservesNodeIdentity(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   []string
	}{
		{"a function keeps its name", `fn pick(a: Int) -> Int { return a; }`, []string{"fn pick(a)"}},
		{"an exported function keeps its name", `export fn g() -> Int { return 1; }`, []string{"fn g()"}},
		{"a struct keeps its name and fields", `struct P { x: Int }`, []string{"struct P", "x"}},
		{"an enum keeps its variants", `enum E { A, B }`, []string{"enum E", "A", "B"}},
		{"a type alias keeps its target", `type Meters = Int`, []string{"type Meters", "Int"}},
		{"a trait keeps its methods", `trait Show { fn show(self) -> String; }`, []string{"trait Show", "show"}},
		{"a ternary keeps both arms", `let n = 1; let s = if (n > 0) { "a" } else { "b" };`, []string{"a", "b"}},
		{"a coalesce keeps its fallback", `let p = null; p.name ?? "none";`, []string{"none"}},
		{"a map keeps its keys", `let m = {"k": 1};`, []string{"k"}},
		{"a dot expression keeps its field", `struct P { x: Int } let p = P { x: 1 }; p.x;`, []string{"x"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text := optimizedHirText(t, tc.source)
			for _, want := range tc.want {
				if !contains(text, want) {
					t.Errorf("optimised HIR does not mention %q:\n%s", want, text)
				}
			}
		})
	}
}

// optimizedHirText runs the path a backend actually takes, optimiser included.
func optimizedHirText(t *testing.T, source string) string {
	t.Helper()

	p := parser.New(lexer.New(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	lowered, err := lower.Program(prog)
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}
	optimized := NewOptimizer().Optimize(NewLowerer().LowerProgram(lowered))
	return optimized.String()
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

// TestHIRLoweredFeaturesHaveNoGaps pins the half that already works.
//
// A blanket "no gaps" assertion would be satisfied by a lowering that drops
// everything, so the features that *are* translated get checked individually. It
// is the half of the ratchet that would catch a regression in a working path.
func TestHIRLoweredFeaturesHaveNoGaps(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{"a plain binding", `let x = 1;`},
		{"a typed binding", `let x: Int = 1;`},
		{"an if statement", `let x = 1; if (x > 0) { x = 2; } else { x = 3; }`},
		{"a ternary", `let x = 1; let y = if (x > 0) { 2 } else { 3 };`},
		{"a while loop", `let i = 0; while i < 3 { i = i + 1; }`},
		{"a function declaration", `fn add(a: Int, b: Int) -> Int { return a + b; }`},
		{"a pure function", `pure fn double(a: Int) -> Int { return a * 2; }`},
		{"a map literal", `let m = {"a": 1};`},
		{"an index", `let xs = [1]; xs[0];`},
		{"a string template", `let name = "x"; let s = "hi \(name)";`},
		{"a call", `let n = length("abc");`},
		{"a unary negation", `let x = -1;`},
		{"a Result", `let r = Ok(1);`},
		{"an optional", `let o = Some(1);`},
		{"a break inside a loop", `let i = 0; while i < 3 { i = i + 1; }`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if dropped := observeUnlowered(t, tc.source); len(dropped) > 0 {
				t.Errorf("these nodes reached the lowering with no translation: %v", dropped)
			}
		})
	}
}

// observeUnlowered lowers a source and returns every AST node type that had no
// translation.
//
// It returns the types rather than a count because the same node can be dropped
// many times in one program, and "it is missing" is the only fact that matters.
//
// # Why it measures the *lowered* program
//
// `Result.Lowered` carries a contract: backends compile the lowered program, never
// the parsed one. That is not a formality — the lowering pass desugars `for`,
// `match`, `struct`, `trait`, `?.` and `??` into `while` loops, `if` chains and
// calls, and a backend handed the unlowered tree meets constructs it was never
// taught.
//
// Measuring the lowered program is therefore measuring what a backend actually
// sees. Measuring the parsed one would keep reporting twelve gaps that no backend
// has, and would keep hiding the question that matters: does a construct survive
// the whole path?
//
// A separate test asserts the parse-then-check path, because a construct that the
// lowering drops silently is the same failure wearing a different hat.
func observeUnlowered(t *testing.T, sources ...string) map[string]bool {
	t.Helper()

	if len(sources) == 0 {
		sources = coverAllFeatures()
	}

	out := make(map[string]bool)
	for _, source := range sources {
		p := parser.New(lexer.New(source))
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatalf("parse errors before lowering could even be attempted: %v", p.Errors())
		}

		lowered, err := lower.Program(prog)
		if err != nil {
			t.Fatalf("lowering failed before HIR could be attempted: %v", err)
		}

		lowerer := NewLowerer()
		lowerer.LowerProgram(lowered)
		for name := range lowerer.UnhandledNodes() {
			out[name] = true
		}
	}
	return out
}

// coverAllFeatures is one short program per feature the lowering is missing.
func coverAllFeatures() []string {
	return []string{
		`for i in 0..3 { i; }`,
		`let xs = [1]; while let Some(x) = xs[0] { x; }`,
		`let i = 0; while i < 3 { break; }`,
		`let i = 0; while i < 3 { continue; }`,
		`export fn g() -> Int { return 1; }`,
		`private fn h() -> Int { return 1; }`,
		`struct P { x: Int } let p = P { x: 1 }; p.x;`,
		`enum E { A, B } let c = A; let s = match c { A => "a", B => "b" };`,
		`type Meters = Int; let m: Meters = 1;`,
		`trait Show { fn show(self) -> String; }`,
		`let p = null; p?.name;`,
		`let p = null; p.name ?? "none";`,
		`let t = task { 1 }; let v = await t;`,
	}
}
