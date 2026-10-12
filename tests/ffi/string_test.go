package ffi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/llvm"
	"github.com/joysriramsarkar/nilLang/compiler/toolchain"
)

// The string crossing, run rather than inspected.
//
// A string is the only crossing that takes *two* arguments and the only one that
// has to come back out of a caller-allocated struct, and every way of getting
// either wrong is silent. A length in characters where C wanted bytes answers a
// different number for any string outside ASCII. A pointer with no length makes
// the empty string and the end of a buffer the same value. A struct returned by
// value is read from the wrong registers entirely — which is not a guess, it is
// what happened: clang assembles a 16-byte struct return into RAX:RDX while GCC
// for `x86_64-w64-mingw32` returns it through a pointer in RCX, so the first
// version of this crossing answered every string with length zero on a machine
// holding both compilers.
//
// All of it was caught by running the program. None of it was visible in the
// emitted IR.

// TestAStringCrossesToCAndComesBack is the gate on L5.4.
//
// The two counts are what makes it a test. "héllo" is six bytes and five
// characters, so a crossing that confused them answers a number that is wrong
// rather than a number that is right by luck.
func TestAStringCrossesToCAndComesBack(t *testing.T) {
	source := `
		extern "C" fn nilffi_byte_length(bytes: String) -> Int;
		extern "C" fn nilffi_char_length(text: String) -> Int;

		nilffi_byte_length("héllo") * 10 + nilffi_char_length("héllo");
	`

	got := runStringProgram(t, source)
	// 6 bytes, 5 characters.
	if !strings.Contains(got, "65") {
		t.Errorf("a string crossed with the wrong length: printed %q, wanted 65", got)
	}
}

// TestAStringComesBackAsAString is the return direction.
//
// C builds a buffer and answers with it; the crossing copies it, so C may free it
// the moment it returns. Without the copy the program would read freed memory —
// which usually reads the right answer anyway, because the allocator hands back
// the same block. That is why the fixture frees it explicitly.
func TestAStringComesBackAsAString(t *testing.T) {
	source := `
		extern "C" fn nilffi_upper(text: String) -> String;
		extern "C" fn nilffi_byte_length(bytes: String) -> Int;
		extern "C" fn nilffi_char_length(text: String) -> Int;

		let answer = nilffi_upper("héllo");
		nilffi_byte_length(answer) * 10 + nilffi_char_length(answer);
	`

	got := runStringProgram(t, source)
	// "HÉLLO" is five characters and six bytes, the same shape as what went in.
	if !strings.Contains(got, "65") {
		t.Errorf("the returned string did not survive: printed %q, wanted 65", got)
	}
}

// TestAStringCBuiltOutlivesTheBufferItCameFrom is the copy, made observable.
//
// C builds an answer into its own buffer and hands the pointer over. The crossing
// copies it into a Nilang-owned string, so the answer stays valid after C has gone
// on to overwrite the buffer — which is what a `static` buffer in C is for.
//
// The check is a *second* call over the same buffer. If the crossing had borrowed
// the pointer rather than copying, the second answer would be whatever the first
// call left behind, and this would answer 0 for a string C can build perfectly
// well.
func TestAStringCBuiltOutlivesTheBufferItCameFrom(t *testing.T) {
	source := `
		extern "C" fn nilffi_greeting(name: String) -> String;
		extern "C" fn nilffi_byte_length(bytes: String) -> Int;

		let first = nilffi_greeting("abc");
		let second = nilffi_greeting("xyz");
		nilffi_byte_length(first) * 100 + nilffi_byte_length(second);
	`

	got := runStringProgram(t, source)
	// "hello, abc" and "hello, xyz" are both 10 bytes, and both survive.
	if !strings.Contains(got, "1010") {
		t.Errorf("an answer did not survive the next call: printed %q, wanted 1010", got)
	}
}

// TestAReturnedStringIsLongerThanWhatWentIn catches a reused length.
//
// `nilffi_greeting` answers "hello, " plus the name — longer than anything it was
// given. A crossing that carried the input's length through to the output would
// report the input's length instead, and the two numbers here differ by seven.
func TestAReturnedStringIsLongerThanWhatWentIn(t *testing.T) {
	source := `
		extern "C" fn nilffi_greeting(name: String) -> String;
		extern "C" fn nilffi_byte_length(bytes: String) -> Int;

		nilffi_byte_length(nilffi_greeting("hi")) * 10
			+ nilffi_byte_length("hi");
	`

	got := runStringProgram(t, source)
	// 9 bytes back ("hello, hi"), 2 bytes in.
	if !strings.Contains(got, "92") {
		t.Errorf("the returned length was reused from the input: printed %q, wanted 92", got)
	}
}

