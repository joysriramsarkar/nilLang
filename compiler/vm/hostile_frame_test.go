package vm

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/code"
	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/object"
)

// TestHostileArrayCountCannotLiftCallerSlots pins frame isolation for
// counted stack instructions. The hand-built function below executes
// OpArray with a count that fits the *global* stack depth — the main
// frame's closure slot is still on the stack beneath the call — but
// exceeds the values its own frame pushed. Before the frame-relative
// check, buildArray happily lifted the caller's slot into the array;
// no bound was violated globally, so nothing panicked and the isolation
// break was silent.
func TestHostileArrayCountCannotLiftCallerSlots(t *testing.T) {
	fn := &object.CompiledFunction{
		Instructions: concat(
			code.Make(code.OpArray, 3),
			code.Make(code.OpReturnValue),
		),
		NumLocals:     2,
		NumParameters: 1,
	}
	constants := []object.Object{
		fn,
		&object.Integer{Value: 111},
	}
	// Main: build the closure, push one argument above it, call. Inside
	// the call the stack is [closure, arg, local] — sp is 3 globally but
	// the callee frame owns only the 2 slots from its base pointer.
	ins := concat(
		code.Make(code.OpClosure, 0, 0),
		code.Make(code.OpConstant, 1),
		code.Make(code.OpCall, 1),
		code.Make(code.OpPop),
	)
	machine := New(&compiler.Bytecode{Instructions: ins, Constants: constants})
	err := machine.Run()
	if err == nil {
		t.Fatal("OpArray with a caller-crossing count must be refused, got nil error")
	}
	if !strings.Contains(err.Error(), "array of 3 elements") {
		t.Fatalf("error = %q, want it to name the refused array count", err)
	}
}

// TestHostileHashCountCannotLiftCallerSlots is the OpHash counterpart:
// an even count that fits the global depth but not the frame's own.
func TestHostileHashCountCannotLiftCallerSlots(t *testing.T) {
	fn := &object.CompiledFunction{
		Instructions: concat(
			code.Make(code.OpHash, 4),
			code.Make(code.OpReturnValue),
		),
		NumLocals:     3,
		NumParameters: 1,
	}
	constants := []object.Object{
		fn,
		&object.Integer{Value: 111},
	}
	ins := concat(
		code.Make(code.OpClosure, 0, 0),
		code.Make(code.OpConstant, 1),
		code.Make(code.OpCall, 1),
		code.Make(code.OpPop),
	)
	machine := New(&compiler.Bytecode{Instructions: ins, Constants: constants})
	err := machine.Run()
	if err == nil {
		t.Fatal("OpHash with a caller-crossing count must be refused, got nil error")
	}
	if !strings.Contains(err.Error(), "hash of 4 elements") {
		t.Fatalf("error = %q, want it to name the refused hash count", err)
	}
}
