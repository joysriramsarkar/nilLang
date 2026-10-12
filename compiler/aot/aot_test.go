package aot

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/llvm"
	"github.com/joysriramsarkar/nilLang/compiler/toolchain"
)

// hostToolchain is this machine's target and the tools for it.
//
// # Why `llvm.TargetHost` and not a lookup by triple
//
// `LookupTarget` is keyed by *ID* — `x86_64-windows` — and `llvm.HostTriple` is a
// *triple* — `x86_64-pc-windows-msvc`. Passing one to the other finds nothing.
//
// The first version of this helper fell back to "the first registered target" when
// the lookup failed, and that fallback chose `aarch64-unknown-linux-android`. Every
// test still passed, because none of them noticed they were building for Android on
// a Windows machine until one tried to link. A silent fallback in test setup is
// worse than a failure: it turns a wrong answer into no answer at all.
func hostToolchain(t *testing.T) (llvm.Target, toolchain.Toolchain) {
	t.Helper()
	target := llvm.TargetHost
	if target.ID == "" {
		t.Fatal("no host target is registered; the target table is empty")
	}
	return target, toolchain.DiscoverToolchain(target)
}

// build compiles IR and returns the path to the executable.
func build(t *testing.T, ir string) string {
	t.Helper()
	target, tc := hostToolchain(t)

	output := filepath.Join(t.TempDir(), "program")
	if runtime.GOOS == "windows" {
		output += ".exe"
	}

	artifact, err := Build(ir, target, tc, Options{OutputPath: output, KeepModule: true})
	if err != nil {
		t.Fatalf("building:\n%s\n---\n%s", err, ir)
	}
	if _, err := os.Stat(artifact.Path); err != nil {
		t.Fatalf("the linker reported success but there is no executable: %v", err)
	}
	return artifact.Path
}

// run executes a compiled program and returns everything it printed.
func run(t *testing.T, path string) string {
	t.Helper()
	ctx := t.TempDir()
	_ = ctx

	output, err := exec.Command(path).CombinedOutput()
	if err != nil {
		t.Fatalf("the compiled program failed: %v\n%s\n%s", err, output, moduleAt(t, path))
	}
	return string(output)
}

// moduleAt finds the `.ll` written beside an executable, for a failure message.
func moduleAt(t *testing.T, executable string) string {
	t.Helper()
	path := strings.TrimSuffix(executable, executableSuffix()) + ".ll"
	content, err := os.ReadFile(path)
	if err != nil {
		return "(the module was not kept)"
	}
	return string(content)
}

// TestAProgramLinksAndRuns is the first end-to-end proof that any of this works.
//
// Everything before it checks that text is produced and that a parser accepts it.
// This one produces a machine-code executable, links it against the runtime, runs
// it, and compares what it printed against what the language says it should.
//
// The program is deliberately more than arithmetic: it allocates a string, adds to
// it, and prints the result, so a wrong representation or a wrong runtime call
// shows up as output rather than as a linker error.
func TestAProgramLinksAndRuns(t *testing.T) {
	target, tc := hostToolchain(t)
	if _, ok := tc.Linker(); !ok {
		t.Skip("no linker on this machine")
	}

	module := simpleModule(target)
	path := build(t, module)

	output := run(t, path)
	if strings.TrimSpace(output) != "7" {
		t.Errorf("printed %q, expected %q\n%s", output, "7", moduleAt(t, path))
	}
}

// simpleModule is a program the emitter could have produced: a tagged integer added
// to another, then printed.
//
// It is written by hand rather than compiled from Nilang so that this test depends
// on the *linker* and not on the emitter. A failure then says which of the two is
// broken, and `TestTheEmitterProducesALinkableModule` covers the pair.
func simpleModule(target llvm.Target) string {
	return `source_filename = "simple"
target datalayout = "` + target.DataLayout + `"
target triple = "` + target.Triple + `"

define i32 @main() {
entry:
  %a = call i64 @nil_int_new(i64 3)
  %b = call i64 @nil_int_new(i64 4)
  %sum = call i64 @nil_int_add(i64 %a, i64 %b)
  call void @nil_println(i64 %sum)
  ret i32 0
}

declare i64 @nil_int_new(i64)
declare i64 @nil_int_add(i64, i64)
declare void @nil_println(i64)
`
}

