package ffi

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/llvm"
	"github.com/joysriramsarkar/nilLang/compiler/nilcore"
	"github.com/joysriramsarkar/nilLang/compiler/toolchain"
)

// L5.9: whether the boundary is stable enough to build things on.
//
// Everything above this asks "does it work". This asks "will it still work when
// something else changes" — and for most of this the honest answer was no. A
// function added to the boundary, a struct whose size a program had guessed, a
// runtime and an object built from different versions of the same header. Each of
// those produced a wrong answer with no diagnostic anywhere.

// layoutQueries are the runtime's types the boundary promises to measure.
//
// Written as a list because both halves of every comparison below are generated
// from it: the probe each compiler compiles, and the order the driver prints them
// in. Two hand-written lists would agree today and disagree the first time a type
// was added — which is exactly the change these tests exist to catch.
var layoutQueries = []string{"value", "object", "string", "closure", "callback", "crossing"}

// TestBothCompilersAgreeOnTheRuntimesLayout is the claim, verified rather than
// assumed.
//
// ADR-0072 established that the two compilers in a link *disagree* about how a small
// struct is returned. That made every size in this FFI suspect, and the answer to
// `sizeof` was only ever as good as a belief.
//
// So this asks both, in one link, and compares. Not their assembly and not their
// objects — their *answers*, from the same header, at the same moment. A
// disagreement is not a style problem: it means a C author who wrote
// `sizeof(NilString)` got a different number from the runtime they linked, and the
// buffer they allocated is the wrong size.
//
// The two probes are compiled by different compilers and linked into one program.
// That is deliberate — comparing two text files compares two compilers' opinions;
// calling both through the same link is what a real program would see.
func TestBothCompilersAgreeOnTheRuntimesLayout(t *testing.T) {
	tc := toolchain.DiscoverToolchain(llvm.TargetHost)
	cc, hasC := tc.CCompiler()
	if !hasC {
		t.Skip("no C compiler on this machine; the layout was not compared")
	}

	dir := t.TempDir()
	includes := []string{
		"-I", filepath.Join("..", "..", "runtime", "core", "include"),
		"-I", filepath.Join("..", "..", "runtime", "ffi", "include"),
	}

	probes := []struct {
		prefix   string
		compiler toolchain.Tool
	}{
		{"runtime", cc},
	}
	if tc.Clang.Available() {
		// Clang reads this project's headers even where it cannot link — it needs
		// an SDK for the C library's headers, not for a header that declares its
		// own types. Assembling the module is clang's job in every build, so its
		// opinion about a layout is the one that matters most.
		probes = append(probes, struct {
			prefix   string
			compiler toolchain.Tool
		}{"clang", tc.Clang})
	}

	objects := make([]string, 0, len(probes)+1)
	for _, probe := range probes {
		source := filepath.Join(dir, probe.prefix+".c")
		if err := os.WriteFile(source, []byte(layoutProbe(probe.prefix)), 0o644); err != nil {
			t.Fatal(err)
		}
		object := filepath.Join(dir, probe.prefix+".o")
		args := append([]string{"-c", source, "-o", object, "-O0", "-std=c11"}, includes...)
		if out, err := toolchain.RunTool(probe.compiler, args...); err != nil {
			t.Fatalf("the %s probe did not compile:\n%s", probe.prefix, strings.TrimSpace(out))
		}
		objects = append(objects, object)
	}

	driver := filepath.Join(dir, "driver.c")
	if err := os.WriteFile(driver, []byte(layoutDriver(probes)), 0o644); err != nil {
		t.Fatal(err)
	}
	driverObject := filepath.Join(dir, "driver.o")
	args := append([]string{"-c", driver, "-o", driverObject, "-O0", "-std=c11"}, includes...)
	if out, err := toolchain.RunTool(cc, args...); err != nil {
		t.Fatalf("the driver did not compile:\n%s", strings.TrimSpace(out))
	}
	objects = append(objects, driverObject)

	// Linked with the C compiler rather than clang, because on a machine where
	// clang cannot find a C runtime — a normal state for an LLVM-only install —
	// this is the tool that can finish the job. That is also the realistic shape:
	// one module assembled by clang, everything else by the C compiler.
	//
	// The runtime's own objects come from `nilcore.Build` rather than being
	// listed here, so this links against exactly the library a real program gets —
	// and so the probe cannot pass against a stale or partial one.
	library, err := nilcore.Build(filepath.Join(dir, "nilcore"), tc, nil)
	if err != nil {
		t.Fatalf("building the runtime failed: %v", err)
	}

	executable := filepath.Join(dir, "layout")
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	linkArgs := append(append([]string{}, objects...), library.Objects...)
	linkArgs = append(linkArgs, "-o", executable)
	if out, err := toolchain.RunTool(cc, linkArgs...); err != nil {
		t.Fatalf("the probes did not link:\n%s", strings.TrimSpace(out))
	}

	result, err := exec.Command(executable).CombinedOutput()
	if err != nil {
		t.Fatalf("running the layout comparison failed: %v\n%s", err, result)
	}

	lines := strings.Split(strings.TrimSpace(string(result)), "\n")
	if len(lines) < 2 {
		t.Skipf("only one compiler answered the layout question:\n%s", result)
	}
	runtimeLine := strings.TrimSpace(lines[0])
	clangLine := strings.TrimSpace(lines[1])

	// Compared as text rather than parsed into numbers: what matters is that the
	// two produced *the same numbers*, and a difference shows up as a different
	// line whatever the numbers are. A parser here would only give a worse message
	// about the same fact.
	if runtimeLine != clangLine {
		t.Errorf("the runtime and clang disagree about the runtime's own layout:\n"+
			"  runtime: %s\n  clang:   %s", runtimeLine, clangLine)
	}

	// And the numbers are the ones a reader would recognise, so a change to the
	// runtime's structs shows up here rather than as a crash on somebody's
	// machine. A checked constant that silently changes is worse than none.
	if want := "8 8 24 8 32 8 56 8 8 8 16 8"; runtimeLine != want {
		t.Errorf("the layout changed: got %s, want %s.\n"+
			"If the change is intended, the FFI's ABI version must go up — a program "+
			"that measured this once has the old numbers and nothing tells it.",
			runtimeLine, want)
	}
}