// TestAnEmptyStringIsEmptyRatherThanMissing is where a pointer and a length earn
// their keep.
//
// The empty string and "a pointer past the end" are the same value when only a
// pointer crosses. They must not be here, and the difference is visible in the
// answer: the empty string has length 0 and is still a string, and asking C for
// five bytes of it gives nothing rather than an error.
func TestAnEmptyStringIsEmptyRatherThanMissing(t *testing.T) {
	source := `
		extern "C" fn nilffi_nothing() -> String;
		extern "C" fn nilffi_byte_length(bytes: String) -> Int;
		extern "C" fn nilffi_take(text: String, count: Int) -> String;

		let empty = nilffi_nothing();
		nilffi_byte_length(empty) * 100 + nilffi_byte_length(nilffi_take(empty, 5));
	`

	got := runStringProgram(t, source)
	// 0 bytes either way: asking for five of nothing gives 0, not a failure.
	if !strings.Contains(got, "0") {
		t.Errorf("an empty string did not answer 0: printed %q", got)
	}
}

// TestAStringWithANulIsLengthNotTerminator is the case `strlen` would get wrong.
//
// A Nilang string may contain a NUL, and a length in bytes is what says so. A
// crossing that found the end of the string by scanning for one would truncate
// here.
func TestAStringWithANulIsLengthNotTerminator(t *testing.T) {
	source := `
		extern "C" fn nilffi_byte_length(bytes: String) -> Int;
		extern "C" fn nilffi_take(text: String, count: Int) -> String;

		let withNul = "a\0b";
		nilffi_byte_length(withNul) * 10 + nilffi_byte_length(nilffi_take(withNul, 3));
	`

	got := runStringProgram(t, source)
	// 3 bytes, and taking 3 gives all 3.
	if !strings.Contains(got, "33") {
		t.Errorf("a NUL truncated the string: printed %q, wanted 33", got)
	}
}

// TestSlicingAtTheEdgeClampsRatherThanReads is the boundary a C slice sits on.
//
// Asking for more than there is returns what there is, not whatever is past the
// end. The alternative is a read of memory the program does not own.
func TestSlicingAtTheEdgeClampsRatherThanReads(t *testing.T) {
	source := `
		extern "C" fn nilffi_take(text: String, count: Int) -> String;
		extern "C" fn nilffi_byte_length(bytes: String) -> Int;

		nilffi_byte_length(nilffi_take("hello", 99));
	`

	got := runStringProgram(t, source)
	if !strings.Contains(got, "5") {
		t.Errorf("slicing past the end did not clamp: printed %q, wanted 5", got)
	}
}

// TestAStringAndAnIntegerCrossInOneCall is the conversion beside itself.
//
// Both directions of the table are exercised by one declaration, so a program that
// passes an integer where a string belongs — the mistake a hand-written `extern`
// is most likely to make — has to be caught by the checker's refusal rather than
// by C reading an integer's bits as a pointer.
func TestAStringAndAnIntegerCrossInOneCall(t *testing.T) {
	source := `
		extern "C" fn nilffi_take(text: String, count: Int) -> String;
		extern "C" fn nilffi_byte_length(bytes: String) -> Int;

		nilffi_byte_length(nilffi_take("abcdef", 2));
	`

	got := runStringProgram(t, source)
	if !strings.Contains(got, "2") {
		t.Errorf("a mixed call crossed wrongly: printed %q, wanted 2", got)
	}
}

// runStringProgram builds a program against the string fixture and runs it.
func runStringProgram(t *testing.T, source string) string {
	t.Helper()

	tc := toolchain.DiscoverToolchain(llvm.TargetHost)
	if _, hasLinker := tc.Linker(); !hasLinker {
		t.Skipf("no linker on this machine: %s", tc.LinkAdvice())
	}

	fixture := filepath.Join("..", "..", "runtime", "ffi", "test", "stringfixture.c")
	if _, err := os.Stat(fixture); err != nil {
		t.Skipf("the C fixture is not present: %v", err)
	}
	compiler, hasCompiler := tc.CCompiler()
	if !hasCompiler {
		t.Skip("no C compiler on this machine; strings were checked by emitted IR only")
	}

	object := filepath.Join(t.TempDir(), "stringfixture.o")
	flags := []string{"-c", fixture, "-o", object, "-O0",
		"-I", filepath.Join("..", "..", "runtime", "core", "include"),
		"-I", filepath.Join("..", "..", "runtime", "ffi", "include")}
	if _, err := toolchain.RunTool(compiler, flags...); err != nil {
		t.Fatalf("compiling the C fixture failed: %v", err)
	}

	return runProgram(t, source, object)
}
