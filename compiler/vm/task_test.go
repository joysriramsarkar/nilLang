package vm

import (
	"strings"
	"testing"
	"time"

	compilercap "github.com/joysriramsarkar/nilLang/compiler/capability"
	"github.com/joysriramsarkar/nilLang/compiler/code"
	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/runtime/concurrency"
)

// compileSource compiles Nilang source with no type checking, so the tests
// below can drive programs the static gate would reject.
func compileSource(t *testing.T, source string) *compiler.Bytecode {
	t.Helper()
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors in %q: %v", source, p.Errors())
	}
	comp := compiler.New()
	if err := comp.Compile(program); err != nil {
		t.Fatalf("compile %q: %v", source, err)
	}
	return comp.Bytecode()
}

// TestOpTaskSharesGlobalsWithParent pins L6.2 at the VM level: the child and
// the parent use the same backing store, so the task's write is what the
// parent reads after the await.
func TestOpTaskSharesGlobalsWithParent(t *testing.T) {
	machine := New(compileSource(t, `let x = 1; let tk = task { x = 42; }; await tk; x;`))
	if err := machine.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := machine.LastPoppedStackElem()
	integer, ok := got.(*object.Integer)
	if !ok || integer.Value != 42 {
		t.Fatalf("got %v, want 42", got)
	}
	if scopes := machine.TaskChildScopes(); len(scopes) != 1 {
		t.Fatalf("want 1 recorded child scope, got %d", len(scopes))
	}
}

// TestOpTaskFaultCancelsSiblings pins the structured tree at the VM level:
// the faulted child's scope is Faulted, and the fault travelled through the
// shared parent, which is what cancels the siblings (CONCURRENCY.md §3.1).
func TestOpTaskFaultCancelsSiblings(t *testing.T) {
	machine := New(compileSource(t, `let bad = task { 10 / 0; }; await bad;`))
	err := machine.Run()
	if err == nil {
		t.Fatal("expected a fault, got nil")
	}
	category, ok := object.CategoryOf(err)
	if !ok || category != object.CategoryTaskFaulted {
		t.Fatalf("category = %q, want TaskFaulted (got %v)", category, err)
	}
	scopes := machine.TaskChildScopes()
	if len(scopes) != 1 {
		t.Fatalf("want 1 recorded child scope, got %d", len(scopes))
	}
	if status := scopes[0].Status(); status != concurrency.StatusFaulted {
		t.Errorf("child scope status = %s, want FAULTED", status)
	}
}

// TestOpTaskPanicBecomesFault feeds the child a closure whose first
// instruction indexes past the constant pool: a genuine Go panic inside the
// child's Run, which must surface as a TaskFaulted fault rather than crash
// the test process (L6.5).
func TestOpTaskPanicBecomesFault(t *testing.T) {
	badFn := &object.CompiledFunction{
		Instructions:  code.Instructions{byte(code.OpConstant), 0x00, 0x63},
		NumLocals:     0,
		NumParameters: 0,
		Name:          "bad",
	}
	var mainIns code.Instructions
	mainIns = append(mainIns, code.Make(code.OpClosure, 0, 0)...)
	mainIns = append(mainIns, code.Make(code.OpTask)...)
	mainIns = append(mainIns, code.Make(code.OpAwait)...)
	mainIns = append(mainIns, code.Make(code.OpPop)...)
	machine := New(&compiler.Bytecode{
		Instructions: mainIns,
		Constants:    []object.Object{badFn},
	})
	err := machine.Run()
	if err == nil {
		t.Fatal("expected a fault, got nil")
	}
	category, ok := object.CategoryOf(err)
	if !ok || category != object.CategoryTaskFaulted {
		t.Fatalf("category = %q, want TaskFaulted (got %v)", category, err)
	}
	if !strings.Contains(err.Error(), "panicked") {
		t.Errorf("fault should report the panic, got %q", err.Error())
	}
}

