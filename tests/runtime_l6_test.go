package tests

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	compilercap "github.com/joysriramsarkar/nilLang/compiler/capability"
	"github.com/joysriramsarkar/nilLang/compiler/code"
	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
)

// This file is the L6 (runtime completion) conformance suite: every case runs
// the same program through the tree-walking evaluator and the bytecode VM and
// requires both engines to agree on the value or on the typed error category.

// parseDirect parses source with no type checking and no lowering.
//
// Runtime gates (L6.3 effect enforcement in particular) must be tested below
// the static checker: the checker rejects an effectful call inside `pure fn`
// before either engine runs, so a test that routes through the frontend gate
// could never observe what the engines do with such a program. The sources
// here avoid constructs that need lowering (for, match, ?. and struct
// literals), which is what makes compiling the unlowered tree sound.
func parseDirect(t *testing.T, source string) *ast.Program {
	t.Helper()
	l := lexer.New(source)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors in %q: %v", source, p.Errors())
	}
	return prog
}

// evalDirect runs a parse-only program in the evaluator with builtins.
func evalDirect(t *testing.T, source string) object.Object {
	t.Helper()
	return evaluator.Eval(parseDirect(t, source), newEvaluatorEnv())
}

// vmDirect compiles a parse-only program and runs it in a fresh VM.
func vmDirect(t *testing.T, source string) (object.Object, error) {
	t.Helper()
	comp := compiler.New()
	if err := comp.Compile(parseDirect(t, source)); err != nil {
		t.Fatalf("compile %q: %v", source, err)
	}
	machine := vm.New(comp.Bytecode())
	err := machine.Run()
	if err != nil {
		return nil, err
	}
	return machine.LastPoppedStackElem(), nil
}

// l6ErrorMessage extracts the message from either error shape the evaluator
// can return: a categorised *object.RuntimeError or a legacy *object.Error.
func l6ErrorMessage(obj object.Object) string {
	switch err := obj.(type) {
	case *object.RuntimeError:
		return err.Message
	case *object.Error:
		return err.Message
	default:
		return ""
	}
}

// requireCategory asserts both engines report the same typed error category
// for source, and that it is the expected one.
func requireCategory(t *testing.T, source string, want object.ErrorCategory) {
	t.Helper()

	evaluated := evalDirect(t, source)
	evalCategory, ok := object.CategoryOf(evaluated)
	if !ok {
		t.Fatalf("evaluator reported no category for %q, want %q (got %v)", source, want, evaluated)
	}
	if evalCategory != want {
		t.Errorf("evaluator category = %q, want %q (source %q)", evalCategory, want, source)
	}

	_, vmErr := vmDirect(t, source)
	if vmErr == nil {
		t.Fatalf("VM did not fail for %q, want category %q", source, want)
	}
	vmCategory, ok := object.CategoryOf(vmErr)
	if !ok {
		t.Fatalf("VM error carries no category for %q, want %q (got %v)", source, want, vmErr)
	}
	if vmCategory != want {
		t.Errorf("VM category = %q, want %q (source %q)", vmCategory, want, source)
	}
}

// requireValue asserts both engines evaluate source to the same inspected
// value.
func requireValue(t *testing.T, source, want string) {
	t.Helper()

	evaluated := evalDirect(t, source)
	if errObj, ok := evaluated.(*object.Error); ok {
		t.Fatalf("evaluator error for %q: %s", source, errObj.Inspect())
	}
	if runtimeErr, ok := evaluated.(*object.RuntimeError); ok {
		t.Fatalf("evaluator runtime error for %q: %s", source, runtimeErr.Inspect())
	}
	if evaluated == nil {
		t.Fatalf("evaluator returned nil for %q", source)
	}
	if got := evaluated.Inspect(); got != want {
		t.Errorf("evaluator = %q, want %q (source %q)", got, want, source)
	}

	vmObj, vmErr := vmDirect(t, source)
	if vmErr != nil {
		t.Fatalf("VM error for %q: %v", source, vmErr)
	}
	if vmObj == nil {
		t.Fatalf("VM returned nil for %q", source)
	}
	if got := vmObj.Inspect(); got != want {
		t.Errorf("VM = %q, want %q (source %q)", got, want, source)
	}
}

// L6.2 — a task shares its parent's globals: a write the task makes is
// visible after the await, in both engines. A globals copy would leave the
// parent reading the old value.
func TestL6TaskSharesGlobals(t *testing.T) {
	requireValue(t, `let x = 1; let tk = task { x = 42; }; await tk; x;`, "42")
}

