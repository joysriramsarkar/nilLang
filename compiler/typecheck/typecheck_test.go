package typecheck

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

func parseProgram(t *testing.T, input string) *parser.Parser {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	return p
}

func TestTypecheckValid(t *testing.T) {
	input := `
	let x = 10;
	let y = 20;
	let z = x + y;
	puts(z);
	`
	p := parseProgram(t, input)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	checker := NewChecker()
	ok := checker.CheckProgram(prog)
	if !ok {
		for _, d := range checker.Diagnostics {
			t.Log(d.String())
		}
		t.Fatalf("Expected valid typecheck, got diagnostics")
	}
}

func TestTypecheckUndefinedVariable(t *testing.T) {
	input := `
	let a = 10;
	let b = a + undefinedVar;
	`
	p := parseProgram(t, input)
	prog := p.ParseProgram()

	checker := NewChecker()
	ok := checker.CheckProgram(prog)
	if ok {
		t.Fatalf("Expected typecheck to fail due to undefined identifier")
	}

	found := false
	for _, d := range checker.Diagnostics {
		if d.Code == "E0102" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected diagnostic E0102 for undefined identifier")
	}
}

func TestTypecheckWhileAndTemplate(t *testing.T) {
	input := `
	let name = "Nilang";
	let msg = "Hello, \(name)!";
	let i = 0;
	while (i < 5) {
		let i = i + 1;
	}
	`
	p := parseProgram(t, input)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	checker := NewChecker()
	ok := checker.CheckProgram(prog)
	if !ok {
		for _, d := range checker.Diagnostics {
			t.Log(d.String())
		}
		t.Fatalf("Expected valid typecheck for while and template")
	}
}
