package mir

import (
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
)

// `L3.6`'s gate is a number: at least twenty per cent faster.
//
// # Why this file measures rather than asserts
//
// The other tests in this package assert *shape* — fewer instructions, fewer
// blocks, an answer that is still right. Those are the things a test can pin down.
// Speed is not one of them: it depends on the machine, on what else is running, and
// on how hot the CPU is. A test asserting "twenty per cent faster" would fail on a
// busy CI machine and pass on an idle laptop, which makes it a test that reports
// the weather.
//
// So the measurement lives here, is printed, and is *not* a pass/fail condition.
// It is there so the claim in the plan has a number behind it that a reader can
// check, rather than a direction that someone believed once.

// benchSources are chosen for what a program does, not for how it is written.
//
// The loop is the one that matters: it is the only shape where hoisting and copy
// propagation change the work *per iteration* rather than the work *per program*.
// Everything else is measured to show it did not get slower.
var benchSources = map[string]string{
	"a loop summing a range": `
		let i = 0;
		let total = 0;
		while i < 20000 {
			let step = i + 1;
			total = total + step;
			i = step;
		}
		let result = total;
	`,
	"a loop with an invariant read": `
		let xs = [1, 2, 3, 4, 5, 6, 7, 8];
		let i = 0;
		let seen = 0;
		while i < 20000 {
			let n = len(xs);
			if n > 3 { seen = seen + 1; }
			i = i + 1;
		}
		let result = seen;
	`,
	"a loop over a repeated computation": `
		let n = 6;
		let i = 0;
		let total = 0;
		while i < 8000 {
			let a = n * n;
			let b = n * n;
			total = total + a + b;
			i = i + 1;
		}
		let result = total;
	`,
	"a straight-line program": `
		let a = 1;
		let b = 2;
		let c = 3;
		let d = a + b;
		let e = b + c;
		let f = c + d;
		let g = d + e;
		let h = e + f;
		let i = g + h;
		let j = i + f;
		let result = j + h;
	`,
}

// TestMeasureOptimisationGain prints the shape and time difference between the two
// backends.
//
// It asserts only that the optimised path gives the same answers. The speed
// difference it prints is evidence, not a gate — see the note above.
func TestMeasureOptimisationGain(t *testing.T) {
	type measurement struct {
		instructions  int
		blocks        int
		unoptimisedNs time.Duration
		optimisedNs   time.Duration
	}

	results := make(map[string]measurement, len(benchSources))
	for name, source := range benchSources {
		t.Run(name, func(t *testing.T) {
			res := frontend.Check(source, frontend.Options{Filename: "bench.nil"})
			if !res.OK {
				t.Fatalf("source was rejected: %s", res.ErrSummary())
			}

			unoptimised := NewLowerer().LowerHIR(hir.NewLowerer().LowerProgram(res.Lowered))
			optimised := NewLowerer().LowerHIR(hir.NewOptimizer().Optimize(hir.NewLowerer().LowerProgram(res.Lowered)))
			NewOptimizer().OptimizeProgram(optimised)

			before := measure(t, unoptimised)
			after := measure(t, optimised)

			// The gate: same answer.
			if before != after {
				t.Fatalf("optimisation changed the answer: %d then %d", before, after)
			}

			results[name] = measurement{
				instructions:  countInstructions(optimised),
				blocks:        countBlocks(optimised),
				unoptimisedNs: bestRun(t, emit(t, unoptimised)),
				optimisedNs:   bestRun(t, emit(t, optimised)),
			}
		})
	}

	t.Log("L3.6 gate — optimised vs unoptimised MIR, both executed on the VM:")
	for name, m := range results {
		saved := m.unoptimisedNs - m.optimisedNs
		perCent := 0.0
		if m.unoptimisedNs > 0 {
			perCent = float64(saved) / float64(m.unoptimisedNs) * 100
		}
		t.Logf("  %-32s %2d blocks %3d instrs  %8s -> %8s  %+6.1f%%",
			name, m.blocks, m.instructions,
			shorten(m.unoptimisedNs), shorten(m.optimisedNs), perCent)
	}
}

// emit turns MIR into bytecode, which is the last step before the VM runs.
func emit(t *testing.T, mir *Program) *compiler.Bytecode {
	t.Helper()

	bc, err := NewBytecodeEmitter(nil).EmitProgram(mir)
	if err != nil {
		t.Fatalf("bytecode generation failed: %v", err)
	}
	return bc
}

// timeOf runs bytecode once and reports how long the run took.
func timeOf(t *testing.T, bc *compiler.Bytecode) time.Duration {
	t.Helper()

	start := time.Now()
	machine := vm.New(bc)
	if err := machine.Run(); err != nil {
		t.Fatalf("program failed to run: %v", err)
	}
	elapsed := time.Since(start)

	if _, ok := machine.GlobalByName("result"); !ok {
		t.Fatal("the program left no global named `result`")
	}
	return elapsed
}

// bestRun is the fastest of several runs.
//
// # Why the fastest and not the average
//
// A single run of a program this short is dominated by whatever else the machine
// was doing: a GC pause, a scheduler tick, another test finishing. Measured once,
// the same program reported both a thirty per cent *slowdown* and a thirty per cent
// speed-up on consecutive runs — a spread far larger than any effect being
// measured, which makes a one-shot number worse than no number.
//
// Taking the minimum removes what is added to the run rather than what the run
// does. The minimum is the honest estimate of the program's cost, and the difference
// between two minima is the difference between the two programs.
//
// Seven runs: enough that one scheduling artefact rarely sets the minimum, few
// enough that the measurement does not become the slowest thing in the suite.
func bestRun(t *testing.T, bc *compiler.Bytecode) time.Duration {
	best := time.Duration(1<<62 - 1)
	for i := 0; i < 7; i++ {
		if elapsed := timeOf(t, bc); elapsed < best {
			best = elapsed
		}
	}
	return best
}

// measure returns a program's answer, which is what the two runs must agree on.
func measure(t *testing.T, mir *Program) int64 {
	t.Helper()

	bc, err := NewBytecodeEmitter(nil).EmitProgram(mir)
	if err != nil {
		t.Fatalf("bytecode generation failed: %v", err)
	}
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

func shorten(d time.Duration) string {
	if d == 0 {
		return "0"
	}
	if d < time.Millisecond {
		return d.Round(time.Microsecond).String()
	}
	return d.Round(time.Millisecond).String()
}
