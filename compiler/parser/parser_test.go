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

