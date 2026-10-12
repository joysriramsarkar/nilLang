package wasm

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/mir"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

// lowerSource runs a Nilang source all the way to MIR, the WASM backend's
// input, failing the test on any frontend error.
func lowerSource(t *testing.T, input string) *mir.Program {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("Parse errors: %v", p.Errors())
	}

	lowerer := hir.NewLowerer()
	hProg := lowerer.LowerProgram(prog)

	mirLowerer := mir.NewLowerer()
	return mirLowerer.LowerHIR(hProg)
}

func compileSource(t *testing.T, input string) *WasmModule {
	t.Helper()
	mod, err := NewCompiler().Compile(lowerSource(t, input))
	if err != nil {
		t.Fatalf("Failed to compile to WASM: %v", err)
	}
	return mod
}

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

// TestWasmLoopControlFlow pins L7.1: a MIR branch that loops back must come
// out as block/loop/br_if/br, not as a comment. The shapes are also asserted
// on the binary side by opcode.
func TestWasmLoopControlFlow(t *testing.T) {
	mod := compileSource(t, `
let i = 0;
let s = 0;
while (i < 5) {
  s = s + i;
  i = i + 1;
}
return s;
`)
	for _, want := range []string{"loop", "block", "br_if", "br "} {
		if !strings.Contains(mod.WAT, want) {
			t.Errorf("WAT should contain %q:\n%s", want, mod.WAT)
		}
	}
	if !containsOpcode(mod.Binary, opLoop) || !containsOpcode(mod.Binary, opBrIf) || !containsOpcode(mod.Binary, opBr) {
		t.Error("binary should encode loop (0x03), br_if (0x0d) and br (0x0c)")
	}
}

// TestWasmBranchControlFlow pins L7.1 for conditionals: an if/else must come
// out as if/else/end with an i32 condition.
func TestWasmBranchControlFlow(t *testing.T) {
	mod := compileSource(t, `
let x = 10;
if (x > 5) {
  x = 100;
} else {
  x = 200;
}
return x;
`)
	for _, want := range []string{"\n    if\n", "\n    else\n", "\n    end\n"} {
		if !strings.Contains(mod.WAT, want) {
			t.Errorf("WAT should contain %q:\n%s", want, mod.WAT)
		}
	}
	if !containsOpcode(mod.Binary, opIf) || !containsOpcode(mod.Binary, opElse) {
		t.Error("binary should encode if (0x04) and else (0x05)")
	}
}

// TestWasmLocals pins L7.2: parameters and MIR variables become declared
// locals read and written with local.get/set/tee, and the binary carries a
// locals section in its function bodies.
func TestWasmLocals(t *testing.T) {
	mod := compileSource(t, `
fn add(a, b) { return a + b; }
let r = add(2, 3);
return r;
`)
	for _, want := range []string{"(param $p_a i64)", "(param $p_b i64)", "(local ", "local.get", "local.set"} {
		if !strings.Contains(mod.WAT, want) {
			t.Errorf("WAT should contain %q:\n%s", want, mod.WAT)
		}
	}
	if !containsOpcode(mod.Binary, opLocalGet) || !containsOpcode(mod.Binary, opLocalSet) {
		t.Error("binary should encode local.get (0x20) and local.set (0x21)")
	}
}

// TestWasmCall pins L7.3: a MIR call becomes a call by function index, in
// both text (named) and binary (0x10) form.
func TestWasmCall(t *testing.T) {
	mod := compileSource(t, `
fn add(a, b) { return a + b; }
let r = add(2, 3);
return r;
`)
	if !strings.Contains(mod.WAT, "call $add") {
		t.Errorf("WAT should call $add:\n%s", mod.WAT)
	}
	if !containsOpcode(mod.Binary, opCall) {
		t.Error("binary should encode call (0x10)")
	}
}

// TestWasmValTypeMapping pins L7.4: the Nilang-to-WASM type mapping covers
// all four value types.
func TestWasmValTypeMapping(t *testing.T) {
	cases := map[string]byte{
		"i32": vtI32, "bool": vtI32, "string": vtI32,
		"i64": vtI64, "int": vtI64,
		"f32": vtF32, "float32": vtF32,
		"f64": vtF64, "float": vtF64, "float64": vtF64,
	}
	for name, want := range cases {
		if got := WasmValType(name); got != want {
			t.Errorf("WasmValType(%q) = 0x%x, want 0x%x", name, got, want)
		}
	}
	if watValTypeName(vtI32) != "i32" || watValTypeName(vtI64) != "i64" ||
		watValTypeName(vtF32) != "f32" || watValTypeName(vtF64) != "f64" {
		t.Error("WAT type names must render i32/i64/f32/f64")
	}
}

