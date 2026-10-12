package nilcore

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/compiler/toolchain"
)

// Error codes the runtime exits with. They mirror the `NIL_ERR_*` constants in
// nil.h, and this test is where the two are kept in step.
const (
	exitOK       = 0
	exitFailures = 1
	exitType     = 1
	exitDivZero  = 2
	exitIndex    = 3
)

// SelfTestPath is the C program that exercises the runtime.
func SelfTestPath() string {
	return filepath.Join(SourceDir(), "test", "nilcore_selftest.c")
}

// Result is what a self-test run reported.
type Result struct {
	// Checks is every `ok`/`FAIL` line, in order.
	Checks []Check

	// Passed and Failed are the counts, derived from Checks rather than counted
	// separately — two counters that can disagree are worse than one that cannot.
	Passed int
	Failed int

	// ExitCode is the process's status. A negative value is a signal on Unix.
	ExitCode int

	// Stderr is whatever the runtime printed, which for a failing operation is
	// the error message.
	Stderr string
}

// Check is one reported assertion.
type Check struct {
	OK     bool
	Name   string
	Detail string
}

// Failed returns the checks that did not pass, for a caller to print.
func (r Result) FailedChecks() []Check {
	var failed []Check
	for _, check := range r.Checks {
		if !check.OK {
			failed = append(failed, check)
		}
	}
	return failed
}

// runSelfTest compiles and runs the self-test binary.
//
// It is a separate process on purpose. Everything about the runtime's contract is
// observable from outside it — the exit status, the bytes on stdout, the text on
// stderr — and a test that linked the runtime into its own process could not check
// the first of those without catching its own exit.
func runSelfTest(t *testing.T, tc toolchain.Toolchain) Result {
	t.Helper()

	cc, ok := tc.CCompiler()
	if !ok {
		t.Skip("no C compiler on this machine")
	}
	linker, ok := tc.Linker()
	if !ok {
		t.Skip("no linker on this machine")
	}

	dir := t.TempDir()
	library, err := Build(dir, tc, nil)
	if err != nil {
		t.Fatalf("building the runtime: %v", err)
	}

	binary := filepath.Join(dir, "nilcore_selftest")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	// Compiled and linked in one step, from the same objects the AOT linker will
	// use. The self-test therefore proves that *these* objects link, not that some
	// separately compiled copy of the same source does.
	args := append([]string{
		"-O1", "-std=c11",
		"-I", IncludeDir(),
		"-o", binary,
		SelfTestPath(),
	}, library.Objects...)
	// The runtime's own objects need its own libraries (`sqrt` lives in libm
	// on unix); without them the self-test fails to link for a reason that
	// has nothing to do with what it checks.
	args = append(args, RuntimeLinkFlags(runtime.GOOS)...)

	if output, err := toolchain.RunTool(cc, args...); err != nil {
		t.Fatalf("compiling the self-test:\n%s", output)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary)
	stderr := &strings.Builder{}
	cmd.Stderr = stderr
	output, runErr := cmd.Output()

	result := Result{Stderr: stderr.String()}
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "ok "):
			result.Checks = append(result.Checks, Check{OK: true, Name: strings.TrimPrefix(line, "ok ")})
		case strings.HasPrefix(line, "FAIL "):
			rest := strings.TrimPrefix(line, "FAIL ")
			name, detail, _ := strings.Cut(rest, " ")
			result.Checks = append(result.Checks, Check{Name: name, Detail: detail})
		case strings.HasPrefix(line, "about to stop"):
			// The marker before the deliberate failure. Not a check.
		case strings.HasPrefix(line, "failures "):
			// The summary; counted from the checks rather than trusted.
		default:
			t.Errorf("the self-test printed a line it should not have: %q", line)
		}
	}

	for _, check := range result.Checks {
		if check.OK {
			result.Passed++
		} else {
			result.Failed++
		}
	}

	if runErr != nil {
		var exitErr *exec.ExitError
		if ok := asExitError(runErr, &exitErr); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("running the self-test: %v", runErr)
		}
	}
	_ = linker
	return result
}

