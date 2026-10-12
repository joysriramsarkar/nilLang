package ffi

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/llvm"
	"github.com/joysriramsarkar/nilLang/compiler/toolchain"
)

// `nil_ffi_can_cross` is the only question a C author asks about a *value* rather
// than a type, and until now nothing in this repository asked it.
//
// It has a real caller, and it is the shape every callback has: a Nilang value
// arrives on the C side, C keeps it, and C wants to treat it as a number. It cannot
// assume it is one. A program may have answered with a string, a list, or nothing at
// all, and `nil_int_unbox` on a list reads a pointer's low bits as a number.
//
// The fixture *keeps* the value in C rather than sending it back, and that is not a
// convenience. A value returned through a `String` crossing arrives as an empty
// string whatever it was, so the question would be asked about the empty string and
// answered yes every time. C only ever holds the original when nothing has converted
// it yet, so that is the only way to test this honestly.

// TestCAsksWhetherAValueCrossesBeforeTreatingItAsANumber is the whole point of
// `nil_ffi_can_cross`.
//
// The program makes each kind of value the language has, hands it to C, and asks.
// The answers have to differ, because the values do — and a function that always
// answered 1 would pass a single case while proving nothing.
func TestCAsksWhetherAValueCrossesBeforeTreatingItAsANumber(t *testing.T) {
	cases := []struct {
		name string
		// body is what the callback answers with.
		body string
		want string
	}{
		{"null", "return null;", "1"},
		{"true", "return true;", "1"},
		{"an integer", "return 42;", "1"},
		{"a string", `return "text";`, "1"},
		// No crossing for a list, and that is the whole reason this function
		// exists: a list would otherwise reach C as a pointer whose low bits are
		// read as a number.
		{"a list", "return [1, 2, 3];", "0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := `
				extern "C" fn nilffi_hand_back(callback: fn(Int) -> Int) -> Void;
				extern "C" fn nilffi_can_cross_held() -> Int;

				fn answer(n: Int) -> Int { ` + tc.body + ` 0; }

				nilffi_hand_back(answer);
				nilffi_can_cross_held();
			`
			got := runWithCallback(t, source)
			if !strings.Contains(got, tc.want) {
				t.Errorf("%s: printed %q, wanted %s", tc.name, strings.TrimSpace(got), tc.want)
			}
		})
	}
}

// TestCrossingAndHavingBytesToReadAreTwoQuestions.
//
// A value can cross and still have no bytes — an integer crosses, and unboxing it as
// a string gives nothing. So a caller needs both answers before it reaches for the
// bytes, and a single predicate would be wrong for every number.
//
// The length is asked through `nil_string_unbox`, which answers 0 for anything that
// is not a string. That is the right answer for a caller about to read: "there is
// nothing here" rather than a question about types.
func TestCrossingAndHavingBytesToReadAreTwoQuestions(t *testing.T) {
	source := `
		extern "C" fn nilffi_hand_back(callback: fn(Int) -> Int) -> Void;
		extern "C" fn nilffi_can_cross_held() -> Int;
		extern "C" fn nilffi_held_length() -> Int;

		fn answer(n: Int) -> Int { return 42; 0; }

		nilffi_hand_back(answer);
		nilffi_can_cross_held() * 100 + nilffi_held_length();
	`
	got := runWithCallback(t, source)
	// Crosses (1), and unboxing an integer as a string gives 0 bytes: 100.
	if !strings.Contains(got, "100") {
		t.Errorf("an integer was read as a string: printed %q, wanted 100", strings.TrimSpace(got))
	}
}

// TestCReadsTheStringTheProgramProduced is the positive case.
//
// Having answered "yes, this crosses" and "yes, this is a string", C reads the
// bytes — and they are the program's own, unmodified, whatever conversion would have
// done to them.
func TestCReadsTheStringTheProgramProduced(t *testing.T) {
	source := `
		extern "C" fn nilffi_hand_back(callback: fn(Int) -> Int) -> Void;
		extern "C" fn nilffi_held_length() -> Int;

		fn answer(n: Int) -> Int { return "measured"; 0; }

		nilffi_hand_back(answer);
		nilffi_held_length();
	`
	got := runWithCallback(t, source)
	// "measured" is eight bytes.
	if !strings.Contains(got, "8") {
		t.Errorf("C did not read the program's own string: printed %q, wanted 8", strings.TrimSpace(got))
	}
}

// runWithCallback builds a program against the callback fixture.
func runWithCallback(t *testing.T, source string) string {
	t.Helper()
	tc := toolchain.DiscoverToolchain(llvm.TargetHost)
	return runProgram(t, source, buildCallbackFixture(t, tc))
}