// L6.2 — the task also reads what the parent wrote before spawning.
func TestL6TaskReadsParentGlobals(t *testing.T) {
	requireValue(t, `let base = 40; let tk = task { base + 2; }; await tk;`, "42")
}

// L6.1 — withTimeout returns the function's value when it finishes in time.
func TestL6WithTimeoutReturnsValue(t *testing.T) {
	requireValue(t, `withTimeout(5000, fn() { return 7; });`, "7")
}

// L6.1 — withTimeout reports TaskCancelled when the deadline expires, in
// both engines. The function parks on an unbuffered channel nobody sends
// to, so it never finishes on its own; the parked goroutine this leaves
// behind is the documented cost of a language without preemption.
func TestL6WithTimeoutReportsTaskCancelled(t *testing.T) {
	requireCategory(t, `withTimeout(50, fn() { return receive(Channel(0)); });`, object.CategoryTaskCancelled)
}

// L6.3 — an effectful builtin inside `pure fn` is a runtime E0202 in both
// engines. The static checker rejects this before execution, so the test
// parses directly: what is asserted here is the dynamic enforcement for
// programs that reach the engines unchecked.
func TestL6PureRejectsEffectfulBuiltin(t *testing.T) {
	source := `pure fn f() { writeFile("l6_effect.txt", "x"); } f();`
	evaluated := evalDirect(t, source)
	evalCategory, ok := object.CategoryOf(evaluated)
	if !ok || evalCategory != object.CategoryTypeError {
		t.Fatalf("evaluator category = %q, want TypeError for %q (got %v)", evalCategory, source, evaluated)
	}
	if msg := l6ErrorMessage(evaluated); !strings.Contains(msg, "E0202") {
		t.Errorf("evaluator error should name E0202, got %q", msg)
	}

	_, vmErr := vmDirect(t, source)
	vmCategory, ok := object.CategoryOf(vmErr)
	if !ok || vmCategory != object.CategoryTypeError {
		t.Fatalf("VM category = %q, want TypeError for %q (got %v)", vmCategory, source, vmErr)
	}
	if !strings.Contains(vmErr.Error(), "E0202") {
		t.Errorf("VM error should name E0202, got %q", vmErr.Error())
	}
}

// L6.3 — a pure function calling a pure function is fine in both engines.
func TestL6PureCallingPureSucceeds(t *testing.T) {
	requireValue(t, `pure fn g() { return 21; } pure fn f() { return g() * 2; } f();`, "42")
}

// L6.3 — a non-pure function called from a pure one is an effect violation
// in both engines, even when the callee itself only computes.
func TestL6PureRejectsEffectfulCall(t *testing.T) {
	requireCategory(t, `fn impure() { return 1; } pure fn f() { return impure(); } f();`, object.CategoryTypeError)
}

// L6.4 — a denied capability reports CategoryCapabilityDenied in both
// engines. The evaluator consults evaluator.CapabilityChecker, which the
// test installs exactly as `nil run` does; it is reset afterwards so the
// default (ungated) behaviour returns for every other test.
func TestL6CapabilityDenialAgreesAcrossEngines(t *testing.T) {
	previous := evaluator.GetCapabilityChecker()
	evaluator.SetCapabilityChecker(compilercap.GrantChecker(nil))
	defer func() { evaluator.SetCapabilityChecker(previous) }()

	for _, source := range []string{
		`writeFile("l6_denied.txt", "x");`,
		`exec("echo hello");`,
	} {
		evaluated := evalDirect(t, source)
		evalCategory, ok := object.CategoryOf(evaluated)
		if !ok || evalCategory != object.CategoryCapabilityDenied {
			t.Errorf("evaluator category = %q, want CapabilityDenied for %q (got %v)",
				evalCategory, source, evaluated)
		}

		vmErr := runWithCapabilities(t, source, nil)
		vmCategory, ok := object.CategoryOf(vmErr)
		if !ok || vmCategory != object.CategoryCapabilityDenied {
			t.Errorf("VM category = %q, want CapabilityDenied for %q (got %v)",
				vmCategory, source, vmErr)
		}
	}
}

// L6.4 — a granted capability runs in both engines.
func TestL6CapabilityGrantRunsInBothEngines(t *testing.T) {
	previous := evaluator.GetCapabilityChecker()
	evaluator.SetCapabilityChecker(compilercap.GrantChecker([]string{compilercap.Stdout}))
	defer func() { evaluator.SetCapabilityChecker(previous) }()

	requireValue(t, `println("l6"); 42;`, "42")
	if err := runWithCapabilities(t, `println("l6");`, []string{compilercap.Stdout}); err != nil {
		t.Errorf("VM should allow println with the stdout grant: %v", err)
	}
}