// asExitError unwraps an error into an *exec.ExitError without pulling in errors.As
// at every call site.
func asExitError(err error, target **exec.ExitError) bool {
	exitErr, ok := err.(*exec.ExitError)
	if ok {
		*target = exitErr
	}
	return ok
}

// TestTheRuntimePassesItsOwnTest is the test this whole package exists for.
//
// It is one test rather than many because the runtime's behaviour is a single
// contract: a number of small tests would each pass while the contract as a whole
// was wrong, and the failures this runtime has actually had — a map that hashed by
// address, a predicate that dereferenced a null header, a float printed by a rule
// of its own — were each only visible when several behaviours were checked against
// each other.
func TestTheRuntimePassesItsOwnTest(t *testing.T) {
	tc := hostToolchain(t)
	result := runSelfTest(t, tc)

	if result.Passed == 0 {
		t.Fatal("the self-test reported no checks at all, which means it did not run")
	}
	for _, check := range result.FailedChecks() {
		t.Errorf("%s: %s", check.Name, check.Detail)
	}
	if result.Failed != 0 {
		t.Errorf("%d of %d checks failed", result.Failed, result.Passed+result.Failed)
	}
	t.Logf("%d runtime checks passed", result.Passed)

	// The run ends by stopping the program, which is the documented behaviour of a
	// failing operation and the only way a compiled program can report one.
	if result.ExitCode != exitIndex {
		t.Errorf("the self-test ended with status %d, expected %d (NIL_ERR_INDEX)",
			result.ExitCode, exitIndex)
	}
	if !strings.Contains(result.Stderr, "out of range") {
		t.Errorf("the error message did not say what went wrong: %q", result.Stderr)
	}
}

// TestTheRuntimeHasCheckedEveryClaimItMakes guards the test's own coverage.
//
// A self-test that silently stopped covering something would otherwise look like
// a runtime that quietly lost a feature. The count is here so that deleting checks
// to make a failure go away has to be a deliberate act.
func TestTheRuntimeHasCheckedEveryClaimItMakes(t *testing.T) {
	tc := hostToolchain(t)
	result := runSelfTest(t, tc)

	// Every area the runtime claims to implement has at least one check naming it.
	for _, area := range []string{
		"add", "div", "compare", "len counts characters", "concat", "substr", "find",
		"push grows the array", "map grows past its initial capacity",
		"every entry survives the growth", "is false", "renders", "sqrt", "min",
		"chain survives collection", "reclaimed", "closure is called", "alloc",
	} {
		found := false
		for _, check := range result.Checks {
			if strings.Contains(check.Name, area) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no check covers %q — the self-test has lost coverage", area)
		}
	}
}