// layoutProbe asks one compiler what the runtime's types measure.
//
// `nil_ffi_sizeof` and not `sizeof`, because the runtime's own answer is what a
// program is *given*, and this compares it against the other compiler computing the
// same thing from the same header. Asking the runtime to check itself proves
// nothing: it would agree with itself by construction.
func layoutProbe(prefix string) string {
	var b strings.Builder
	b.WriteString("#include \"nil_internal.h\"\n#include \"nilffi.h\"\n\n")
	for _, name := range layoutQueries {
		selector := "NIL_FFI_LAYOUT_" + strings.ToUpper(name)
		fmt.Fprintf(&b, "int %s_sizeof_%s(void) { return (int)nil_ffi_sizeof(%s); }\n",
			prefix, name, selector)
		fmt.Fprintf(&b, "int %s_alignof_%s(void) { return (int)nil_ffi_alignof(%s); }\n",
			prefix, name, selector)
	}
	return b.String()
}

// layoutDriver prints each compiler's answers in one fixed order.
func layoutDriver(probes []struct {
	prefix   string
	compiler toolchain.Tool
}) string {
	var b strings.Builder
	b.WriteString("#include <stdio.h>\n")
	for _, probe := range probes {
		for _, name := range layoutQueries {
			fmt.Fprintf(&b, "extern int %s_sizeof_%s(void);\n", probe.prefix, name)
			fmt.Fprintf(&b, "extern int %s_alignof_%s(void);\n", probe.prefix, name)
		}
	}
	b.WriteString("int main(void) {\n")
	for _, probe := range probes {
		for _, name := range layoutQueries {
			fmt.Fprintf(&b, "  printf(\"%%d \", %s_sizeof_%s());\n", probe.prefix, name)
			fmt.Fprintf(&b, "  printf(\"%%d \", %s_alignof_%s());\n", probe.prefix, name)
		}
		b.WriteString("  printf(\"\\n\");\n")
	}
	b.WriteString("  return 0;\n}\n")
	return b.String()
}

