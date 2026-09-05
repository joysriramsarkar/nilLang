package mir

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
)

func TestMIRLoweringAndBytecodeExecution(t *testing.T) {
	input := `
	let a = 15;
	let b = 25;
	let c = a + b;
	return c;
	`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	lowerer := hir.NewLowerer()
	hProg := lowerer.LowerProgram(prog)

	mirLowerer := NewLowerer()
	mirProg := mirLowerer.LowerHIR(hProg)

	if mirProg.Main == nil {
		t.Fatalf("Expected main function in MIR program")
	}

	emitter := NewBytecodeEmitter(nil)
	bc, err := emitter.EmitProgram(mirProg)
	if err != nil {
		t.Fatalf("Bytecode generation failed: %v", err)
	}

	machine := vm.New(bc)
	err = machine.Run()
	if err != nil {
		t.Fatalf("VM run error: %v", err)
	}

	last := machine.StackTop()
	if last == nil {
		last = machine.LastPoppedStackElem()
	}
	if last == nil {
		t.Fatalf("Expected return value from VM, got nil")
	}

	if last.Inspect() != "40" {
		t.Errorf("Expected result 40, got %s", last.Inspect())
	}
}

func TestMIRControlFlow(t *testing.T) {
	input := `
	let i = 0;
	while (i < 3) {
		let i = i + 1;
	}
	let f = fn(x) { return x * 2; };
	let arr = [1, 2, 3];
	`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	lowerer := hir.NewLowerer()
	hProg := lowerer.LowerProgram(prog)

	mirLowerer := NewLowerer()
	mirProg := mirLowerer.LowerHIR(hProg)

	if mirProg == nil || mirProg.Main == nil {
		t.Fatalf("Expected valid MIR program with main function")
	}

	str := mirProg.String()
	if str == "" {
		t.Fatalf("Expected non-empty MIR string")
	}
}