// L6.5 — a task whose body fails faults with TaskFaulted in both engines,
// and the awaiting program observes the fault rather than the inner error.
func TestL6TaskFaultIsolation(t *testing.T) {
	requireCategory(t, `let tk = task { 10 / 0; }; await tk;`, object.CategoryTaskFaulted)
}

// L6.5 — a Go panic inside a task faults the future in the evaluator rather
// than crashing the process. The panicking builtin is registered for this
// test only and removed afterwards.
func TestL6TaskPanicIsFaultInEvaluator(t *testing.T) {
	evaluator.SetBuiltin("__l6boom", &object.Builtin{
		Fn: func(args ...object.Object) object.Object { panic("l6 boom") },
	})
	defer evaluator.DeleteBuiltin("__l6boom")

	evaluated := evalDirect(t, `let tk = task { __l6boom(); }; await tk;`)
	category, ok := object.CategoryOf(evaluated)
	if !ok || category != object.CategoryTaskFaulted {
		t.Fatalf("evaluator category = %q, want TaskFaulted (got %v)", category, evaluated)
	}
	if msg := l6ErrorMessage(evaluated); !strings.Contains(msg, "panicked") {
		t.Errorf("fault should report the panic, got %q", msg)
	}
}

// L6.6 — a mid-program failure halts both engines identically: the
// evaluator used to continue past a categorised error to the next
// statement while the VM unwound, so `10 / 0; 42;` answered 42 in one
// engine and DivisionByZero in the other.
func TestL6MidProgramFailureHaltsBothEngines(t *testing.T) {
	requireCategory(t, `10 / 0; 42;`, object.CategoryDivisionByZero)
}

// L6.6 — integer division and modulo by zero agree on DivisionByZero.
func TestL6DivZeroAgreesAcrossEngines(t *testing.T) {
	requireCategory(t, `10 / 0;`, object.CategoryDivisionByZero)
	requireCategory(t, `10 % 0;`, object.CategoryDivisionByZero)
}

// L6.6 — unbounded recursion agrees on FrameOverflow, not StackOverflow.
func TestL6DeepRecursionIsFrameOverflow(t *testing.T) {
	requireCategory(t, `fn f(n) { return f(n + 1); } f(0);`, object.CategoryFrameOverflow)
}

// L6.1 — concurrent tasks must not share evaluator call depth or purity.
//
// Depth and purity used to live in package globals, so tasks evaluating
// function calls on their own goroutines raced on them (found by `go test
// -race`: concurrent applyFunction calls read and wrote one counter). They
// now travel in the environment chain, so each task counts its own. This
// test spawns several tasks that all call functions — including a pure one
// alongside effectful builtins — and requires both engines to agree. Under
// -race it fails if any evaluator state is shared again.
func TestL6ConcurrentTasksCountTheirOwnDepth(t *testing.T) {
	requireValue(t, `
fn add(a, b) { return a + b; }
pure fn psq(n) { return n * n; }
let t1 = task { add(1, 2); };
let t2 = task { add(10, 20); };
let t3 = task { psq(5); };
let t4 = task { add(psq(2), psq(3)); };
let r1 = await t1;
let r2 = await t2;
let r3 = await t3;
let r4 = await t4;
r1 + r2 + r3 + r4;
`, "71")
}

// L6.6 — a single frame larger than the operand stack is StackOverflow in
// the VM. No source program can declare that many locals, so the test builds
// the call directly: the category is what the cross-engine contract pins,
// and StackOverflow must stay reachable and distinct from FrameOverflow.
func TestL6OversizedFrameIsStackOverflow(t *testing.T) {
	inner := &object.CompiledFunction{
		Instructions:  code.Instructions{byte(code.OpReturn)},
		NumLocals:     vm.StackSize + 1,
		NumParameters: 0,
		Name:          "oversized",
	}
	var mainIns code.Instructions
	mainIns = append(mainIns, code.Make(code.OpClosure, 0, 0)...)
	mainIns = append(mainIns, code.Make(code.OpCall, 0)...)
	bytecode := &compiler.Bytecode{
		Instructions: mainIns,
		Constants:    []object.Object{inner},
	}
	machine := vm.New(bytecode)
	err := machine.Run()
	if err == nil {
		t.Fatal("expected StackOverflow, got nil")
	}
	category, ok := object.CategoryOf(err)
	if !ok || category != object.CategoryStackOverflow {
		t.Fatalf("VM category = %q, want StackOverflow (got %v)", category, err)
	}
}
