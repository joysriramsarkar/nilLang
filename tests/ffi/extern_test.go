package ffi

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/aot"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/llvm"
	"github.com/joysriramsarkar/nilLang/compiler/mir"
	"github.com/joysriramsarkar/nilLang/compiler/toolchain"
)

// TestAnExternCallCrossesTheBoundaryAndComesBack is the gate on L5.1.
//
// Everything before this is a declaration: a name, a signature, and a call the
// backend emits. Everything after it is the only thing that matters, which is that
// a value went into C, C did something to it, and the answer came back correct.
//
// A test that stopped at the emitted IR would pass on a module that calls the
// wrong symbol with the wrong convention and produces a program that runs and
// prints a wrong number. Only running it settles the question, and running it is
// cheap.
func TestAnExternCallCrossesTheBoundaryAndComesBack(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "one argument",
			source: `
				extern "C" fn nilffi_double(value: Int) -> Int;
				nilffi_double(21);
			`,
			want: "42",
		},
		{
			name: "two arguments, in order",
			source: `
				extern "C" fn nilffi_add(a: Int, b: Int) -> Int;
				nilffi_add(20, 22);
			`,
			want: "42",
		},
		{
			name: "no arguments",
			source: `
				extern "C" fn nilffi_answer() -> Int;
				nilffi_answer();
			`,
			want: "42",
		},
		{
			// Past 32 bits. A boundary that truncated rather than converted would
			// answer 0, and a test using only small numbers would never see it.
			name: "a value past 32 bits",
			source: `
				extern "C" fn nilffi_large() -> Int;
				nilffi_large();
			`,
			want: "8589934592",
		},
		{
			name: "a negative value",
			source: `
				extern "C" fn nilffi_negative() -> Int;
				nilffi_negative();
			`,
			want: "-7",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runProgram(t, tc.source, buildFixture(t, toolchain.DiscoverToolchain(llvm.TargetHost)))
			if !strings.Contains(got, tc.want) {
				t.Errorf("the program printed %q, wanted it to contain %q", got, tc.want)
			}
		})
	}
}

// TestAnExternCallIsEmittedAsACSymbol says what the linker has to see.
//
// Two things, and both are load-bearing. The declaration is a `declare` rather
// than a `define`, because a definition would collide with the one in the C file.
// And the symbol carries no `nil.` prefix, because the prefix exists for symbols
// *this module* defines and a C symbol has to be the name the linker looks for.
func TestAnExternCallIsEmittedAsACSymbol(t *testing.T) {
	ir := emit(t, `
		extern "C" fn nilffi_double(value: Int) -> Int;
		nilffi_double(21);
	`)

	if !strings.Contains(ir, "declare i64 @nilffi_double(i64)") {
		t.Errorf("there is no declaration of the C symbol:\n%s", ir)
	}
	if strings.Contains(ir, "define i64 @nilffi_double") {
		t.Errorf("the foreign function was defined rather than declared:\n%s", ir)
	}
	if !strings.Contains(ir, "call i64 @nilffi_double(i64 ") {
		t.Errorf("the call does not name the C symbol:\n%s", ir)
	}
	if strings.Contains(ir, "@nil.nilffi_double") {
		t.Errorf("the C symbol was given this module's prefix, which nothing defines:\n%s", ir)
	}
}

// TestAnExternFunctionHasNoBodyInMIR is the same fact one stage earlier.
//
// MIR says it too: a foreign function has no blocks. A body here would be emitted
// as a definition, and the linker would report a duplicate symbol for a function
// that already exists somewhere the reader has never looked.
func TestAnExternFunctionHasNoBodyInMIR(t *testing.T) {
	program := lower(t, `
		extern "C" fn nilffi_double(value: Int) -> Int;
		nilffi_double(21);
	`)

	fn, found := program.Functions["nilffi_double"]
	if !found {
		t.Fatalf("the declaration is not in the program's functions:\n%s", program.String())
	}
	if !fn.Foreign {
		t.Error("the function is not marked foreign, so a back end would emit a body for it")
	}
	if len(fn.Blocks) != 0 {
		t.Errorf("a foreign function has %d blocks; it is implemented elsewhere", len(fn.Blocks))
	}
	if fn.Abi != "C" {
		t.Errorf("the ABI is %q, wanted \"C\"", fn.Abi)
	}
}