// TestTheVersionInTheGuardIsTheVersionInTheHeader is the one thing two languages
// cannot check between them.
//
// `NIL_FFI_ABI_VERSION` is a C constant and the guard's name is a Go constant.
// Nothing in the toolchain compares them, and a mismatch is invisible until a
// program built against one header links against a runtime built from another —
// which is the exact failure the guard exists to catch, arriving through the guard
// itself.
//
// Read from the *emitted module* rather than from the Go source, so what is checked
// is what a program will actually be linked against.
func TestTheVersionInTheGuardIsTheVersionInTheHeader(t *testing.T) {
	module := emit(t, `
		extern "C" fn answer() -> Int;
		answer();
	`)

	guard := ""
	for _, line := range strings.Split(module, "\n") {
		if strings.HasPrefix(line, "declare void @nil_ffi_abi_v") {
			guard = strings.TrimSuffix(strings.TrimPrefix(line, "declare void @"), "()")
		}
	}
	if guard == "" {
		t.Fatalf("a module declaring an extern does not link an ABI guard:\n%s", module)
	}

	version := strings.TrimPrefix(guard, "nil_ffi_abi_v")
	if version == "" {
		t.Errorf("the guard %q carries no version; without one every version would be "+
			"the same symbol and the linker would have nothing to check", guard)
	}

	header, err := os.ReadFile(filepath.Join("..", "..", "runtime", "ffi", "include", "nilffi.h"))
	if err != nil {
		t.Fatal(err)
	}
	want := "#define NIL_FFI_ABI_VERSION " + version
	if !strings.Contains(string(header), want) {
		t.Errorf("the emitted guard says v%s but nilffi.h does not agree:\n%s",
			version, versionLines(string(header)))
	}
}

// versionLines shows what nilffi.h actually says, so the message carries the answer.
func versionLines(header string) string {
	var out []string
	for _, line := range strings.Split(header, "\n") {
		if strings.Contains(line, "NIL_FFI_ABI_VERSION") && strings.HasPrefix(strings.TrimSpace(line), "#define") {
			out = append(out, strings.TrimSpace(line))
		}
	}
	if len(out) == 0 {
		return "  (nilffi.h defines no version at all)"
	}
	return "  " + strings.Join(out, "\n  ")
}

// TestAModuleWithAnExternIsGuardedAndOneWithoutIsNot keeps the guard honest.
//
// A guard in every module would be a guard that guards nothing: a program with no
// `extern` has no runtime version to disagree with, and a module linked against a
// symbol it will never call can have that symbol satisfied by anything that happens
// to export it — including a library that is not this runtime.
func TestAModuleWithAnExternIsGuardedAndOneWithoutIsNot(t *testing.T) {
	guarded := emit(t, `
		extern "C" fn answer() -> Int;
		answer();
	`)
	if !strings.Contains(guarded, "call void @nil_ffi_abi_v") {
		t.Errorf("a module with an extern is not guarded:\n%s", guarded)
	}
	if !strings.Contains(guarded, "declare void @nil_ffi_abi_v") {
		t.Errorf("the guard is called but never declared:\n%s", guarded)
	}

	plain := emit(t, `let a = 1; a + 1;`)
	if strings.Contains(plain, "nil_ffi_abi_v") {
		t.Errorf("a module with no extern links an ABI guard it can never need:\n%s", plain)
	}
}

// TestTheGuardIsCalledBeforeAnythingElseCanFail is the ordering.
//
// A guard that runs after the first foreign call is a guard for everything after the
// first foreign call, and a mismatch there is a program that has already read a
// value with the wrong convention.
func TestTheGuardIsCalledBeforeAnythingElseCanFail(t *testing.T) {
	module := emit(t, `
		extern "C" fn answer() -> Int;
		answer();
	`)

	main := module[strings.Index(module, "define i32 @main()"):]
	guard := strings.Index(main, "call void @nil_ffi_abi_v")
	firstCall := strings.Index(main, "call i64 @")
	if guard < 0 || firstCall < 0 {
		t.Fatalf("the module does not have both the guard and a call:\n%s", main)
	}
	if guard > firstCall {
		t.Errorf("the guard is called after the program's first call:\n%s", main)
	}
}