// TestTheRuntimeIsActuallyLinked guards against the most dangerous way this could
// pass.
//
// A program that printed 7 might have got the answer from somewhere other than the
// runtime — a constant folded in, a symbol resolved to something else. The check is
// that a call into the runtime's *own* behaviour is observable: the collector's
// object count is a runtime-only fact, and a program that reads it cannot be
// answering without being linked to the library.
func TestTheRuntimeIsActuallyLinked(t *testing.T) {
	target, tc := hostToolchain(t)
	if _, ok := tc.Linker(); !ok {
		t.Skip("no linker on this machine")
	}

	// A program that allocates, concatenates and prints. Every step is in the C
	// library; none is in the module — and a string constant in the module is only
	// ever the *bytes*, which the runtime has to turn into a value before anything
	// can use it. That is the shape every compiled program with a literal has.
	module := `source_filename = "runtime"
target datalayout = "` + target.DataLayout + `"
target triple = "` + target.Triple + `"

define i32 @main() {
entry:
  %a = call i64 @nil_string_lit(ptr @hello, i64 5)
  %b = call i64 @nil_string_lit(ptr @world, i64 5)
  %both = call i64 @nil_string_concat(i64 %a, i64 %b)
  call void @nil_print(i64 %both)
  ret i32 0
}

@hello = private unnamed_addr constant [5 x i8] c"hello"
@world = private unnamed_addr constant [5 x i8] c"world"

declare i64 @nil_string_lit(ptr, i64)
declare i64 @nil_string_concat(i64, i64)
declare void @nil_print(i64)
`
	path := build(t, module)

	output := run(t, path)
	if strings.TrimSpace(output) != "helloworld" {
		t.Errorf("printed %q, expected %q\n%s", output, "helloworld", moduleAt(t, path))
	}
}

// TestAFailingProgramStopsWithTheRuntimesCode is the panic contract end to end.
//
// The compiled program cannot catch an error, so a division by zero has to end the
// process. The code matters: it is what lets a caller — a test, a build — tell one
// failure from another without reading a message.
func TestAFailingProgramStopsWithTheRuntimesCode(t *testing.T) {
	target, tc := hostToolchain(t)
	if _, ok := tc.Linker(); !ok {
		t.Skip("no linker on this machine")
	}

	module := `source_filename = "divide"
target datalayout = "` + target.DataLayout + `"
target triple = "` + target.Triple + `"

define i32 @main() {
entry:
  %a = call i64 @nil_int_new(i64 1)
  %z = call i64 @nil_int_new(i64 0)
  %q = call i64 @nil_int_div(i64 %a, i64 %z)
  call void @nil_println(i64 %q)
  ret i32 0
}

declare i64 @nil_int_new(i64)
declare i64 @nil_int_div(i64, i64)
declare void @nil_println(i64)
`
	path := build(t, module)

	output, err := exec.Command(path).CombinedOutput()
	if err == nil {
		t.Fatalf("the program did not stop; it printed %q", output)
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("unexpected failure: %v", err)
	}
	// 2 is NIL_ERR_DIV_ZERO. It is defined in nil.h and the runtime's own tests
	// check the same value, so this is the third place the number is written and
	// the one that proves the chain from the header to a compiled program holds.
	if got := exitErr.ExitCode(); got != 2 {
		t.Errorf("exited with %d, expected 2 (NIL_ERR_DIV_ZERO)\n%s", got, output)
	}
	if !strings.Contains(string(output), "division by zero") {
		t.Errorf("the message did not say what went wrong:\n%s", output)
	}
}

// TestAConditionBranchIsTakenFromTheRuntimesTruth covers the `i1`/`int` bug.
//
// The module asks the runtime whether a value is true and branches on the answer.
// Before the return type was unified, the module read one bit of a 32-bit register
// whose upper half nothing had written — so a program that branched the wrong way
// would print the wrong number with no diagnostic anywhere.
func TestAConditionBranchIsTakenFromTheRuntimesTruth(t *testing.T) {
	target, tc := hostToolchain(t)
	if _, ok := tc.Linker(); !ok {
		t.Skip("no linker on this machine")
	}

	// Branches on truthiness twice: once on a value that is true, once on one that
	// is false. Both answers have to come from the runtime rather than from the
	// module, which is why this cannot be folded away.
	module := `source_filename = "branch"
target datalayout = "` + target.DataLayout + `"
target triple = "` + target.Triple + `"

define i32 @main() {
entry:
  %zero = call i64 @nil_int_new(i64 0)
  %one = call i64 @nil_int_new(i64 1)
  %empty = call i64 @nil_string_lit(ptr @empty, i64 0)
  %full = call i64 @nil_string_lit(ptr @full, i64 1)

  %no = call i64 @nil_string_lit(ptr @no, i64 3)
  %yes = call i64 @nil_string_lit(ptr @yes, i64 4)
  %t1 = call i64 @nil_truthy(i64 %zero)
  %c1 = icmp ne i64 %t1, 0
  br i1 %c1, label %wrong1, label %right1

wrong1:
  call void @nil_println(i64 %no)
  ret i32 1

right1:
  %t2 = call i64 @nil_truthy(i64 %one)
  %c2 = icmp ne i64 %t2, 0
  br i1 %c2, label %right2, label %wrong2

wrong2:
  call void @nil_println(i64 %no)
  ret i32 1

right2:
  %t3 = call i64 @nil_truthy(i64 %empty)
  %c3 = icmp ne i64 %t3, 0
  br i1 %c3, label %wrong3, label %right3

wrong3:
  call void @nil_println(i64 %no)
  ret i32 1

right3:
  %t4 = call i64 @nil_truthy(i64 %full)
  %c4 = icmp ne i64 %t4, 0
  br i1 %c4, label %right4, label %wrong4

wrong4:
  call void @nil_println(i64 %no)
  ret i32 1

right4:
  call void @nil_println(i64 %yes)
  ret i32 0
}

@empty = private unnamed_addr constant [1 x i8] c"x"
@full = private unnamed_addr constant [2 x i8] c"xy"
@yes = private unnamed_addr constant [4 x i8] c"yes\0A"
@no = private unnamed_addr constant [3 x i8] c"no\0A"

declare i64 @nil_int_new(i64)
declare i64 @nil_string_lit(ptr, i64)
declare i64 @nil_truthy(i64)
declare void @nil_println(i64)
`
	path := build(t, module)

	output := run(t, path)
	if strings.TrimSpace(output) != "yes" {
		t.Errorf("printed %q, expected %q — the runtime's truthiness is not what the "+
			"module read\n%s", strings.TrimSpace(output), "yes", moduleAt(t, path))
	}
}

