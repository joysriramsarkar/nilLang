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

func TestTypecheckAppStatement(t *testing.T) {
	p := parseProgram(t, `
	app {
		let ready = true;
	}
	if (ready) { 1 } else { 0 };
	`)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	checker := NewChecker()
	if !checker.CheckProgram(prog) {
		t.Fatalf("Expected app statement to typecheck, got diagnostics: %v", checker.Diagnostics)
	}
}

func TestTypecheckNamedAppState(t *testing.T) {
	p := parseProgram(t, `app Hello { state count: i32 = 0 } count = count + 1;`)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}
	checker := NewChecker()
	if !checker.CheckProgram(prog) {
		t.Fatalf("Expected named app state to typecheck, got diagnostics: %v", checker.Diagnostics)
	}
}

func TestTypecheckRejectsInvalidStateInitializer(t *testing.T) {
	p := parseProgram(t, `app Hello { state count: i32 = "zero" }`)
	prog := p.ParseProgram()
	checker := NewChecker()
	if checker.CheckProgram(prog) {
		t.Fatal("expected invalid state initializer to fail typechecking")
	}
}

func TestTypecheckDeclarativeComponent(t *testing.T) {
	p := parseProgram(t, `
	component Counter {
		state count: i32 = 0;
		render { return {"type": "Text", "value": count}; }
		on click { count = count + 1; emit("changed", count); }
	}
	Counter.render();
	`)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}
	checker := NewChecker()
	if !checker.CheckProgram(prog) {
		t.Fatalf("Expected component to typecheck, got diagnostics: %v", checker.Diagnostics)
	}
}

func TestTypecheckTensorOperations(t *testing.T) {
	input := `
	let values = tensor([1, 2, 3, 4, 5, 6], [2, 3]);
	let batch = tensorSlice(values, [0, 0], [1, 3]);
	let scaled = tensorMul(batch, tensor([2, 3, 4], [3]));
	let typed = tensorCast(scaled, "float32");
	let dtype = tensorDtype(typed);
	tensorSum(scaled);
	`
	p := parseProgram(t, input)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}
	checker := NewChecker()
	if !checker.CheckProgram(prog) {
		t.Fatalf("Expected tensor operations to typecheck, got diagnostics: %v", checker.Diagnostics)
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