// TestAnExternCallIsCheckedAtTheCallSite is why the declaration goes through the
// type checker.
//
// The signature is the only thing the language knows about the function. A call
// that passes the wrong types should be a diagnostic here — a line and a message —
// rather than a conversion inside the FFI at run time, which is a corrupted call
// and a stack nobody can read back.
func TestAnExternCallIsCheckedAtTheCallSite(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		wantErr string
	}{
		{
			name: "the wrong argument type",
			source: `
				extern "C" fn nilffi_double(value: Int) -> Int;
				nilffi_double("not a number");
			`,
			wantErr: "Int",
		},
		{
			name: "too few arguments",
			source: `
				extern "C" fn nilffi_add(a: Int, b: Int) -> Int;
				nilffi_add(1);
			`,
			wantErr: "2",
		},
		{
			name: "too many arguments",
			source: `
				extern "C" fn nilffi_add(a: Int, b: Int) -> Int;
				nilffi_add(1, 2, 3);
			`,
			wantErr: "2",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := frontend.Check(tc.source, frontend.Options{})
			if result.OK {
				t.Fatalf("the checker accepted a call the declaration does not allow:\n%s", tc.source)
			}
			summary := result.ErrSummary()
			if !strings.Contains(summary, tc.wantErr) {
				t.Errorf("the diagnostic does not mention %q:\n%s", tc.wantErr, summary)
			}
		})
	}
}

// TestAnUnknownTypeInAnExternSignatureIsAnError.
//
// `types.Parse` succeeds for any name, so a typo would otherwise arrive as an
// empty struct type: every parameter would accept anything and the return would be
// an empty struct. That is a declaration which type-checks and means nothing,
// which is worse than one that does not compile.
func TestAnUnknownTypeInAnExternSignatureIsAnError(t *testing.T) {
	result := frontend.Check(`
		extern "C" fn broken(x: NoSuchType) -> Int;
		broken(1);
	`, frontend.Options{})

	if result.OK {
		t.Fatal("the checker accepted an extern declaration whose parameter type does not exist")
	}
	if !strings.Contains(result.ErrSummary(), "NoSuchType") {
		t.Errorf("the diagnostic does not name the unknown type:\n%s", result.ErrSummary())
	}
}

// runProgram builds a program against the given C objects and runs it.
//
// It is a helper rather than an assertion because "the program ran and printed X"
// is the *only* way to check a boundary. A module that calls the wrong symbol with
// the wrong convention is well-formed, assembles, links, and answers with a number
// that is wrong — every text-level check passes and the program is broken.
//
// The C is a parameter because the two tests here need different C: one calls into
// Nilang, the other is called back from it. One function with a fixture argument
// rather than two copies, because two copies is two things to keep in step with
// the linker.
func runProgram(t *testing.T, source string, objects ...string) string {
	t.Helper()

	tc := toolchain.DiscoverToolchain(llvm.TargetHost)
	if _, hasLinker := tc.Linker(); !hasLinker {
		t.Skipf("no linker on this machine: %s", tc.LinkAdvice())
	}

	output := filepath.Join(t.TempDir(), "program")
	if llvm.TargetHost.OS == "windows" {
		output += ".exe"
	}

	module, err := compileModule(source)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, err := aot.Build(module, llvm.TargetHost, tc, aot.Options{
		OutputPath:   output,
		ExtraObjects: objects,
		Libraries:    []string{"m"},
	}); err != nil {
		t.Fatalf("building the program failed:\n%v\n--- module ---\n%s", err, module)
	}

	result, err := exec.Command(output).CombinedOutput()
	if err != nil {
		t.Fatalf("running the program failed: %v\noutput:\n%s", err, result)
	}
	return string(result)
}

// buildFixture compiles the C fixture into an object the linker can use.
//
// The C compiler is the toolchain's own, found by capability rather than by name:
// on a machine where `clang` can read IR but cannot compile C, asking for `clang`
// by name would fail at the compile step with a message about a missing SDK, when
// the machine has a perfectly good compiler under another name.
func buildFixture(t *testing.T, tc toolchain.Toolchain) string {
	t.Helper()

	source := filepath.Join("..", "..", "runtime", "ffi", "test", "ffifixture.c")
	if _, err := os.Stat(source); err != nil {
		t.Skipf("the C fixture is not present: %v", err)
	}

	compiler, hasCompiler := tc.CCompiler()
	if !hasCompiler {
		t.Skip("no C compiler on this machine; the boundary was checked by emitted IR only")
	}

	object := filepath.Join(t.TempDir(), "ffifixture.o")
	if _, err := toolchain.RunTool(compiler, "-c", source, "-o", object, "-O0"); err != nil {
		t.Fatalf("compiling the C fixture failed: %v", err)
	}
	return object
}

func emit(t *testing.T, source string) string {
	t.Helper()

	module, err := compileModule(source)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return module
}

func lower(t *testing.T, source string) *mir.Program {
	t.Helper()

	checked := frontend.Check(source, frontend.Options{})
	if !checked.OK {
		t.Fatalf("the program was rejected: %s", checked.ErrSummary())
	}
	optimized := hir.NewOptimizer().Optimize(hir.NewLowerer().LowerProgram(checked.Lowered))
	return mir.NewLowerer().LowerHIR(optimized)
}