// TestAnUnknownLayoutQuestionIsRefusedRatherThanGuessed covers the default.
//
// `nil_ffi_sizeof` takes an enum, and an enum in C is whatever integer somebody
// passes. A `default:` returning 0 would be a plausible-looking number for a program
// about to allocate with; -1 is a value no size can take.
func TestAnUnknownLayoutQuestionIsRefusedRatherThanGuessed(t *testing.T) {
	tc := toolchain.DiscoverToolchain(llvm.TargetHost)
	cc, hasC := tc.CCompiler()
	if _, hasLinker := tc.Linker(); !hasC || !hasLinker {
		t.Skip("no C toolchain on this machine; the refusal was not checked")
	}

	dir := t.TempDir()
	source := filepath.Join(dir, "unknown.c")
	body := `
#include "nilffi.h"
int main(void) {
  int unknown_size = (int)nil_ffi_sizeof((NilFFILayout)99);
  int unknown_align = (int)nil_ffi_alignof((NilFFILayout)99);
  int negative = (int)nil_ffi_sizeof((NilFFILayout)-1);
  return (unknown_size == -1 && unknown_align == -1 && negative == -1) ? 0 : 1;
}
`
	if err := os.WriteFile(source, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	object := filepath.Join(dir, "unknown.o")
	args := []string{"-c", source, "-o", object, "-O0", "-std=c11",
		"-I", filepath.Join("..", "..", "runtime", "core", "include"),
		"-I", filepath.Join("..", "..", "runtime", "ffi", "include")}
	if out, err := toolchain.RunTool(cc, args...); err != nil {
		t.Fatalf("the probe did not compile:\n%s", strings.TrimSpace(out))
	}

	// Linked against the runtime the FFI's own build produces, so this is the same
	// library a program would link — not a list of sources named here, which would
	// drift from what ships the moment a file moved.
	library, err := nilcore.Build(filepath.Join(dir, "nilcore"), tc, nil)
	if err != nil {
		t.Fatalf("building the runtime failed: %v", err)
	}
	executable := filepath.Join(dir, "unknown")
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	linkArgs := append([]string{object}, library.Objects...)
	linkArgs = append(linkArgs, "-o", executable)
	if out, err := toolchain.RunTool(cc, linkArgs...); err != nil {
		t.Fatalf("the probe did not link:\n%s", strings.TrimSpace(out))
	}
	if err := exec.Command(executable).Run(); err != nil {
		t.Errorf("an unknown layout question was answered with something other than -1; "+
			"a program that allocated with the answer would have asked for %v", err)
	}
}

// TestTheRuntimeReportsItsOwnVersion says the loaded runtime answers what it is.
//
// The link-time guard catches most mismatches at build time. This covers the ones
// that slip past — a library exporting both versions, or a symbol found somewhere
// unexpected — by asking the runtime that actually got loaded.
//
// # Why the expectation is read out of the header rather than written down
//
// Because it was written down, as `1`, and raising `NIL_FFI_ABI_VERSION` for the
// channel work made it wrong in a way that looked like a runtime bug: the runtime
// said `2` and the test said "wanted 1". A test whose expected value has to be
// edited by hand when the thing it watches changes is one that will be edited
// without reading. The header is the single answer, so it is the single source here
// too — and the match is on the whole line rather than on a substring, because
// `Contains(got, "1")` passes for a runtime that printed `21`.
//
// A header that cannot be read is a skip rather than a guess: the test is about the
// runtime agreeing with the header, and with no header there is nothing to compare.
func TestTheRuntimeReportsItsOwnVersion(t *testing.T) {
	wanted := abiVersionFromHeader(t)

	source := `
		extern "C" fn nil_ffi_version() -> Int;
		nil_ffi_version();
	`
	got := strings.TrimSpace(runProgram(t, source, buildFixture(t, toolchain.DiscoverToolchain(llvm.TargetHost))))
	if got != wanted {
		t.Errorf("the runtime reports a version of %q, and nilffi.h says %s", got, wanted)
	}
}

// abiVersionFromHeader reads `NIL_FFI_ABI_VERSION` out of `nilffi.h`.
//
// Parsed with a line scan rather than a regexp over the whole file because the header
// *discusses* the constant several times in prose — a regexp over the text would find
// the first mention, which is a sentence rather than a definition, and this test would
// then compare the runtime against a comment.
func abiVersionFromHeader(t *testing.T) string {
	t.Helper()

	header, err := os.ReadFile(filepath.Join("..", "..", "runtime", "ffi", "include", "nilffi.h"))
	if err != nil {
		t.Skipf("the FFI header is not readable: %v", err)
	}

	for _, line := range strings.Split(string(header), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#define NIL_FFI_ABI_VERSION") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 3 {
			return fields[2]
		}
		t.Fatalf("cannot read a version out of %q", trimmed)
	}
	t.Skip("nilffi.h does not define NIL_FFI_ABI_VERSION")
	return ""
}
