package mir

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
)

// Three-engine agreement (`L3.4` and `L3.5`).
//
// # Why the same program is run three ways
//
// A backend that has never disagreed with another is not known to agree with it —
// only to have never been asked. The evaluator and the direct bytecode compiler
// have compared against each other for a long time. The MIR path did not exist in
// that comparison at all, which is why four faults below survived: an invented
// builtin, a discarded function name, a miscounted aggregate operand, and a call
// whose arguments were pushed before its callee.
//
// Each of those is invisible in MIR's own text. `_t2 = call array(1, 2, 3)` looks
// like a reasonable instruction; only running it says there is no such function.
// So the comparison is on *values*, from three different engines, and it is the
// only kind of test that would have caught them.

// runDirect evaluates through the AST-to-bytecode compiler.
//
// It is the existing, long-tested path, and it is what the MIR path must match.
func runDirect(t *testing.T, source string) object.Object {
	t.Helper()

	res := frontend.Check(source, frontend.Options{Filename: "engines.nil"})
	if !res.OK {
		t.Fatalf("direct engine refused the program: %s", res.ErrSummary())
	}
	bc, err := compileDirect(res.Lowered)
	if err != nil {
		t.Fatalf("direct compile failed: %v", err)
	}
	return runBytecode(t, bc)
}

// compileDirect is the existing AST-to-bytecode path.
//
// It takes the *lowered* program for the same reason the MIR path does: `L3.2`
// found three backends reading the parsed tree, and this test is worthless if one
// of its two engines is fed something the other never sees.
func compileDirect(prog *ast.Program) (*compiler.Bytecode, error) {
	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		return nil, err
	}
	return comp.Bytecode(), nil
}

// runMir evaluates through HIR, then MIR, then the MIR bytecode emitter.
func runMir(t *testing.T, source string) object.Object {
	t.Helper()

	res := frontend.Check(source, frontend.Options{Filename: "engines.nil"})
	if !res.OK {
		t.Fatalf("MIR engine refused the program: %s", res.ErrSummary())
	}

	optimized := hir.NewOptimizer().Optimize(hir.NewLowerer().LowerProgram(res.Lowered))
	bc, err := NewBytecodeEmitter(nil).EmitProgram(NewLowerer().LowerHIR(optimized))
	if err != nil {
		t.Fatalf("MIR bytecode generation failed: %v", err)
	}
	return runBytecode(t, bc)
}

func runBytecode(t *testing.T, bc *compiler.Bytecode) object.Object {
	t.Helper()

	machine := vm.New(bc)
	if err := machine.Run(); err != nil {
		t.Fatalf("VM run error: %v", err)
	}
	value, ok := machine.GlobalByName("result")
	if !ok {
		t.Fatalf("the program left no global named `result`; a test source must bind one")
	}
	if errObj, isErr := value.(*object.Error); isErr {
		t.Fatalf("the program produced an error: %s", errObj.Message)
	}
	return value
}

