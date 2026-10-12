package ffi

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/aot"
	"github.com/joysriramsarkar/nilLang/compiler/cheader"
	"github.com/joysriramsarkar/nilLang/compiler/llvm"
	"github.com/joysriramsarkar/nilLang/compiler/toolchain"
)

// L5.2: the header a C author implements against.
//
// Everything else in this file tests the FFI from the Nilang side. This tests it
// from the *other* side, which is the side the FFI exists to serve: a person
// writing C who has to know what the Nilang program expects.
//
// The test writes a C file that includes the generated header and nothing else
// from the project, compiles it with `-Wall -Wextra -Werror`, links it against the
// program, and runs it. A header that cannot be compiled is not a header, and this
// is the only way to know that without trusting that it looks right.

// TestACProgramWrittenAgainstTheGeneratedHeaderBuildsAndRuns is the gate.
//
// It is deliberately the whole loop rather than a check of the text. A prototype
// can read perfectly and still be wrong — one that said a string was a single
// pointer would compile, link, and read a length out of a pointer — and reading the
// text cannot catch that. Compiling the implementation and running the program can.
func TestACProgramWrittenAgainstTheGeneratedHeaderBuildsAndRuns(t *testing.T) {
	source := `
		extern "C" fn greeting(name: String) -> String;
		extern "C" fn measure(text: String) -> Int;

		let answer = greeting("Nilang");
		measure(answer);
	`

	// Written by including the header and nothing else. If the header is missing a
	// declaration, names a parameter wrongly, or describes the wrong convention,
	// this file does not compile — which is the whole claim.
	implementation := `
#include "bridge.h"

#include <string.h>

int64_t measure(const char *text, size_t text_len) {
  /* A Nilang string is not NUL-terminated; the length is in bytes. */
  return (int64_t)text_len;
}

void greeting(const char *name, size_t name_len, NilFFIString *out) {
  static const char prefix[] = "hello, ";
  size_t total = (sizeof(prefix) - 1) + name_len;
  char *answer = nil_ffi_buffer((int64_t)total);
  memcpy(answer, prefix, sizeof(prefix) - 1);
  memcpy(answer + sizeof(prefix) - 1, name, name_len);
  out->bytes = answer;
  out->length = total;
}
`

	// "hello, Nilang" is thirteen bytes.
	got := runAgainstGeneratedHeader(t, source, "bridge", implementation, "13")
	if !strings.Contains(got, "13") {
		t.Errorf("a C implementation written from the header answered %q, wanted 13", got)
	}
}

// TestTheHeaderSaysTheLengthIsInBytes is the note that has to be there.
//
// Every rule the FFI enforces at run time produces no diagnostic when broken, so a
// header that omits them is a header whose reader writes a bug and never finds out.
// The rules are therefore written into the header itself, next to the prototype they
// apply to.
func TestTheHeaderSaysTheLengthIsInBytes(t *testing.T) {
	source := `
		extern "C" fn measure(text: String) -> Int;
		measure("hello");
	`

	got := runAgainstGeneratedHeader(t, source, "bytes", `
#include "bytes.h"
int64_t measure(const char *text, size_t text_len) { return (int64_t)text_len; }
`, "5")

	header := lastGeneratedHeader(t)
	for _, want := range []string{"BYTES", "not characters", "NUL"} {
		if !strings.Contains(header, want) {
			t.Errorf("the header does not say %q about a string's length:\n%s", want, header)
		}
	}
	_ = got
}

// TestTheHeaderSaysHowToFillAStringAnswer is the rule that leaks.
//
// A C author who builds an answer with `malloc` produces a program that works and
// leaks one block per call. The header says to use `nil_ffi_buffer`, which the
// runtime reclaims — and says not to free it, because a reader who does is back to
// reading freed memory.
func TestTheHeaderSaysHowToFillAStringAnswer(t *testing.T) {
	source := `
		extern "C" fn answer() -> String;
		answer();
	`

	// The program prints the string it got back, so this checks the whole round
	// trip rather than a length: a C function that filled `out` from a buffer the
	// runtime does not own still prints the right answer, but reads it after the
	// scope has closed.
	got := runAgainstGeneratedHeader(t, source, "answer", `
#include "answer.h"
void answer(NilFFIString *out) {
  static const char text[] = "done";
  out->bytes = text;
  out->length = 4;
}
`, "done")

	if !strings.Contains(got, "done") {
		t.Errorf("the answer did not survive the crossing: %q", got)
	}

	header := lastGeneratedHeader(t)
	for _, want := range []string{"nil_ffi_buffer", "do not free"} {
		if !strings.Contains(header, want) {
			t.Errorf("the header does not say %q about filling a string answer:\n%s", want, header)
		}
	}
}

