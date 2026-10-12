package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	compilercap "github.com/joysriramsarkar/nilLang/compiler/capability"
	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
)

// newEvaluatorEnv builds an environment with the builtin set installed, which
// is what every evaluator based test needs.
func newEvaluatorEnv() *object.Environment {
	env := object.NewEnvironment()
	for name, builtin := range evaluator.Builtins {
		env.Set(name, builtin)
	}
	return env
}

// runWithCapabilities compiles source and runs it under the bytecode VM with
// the given capability grant, returning the run error.
//
// The VM's CapabilityChecker is now reached by real programs: the compiler emits
// OpNativeCall for gated builtins. Before that, the checker was dead code that
// only tests ever set, and the sandbox it was supposed to provide did not exist.
func runWithCapabilities(t *testing.T, source string, granted []string) error {
	t.Helper()

	res := evalSource(t, source)
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}

	comp := compiler.New()
	if err := comp.Compile(res.Program); err != nil {
		t.Fatalf("compile: %v", err)
	}

	machine := vm.New(comp.Bytecode())
	machine.CapabilityChecker = compilercap.GrantChecker(granted)
	return machine.Run()
}

// TestGatedBuiltinCallIsRefusedWithoutGrant is the core enforcement test. A
// program that writes a file must be refused when no filesystem capability was
// granted.
func TestGatedBuiltinCallIsRefusedWithoutGrant(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "secret.txt")
	source := `writeFile("` + filepath.ToSlash(target) + `", "classified");`

	err := runWithCapabilities(t, source, nil)
	if err == nil {
		t.Fatal("writeFile must be refused when no capabilities are granted")
	}
	denied, ok := compilercap.AsDenied(err)
	if !ok {
		t.Fatalf("expected a capability denial, got %T: %v", err, err)
	}
	if denied.Operation != "writeFile" {
		t.Errorf("Operation = %q, want writeFile", denied.Operation)
	}
	if len(denied.Required) != 1 || denied.Required[0] != compilercap.Filesystem {
		t.Errorf("Required = %v, want [%s]", denied.Required, compilercap.Filesystem)
	}

	if _, statErr := os.Stat(target); statErr == nil {
		t.Error("the refused program still wrote a file")
	}
}

// TestGatedBuiltinCallIsAllowedWithGrant confirms the gate is a grant check and
// not a blanket refusal.
func TestGatedBuiltinCallIsAllowedWithGrant(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "allowed.txt")
	source := `writeFile("` + filepath.ToSlash(target) + `", "public");`

	if err := runWithCapabilities(t, source, []string{compilercap.Filesystem}); err != nil {
		t.Fatalf("writeFile should be allowed with the filesystem grant: %v", err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("the allowed program did not write the file: %v", err)
	}
	if string(content) != "public" {
		t.Errorf("file content = %q, want %q", content, "public")
	}
}

// TestGrantNameSpellingIsAcceptedAtRuntime covers the manifest vocabulary: a
// project may declare "Filesystem" or "filesystem".
func TestGrantNameSpellingIsAcceptedAtRuntime(t *testing.T) {
	for _, spelling := range []string{"filesystem", "Filesystem", "FILE_SYSTEM"} {
		dir := t.TempDir()
		target := filepath.Join(dir, "spelling.txt")
		source := `writeFile("` + filepath.ToSlash(target) + `", "x");`

		if err := runWithCapabilities(t, source, []string{spelling}); err != nil {
			t.Errorf("grant %q should authorise writeFile: %v", spelling, err)
		}
	}
}

// TestExecIsGated covers a second, more dangerous operation. Spawning a child
// process must require an explicit process grant.
func TestExecIsGated(t *testing.T) {
	err := runWithCapabilities(t, `exec("echo hello");`, nil)
	if err == nil {
		t.Fatal("exec must be refused without a process grant")
	}
	denied, ok := compilercap.AsDenied(err)
	if !ok {
		t.Fatalf("expected a capability denial, got %T: %v", err, err)
	}
	if len(denied.Required) != 1 || denied.Required[0] != compilercap.Process {
		t.Errorf("Required = %v, want [%s]", denied.Required, compilercap.Process)
	}
}

// TestRemoveFileIsGated covers a destructive filesystem operation.
func TestRemoveFileIsGated(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "doomed.txt")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	source := `removeFile("` + filepath.ToSlash(target) + `");`

	if err := runWithCapabilities(t, source, nil); err == nil {
		t.Error("removeFile must be refused without a filesystem grant")
	}
	if _, statErr := os.Stat(target); statErr != nil {
		t.Error("the refused program still deleted the file")
	}

	if err := runWithCapabilities(t, source, []string{compilercap.Filesystem}); err != nil {
		t.Errorf("removeFile should be allowed with the filesystem grant: %v", err)
	}
	if _, statErr := os.Stat(target); statErr == nil {
		t.Error("the allowed program did not delete the file")
	}
}

// TestPrintIsGated confirms that even the most ordinary operation goes through
// the gate. print is the operation most likely to be waved through as harmless,
// so it is the one most worth asserting on.
func TestPrintIsGated(t *testing.T) {
	if err := runWithCapabilities(t, `println("hi");`, nil); err == nil {
		t.Error("println must be refused without a stdout grant")
	}
	if err := runWithCapabilities(t, `println("hi");`, []string{compilercap.Stdout}); err != nil {
		t.Errorf("println should be allowed with the stdout grant: %v", err)
	}
}

// TestPureOperationsAreNotGated confirms the gate does not refuse ordinary
// computation, which would make the sandbox useless.
func TestPureOperationsAreNotGated(t *testing.T) {
	source := `
let xs = [3, 1, 2];
let total = 0;
let i = 0;
while (i < len(xs)) {
  total = total + xs[i];
  i = i + 1;
}
let h = {"total": total, "count": len(xs)};
assert(h["total"] == 6, "total should be 6");
assert(h["count"] == 3, "count should be 3");
`
	if err := runWithCapabilities(t, source, nil); err != nil {
		t.Errorf("pure computation must run with no capabilities: %v", err)
	}
}