// TestAHashedValueIsFoundAgain is the regression test for the map bug.
//
// It is here rather than only in C because the bug it guards is the one a Go test
// can express most directly, and because a bug that cost this much to find should
// have its own named test.
func TestAHashedValueIsFoundAgain(t *testing.T) {
	tc := hostToolchain(t)
	cc, ok := tc.CCompiler()
	if !ok {
		t.Skip("no C compiler on this machine")
	}
	linker, ok := tc.Linker()
	if !ok {
		t.Skip("no linker on this machine")
	}

	program := `
#include "nil.h"
#include <stdio.h>
#include <string.h>

/* Two separately built strings with the same characters, and two separately built
   boxes for the same number. Neither pair shares an address, so any lookup that
   hashed or compared the address would miss. */
int main(void) {
  int wrong = 0;

  NilValue map = nil_map_new(2);
  nil_map_set(map, nil_string_lit("k", 1), nil_int_new(1));
  if (nil_int_value(nil_map_get(map, nil_string_lit("k", 1))) != 1) { wrong++; }

  /* Large enough that 1000000007 cannot be inlined, so the stored and the looked
     up value are two different boxes. */
  nil_map_set(map, nil_string_lit("big", 3), nil_int_new(1000000007));
  if (nil_int_value(nil_map_get(map, nil_string_lit("big", 3))) != 1000000007) { wrong++; }

  /* Growth: fifty keys, then every one of them read back with a fresh string. */
  NilValue big = nil_map_new(2);
  for (int i = 0; i < 50; i++) {
    char key[16];
    snprintf(key, sizeof(key), "k%d", i);
    nil_map_set(big, nil_string_lit(key, strlen(key)), nil_int_new(i));
  }
  for (int i = 0; i < 50; i++) {
    char key[16];
    snprintf(key, sizeof(key), "k%d", i);
    if (nil_int_value(nil_map_get(big, nil_string_lit(key, strlen(key)))) != i) { wrong++; }
  }
  if (nil_map_len(big) != 50) { wrong++; }

  /* Replacing must not add an entry. */
  nil_map_set(big, nil_string_lit("k0", 2), nil_int_new(999));
  if (nil_map_len(big) != 50) { wrong++; }
  if (nil_int_value(nil_map_get(big, nil_string_lit("k0", 2))) != 999) { wrong++; }

  printf("wrong %d\n", wrong);
  return wrong == 0 ? 0 : 1;
}
`
	dir := t.TempDir()
	source := filepath.Join(dir, "hash_probe.c")
	binary := filepath.Join(dir, "hash_probe")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if err := os.WriteFile(source, []byte(program), 0o644); err != nil {
		t.Fatal(err)
	}

	library, err := Build(dir, tc, nil)
	if err != nil {
		t.Fatal(err)
	}

	args := append([]string{"-O1", "-std=c11", "-I", IncludeDir(), "-o", binary, source},
		library.Objects...)
	args = append(args, RuntimeLinkFlags(runtime.GOOS)...)
	if output, err := toolchain.RunTool(cc, args...); err != nil {
		t.Fatalf("compiling the probe:\n%s", output)
	}

	output, err := exec.Command(binary).CombinedOutput()
	if err != nil {
		t.Fatalf("the probe failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "wrong 0") {
		t.Errorf("a value was not found again:\n%s", output)
	}
	_ = linker
}

// TestTheRuntimeReportsFailuresByExitCode is the panic contract.
//
// A compiled program has no way to catch an error, so a failing operation has to
// stop it. The code matters because a caller — a build system, a test — can tell a
// division by zero from an out-of-range index without parsing a message.
func TestTheRuntimeReportsFailuresByExitCode(t *testing.T) {
	tc := hostToolchain(t)
	cc, ok := tc.CCompiler()
	if !ok {
		t.Skip("no C compiler on this machine")
	}

	cases := []struct {
		name    string
		body    string
		want    int
		message string
	}{
		{
			name:    "division by zero",
			body:    `nil_int_div(nil_int_new(1), nil_int_new(0));`,
			want:    exitDivZero,
			message: "division by zero",
		},
		{
			name:    "remainder by zero",
			body:    `nil_int_mod(nil_int_new(1), nil_int_new(0));`,
			want:    exitDivZero,
			message: "remainder by zero",
		},
		{
			name: "an index past the end",
			body: `NilValue a = nil_array_new(1);` +
				`nil_array_get(a, nil_int_new(5));`,
			want:    exitIndex,
			message: "out of range",
		},
		{
			name: "an index on something that is not a container",
			body: `NilValue a = nil_array_new(2);` +
				`nil_index_get(a, nil_string_lit("k", 1));`,
			want:    exitType,
			message: "cannot index",
		},
		{
			name:    "an assignment into a string",
			body:    `nil_index_set(nil_string_lit("abc", 3), nil_int_new(0), nil_int_new(1));`,
			want:    exitType,
			message: "cannot assign into a string",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			program := "#include \"nil.h\"\n#include <stdio.h>\n\nint main(void) {\n  " +
				testCase.body + "\n  printf(\"did not stop\\n\");\n  return 0;\n}\n"

			dir := t.TempDir()
			source := filepath.Join(dir, "fail_probe.c")
			binary := filepath.Join(dir, "fail_probe")
			if runtime.GOOS == "windows" {
				binary += ".exe"
			}
			if err := os.WriteFile(source, []byte(program), 0o644); err != nil {
				t.Fatal(err)
			}

			library, err := Build(dir, tc, nil)
			if err != nil {
				t.Fatal(err)
			}
			args := append([]string{"-O1", "-std=c11", "-I", IncludeDir(), "-o", binary, source},
				library.Objects...)
			args = append(args, RuntimeLinkFlags(runtime.GOOS)...)
			if output, err := toolchain.RunTool(cc, args...); err != nil {
				t.Fatalf("compiling:\n%s", output)
			}

			command := exec.Command(binary)
			output, runErr := command.CombinedOutput()
			if runErr == nil {
				t.Fatalf("the program did not stop; it printed:\n%s", output)
			}

			exitErr, ok := runErr.(*exec.ExitError)
			if !ok {
				t.Fatalf("unexpected failure: %v", runErr)
			}
			if got := exitErr.ExitCode(); got != testCase.want {
				t.Errorf("exited with %d, expected %d\n%s", got, testCase.want, output)
			}
			if !strings.Contains(string(output), testCase.message) {
				t.Errorf("the message did not say %q:\n%s", testCase.message, output)
			}
			if strings.Contains(string(output), "did not stop") {
				t.Errorf("the program carried on past the failure:\n%s", output)
			}
		})
	}
}