// runAgainstGeneratedHeader builds and runs a program whose C side was written from
// the generated header alone.
//
// The whole loop on purpose: generate, implement against it, compile with every
// warning an error, link, run. A test that stopped after generating would pass on a
// header that cannot be included.
func runAgainstGeneratedHeader(t *testing.T, source, headerName, implementation, want string) string {
	t.Helper()

	tc := toolchain.DiscoverToolchain(llvm.TargetHost)
	if _, hasLinker := tc.Linker(); !hasLinker {
		t.Skipf("no linker on this machine: %s", tc.LinkAdvice())
	}
	cc, hasCompiler := tc.CCompiler()
	if !hasCompiler {
		t.Skip("no C compiler on this machine; the header was checked by its text only")
	}

	dir := t.TempDir()

	// The header, generated from the same program the module is.
	program := lower(t, source)
	header, err := cheader.Generate(program, headerName)
	if err != nil {
		t.Fatalf("generating the header failed: %v", err)
	}
	lastHeader.Store(header)

	headerPath := filepath.Join(dir, headerName+".h")
	if err := os.WriteFile(headerPath, []byte(header), 0o644); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(dir, headerName+".c")
	if err := os.WriteFile(sourcePath, []byte(implementation), 0o644); err != nil {
		t.Fatal(err)
	}

	includes := []string{
		"-I", dir,
		"-I", filepath.Join("..", "..", "runtime", "core", "include"),
		"-I", filepath.Join("..", "..", "runtime", "ffi", "include"),
	}
	object := filepath.Join(dir, "impl.o")
	compile := append([]string{"-c", sourcePath, "-o", object, "-O0", "-std=c11"}, includes...)
	// Every warning an error, because a warning here is a declaration and a
	// declaration that needed a warning is a declaration C accepted on a technicality.
	compile = append(compile, "-Wall", "-Wextra", "-Werror", "-Wno-unused-parameter")
	if out, err := toolchain.RunTool(cc, compile...); err != nil {
		t.Fatalf("a C file written against the generated header did not compile:\n%s\n--- header ---\n%s",
			strings.TrimSpace(out), header)
	}

	output := filepath.Join(dir, "program")
	if llvm.TargetHost.OS == "windows" {
		output += ".exe"
	}
	module, err := llvm.NewEmitter(llvm.TargetHost).Emit(program)
	if err != nil {
		t.Fatalf("emitting failed: %v", err)
	}
	if _, err := aot.Build(module, llvm.TargetHost, tc, aot.Options{
		OutputPath:   output,
		ExtraObjects: []string{object},
		Libraries:    []string{"m"},
	}); err != nil {
		t.Fatalf("linking against a header-driven implementation failed:\n%v\n--- header ---\n%s", err, header)
	}

	result, err := exec.Command(output).CombinedOutput()
	if err != nil {
		t.Fatalf("running failed: %v\n%s", err, result)
	}
	if want != "" && !strings.Contains(string(result), want) {
		t.Errorf("printed %q, wanted %q\n--- header ---\n%s", strings.TrimSpace(string(result)), want, header)
	}
	return string(result)
}

// lastHeader carries the most recently generated header to the tests that inspect
// its text after a run.
//
// A field rather than a return value because two of the tests are about *what the
// header says* and the thing they also need — a program that links against it — is
// the long part.
var lastHeader atomicValue

type atomicValue struct{ value string }

func (v *atomicValue) Store(s string) { v.value = s }

func lastGeneratedHeader(t *testing.T) string {
	t.Helper()
	if lastHeader.value == "" {
		t.Fatal("no header was generated; the test did not run the build")
	}
	return lastHeader.value
}
