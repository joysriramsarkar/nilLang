package parser

// TestEBNFStatementProductions — P0.1 grammar freeze (docs/CORE_STABILIZATION.md).
//
// Every Statement production in docs/spec/LANGUAGE_SPEC.md §3 (EBNF Summary)
// must parse to the documented AST shape. If the parser and the EBNF disagree,
// this table fails: fix the parser (mismatch fix, freeze-compliant) or fix the
// spec — never silently drift.

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
)

func parseOne(t *testing.T, src string) *ast.Program {
	t.Helper()
	p := New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("source %q: unexpected parser errors: %q", src, p.Errors())
	}
	if len(prog.Statements) != 1 {
		t.Fatalf("source %q: expected 1 statement, got %d", src, len(prog.Statements))
	}
	return prog
}

func exprStmt(t *testing.T, src string) *ast.ExpressionStatement {
	t.Helper()
	stmt, ok := parseOne(t, src).Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("source %q: expected *ast.ExpressionStatement, got %T", src, parseOne(t, src).Statements[0])
	}
	return stmt
}

func TestEBNFStatementProductions(t *testing.T) {
	t.Run("LetStatement/plain", func(t *testing.T) {
		s, ok := parseOne(t, `let x = 5;`).Statements[0].(*ast.LetStatement)
		if !ok {
			t.Fatalf("got %T", parseOne(t, `let x = 5;`).Statements[0])
		}
		if s.Name.Value != "x" || s.Constant || s.Value == nil {
			t.Fatalf("unexpected let: %+v", s)
		}
	})

	t.Run("LetStatement/annotated", func(t *testing.T) {
		s := parseOne(t, `let count: Int = 0;`).Statements[0].(*ast.LetStatement)
		if s.Type != "Int" {
			t.Fatalf("annotation lost: %+v", s)
		}
	})

	t.Run("LetStatement/no-initializer", func(t *testing.T) {
		s := parseOne(t, `let x;`).Statements[0].(*ast.LetStatement)
		if s.Value != nil {
			t.Fatalf("expected nil value, got %+v", s.Value)
		}
	})

	t.Run("LetStatement/annotated-no-initializer", func(t *testing.T) {
		s := parseOne(t, `let x: Int;`).Statements[0].(*ast.LetStatement)
		if s.Type != "Int" || s.Value != nil {
			t.Fatalf("unexpected let: %+v", s)
		}
	})

	t.Run("ConstStatement", func(t *testing.T) {
		s := parseOne(t, `const k = 1;`).Statements[0].(*ast.LetStatement)
		if !s.Constant {
			t.Fatalf("const must set Constant flag: %+v", s)
		}
	})

	t.Run("AssignStatement/plain", func(t *testing.T) {
		s := parseOne(t, `x = 1;`).Statements[0].(*ast.AssignStatement)
		if s.Operator != "=" {
			t.Fatalf("unexpected operator %q", s.Operator)
		}
	})

	t.Run("AssignStatement/compound", func(t *testing.T) {
		for _, tc := range []struct{ src, op string }{
			{`x += 1;`, "+="},
			{`x -= 1;`, "-="},
		} {
			s := parseOne(t, tc.src).Statements[0].(*ast.AssignStatement)
			if s.Operator != tc.op {
				t.Fatalf("source %q: unexpected operator %q", tc.src, s.Operator)
			}
		}
	})

	t.Run("ReturnStatement", func(t *testing.T) {
		fn := exprStmt(t, `fn f() -> Int { return 1; }`).Expression.(*ast.FunctionLiteral)
		if fn.Name != "f" || fn.ReturnType != "Int" {
			t.Fatalf("unexpected fn: %+v", fn)
		}
		ret, ok := fn.Body.Statements[0].(*ast.ReturnStatement)
		if !ok || ret.ReturnValue == nil {
			t.Fatalf("expected return with value, got %+v", fn.Body.Statements[0])
		}
	})

	t.Run("FunctionDecl/forms", func(t *testing.T) {
		for _, tc := range []struct {
			src   string
			check func(*ast.FunctionLiteral) bool
		}{
			{`fn f() { x; }`, func(f *ast.FunctionLiteral) bool { return f.Name == "f" && len(f.Body.Statements) == 1 }},
			{`fn f(a: Int) -> Int { return a; }`, func(f *ast.FunctionLiteral) bool { return len(f.Parameters) == 1 && f.ReturnType == "Int" }},
			{`fn f() requires "net" { x; }`, func(f *ast.FunctionLiteral) bool { return len(f.Requires) == 1 && f.Requires[0] == "net" }},
			{`pure fn f() { x; }`, func(f *ast.FunctionLiteral) bool { return f.IsPure }},
			{`unsafe fn f() { x; }`, func(f *ast.FunctionLiteral) bool { return f.IsUnsafe }},
			{`fn id<T>(x: T) -> T { return x; }`, func(f *ast.FunctionLiteral) bool { return f.Name == "id" }},
		} {
			fn, ok := exprStmt(t, tc.src).Expression.(*ast.FunctionLiteral)
			if !ok {
				t.Fatalf("source %q: not a function literal: %T", tc.src, exprStmt(t, tc.src).Expression)
			}
			if !tc.check(fn) {
				t.Fatalf("source %q: shape check failed: %+v", tc.src, fn)
			}
		}
	})

	t.Run("IfStatement", func(t *testing.T) {
		ie, ok := exprStmt(t, `if (x) { y; } else { z; }`).Expression.(*ast.IfExpression)
		if !ok {
			t.Fatal("not an if expression")
		}
		if ie.Condition == nil || ie.Consequence == nil || ie.Alternative == nil {
			t.Fatalf("missing if parts: %+v", ie)
		}
		if len(ie.Consequence.Statements) != 1 || len(ie.Alternative.Statements) != 1 {
			t.Fatal("if branches must be block statements with 1 statement each")
		}
	})

	t.Run("WhileStatement", func(t *testing.T) {
		s := parseOne(t, `while (x) { y; }`).Statements[0].(*ast.WhileStatement)
		if s.Condition == nil || s.Body == nil || len(s.Body.Statements) != 1 {
			t.Fatalf("unexpected while: %+v", s)
		}
	})

	t.Run("WhileLetStatement", func(t *testing.T) {
		if _, ok := parseOne(t, `while let m = opt { x; }`).Statements[0].(*ast.WhileLetStatement); !ok {
			t.Fatal("not a while-let statement")
		}
	})

	t.Run("ForStatement", func(t *testing.T) {
		s := parseOne(t, `for i in items { x; }`).Statements[0].(*ast.ForStatement)
		if s.Body == nil || s.Iterable == nil {
			t.Fatalf("unexpected for: %+v", s)
		}
		kv := parseOne(t, `for k, v in items { x; }`).Statements[0].(*ast.ForStatement)
		if kv.Key.Value != "k" || kv.Value.Value != "v" {
			t.Fatalf("unexpected for bindings: %+v", kv)
		}
	})

	t.Run("StructDecl", func(t *testing.T) {
		s := parseOne(t, `struct User { id: Int, name: String }`).Statements[0].(*ast.StructDecl)
		if s.Name.Value != "User" || len(s.Fields) != 2 {
			t.Fatalf("unexpected struct: %+v", s)
		}
	})

	t.Run("StructLiteral", func(t *testing.T) {
		s := parseOne(t, `let u = User { id: 1 };`).Statements[0].(*ast.LetStatement)
		if _, ok := s.Value.(*ast.StructLiteral); !ok {
			t.Fatalf("not a struct literal: %T", s.Value)
		}
	})

	t.Run("EnumDecl", func(t *testing.T) {
		s := parseOne(t, `enum Status { Pending, Active(Int) }`).Statements[0].(*ast.EnumDecl)
		if s.Name.Value != "Status" || len(s.Variants) != 2 {
			t.Fatalf("unexpected enum: %+v", s)
		}
	})

	t.Run("TraitDecl", func(t *testing.T) {
		s := parseOne(t, `trait Serializable { fn serialize() -> String; }`).Statements[0].(*ast.TraitDecl)
		if s.Name.Value != "Serializable" || len(s.Methods) != 1 {
			t.Fatalf("unexpected trait: %+v", s)
		}
	})

	t.Run("ImplDecl", func(t *testing.T) {
		s := parseOne(t, `impl Printer for User { fn print() { x; } }`).Statements[0].(*ast.ImplDecl)
		if s.Trait.Value != "Printer" || s.Type.Value != "User" {
			t.Fatalf("unexpected impl: %+v", s)
		}
	})

	t.Run("VisibilityStatement/export", func(t *testing.T) {
		s := parseOne(t, `export fn f() { x; }`).Statements[0].(*ast.ExportStatement)
		if s.Visibility != ast.VisibilityExported {
			t.Fatalf("unexpected visibility %q", s.Visibility)
		}
		fn, ok := s.Decl.(*ast.ExpressionStatement).Expression.(*ast.FunctionLiteral)
		if !ok || fn.Name != "f" {
			t.Fatalf("export must wrap the fn declaration: %+v", s.Decl)
		}
	})

	t.Run("VisibilityStatement/private", func(t *testing.T) {
		s := parseOne(t, `private let x = 1;`).Statements[0].(*ast.ExportStatement)
		if s.Visibility != ast.VisibilityPrivate {
			t.Fatalf("unexpected visibility %q", s.Visibility)
		}
		if _, ok := s.Decl.(*ast.LetStatement); !ok {
			t.Fatalf("private must wrap the declaration: %+v", s.Decl)
		}
	})

	t.Run("TypeAliasDecl", func(t *testing.T) {
		if _, ok := parseOne(t, `type ID = Int;`).Statements[0].(*ast.TypeAliasDecl); !ok {
			t.Fatal("not a type alias")
		}
	})

	t.Run("MatchExpression", func(t *testing.T) {
		s := parseOne(t, `let r = match x { 1 => 2, _ => 3 };`).Statements[0].(*ast.LetStatement)
		m, ok := s.Value.(*ast.MatchExpression)
		if !ok || m.Subject == nil || len(m.Arms) != 2 {
			t.Fatalf("unexpected match: %+v", s.Value)
		}
	})

	t.Run("ComponentDeclaration", func(t *testing.T) {
		s := parseOne(t, `component C { state s: Int = 0; }`).Statements[0].(*ast.ComponentLiteral)
		if s.Name.Value != "C" || len(s.States) != 1 {
			t.Fatalf("unexpected component: %+v", s)
		}
	})

	t.Run("AppStatement", func(t *testing.T) {
		s := parseOne(t, `app H { let x = 1; }`).Statements[0].(*ast.AppStatement)
		if s.Name.Value != "H" || len(s.Body.Statements) != 1 {
			t.Fatalf("unexpected app: %+v", s)
		}
	})

	t.Run("StateDeclaration", func(t *testing.T) {
		s := parseOne(t, `state s: Int = 0;`).Statements[0].(*ast.StateDeclaration)
		if s.Name.Value != "s" {
			t.Fatalf("unexpected state: %+v", s)
		}
	})

	t.Run("ExpressionStatement/call", func(t *testing.T) {
		if _, ok := exprStmt(t, `foo(1, 2);`).Expression.(*ast.CallExpression); !ok {
			t.Fatal("not a call expression")
		}
	})

	t.Run("CoalesceExpression", func(t *testing.T) {
		s := parseOne(t, `let a = b ?? c;`).Statements[0].(*ast.LetStatement)
		if _, ok := s.Value.(*ast.CoalesceExpression); !ok {
			t.Fatalf("not a coalesce expression: %T", s.Value)
		}
	})

	t.Run("SafeAccessExpression", func(t *testing.T) {
		s := parseOne(t, `let a = b?.c;`).Statements[0].(*ast.LetStatement)
		if _, ok := s.Value.(*ast.SafeAccessExpression); !ok {
			t.Fatalf("not a safe-access expression: %T", s.Value)
		}
	})
}

