package wasm

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/mir"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

func TestWasmCompilation(t *testing.T) {
	input := `
	let a = 10;
	let b = 20;
	let c = a + b;
	return c;
	`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	lowerer := hir.NewLowerer()
	hProg := lowerer.LowerProgram(prog)

	mirLowerer := mir.NewLowerer()
	mirProg := mirLowerer.LowerHIR(hProg)

	wasmCompiler := NewCompiler()
	module, err := wasmCompiler.Compile(mirProg)
	if err != nil {
		t.Fatalf("Failed to compile to WASM: %v", err)
	}

	if module.WAT == "" {
		t.Errorf("Expected non-empty WAT output")
	}

	// Verify WASM Magic header: 0x00 0x61 0x73 0x6d
	if len(module.Binary) < 8 {
		t.Fatalf("WASM binary too short: %d bytes", len(module.Binary))
	}
	if module.Binary[0] != 0x00 || module.Binary[1] != 0x61 || module.Binary[2] != 0x73 || module.Binary[3] != 0x6d {
		t.Errorf("Invalid WASM magic header: %v", module.Binary[:4])
	}
}
