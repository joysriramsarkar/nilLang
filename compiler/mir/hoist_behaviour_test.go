package mir

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
)

// # What hoisting costs if it is wrong
//
// A hoist changes *when* a value is computed. Getting it wrong does not crash and
// does not produce a warning — it produces a number, once, on a program that
// looked correct. Every test here therefore runs a loop with a hoisted binding and
// requires the right answer, not merely a smaller program.
//
// The cases are chosen so a hoist that is too eager fails *loudly*: the loop counts
// to five, and a binding hoisted from inside it would freeze at its first value.

// TestHoistingKeepsLoopAnswersCorrect covers the hoistable case.
func TestHoistingKeepsLoopAnswersCorrect(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   int64
	}{
		{
			name: "a length read inside a loop",
			source: `
			let xs = [1, 2, 3, 4, 5];
			let i = 0;
			let seen = 0;
			while i < 5 {
				let n = len(xs);
				if n > 0 { seen = seen + 1; }
				i = i + 1;
			}
			let result = seen;
			`,
			want: 5,
		},
		{
			name: "a binding the loop changes stays inside",
			source: `
			let i = 0;
			let total = 0;
			while i < 5 {
				let step = i + 1;
				total = total + step;
				i = step;
			}
			let result = total;
			`,
			want: 15,
		},
		{
			name: "a nested loop still counts every iteration",
			source: `
			let i = 0;
			let count = 0;
			while i < 3 {
				let j = 0;
				while j < 3 {
					count = count + 1;
					j = j + 1;
				}
				i = i + 1;
			}
			let result = count;
			`,
			want: 9,
		},
		{
			name: "an accumulator is not hoisted",
			source: `
			let i = 0;
			let total = 0;
			while i < 4 {
				total = total + i;
				i = i + 1;
			}
			let result = total;
			`,
			want: 6,
		},
		{
			name: "a loop that runs once gives the same answer",
			source: `
			let i = 0;
			let result = 0;
			while i < 1 {
				let doubled = i + i;
				result = doubled;
				i = i + 1;
			}
			`,
			want: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := runThroughFullPipeline(t, tc.source); got != tc.want {
				t.Errorf("optimised program produced %d, want %d", got, tc.want)
			}
		})
	}
}

// TestHoistingDoesNotChangeUnhoistablePrograms is the control.
//
// The same programs run through the *unoptimised* path must give the same answers.
// Without this, a hoisting bug could be mistaken for a lowering bug and the two
// would be investigated together until both were fixed by accident.
func TestHoistingDoesNotChangeUnhoistablePrograms(t *testing.T) {
	sources := []string{
		"let i = 0; let t = 0; while i < 5 { t = t + i; i = i + 1; } let result = t;",
		"let xs = [1, 2]; let i = 0; let s = 0; while i < 2 { s = s + len(xs); i = i + 1; } let result = s;",
	}

	for _, source := range sources {
		t.Run(source[:min(len(source), 32)], func(t *testing.T) {
			direct := runDirectResult(t, source)
			optimised := runThroughFullPipeline(t, source)

			if direct != optimised {
				t.Errorf("the optimiser changed the answer: direct %d, optimised %d", direct, optimised)
			}
		})
	}
}

// runThroughFullPipeline is the whole path a `--backend=mir` build takes:
// HIR lowering, HIR optimisation (including hoisting), MIR lowering, MIR
// optimisation, bytecode.
func runThroughFullPipeline(t *testing.T, source string) int64 {
	t.Helper()

	res := frontend.Check(source, frontend.Options{Filename: "hoist.nil"})
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}

	h := hir.NewOptimizer().Optimize(hir.NewLowerer().LowerProgram(res.Lowered))
	prog := NewLowerer().LowerHIR(h)
	NewOptimizer().OptimizeProgram(prog)

	bc, err := NewBytecodeEmitter(nil).EmitProgram(prog)
	if err != nil {
		t.Fatalf("bytecode generation failed: %v", err)
	}
	return resultOf(t, bc)
}

// runDirectResult runs the same program through the AST-to-bytecode compiler.
func runDirectResult(t *testing.T, source string) int64 {
	t.Helper()

	res := frontend.Check(source, frontend.Options{Filename: "hoist.nil"})
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}
	comp := compiler.New()
	if err := comp.Compile(res.Lowered); err != nil {
		t.Fatalf("direct compile failed: %v", err)
	}
	return resultOf(t, comp.Bytecode())
}

func resultOf(t *testing.T, bc *compiler.Bytecode) int64 {
	t.Helper()

	machine := vm.New(bc)
	if err := machine.Run(); err != nil {
		t.Fatalf("program failed to run: %v", err)
	}
	value, ok := machine.GlobalByName("result")
	if !ok {
		t.Fatal("the program left no global named `result`")
	}
	if errObj, isErr := value.(*object.Error); isErr {
		t.Fatalf("the program produced an error: %s", errObj.Message)
	}
	integer, isInt := value.(*object.Integer)
	if !isInt {
		t.Fatalf("expected an integer result, got %s", value.Type())
	}
	return integer.Value
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
