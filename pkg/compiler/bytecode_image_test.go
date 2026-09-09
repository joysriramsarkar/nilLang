package compiler

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
)

func TestBytecodeImageRoundTripExecution(t *testing.T) {
	pipeline := NewPipeline(`
let format = fn(prefix, value) { prefix + str(value); };
format("score=", 40 + 2.5);
`, "image-test.nil")
	if err := pipeline.Compile(); err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	image, err := pipeline.GetBytecodeImage()
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	bytecode, err := DecodeBytecode(image)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	machine := vm.New(bytecode)
	if err := machine.Run(); err != nil {
		t.Fatalf("decoded bytecode execution failed: %v", err)
	}

	result, ok := machine.LastPoppedStackElem().(*object.String)
	if !ok || result.Value != "score=42.5" {
		t.Fatalf("expected score=42.5, got %T (%+v)", machine.LastPoppedStackElem(), machine.LastPoppedStackElem())
	}
}

func TestDecodeBytecodeRejectsInvalidImages(t *testing.T) {
	for _, image := range [][]byte{
		[]byte("legacy instructions"),
		append([]byte{'N', 'A', 'B', 'C', 0, 2}, make([]byte, 8)...),
	} {
		if _, err := DecodeBytecode(image); err == nil {
			t.Fatalf("expected invalid image %v to be rejected", image)
		}
	}
}

func TestDecodeBytecodeRejectsTrailingBytes(t *testing.T) {
	pipeline := NewPipeline("1;", "trailing-test.nil")
	if err := pipeline.Compile(); err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	image, err := pipeline.GetBytecodeImage()
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	if _, err := DecodeBytecode(append(image, 0)); err == nil {
		t.Fatal("expected trailing byte to be rejected")
	}
}
