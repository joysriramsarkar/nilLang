package compiler

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

func TestCompileRejectsAssignmentToUndefinedVariable(t *testing.T) {
	l := lexer.New(`missing = 1;`)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}

	err := New().Compile(program)
	if err == nil || !strings.Contains(err.Error(), "undefined variable missing") {
		t.Fatalf("expected undefined variable error, got %v", err)
	}
}
