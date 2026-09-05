package hir

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

func TestHIRLoweringAndOptimization(t *testing.T) {
	input := `
	let a = 10 + 20 * 2;
	let b = "hello " + "world";
	let c = a + 5;
	`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	lowerer := NewLowerer()
	hProg := lowerer.LowerProgram(prog)
	if len(hProg.Statements) != 3 {
		t.Fatalf("Expected 3 HIR statements, got %d", len(hProg.Statements))
	}

	optimizer := NewOptimizer()
	optProg := optimizer.Optimize(hProg)

	// Verify constant folding on 'a': 10 + (20 * 2) = 50
	letA, ok := optProg.Statements[0].(*LetStmt)
	if !ok {
		t.Fatalf("Statement 0 is not a LetStmt")
	}
	intA, ok := letA.Value.(*IntLit)
	if !ok || intA.Val != 50 {
		t.Errorf("Expected constant folded value 50, got %v", letA.Value)
	}

	// Verify constant folding on 'b': "hello " + "world" = "hello world"
	letB, ok := optProg.Statements[1].(*LetStmt)
	if !ok {
		t.Fatalf("Statement 1 is not a LetStmt")
	}
	strB, ok := letB.Value.(*StringLit)
	if !ok || strB.Val != "hello world" {
		t.Errorf("Expected constant folded string 'hello world', got %v", letB.Value)
	}
}

func TestHIRControlFlowAndTemplates(t *testing.T) {
	input := `
	let name = "Nilang";
	let msg = "Hello, \(name)!";
	let i = 0;
	while (i < 5) {
		let i = i + 1;
	}
	let f = fn(x) { return x * 2; };
	let arr = [1, 2, 3];
	let obj = { "key": 42 };
	`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	lowerer := NewLowerer()
	hProg := lowerer.LowerProgram(prog)
	if hProg == nil || len(hProg.Statements) == 0 {
		t.Fatalf("Expected non-empty HIR program")
	}

	// Make sure String() doesn't panic
	str := hProg.String()
	if str == "" {
		t.Fatalf("Expected non-empty string output from HIR program")
	}

	optimizer := NewOptimizer()
	optProg := optimizer.Optimize(hProg)
	if optProg == nil || len(optProg.Statements) == 0 {
		t.Fatalf("Expected non-empty optimized HIR program")
	}
}

func TestHIRNilSafety(t *testing.T) {
	call := &CallExpr{Callee: nil, Args: []Expression{nil}}
	_ = call.String()

	bin := &BinaryExpr{Left: nil, Op: "+", Right: nil}
	_ = bin.String()

	unary := &UnaryExpr{Op: "-", Right: nil}
	_ = unary.String()

	list := &ListLit{Elements: []Expression{nil}}
	_ = list.String()

	idx := &IndexExpr{Left: nil, Index: nil}
	_ = idx.String()

	dot := &DotExpr{Left: nil, Field: "foo"}
	_ = dot.String()

	mapLit := &MapLit{Keys: []Expression{nil}, Values: []Expression{nil}}
	_ = mapLit.String()
}
