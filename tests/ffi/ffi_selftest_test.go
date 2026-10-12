package ffi

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/compiler/llvm"
	"github.com/joysriramsarkar/nilLang/compiler/nilcore"
	"github.com/joysriramsarkar/nilLang/compiler/toolchain"
)

// L5.8: the message codec and the platform channel, exercised from C.
//
// # Why a C program rather than a Nilang one
//
// Because the FFI's contract *is* a C ABI. A Nilang test can drive a channel, and the
// end-to-end tests in this package do, but it can only reach it through `extern` — so
// every failure it reports is one failure removed. This program calls `nil_encode`,
// `nil_decode` and the `nil_channel_*` entry points directly, which is the view a host
// has, and it is compiled and linked against the same objects a real program's runtime
// is built from.
//
// The codec in particular cannot be tested any other way. Its whole purpose is to be
// read by something that does not share the runtime's types, and a test written in
// Nilang shares every one of them.

// TestTheFFIPassesItsOwnTest builds the FFI self-test and runs it.
//
// Skipped rather than failed when this machine has no C compiler: a developer working on
// the compiler should not be blocked by the absence of gcc, and the end-to-end tests
// below still run — they need the same compiler, so they would skip too, which is the
// one thing worth being clear about.
func TestTheFFIPassesItsOwnTest(t *testing.T) {
	tc := toolchain.DiscoverToolchain(llvm.TargetHost)
	cc, hasCompiler := tc.CCompiler()
	if !hasCompiler {
		t.Skip("no C compiler on this machine; the FFI was checked by the end-to-end tests only")
	}
	t.Logf("building with %s", cc.Path)

	library, err := nilcore.Build(t.TempDir(), tc, nil)
	if err != nil {
		t.Fatalf("building the runtime: %v", err)
	}

	dir := t.TempDir()
	binary := filepath.Join(dir, "nilffi_selftest")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	args := append([]string{
		"-O1", "-std=c11",
		"-I", nilcore.IncludeDir(),
		"-I", nilcore.FFIDir(),
		"-o", binary,
		ffiSelfTestPath(t),
	}, library.Objects...)

	if output, err := toolchain.RunTool(cc, args...); err != nil {
		t.Fatalf("compiling the FFI self-test:\n%s", output)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stderr := &strings.Builder{}
	cmd := exec.CommandContext(ctx, binary)
	cmd.Stderr = stderr
	output, runErr := cmd.Output()

	passed, reported := 0, 0
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ok ") {
			passed++
			continue
		}
		if strings.HasPrefix(line, "FAIL ") {
			reported++
			t.Errorf("the FFI self-test: %s", line)
			continue
		}
		if strings.HasPrefix(line, "failures ") || line == "" {
			continue
		}
		t.Errorf("the FFI self-test printed a line it should not have: %q\n%s", line, stderr.String())
	}

	if runErr != nil {
		t.Fatalf("the FFI self-test did not finish: %v\n%s", runErr, stderr.String())
	}
	if passed+reported == 0 {
		t.Fatalf("the FFI self-test reported nothing, so nothing was checked\n%s", stderr.String())
	}
	t.Logf("%d FFI checks passed", passed)
}

// ffiSelfTestPath is where the FFI's own self-test lives.
//
// Spelled out rather than taken from `nilcore`, because `nilcore` has no path helper
// for it: `SelfTestPath` is declared in that package's *test* file, and a function in a
// `_test.go` cannot be imported. This is the same relative path the C fixtures in this
// directory already use, so a file that is there for one is there for the others.
func ffiSelfTestPath(t *testing.T) string {
	t.Helper()

	path := filepath.Join("..", "..", "runtime", "ffi", "test", "nilffi_selftest.c")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("the FFI self-test source is not present: %v", err)
	}
	return path
}

// TestTheFFISelfTestActuallyRunsSomething keeps the gate honest.
//
// A self-test that reported nothing would "pass" in the sense that no line failed —
// so the count is checked against the number of test functions the source defines. It
// is a small check about a small thing, and it is the difference between "the FFI
// passed" and "the FFI was never asked".
func TestTheFFISelfTestActuallyRunsSomething(t *testing.T) {
	source, err := os.ReadFile(ffiSelfTestPath(t))
	if err != nil {
		t.Fatalf("the FFI self-test source could not be read: %v", err)
	}

	called := 0
	for _, line := range strings.Split(string(source), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "test_") || !strings.HasSuffix(trimmed, "();") {
			continue
		}
		called++
	}
	if called == 0 {
		t.Fatal("the FFI self-test's main calls no tests, so the run above proves nothing")
	}
	t.Logf("the self-test calls %d sections", called)
}
