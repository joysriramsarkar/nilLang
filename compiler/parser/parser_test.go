package parser

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
)

func TestLetStatements(t *testing.T) {
	input := `
let x = 5;
let y = 10;
let foobar = 838383;
`

	l := lexer.New(input)
	p := New(l)

	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 3 {
		t.Fatalf("program.Statements does not contain 3 statements. got=%d",
			len(program.Statements))
	}

	tests := []struct {
		expectedIdentifier string
	}{
		{"x"},
		{"y"},
		{"foobar"},
	}

	for i, tt := range tests {
		stmt := program.Statements[i]
		if !testLetStatement(t, stmt, tt.expectedIdentifier) {
			return
		}
	}
}

func TestAppStatement(t *testing.T) {
	input := `
app Hello {
	let launched = true;
}
let app = 7;
`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(program.Statements))
	}
	appStmt, ok := program.Statements[0].(*ast.AppStatement)
	if !ok {
		t.Fatalf("statement 0 is not *ast.AppStatement, got %T", program.Statements[0])
	}
	if appStmt.Name == nil || appStmt.Name.Value != "Hello" {
		t.Fatalf("expected app name Hello, got %+v", appStmt.Name)
	}
	if appStmt.Body == nil || len(appStmt.Body.Statements) != 1 {
		t.Fatalf("expected app body with 1 statement, got %+v", appStmt.Body)
	}
	if !testLetStatement(t, program.Statements[1], "app") {
		t.Fatal("app should remain valid as an identifier")
	}
}

func TestTaskAndAwaitExpressions(t *testing.T) {
	p := New(lexer.New(`let work = task { 40 + 2; }; await work;`))
	program := p.ParseProgram()
	checkParserErrors(t, p)
	letStatement := program.Statements[0].(*ast.LetStatement)
	if _, ok := letStatement.Value.(*ast.TaskExpression); !ok {
		t.Fatalf("let value is not a task expression: %T", letStatement.Value)
	}
	expression := program.Statements[1].(*ast.ExpressionStatement)
	if _, ok := expression.Expression.(*ast.AwaitExpression); !ok {
		t.Fatalf("expression is not an await expression: %T", expression.Expression)
	}
}

func TestNamedAppStateBlueprint(t *testing.T) {
	p := New(lexer.New(`app Hello { state count: i32 = 0 }`))
	program := p.ParseProgram()
	checkParserErrors(t, p)
	app := program.Statements[0].(*ast.AppStatement)
	state, ok := app.Body.Statements[0].(*ast.StateDeclaration)
	if !ok || state.Name.Value != "count" || state.Type != "i32" {
		t.Fatalf("unexpected state declaration: %T (%+v)", app.Body.Statements[0], app.Body.Statements[0])
	}
	value, ok := state.Value.(*ast.IntegerLiteral)
	if !ok || value.Value != 0 {
		t.Fatalf("unexpected state initializer: %T (%+v)", state.Value, state.Value)
	}
}

func TestStateTypeNameRequired(t *testing.T) {
	p := New(lexer.New(`app Hello { state count: = 0 }`))
	p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected parser error for missing state type name")
	}
}

func TestDeclarativeComponentMembers(t *testing.T) {
	input := `
component Counter {
	state count: i32 = 0;
	render { return {"type": "Text", "value": "count"}; }
	build { return count; }
	on click { emit("changed", count); }
}
`
	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkParserErrors(t, p)

	component, ok := program.Statements[0].(*ast.ComponentLiteral)
	if !ok {
		t.Fatalf("expected component declaration, got %T", program.Statements[0])
	}
	if len(component.States) != 1 || component.States[0].Name.Value != "count" {
		t.Fatalf("unexpected component states: %+v", component.States)
	}
	if component.Render == nil || component.Render.Body == nil {
		t.Fatal("expected render block")
	}
	if component.Build == nil || component.Build.Body == nil {
		t.Fatal("expected build block")
	}
	if len(component.Handlers) != 1 || component.Handlers[0].Event.Value != "click" {
		t.Fatalf("unexpected event handlers: %+v", component.Handlers)
	}
}

func testLetStatement(t *testing.T, s ast.Statement, name string) bool {
	if s.TokenLiteral() != "let" {
		t.Errorf("s.TokenLiteral not 'let'. got=%q", s.TokenLiteral())
		return false
	}

	letStmt, ok := s.(*ast.LetStatement)
	if !ok {
		t.Errorf("s not *ast.LetStatement. got=%T", s)
		return false
	}

	if letStmt.Name.Value != name {
		t.Errorf("letStmt.Name.Value not '%s'. got=%s", name, letStmt.Name.Value)
		return false
	}

	return true
}

func checkParserErrors(t *testing.T, p *Parser) {
	errors := p.Errors()
	if len(errors) == 0 {
		return
	}

	t.Errorf("parser has %d errors", len(errors))
	for _, msg := range errors {
		t.Errorf("parser error: %q", msg)
	}
	t.FailNow()
}

func TestImportStatements(t *testing.T) {
	input := `
import "web";
import "data" as db;
import { Button, Text } from "alap/web";
`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(program.Statements))
	}

	// 1st: import "web"
	s1, ok := program.Statements[0].(*ast.ImportStatement)
	if !ok || s1.Path.Value != "web" {
		t.Fatalf("statement 0 not expected import. got=%+v", program.Statements[0])
	}

	// 2nd: import "data" as db
	s2, ok := program.Statements[1].(*ast.ImportStatement)
	if !ok || s2.Path.Value != "data" || s2.Alias == nil || s2.Alias.Value != "db" {
		t.Fatalf("statement 1 not expected import as. got=%+v", program.Statements[1])
	}

	// 3rd: import { Button, Text } from "alap/web"
	s3, ok := program.Statements[2].(*ast.ImportStatement)
	if !ok || s3.Path.Value != "alap/web" || len(s3.Names) != 2 {
		t.Fatalf("statement 2 not expected destructured import. got=%+v", program.Statements[2])
	}
}
