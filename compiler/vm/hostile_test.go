package vm

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/code"
	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/object"
)

// runRaw executes hand-built instructions. Hostile or corrupted bytecode must
// surface as an error, never as a Go panic in the host process.
func runRaw(t *testing.T, ins code.Instructions, constants []object.Object) error {
	t.Helper()
	machine := New(&compiler.Bytecode{Instructions: ins, Constants: constants})
	return machine.Run()
}

func TestHostileBytecodeReturnsErrors(t *testing.T) {
	one := []object.Object{&object.Integer{Value: 1}}

	cases := []struct {
		name      string
		ins       code.Instructions
		constants []object.Object
		wantIn    string
	}{
		{
			name:      "truncated constant operand",
			ins:       code.Instructions{byte(code.OpConstant), 0x00},
			constants: one,
			wantIn:    "truncated",
		},
		{
			name:      "constant index out of range",
			ins:       append(code.Make(code.OpConstant, 7), code.Make(code.OpPop)...),
			constants: one,
			wantIn:    "constant index",
		},
		{
			name:      "jump past end of instructions",
			ins:       code.Make(code.OpJump, 9999),
			constants: nil,
			wantIn:    "jump target",
		},
		{
			name:      "unknown opcode",
			ins:       code.Instructions{0xFF},
			constants: nil,
			wantIn:    "unknown opcode",
		},
		{
			name:      "free variable outside a closure",
			ins:       append(code.Make(code.OpGetFree, 0), code.Make(code.OpPop)...),
			constants: nil,
			wantIn:    "free index 0",
		},
		{
			name:      "free index out of range",
			ins:       append(code.Make(code.OpGetFree, 200), code.Make(code.OpPop)...),
			constants: nil,
			wantIn:    "free",
		},
		{
			name:      "call with empty stack",
			ins:       code.Make(code.OpCall, 3),
			constants: nil,
			wantIn:    "call with",
		},
		{
			name: "string concat of integers",
			ins: concat(
				code.Make(code.OpConstant, 0),
				code.Make(code.OpConstant, 0),
				code.Make(code.OpStringConcat),
			),
			constants: one,
			wantIn:    "concatenation",
		},
		{
			name: "native call naming a non-string",
			ins: concat(
				code.Make(code.OpNativeCall, 0, 0),
			),
			constants: []object.Object{&object.Integer{Value: 1}},
			wantIn:    "native call name",
		},
		{
			name:      "truncated jump operand",
			ins:       code.Instructions{byte(code.OpJump)},
			constants: nil,
			wantIn:    "truncated",
		},
		{
			name: "array larger than the stack",
			ins: concat(
				code.Make(code.OpConstant, 0),
				code.Make(code.OpArray, 5),
			),
			constants: one,
			wantIn:    "array of 5",
		},
		{
			name: "odd hash element count",
			ins: concat(
				code.Make(code.OpConstant, 0),
				code.Make(code.OpHash, 1),
			),
			constants: one,
			wantIn:    "hash of 1",
		},
		{
			name: "negation of an unset global",
			ins: concat(
				code.Make(code.OpGetGlobal, 0),
				code.Make(code.OpMinus),
			),
			constants: nil,
			wantIn:    "nil",
		},
		{
			name: "global index past a short store",
			ins: concat(
				code.Make(code.OpNull),
				code.Make(code.OpSetGlobal, 40),
			),
			constants: nil,
			wantIn:    "global index",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if tc.name == "global index past a short store" {
				machine := NewWithGlobalsStore(
					&compiler.Bytecode{Instructions: tc.ins, Constants: tc.constants},
					make([]object.Object, 8),
				)
				err = machine.Run()
			} else {
				err = runRaw(t, tc.ins, tc.constants)
			}
			if err == nil {
				t.Fatalf("hostile bytecode ran without an error")
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Fatalf("error %q does not mention %q", err.Error(), tc.wantIn)
			}
		})
	}
}

func concat(parts ...code.Instructions) code.Instructions {
	var out code.Instructions
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func TestDisassemblerHandlesCorruptInput(t *testing.T) {
	// An undefined opcode is one corrupt byte, not an infinite loop.
	if got := (code.Instructions{0xFF}).String(); !strings.Contains(got, "ERROR") {
		t.Fatalf("expected an error line, got %q", got)
	}
	// A valid opcode with missing operand bytes reports truncation.
	if got := (code.Instructions{byte(code.OpConstant), 0x00}).String(); !strings.Contains(got, "truncated") {
		t.Fatalf("expected a truncation report, got %q", got)
	}
	if got := DisassembleWithConstants(code.Instructions{0xFF}, nil); !strings.Contains(got, "ERROR") {
		t.Fatalf("expected an error line, got %q", got)
	}
}

func TestDecoderRefusesAbsurdCounts(t *testing.T) {
	// Covered at the pkg/compiler level by the constant-count limit; here we
	// pin the VM side: a 2-byte constant index space cannot address absurd
	// pools, and the decoder refuses them first. This test guards the
	// constantAt path with the largest encodable index.
	machine := New(&compiler.Bytecode{
		Instructions: code.Make(code.OpConstant, 65535),
		Constants:    []object.Object{&object.Integer{Value: 1}},
	})
	if err := machine.Run(); err == nil {
		t.Fatal("expected an error for constant index 65535 with 1 constant")
	}
}