// TestWasmMemoryAndData pins L7.5: string constants land in a data segment
// under a declared linear memory.
func TestWasmMemoryAndData(t *testing.T) {
	mod := compileSource(t, `
let h = "hello";
return h;
`)
	for _, want := range []string{"(memory", "(data"} {
		if !strings.Contains(mod.WAT, want) {
			t.Errorf("WAT should contain %q:\n%s", want, mod.WAT)
		}
	}
	if !strings.Contains(mod.WAT, "hello") {
		t.Errorf("WAT data segment should carry the string:\n%s", mod.WAT)
	}
	ids := wasmSectionIDs(t, mod.Binary)
	if !hasSection(ids, 5) || !hasSection(ids, 11) {
		t.Errorf("binary should have memory (5) and data (11) sections, got %v", ids)
	}
}

// TestWasmBinaryStructure pins L7.6: magic, version and section order
// (type, import, func, memory, export, code, data). A binary in this shape
// is what WebAssembly.validate accepts structurally.
func TestWasmBinaryStructure(t *testing.T) {
	mod := compileSource(t, `
let i = 0;
let s = 0;
while (i < 5) {
  s = s + i;
  i = i + 1;
}
return s;
`)
	ids := wasmSectionIDs(t, mod.Binary)
	want := []byte{1, 2, 3, 5, 7, 10}
	if len(ids) != len(want) {
		t.Fatalf("section order = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("section order = %v, want %v", ids, want)
		}
	}
}

// TestWasmNestedLoops guards the region emitter's hardest shape: an inner
// loop whose exit reaches the outer header must still decompose into two
// loop/block pairs rather than an if with a stray back edge.
func TestWasmNestedLoops(t *testing.T) {
	mod := compileSource(t, `
let s = 0;
let i = 0;
while (i < 3) {
  let j = 0;
  while (j < 3) {
    s = s + 1;
    j = j + 1;
  }
  i = i + 1;
}
return s;
`)
	if count := strings.Count(mod.WAT, "(loop"); count != 2 {
		t.Errorf("WAT should nest 2 loops, found %d:\n%s", count, mod.WAT)
	}
	// Comments may say "unreachable"; an actual unreachable instruction may
	// not appear: every edge in a reducible nest has a structured target.
	for _, line := range strings.Split(mod.WAT, "\n") {
		if strings.TrimSpace(line) == "unreachable" || strings.HasPrefix(strings.TrimSpace(line), "unreachable ") {
			t.Errorf("WAT should need no unreachable fallback:\n%s", mod.WAT)
			break
		}
	}
}

// TestNilRuntimeJS pins L7.7: the runtime bridge exists and covers DOM,
// canvas, timers, fetch and puts/log.
func TestNilRuntimeJS(t *testing.T) {
	js := NilRuntimeJS()
	if js == "" {
		t.Fatal("NilRuntimeJS must not be empty")
	}
	for _, want := range []string{
		"NilRuntime", "puts", "setTimeout", "fetch", "getContext",
		"makeImportObject", "querySelector",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("nil_runtime.js should contain %q", want)
		}
	}
}

// TestBrowserHarnessIncludesRuntime pins L7.8: the harness loads the bridge,
// mounts the DOM container and instantiates the module through it.
func TestBrowserHarnessIncludesRuntime(t *testing.T) {
	html := GenerateBrowserHarness("demo", "app.wasm")
	for _, want := range []string{
		"nil_runtime.js", "nil-app", "NilRuntime", "instantiate",
		"app.wasm", "nil-canvas",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("harness should contain %q", want)
		}
	}
}

// wasmSectionIDs parses a binary's section headers and returns their IDs in
// order, checking magic and version on the way (L7.6).
func wasmSectionIDs(t *testing.T, bin []byte) []byte {
	t.Helper()
	if len(bin) < 8 {
		t.Fatalf("WASM binary too short: %d bytes", len(bin))
	}
	if bin[0] != 0x00 || bin[1] != 0x61 || bin[2] != 0x73 || bin[3] != 0x6d {
		t.Fatalf("Invalid WASM magic header: %v", bin[:4])
	}
	if bin[4] != 0x01 || bin[5] != 0x00 || bin[6] != 0x00 || bin[7] != 0x00 {
		t.Fatalf("Invalid WASM version: %v", bin[4:8])
	}
	var ids []byte
	off := 8
	for off < len(bin) {
		id := bin[off]
		off++
		size, n := decodeULEB(bin[off:])
		off += n + int(size)
		if off > len(bin) {
			t.Fatalf("section %d overruns the binary", id)
		}
		ids = append(ids, id)
	}
	return ids
}

// decodeULEB reads an unsigned LEB128 at the front of buf, returning the
// value and the bytes consumed.
func decodeULEB(buf []byte) (uint64, int) {
	var result uint64
	var shift uint
	for i, b := range buf {
		result |= uint64(b&0x7F) << shift
		if b&0x80 == 0 {
			return result, i + 1
		}
		shift += 7
	}
	return result, len(buf)
}

func hasSection(ids []byte, want byte) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func containsOpcode(bin []byte, opcode byte) bool {
	for _, b := range bin {
		if b == opcode {
			return true
		}
	}
	return false
}
