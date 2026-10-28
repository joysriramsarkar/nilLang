package ast_test

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

func parseProgram(t *testing.T, src string) *ast.Program {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v\nsource:\n%s", p.Errors(), src)
	}
	return prog
}

// calledNames returns the names of every plain function call Inspect reaches,
// in walk order. Call collection is what Inspect's real consumers do (the
// capability scanner in cmd/nil collects CallExpressions this way), so it is
// the behaviour worth pinning.
func calledNames(t *testing.T, src string) []string {
	t.Helper()
	var names []string
	ast.Inspect(parseProgram(t, src), func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpression); ok {
			if name, ok := call.Function.(*ast.Identifier); ok {
				names = append(names, name.Value)
			}
		}
		return true
	})
	return names
}

func assertCalls(t *testing.T, src string, want ...string) {
	t.Helper()
	got := calledNames(t, src)
	if len(got) != len(want) {
		t.Fatalf("Inspect collected calls %v, want %v\nsource:\n%s", got, want, src)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Inspect collected calls %v, want %v\nsource:\n%s", got, want, src)
		}
	}
}

// TestInspectReachesInsideTaskAndAwait pins the fix for await/task being
// treated as leaves: the calls inside either wrapper were invisible to every
// Inspect-based analysis.
func TestInspectReachesInsideTaskAndAwait(t *testing.T) {
	assertCalls(t, `let f = task { return g(1); };`, "g")
	assertCalls(t, `let w = await h(2);`, "h")
	assertCalls(t, `let f = task { return await g(1); };`, "g")
	assertCalls(t, `let w = await task { return g(1); };`, "g")
	assertCalls(t, `let y = top(1);`, "top")
}

// TestInspectReachesInsideImplMethods pins the ImplDecl case: a method body
// is where an impl keeps its expressions, and the walk once skipped the
// whole declaration.
func TestInspectReachesInsideImplMethods(t *testing.T) {
	src := `
trait Greeter { fn greet(self) -> Int }
struct Dog { name: String }
impl Greeter for Dog { fn greet(self) -> Int { return compute(self); } }
`
	assertCalls(t, src, "compute")
}

// TestInspectReachesInsideTraitDecl checks the trait's own subtree: the
// declaration name and the methods' parameter names are visited even though
// a trait carries no bodies.
func TestInspectReachesInsideTraitDecl(t *testing.T) {
	idents := map[string]bool{}
	ast.Inspect(parseProgram(t, `trait Greeter { fn greet(self) -> Int }`), func(n ast.Node) bool {
		if id, ok := n.(*ast.Identifier); ok {
			idents[id.Value] = true
		}
		return true
	})
	for _, want := range []string{"Greeter", "self"} {
		if !idents[want] {
			t.Errorf("Inspect did not visit identifier %q inside the trait declaration (visited: %v)", want, idents)
		}
	}
}

// TestInspectReachesInsideExtern pins the ExternStatement case: the walk
// visits the signature's FunctionLiteral instead of skipping the declaration.
func TestInspectReachesInsideExtern(t *testing.T) {
	fnCount := 0
	ast.Inspect(parseProgram(t, `extern "C" fn measure(x) -> Int;`), func(n ast.Node) bool {
		if _, ok := n.(*ast.FunctionLiteral); ok {
			fnCount++
		}
		return true
	})
	if fnCount != 1 {
		t.Fatalf("Inspect visited %d FunctionLiterals inside the extern declaration, want 1", fnCount)
	}
}
