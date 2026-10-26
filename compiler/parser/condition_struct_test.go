package parser

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/token"
)

// TestStructLiteralInConditionContexts pins the conditionDepth fix: a `{`
// inside a function literal, a call argument list or an array literal belongs
// to a struct literal or a nested body even when the whole construct sits in
// a `while` or `if` condition. The loop body that follows must still parse as
// a body.
func TestStructLiteralInConditionContexts(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"struct in fn body in while condition", `while f == fn() { return Point { x: 1 }; } { nop(); }`},
		{"struct as call argument in while condition", `while f(Point { x: 1 }) { nop(); }`},
		{"struct in array in while condition", `while [Point { x: 1 }][0].x { nop(); }`},
		{"struct as call argument in if condition", `if f(Point { x: 1 }) { nop(); }`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := New(lexer.New(tc.src))
			program := p.ParseProgram()
			checkParserErrors(t, p)
			rendered := program.String()
			if !strings.Contains(rendered, "Point") || !strings.Contains(rendered, "nop()") {
				t.Fatalf("struct literal or loop body lost for %q: %s", tc.src, rendered)
			}
		})
	}
}

// TestWhileBodyStillABody guards the other half: resetting the depth inside
// lists and literals must not turn the loop body itself into a struct.
func TestWhileBodyStillABody(t *testing.T) {
	p := New(lexer.New(`while x { nop(); }`))
	program := p.ParseProgram()
	checkParserErrors(t, p)
	ws, ok := program.Statements[0].(*ast.WhileStatement)
	if !ok {
		t.Fatalf("expected a while statement, got %T", program.Statements[0])
	}
	if len(ws.Body.Statements) != 1 {
		t.Fatalf("loop body lost, got %q", program.String())
	}
}

// TestNestingDepthGuard pins the stack-exhaustion fix: pathological nesting
// fails with a parse error instead of crashing the process, while deep but
// legitimate shapes — long prefix runs, flat operator chains — still parse.
func TestNestingDepthGuard(t *testing.T) {
	p := New(lexer.New(strings.Repeat("(", 200000)))
	p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("200000 nested parens parsed without an error")
	}
	found := false
	for _, e := range p.Errors() {
		if strings.Contains(e, "nesting too deep") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no nesting error reported: %v", p.Errors()[0])
	}

	for name, src := range map[string]string{
		"long prefix run": "let x = " + strings.Repeat("-", 20000) + "1;",
		"flat chain":      "let x = 1" + strings.Repeat("+1", 20000) + ";",
	} {
		p := New(lexer.New(src))
		prog := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatalf("%s: unexpected errors: %v", name, p.Errors())
		}
		if len(prog.Statements) != 1 {
			t.Fatalf("%s: expected 1 statement, got %d", name, len(prog.Statements))
		}
	}
}

// TestIntegerTokenAfterRadixError pins the token stream the adjacency check
// relies on: `0b102` lexes as `0b10` `2`.
func TestIntegerTokenAfterRadixError(t *testing.T) {
	l := lexer.New(`0b102`)
	if tok := l.NextToken(); tok.Type != token.INT || tok.Literal != "0b10" {
		t.Fatalf("got (%s, %q), want (INT, 0b10)", tok.Type, tok.Literal)
	}
	if tok := l.NextToken(); tok.Type != token.INT || tok.Literal != "2" {
		t.Fatalf("got (%s, %q), want (INT, 2)", tok.Type, tok.Literal)
	}
}

func templateOf(t *testing.T, src string) *ast.StringTemplate {
	t.Helper()
	prog := mustParse(t, src)
	let, ok := prog.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("%q: expected a let statement, got %T", src, prog.Statements[0])
	}
	tmpl, ok := let.Value.(*ast.StringTemplate)
	if !ok {
		t.Fatalf("%q: expected a string template, got %T", src, let.Value)
	}
	return tmpl
}

// TestInterpolationParenInString pins that a `)` inside a string literal in
// the interpolated expression does not end the interpolation: `\("a)b")`
// yields one expression part, not a truncated `"a`.
func TestInterpolationParenInString(t *testing.T) {
	tmpl := templateOf(t, `let s = "x\("a)b")y";`)
	if len(tmpl.Parts) != 3 {
		t.Fatalf("expected 3 parts (literal, expression, literal), got %d: %+v", len(tmpl.Parts), tmpl.Parts)
	}
	if !tmpl.Parts[1].IsExpression {
		t.Fatalf("middle part is not an expression: %+v", tmpl.Parts[1])
	}
	if tmpl.Parts[0].Literal != "x" || tmpl.Parts[2].Literal != "y" {
		t.Fatalf("literal parts wrong: %+v", tmpl.Parts)
	}
}

// TestInterpolationEscapedBackslash pins that `\\(` stays literal: an
// even-length backslash run is an escaped backslash followed by a plain
// paren, not an interpolation.
func TestInterpolationEscapedBackslash(t *testing.T) {
	tmpl := templateOf(t, `let s = "a\\(b";`)
	if len(tmpl.Parts) != 1 || tmpl.Parts[0].IsExpression {
		t.Fatalf("expected one literal part, got %+v", tmpl.Parts)
	}
}

// TestInterpolationUnbalancedIsAnError pins that `\((a)` is rejected instead
// of being silently truncated.
func TestInterpolationUnbalancedIsAnError(t *testing.T) {
	p := NewWithSource(lexer.New(`let s = "x\((a)";`), `let s = "x\((a)";`)
	p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("unbalanced interpolation parsed without an error")
	}
}

// TestBareReturn pins that `return` with no value parses and carries no
// ReturnValue, in every position a block can end.
func TestBareReturn(t *testing.T) {
	cases := []string{
		`fn f() { return; }`,
		"fn f() { return; }",
		"fn f() { if true { return; } return 1; }",
		"fn f() { while true { return; } }",
	}
	for _, src := range cases {
		p := New(lexer.New(src))
		program := p.ParseProgram()
		checkParserErrors(t, p)
		found := false
		ast.Inspect(program, func(node ast.Node) bool {
			if rs, ok := node.(*ast.ReturnStatement); ok && rs.ReturnValue == nil {
				found = true
			}
			return true
		})
		if !found {
			t.Fatalf("no bare return parsed for %q: %s", src, program.String())
		}
	}
}