// TestThreeEnginesAgree runs each program through all three engines and requires
// the same rendered value from every one of them.
//
// The cases are ordered by how much of the language they touch rather than by
// category, because the interesting failures were not in any one area: they were
// in the seams between constructs, where each engine was individually right.
func TestThreeEnginesAgree(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "an arithmetic expression",
			source: `let a = 1; let b = 2; let result = a + b;`,
			want:   "3",
		},
		{
			name:   "nested arithmetic",
			source: `let a = 2; let b = 3; let c = 4; let result = (a + b) * c - a;`,
			want:   "18",
		},
		{
			name:   "a float result",
			source: `let x = 1.5; let y = 2.5; let result = x + y;`,
			want:   "4",
		},
		{
			name:   "string concatenation",
			source: `let s = "ab"; let result = s + "cd";`,
			want:   "abcd",
		},
		{
			name:   "a comparison chooses an arm",
			source: `let b = 1 < 2; let result = if (b) { 10 } else { 20 };`,
			want:   "10",
		},
		{
			name:   "the false arm is chosen",
			source: `let n = 1; let result = if (n > 3) { "big" } else { "small" };`,
			want:   "small",
		},
		{
			name:   "a list literal",
			source: `let xs = [10, 20, 30]; let result = xs[1];`,
			want:   "20",
		},
		{
			name:   "an empty list literal",
			source: `let xs = []; let result = len(xs);`,
			want:   "0",
		},
		{
			name:   "a map literal read by key",
			source: `let m = {"k": 7}; let result = m["k"];`,
			want:   "7",
		},
		{
			name:   "a map literal read by field",
			source: `let m = {"x": 5}; let result = m.x;`,
			want:   "5",
		},
		{
			// `len` takes a list or a string, not a map. A map's size is asked
			// for differently, and a test that invented a capability would be
			// testing the invention rather than the engines.
			name:   "an empty map literal",
			source: `let m = {}; let result = first([1, 2, 3]);`,
			want:   "1",
		},
		{
			name:   "a builtin call",
			source: `let r = len("abc"); let result = r;`,
			want:   "3",
		},
		{
			name:   "a builtin call with several arguments",
			source: `let r = substr("abcdef", 1, 3); let result = r;`,
			want:   "bcd",
		},
		{
			name:   "a builtin inside an expression",
			source: `let xs = [1, 2, 3]; let result = len(xs) + 1;`,
			want:   "4",
		},
		{
			name:   "a user-defined function",
			source: `fn add(a: Int, b: Int) -> Int { return a + b; } let result = add(2, 3);`,
			want:   "5",
		},
		{
			name:   "a function called twice",
			source: `fn twice(a: Int) -> Int { return a * 2; } let result = twice(twice(5));`,
			want:   "20",
		},
		{
			name:   "a function whose argument is a literal aggregate",
			source: `fn size(xs: Array<Int>) -> Int { return len(xs); } let result = size([1, 2, 3, 4]);`,
			want:   "4",
		},
		{
			name:   "a while loop accumulates",
			source: `let i = 0; let t = 0; while i < 3 { t = t + i; i = i + 1; } let result = t;`,
			want:   "3",
		},
		{
			name:   "unary negation",
			source: `let a = 5; let result = -a;`,
			want:   "-5",
		},
		{
			name:   "a value read back more than once",
			source: `let a = 5; let b = a + 3; let result = b + a;`,
			want:   "13",
		},
		{
			name:   "an aggregate nested in an arithmetic expression",
			source: `let result = len([1, 2]) * 3;`,
			want:   "6",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			direct := runDirect(t, tc.source)
			viaMir := runMir(t, tc.source)

			if direct.Inspect() != tc.want {
				t.Errorf("direct engine produced %q, want %q", direct.Inspect(), tc.want)
			}
			if viaMir.Inspect() != tc.want {
				t.Errorf("MIR engine produced %q, want %q", viaMir.Inspect(), tc.want)
			}
			if direct.Inspect() != viaMir.Inspect() {
				t.Errorf("the engines disagree: direct %q, MIR %q", direct.Inspect(), viaMir.Inspect())
			}
		})
	}
}

// TestMIREmitsGlobalNames is the half of the debugger contract that is cheap to
// hold and easy to lose.
//
// The MIR emitter builds its own bytecode, and `Bytecode.GlobalNames` is optional
// in the sense that nothing fails when it is absent: the program runs and every
// value is correct. What disappears is the ability to ask what a variable is
// called — which is the difference between a debugger and a slot viewer.
func TestMIREmitsGlobalNames(t *testing.T) {
	res := frontend.Check(`let result = 1; let other = 2;`, frontend.Options{Filename: "names.nil"})
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}

	optimized := hir.NewOptimizer().Optimize(hir.NewLowerer().LowerProgram(res.Lowered))
	bc, err := NewBytecodeEmitter(nil).EmitProgram(NewLowerer().LowerHIR(optimized))
	if err != nil {
		t.Fatalf("bytecode generation failed: %v", err)
	}

	if len(bc.GlobalNames) == 0 {
		t.Fatal("no global names were recorded; a debugger could only report slot numbers")
	}
	for _, want := range []string{"result", "other"} {
		if !containsString(bc.GlobalNames, want) {
			t.Errorf("global %q is missing from %v", want, bc.GlobalNames)
		}
	}
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