// TestCompilerEmitsNativeCallForGatedBuiltins inspects the bytecode directly, so
// that the test fails if the instrumentation is reverted even though the runtime
// gate would then silently never fire.
func TestCompilerEmitsNativeCallForGatedBuiltins(t *testing.T) {
	cases := []struct {
		source   string
		wantName string
	}{
		{`writeFile("a.txt", "b");`, "writeFile"},
		{`exec("ls");`, "exec"},
		{`println("x");`, "println"},
		{`removeFile("a.txt");`, "removeFile"},
		{`let x = time();`, "time"},
	}

	for _, tc := range cases {
		res := evalSource(t, tc.source)
		if !res.OK {
			t.Fatalf("%q rejected: %s", tc.source, res.ErrSummary())
		}
		comp := compiler.New()
		if err := comp.Compile(res.Program); err != nil {
			t.Fatalf("%q compile: %v", tc.source, err)
		}

		// DisassembleWithConstants resolves the constant pool, so the listing
		// shows the operation name the call site carries.
		disasm := vm.DisassembleWithConstants(comp.Bytecode().Instructions, comp.Bytecode().Constants)
		if !strings.Contains(disasm, "OpNativeCall") {
			t.Errorf("%q did not emit OpNativeCall:\n%s", tc.source, disasm)
			continue
		}
		// The constant pool resolves the operation name; a disassembly that
		// showed a bare index would not prove which call it was.
		if !strings.Contains(disasm, "("+tc.wantName+")") {
			t.Errorf("%q emitted OpNativeCall without naming %q:\n%s", tc.source, tc.wantName, disasm)
		}
	}
}

// TestPureBuiltinsStayOrdinaryCalls confirms the instrumentation is targeted:
// pure operations keep the cheaper OpCall form and are not gated.
func TestPureBuiltinsStayOrdinaryCalls(t *testing.T) {
	for _, source := range []string{`len([1,2,3]);`, `upper("abc");`, `first([1,2]);`} {
		res := evalSource(t, source)
		if !res.OK {
			t.Fatalf("%q rejected: %s", source, res.ErrSummary())
		}
		comp := compiler.New()
		if err := comp.Compile(res.Program); err != nil {
			t.Fatalf("%q compile: %v", source, err)
		}
		if disasm := vm.Disassemble(comp.Bytecode().Instructions); strings.Contains(disasm, "OpNativeCall") {
			t.Errorf("%q emitted OpNativeCall but should be a pure OpCall:\n%s", source, disasm)
		}
	}
}

// TestGatingIsPerOperationNotPerProgram confirms that granting one capability
// does not grant another.
func TestGatingIsPerOperationNotPerProgram(t *testing.T) {
	dir := t.TempDir()
	source := `writeFile("` + filepath.ToSlash(filepath.Join(dir, "a.txt")) + `", "x"); exec("echo hi");`

	err := runWithCapabilities(t, source, []string{compilercap.Filesystem})
	if err == nil {
		t.Fatal("a filesystem grant must not authorise exec")
	}
	denied, ok := compilercap.AsDenied(err)
	if !ok {
		t.Fatalf("expected a capability denial for exec, got %T: %v", err, err)
	}
	if denied.Operation != "exec" {
		t.Errorf("Operation = %q, want exec", denied.Operation)
	}
}

// TestEvaluatorIsGatedWhenACheckerIsInstalled covers the L6-closed gap: the
// tree-walking evaluator enforces the capability manifest at the call site
// once a checker is installed (exactly as `nil run` installs it), failing
// with the same CategoryCapabilityDenied the VM reports. With no checker
// the historical behaviour stays: every builtin runs, which is what the
// conformance suite relies on.
func TestEvaluatorIsGatedWhenACheckerIsInstalled(t *testing.T) {
	sourceFor := func(dir string) string {
		return `writeFile("` + filepath.ToSlash(filepath.Join(dir, "evaluator.txt")) + `", "x");`
	}

	// Without a checker the evaluator runs everything, as it always has.
	dir := t.TempDir()
	res := evalSource(t, sourceFor(dir))
	if !res.OK {
		t.Fatalf("source rejected: %s", res.ErrSummary())
	}
	evaluated := evaluator.Eval(res.Program, newEvaluatorEnv())
	if errObj, ok := evaluated.(*object.Error); ok {
		t.Fatalf("evaluator error: %s", errObj.Inspect())
	}
	if _, err := os.Stat(filepath.Join(dir, "evaluator.txt")); err != nil {
		t.Error("the unchecked evaluator path is expected to write without a grant")
	}

	// With the manifest's checker installed the same program is refused with
	// the same category the VM reports.
	previous := evaluator.CapabilityChecker
	evaluator.CapabilityChecker = compilercap.GrantChecker(nil)
	defer func() { evaluator.CapabilityChecker = previous }()

	dir2 := t.TempDir()
	res2 := evalSource(t, sourceFor(dir2))
	if !res2.OK {
		t.Fatalf("source rejected: %s", res2.ErrSummary())
	}
	denied := evaluator.Eval(res2.Program, newEvaluatorEnv())
	category, ok := object.CategoryOf(denied)
	if !ok || category != object.CategoryCapabilityDenied {
		t.Fatalf("evaluator category = %q, want CapabilityDenied (got %v)", category, denied)
	}
	if _, statErr := os.Stat(filepath.Join(dir2, "evaluator.txt")); statErr == nil {
		t.Error("the refused program still wrote a file")
	}
}
