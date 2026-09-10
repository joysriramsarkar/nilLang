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

func TestTypecheckAnnotatedLet(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantValid   bool
		wantErrCode string
	}{
		{name: "matching initializer", input: `let count: i32 = 0;`, wantValid: true},
		{name: "mismatched initializer", input: `let count: i32 = "zero";`, wantErrCode: "E0101"},
		{name: "unknown annotation", input: `let count: Missing = 0;`, wantErrCode: "E0101"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := parseProgram(t, test.input)
			program := p.ParseProgram()
			if len(p.Errors()) > 0 {
				t.Fatalf("parse errors: %v", p.Errors())
			}

			checker := NewChecker()
			valid := checker.CheckProgram(program)
			if valid != test.wantValid {
				t.Fatalf("valid=%v, diagnostics=%v", valid, checker.Diagnostics)
			}
			if test.wantErrCode != "" {
				for _, diagnostic := range checker.Diagnostics {
					if diagnostic.Code == test.wantErrCode {
						return
					}
				}
				t.Fatalf("expected %s, diagnostics=%v", test.wantErrCode, checker.Diagnostics)
			}
		})
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

func TestTypecheckRejectsAssignmentToUndefinedVariable(t *testing.T) {
	p := parseProgram(t, `missing = 1;`)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	checker := NewChecker()
	if checker.CheckProgram(program) {
		t.Fatal("expected assignment to an undefined variable to fail typechecking")
	}

	for _, diagnostic := range checker.Diagnostics {
		if diagnostic.Code == "E0102" {
			return
		}
	}
	t.Fatalf("expected E0102, got diagnostics: %v", checker.Diagnostics)
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

func TestTypecheckRejectsConstantMutation(t *testing.T) {
	p := parseProgram(t, `
	const MAX = 100;
	MAX = 200;
	`)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	checker := NewChecker()
	if checker.CheckProgram(prog) {
		t.Fatal("expected mutating a const to fail typechecking")
	}

	found := false
	for _, d := range checker.Diagnostics {
		if d.Code == "E0104" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected diagnostic E0104, got: %v", checker.Diagnostics)
	}
}

func TestTypecheckScopeShadowingAndClosureCapture(t *testing.T) {
	p := parseProgram(t, `
	let x = 10;
	fn outer() {
		let x = "shadowed";
		fn inner() {
			return x;
		}
		return inner();
	}
	let result = outer();
	`)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	checker := NewChecker()
	if !checker.CheckProgram(prog) {
		t.Fatalf("expected valid shadowing and closure to pass, got: %v", checker.Diagnostics)
	}
}

func TestTypecheckCompoundAssignment(t *testing.T) {
	p := parseProgram(t, `
	let count = 0;
	count += 5;
	count -= 2;
	`)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	checker := NewChecker()
	if !checker.CheckProgram(prog) {
		t.Fatalf("expected compound assignment to pass, got: %v", checker.Diagnostics)
	}
}

func TestTypecheckEntityValidation(t *testing.T) {
	// 1. Duplicate entity field should emit E0203
	p := parseProgram(t, `
	entity Product {
		id: UUID primary;
		name: String;
		name: String;
	}
	`)
	prog := p.ParseProgram()
	checker := NewChecker()
	if checker.CheckProgram(prog) {
		t.Fatal("expected duplicate field to fail typecheck")
	}
	foundE0203 := false
	for _, d := range checker.Diagnostics {
		if d.Code == "E0203" {
			foundE0203 = true
			break
		}
	}
	if !foundE0203 {
		t.Fatalf("expected E0203 for duplicate field, got: %v", checker.Diagnostics)
	}

	// 2. Multiple primary keys should emit E0205
	p2 := parseProgram(t, `
	entity Account {
		id: UUID primary;
		email: String primary;
	}
	`)
	prog2 := p2.ParseProgram()
	checker2 := NewChecker()
	if checker2.CheckProgram(prog2) {
		t.Fatal("expected multiple primary keys to fail typecheck")
	}
	foundE0205 := false
	for _, d := range checker2.Diagnostics {
		if d.Code == "E0205" {
			foundE0205 = true
			break
		}
	}
	if !foundE0205 {
		t.Fatalf("expected E0205 for multiple primary keys, got: %v", checker2.Diagnostics)
	}
}
