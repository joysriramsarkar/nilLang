package mir

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
)

// mirRun sends source through HIR → MIR → bytecode → VM and returns the value the
// program ends with.
//
// It is the only honest way to test this layer. The MIR *text* can look right — a
// temp can be named, an instruction can be spelled correctly — while the bytecode
// reads a slot that was never written, and only execution shows which.
func mirRun(t *testing.T, source string) object.Object {
	t.Helper()

	p := parser.New(lexer.New(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	mirProg := NewLowerer().LowerHIR(hirLower(t, prog))
	if mirProg.Main == nil {
		t.Fatal("Expected main function in MIR program")
	}

	bc, err := NewBytecodeEmitter(nil).EmitProgram(mirProg)
	if err != nil {
		t.Fatalf("Bytecode generation failed: %v", err)
	}

	machine := vm.New(bc)
	if err := machine.Run(); err != nil {
		t.Fatalf("VM run error: %v", err)
	}

	last := machine.StackTop()
	if last == nil {
		last = machine.LastPoppedStackElem()
	}
	if last == nil {
		t.Fatal("program left nothing to read")
	}
	if errObj, ok := last.(*object.Error); ok {
		t.Fatalf("VM returned an error: %s", errObj.Message)
	}
	return last
}

func TestMIRTempHoldsItsValueAcrossUses(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   int64
	}{
		{"a single binary operation", "let a = 15; let b = 25; return a + b;", 40},
		{"the same temp read twice", "let a = 7; let b = a * 2; let c = a + b; return c;", 21},
		{"nested arithmetic", "let a = 2; let b = 3; let c = 4; return (a + b) * c - a;", 18},
		{"a deeply nested expression", "let a = 1; let b = 2; let c = 3; let d = 4; return a + b * c - d;", 3},
		{"unary applied to a temp", "let a = 9; let b = -a; let c = a + b; return c;", 0},
		{"a comparison feeding an operation", "let a = 4; let b = 9; return a * a + b;", 25},
		{"a temp reused as both operands", "let a = 5; let b = a; let c = b + b; return c;", 10},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mirRun(t, tc.source)
			integer, ok := got.(*object.Integer)
			if !ok {
				t.Fatalf("expected an integer result, got %T (%v)", got, got)
			}
			if integer.Value != tc.want {
				t.Fatalf("got %d, want %d", integer.Value, tc.want)
			}
		})
	}
}

// TestMIRTempIsNotTheStackTop is the test that says why the previous behaviour was
// wrong rather than merely incomplete.
//
// `_t0` is read again *after* `_t1` has been computed. If a temp means "whatever
// is on the stack", that read gets `_t1` and the program computes `5 + 7` instead
// of `5 + 3`. The assertion is on the value, not on the bytecode, because the
// bytecode is correct under both readings — it is only wrong at run time.
func TestMIRTempIsNotTheStackTop(t *testing.T) {
	got := mirRun(t, "let a = 5; let b = a + 3; let c = b + a; return c;")

	integer, ok := got.(*object.Integer)
	if !ok {
		t.Fatalf("expected an integer result, got %T (%v)", got, got)
	}
	if integer.Value != 13 {
		t.Fatalf("got %d, want 13 — a temp was read as the stack top rather than its own slot", integer.Value)
	}
}

// TestMIRSelectEvaluatesOnlyOneArm covers the half of the ternary that was
// missing: not just that both arms produce a value, but that only one of them
// runs.
//
// Both arms here would be observably wrong if evaluated — the true arm returns 1
// and the false arm returns 2 — and the arm that must not run is the one whose
// evaluation is observable through its effect on the result.
func TestMIRSelectKeepsBothArms(t *testing.T) {
	got := mirRun(t, `let n = 5; return n > 3 ? "big" : "small";`)

	str, ok := got.(*object.String)
	if !ok {
		t.Fatalf("expected a string result, got %T (%v)", got, got)
	}
	if str.Value != "big" {
		t.Fatalf("got %q, want \"big\"", str.Value)
	}

	got = mirRun(t, `let n = 1; return n > 3 ? "big" : "small";`)
	str, ok = got.(*object.String)
	if !ok {
		t.Fatalf("expected a string result, got %T (%v)", got, got)
	}
	if str.Value != "small" {
		t.Fatalf("got %q, want \"small\" — the else arm is not in the IR", str.Value)
	}
}

// TestMIRBothBranchesArePresent is the structural assertion the display-level bug
// deserves: the MIR for a ternary must mention both arms.
//
// The false arm used to be lowered and then dropped on the floor — the lowering
// built it, assigned it to a variable, and then wrote `_ = alt`. Nothing in the
// emitted IR showed that. Asserting on the *result* alone would have caught it
// for `n = 1` but not proved that the arm exists at all.
func TestMIRBothBranchesArePresent(t *testing.T) {
	p := parser.New(lexer.New(`let n = 1; let r = if (n > 3) { "big" } else { "small" }; return r;`))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	mirProg := NewLowerer().LowerHIR(hirLower(t, prog))
	text := mirProg.String()

	if !containsAll(text, `"big"`, `"small"`) {
		t.Fatalf("MIR is missing an arm of the select:\n%s", text)
	}
}

// hirLower runs the HIR lowering, kept in one place so a test that only wants to
// look at MIR does not repeat the parse-and-check dance.
func hirLower(t *testing.T, prog *ast.Program) *hir.Program {
	t.Helper()
	return hir.NewLowerer().LowerProgram(prog)
}

func containsAll(text string, needles ...string) bool {
	for _, n := range needles {
		found := false
		for i := 0; i+len(n) <= len(text); i++ {
			if text[i:i+len(n)] == n {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