// TestOpTaskCancellationStopsSpinningChild pins cooperative cancellation: a
// task looping forever observes its scope's cancellation at the next poll
// and reports TaskCancelled, so awaiting it returns instead of hanging.
func TestOpTaskCancellationStopsSpinningChild(t *testing.T) {
	machine := New(compileSource(t, `let spin = task { while (true) { } }; await spin;`))
	done := make(chan error, 1)
	go func() { done <- machine.Run() }()

	deadline := time.Now().Add(10 * time.Second)
	for {
		if scopes := machine.TaskChildScopes(); len(scopes) > 0 {
			scopes[0].Cancel()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("task scope never appeared")
		}
		time.Sleep(5 * time.Millisecond)
	}

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected TaskCancelled, got nil")
		}
		category, ok := object.CategoryOf(err)
		if !ok || category != object.CategoryTaskCancelled {
			t.Fatalf("category = %q, want TaskCancelled (got %v)", category, err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("awaiting a cancelled task hung")
	}
}

// TestPureDepthIsTrackedAcrossCalls drives a pure function calling an
// effectful builtin with no type checking: the VM must refuse with E0202.
func TestPureDepthIsTrackedAcrossCalls(t *testing.T) {
	machine := New(compileSource(t, `pure fn f() { writeFile("vm_pure.txt", "x"); } f();`))
	err := machine.Run()
	if err == nil {
		t.Fatal("expected E0202, got nil")
	}
	category, ok := object.CategoryOf(err)
	if !ok || category != object.CategoryTypeError {
		t.Fatalf("category = %q, want TypeError (got %v)", category, err)
	}
	if !strings.Contains(err.Error(), "E0202") {
		t.Errorf("error should name E0202, got %q", err.Error())
	}
}

// TestPureFunctionCallingPureFunctionSucceeds is the negative control: the
// depth tracker must not refuse what the declaration allows.
func TestPureFunctionCallingPureFunctionSucceeds(t *testing.T) {
	machine := New(compileSource(t, `pure fn g() { return 21; } pure fn f() { return g() * 2; } f();`))
	if err := machine.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := machine.LastPoppedStackElem()
	integer, ok := got.(*object.Integer)
	if !ok || integer.Value != 42 {
		t.Fatalf("got %v, want 42", got)
	}
}

// TestIndirectGatedCallIsDenied covers the alias form the OpNativeCall
// instrumentation cannot see: a gated builtin reached through a variable
// still meets the capability gate when one is installed.
func TestIndirectGatedCallIsDenied(t *testing.T) {
	machine := New(compileSource(t, `let f = writeFile; f("vm_alias.txt", "x");`))
	machine.CapabilityChecker = compilercap.GrantChecker(nil)
	err := machine.Run()
	if err == nil {
		t.Fatal("expected a capability denial, got nil")
	}
	category, ok := object.CategoryOf(err)
	if !ok || category != object.CategoryCapabilityDenied {
		t.Fatalf("category = %q, want CapabilityDenied (got %v)", category, err)
	}
}

// TestWithTimeoutBuiltinCompletesInTime checks the VM-native withTimeout:
// the evaluator's builtin cannot run a VM closure, so the VM dispatches it
// itself on a shared-store child.
func TestWithTimeoutBuiltinCompletesInTime(t *testing.T) {
	machine := New(compileSource(t, `withTimeout(5000, fn() { return 7; });`))
	if err := machine.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := machine.LastPoppedStackElem()
	integer, ok := got.(*object.Integer)
	if !ok || integer.Value != 7 {
		t.Fatalf("got %v, want 7", got)
	}
}

// TestWithTimeoutBuiltinReportsTimeout parks the timed closure on a channel
// nobody sends to and requires the deadline to win with TaskCancelled. The
// parked child is abandoned by design: the language has no preemption, so
// the deadline ends the wait, not the work.
func TestWithTimeoutBuiltinReportsTimeout(t *testing.T) {
	machine := New(compileSource(t, `withTimeout(50, fn() { return receive(Channel(0)); });`))
	err := machine.Run()
	if err == nil {
		t.Fatal("expected TaskCancelled, got nil")
	}
	category, ok := object.CategoryOf(err)
	if !ok || category != object.CategoryTaskCancelled {
		t.Fatalf("category = %q, want TaskCancelled (got %v)", category, err)
	}
}