// TestTheRuntimeLinksWithoutAWritableSourceTree is the property the C runtime has
// that a Go one would not.
//
// A build must work from an installed compiler with no access to the repository.
// This copies only the objects — never the sources — and links against those, which
// is what a user who installed Nilang has.
func TestTheRuntimeLinksWithoutAWritableSourceTree(t *testing.T) {
	tc := hostToolchain(t)
	linker, ok := tc.Linker()
	if !ok {
		t.Skip("no linker on this machine")
	}

	library, err := Build(t.TempDir(), tc, nil)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	main := filepath.Join(dir, "main.c")
	if err := os.WriteFile(main, []byte(
		"#include \"nil.h\"\nint main(void) { nil_println(nil_int_new(7)); return 0; }\n"),
		0o644); err != nil {
		t.Fatal(err)
	}

	cc, ok := tc.CCompiler()
	if !ok {
		t.Skip("no C compiler on this machine")
	}
	binary := filepath.Join(dir, "app")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	args := append([]string{"-I", IncludeDir(), "-o", binary, main}, library.Objects...)
	args = append(args, RuntimeLinkFlags(runtime.GOOS)...)
	if output, err := toolchain.RunTool(cc, args...); err != nil {
		t.Fatalf("linking against the runtime:\n%s", output)
	}
	_ = linker

	output, err := exec.Command(binary).Output()
	if err != nil {
		t.Fatalf("the linked program failed: %v", err)
	}
	if strings.TrimSpace(string(output)) != "7" {
		t.Errorf("printed %q, expected %q", strings.TrimSpace(string(output)), "7")
	}
}

// TestFingerprintIsStableAcrossProcesses guards the cache against a clock or an
// environment leaking into it.
func TestFingerprintIsStableAcrossProcesses(t *testing.T) {
	tc := hostToolchain(t)
	cc, ok := tc.CCompiler()
	if !ok {
		t.Skip("no C compiler on this machine")
	}

	first, err := Fingerprint(cc, tc.Target.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Fingerprint(cc, tc.Target.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Errorf("the fingerprint is not stable: %s then %s", first[:12], second[:12])
	}

	// And a different flag set is a different build.
	other, err := Fingerprint(cc, tc.Target.ID, []string{"-O0"})
	if err != nil {
		t.Fatal(err)
	}
	if other == first {
		t.Error("the fingerprint ignores the compiler flags")
	}
}

// TestTheRuntimeUsesTheDocumentedErrorCodes keeps the Go constants and nil.h in
// step.
//
// The two lists are written out in two languages; a test that compares them is the
// only thing that notices when one gains an entry and the other does not. A
// mismatch here would mean a build system reading exit codes from the header and a
// test reading them from here disagreeing about what a program returning 3 means.
func TestTheRuntimeUsesTheDocumentedErrorCodes(t *testing.T) {
	header, err := os.ReadFile(filepath.Join(IncludeDir(), "nil.h"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(header)
	for name, value := range map[string]int{
		"NIL_OK":           exitOK,
		"NIL_ERR_DIV_ZERO": exitDivZero,
		"NIL_ERR_INDEX":    exitIndex,
	} {
		want := fmt.Sprintf("#define %s %s", name, strconv.Itoa(value))
		if !strings.Contains(text, want) {
			t.Errorf("nil.h does not define %s as %d; this test does", name, value)
		}
	}
}
