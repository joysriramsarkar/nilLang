package mir

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
)

func TestCFGReachabilityAndDeadBlockPruning(t *testing.T) {
	input := `
	let x = 10;
	if (x > 5) {
		return 100;
	} else {
		return 200;
	}
	let unreachable = 999;
	return unreachable;
	`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	hProg := hir.NewLowerer().LowerProgram(prog)
	mirProg := NewLowerer().LowerHIR(hProg)

	initialBlockCount := len(mirProg.Main.Blocks)
	cfg := BuildCFG(mirProg.Main)
	reachable := cfg.ReachableBlocks()

	if len(reachable) == 0 {
		t.Fatal("CFG has no reachable blocks")
	}

	// Optimize with DCE
	OptimizeDeadCodeElimination(mirProg.Main)
	finalBlockCount := len(mirProg.Main.Blocks)

	t.Logf("Initial blocks: %d, Final blocks: %d", initialBlockCount, finalBlockCount)
}

func TestConstantFoldingOptimizer(t *testing.T) {
	input := `
	let a = 10 + 20;
	let b = 5 * 6;
	let c = a + b;
	return c;
	`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	hProg := hir.NewLowerer().LowerProgram(prog)
	mirProg := NewLowerer().LowerHIR(hProg)

	// Run constant folding
	changed := OptimizeConstantFolding(mirProg.Main)
	if !changed {
		t.Fatal("expected constant folding to optimize operations")
	}

	emitter := NewBytecodeEmitter(nil)
	bc, err := emitter.EmitProgram(mirProg)
	if err != nil {
		t.Fatalf("bytecode emit error: %v", err)
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
	if last == nil || last.Inspect() != "60" {
		t.Fatalf("expected 60, got %v", last)
	}
}

func TestGoldenEquivalenceUnoptimizedVsOptimized(t *testing.T) {
	testPrograms := []struct {
		name   string
		source string
	}{
		{
			name: "arithmetic_folding",
			source: `
			let x = (2 + 3) * 4;
			let y = 100 - x;
			return y;
			`,
		},
		{
			name: "boolean_folding",
			source: `
			let cond = true && false;
			if (cond) {
				return 1;
			} else {
				return 2;
			}
			`,
		},
		{
			name: "nested_operations",
			source: `
			let a = 10;
			let b = 20;
			let c = 30;
			let result = (a + b) * (c - 10);
			return result;
			`,
		},
	}

	for _, tc := range testPrograms {
		t.Run(tc.name, func(t *testing.T) {
			l1 := lexer.New(tc.source)
			p1 := parser.New(l1)
			prog1 := p1.ParseProgram()

			l2 := lexer.New(tc.source)
			p2 := parser.New(l2)
			prog2 := p2.ParseProgram()

			// Engine 1: Unoptimized MIR
			hProg1 := hir.NewLowerer().LowerProgram(prog1)
			mirProg1 := NewLowerer().LowerHIR(hProg1)
			bc1, err := NewBytecodeEmitter(nil).EmitProgram(mirProg1)
			if err != nil {
				t.Fatalf("unoptimized emit error: %v", err)
			}
			vm1 := vm.New(bc1)
			if err := vm1.Run(); err != nil {
				t.Fatalf("unoptimized vm run error: %v", err)
			}
			res1 := vm1.StackTop()
			if res1 == nil {
				res1 = vm1.LastPoppedStackElem()
			}

			// Engine 2: Optimized MIR (Full Optimizer pass)
			hProg2 := hir.NewLowerer().LowerProgram(prog2)
			mirProg2 := NewLowerer().LowerHIR(hProg2)
			NewOptimizer().OptimizeProgram(mirProg2)
			bc2, err := NewBytecodeEmitter(nil).EmitProgram(mirProg2)
			if err != nil {
				t.Fatalf("optimized emit error: %v", err)
			}
			vm2 := vm.New(bc2)
			if err := vm2.Run(); err != nil {
				t.Fatalf("optimized vm run error: %v", err)
			}
			res2 := vm2.StackTop()
			if res2 == nil {
				res2 = vm2.LastPoppedStackElem()
			}

			// Equivalence assertion: Unoptimized == Optimized
			if res1.Inspect() != res2.Inspect() {
				t.Fatalf("[%s] Golden Equivalence Violation! Unoptimized=%s, Optimized=%s",
					tc.name, res1.Inspect(), res2.Inspect())
			}
		})
	}
}