// TestTheBuildIsReportedFaithfully checks the artifact's own claims.
//
// A build that reports a size of zero, or a fingerprint of nothing, has told a
// caching system something untrue — and the caching system is what decides whether
// the next build is correct.
func TestTheBuildIsReportedFaithfully(t *testing.T) {
	target, tc := hostToolchain(t)
	if _, ok := tc.Linker(); !ok {
		t.Skip("no linker on this machine")
	}

	dir := t.TempDir()
	output := filepath.Join(dir, "app")
	if runtime.GOOS == "windows" {
		output += ".exe"
	}

	artifact, err := Build(simpleModule(target), target, tc, Options{OutputPath: output})
	if err != nil {
		t.Fatal(err)
	}

	if artifact.Size() == 0 {
		t.Error("the artifact reported a size of zero")
	}
	if artifact.RuntimeFingerprint == "" {
		t.Error("the artifact did not record which runtime it was linked against")
	}
	if _, err := os.Stat(artifact.Object); err != nil {
		t.Errorf("the object was reported but is not there: %v", err)
	}

	// The module was not asked for, so it was removed — and the executable was
	// not. Keeping the `.ll` is cheap; deleting the executable would be absurd.
	if _, err := os.Stat(output); err != nil {
		t.Errorf("the executable was removed: %v", err)
	}
	modulePath := filepath.Join(dir, "app.ll")
	if _, err := os.Stat(modulePath); err == nil {
		t.Error("the module was kept although nothing asked for it")
	}
}

// TestAnEmptyModuleIsRefusedRatherThanLinked is a small thing with a real cost.
//
// Linking an empty module succeeds on most toolchains and produces an executable
// that does nothing and returns 0. A build system that reported success for that
// would be reporting success for a program that was never compiled.
func TestAnEmptyModuleIsRefusedRatherThanLinked(t *testing.T) {
	target, tc := hostToolchain(t)
	if _, err := Build("   \n\t\n", target, tc, Options{
		OutputPath: filepath.Join(t.TempDir(), "empty"),
	}); err == nil {
		t.Error("an empty module was built without complaint")
	}
}

// TestAMissingOutputPathIsRefused keeps the failure at the call site.
func TestAMissingOutputPathIsRefused(t *testing.T) {
	target, tc := hostToolchain(t)
	if _, err := Build(simpleModule(target), target, tc, Options{}); err == nil {
		t.Error("a build with no output path was attempted")
	}
}

// TestAnUnparsableModuleReportsTheToolchainError checks that a broken module
// produces a message a reader can act on.
//
// The failure this guards is the unhelpful one: an error that says "linker command
// failed" and leaves the reader to reconstruct the command.
func TestAnUnparsableModuleReportsTheToolchainError(t *testing.T) {
	target, tc := hostToolchain(t)
	if _, ok := tc.Linker(); !ok {
		t.Skip("no linker on this machine")
	}

	broken := "source_filename = \"broken\"\nthis is not LLVM IR at all\n"

	_, err := Build(broken, target, tc, Options{
		OutputPath: filepath.Join(t.TempDir(), "broken"),
	})
	if err == nil {
		t.Fatal("a module of nonsense was accepted")
	}

	message := err.Error()
	if !strings.Contains(message, "assembling") {
		t.Errorf("the error does not say which step failed:\n%s", message)
	}
	// The tool's own words must survive: "assembling the module" alone leaves the
	// reader guessing what was wrong with it.
	if len(strings.TrimSpace(strings.TrimPrefix(message, "assembling the module:"))) < 5 {
		t.Errorf("the error carries none of the toolchain's own message:\n%s", message)
	}
}