// TestControlBodiesAreBlockStatements locks the resolution of the EBNF
// BlockStatement ambiguity: a bare `{ ... }` at statement position parses as a
// composite literal, NOT a block (see TestBareBraceIsNotABlockStatement), while
// every control body IS a *ast.BlockStatement.
func TestControlBodiesAreBlockStatements(t *testing.T) {
	ie := exprStmt(t, `if (x) { y; }`).Expression.(*ast.IfExpression)
	if ie.Consequence == nil {
		t.Fatal("if body is not a block statement")
	}
	w := parseOne(t, `while (x) { y; }`).Statements[0].(*ast.WhileStatement)
	if w.Body == nil {
		t.Fatal("while body is not a block statement")
	}
	f := parseOne(t, `for i in items { x; }`).Statements[0].(*ast.ForStatement)
	if f.Body == nil {
		t.Fatal("for body is not a block statement")
	}
}

func TestBareBraceIsNotABlockStatement(t *testing.T) {
	// EBNF §3 lists BlockStatement as a Statement alternative, but a bare
	// `{ x; }` at statement position is a composite-literal attempt and fails.
	// TODO(P0.1): resolve one way — remove BlockStatement from the Statement
	// alternatives in the spec, or add parser lookahead that disambiguates
	// block vs literal. Until then this test pins the implemented behavior.
	p := New(lexer.New(`{ x; }`))
	p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("bare brace must not silently parse as a block statement")
	}
}

// TestImplementedButUnspeced pins constructs the parser accepts that have NO
// EBNF production in LANGUAGE_SPEC.md §3. Each is a spec gap, not a parser bug:
// TODO(P0.1): add EBNF productions for import/entity/style/extern.
func TestImplementedButUnspeced(t *testing.T) {
	if _, ok := parseOne(t, `import "web";`).Statements[0].(*ast.ImportStatement); !ok {
		t.Fatal("import must parse")
	}
	if _, ok := parseOne(t, `entity User { id: Int; }`).Statements[0].(*ast.EntityStatement); !ok {
		t.Fatal("entity must parse")
	}
	if _, ok := parseOne(t, `style Primary { color: "red"; }`).Statements[0].(*ast.StyleStatement); !ok {
		t.Fatal("style must parse")
	}
	ext, ok := parseOne(t, `extern "C" fn sin(x: Float) -> Float;`).Statements[0].(*ast.ExternStatement)
	if !ok {
		t.Fatal("extern must parse")
	}
	if ext.Abi != "C" || ext.Fn == nil || ext.Fn.Name != "sin" {
		t.Fatalf("unexpected extern: %+v", ext)
	}
}
