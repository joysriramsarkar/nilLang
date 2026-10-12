package llvm

import (
	"strings"
	"testing"
)

// TestAStringCrossesAsAPointerAndALength is the shape of L5.4.
//
// A C string has no length, and a length is the only thing that distinguishes the
// empty string from the end of one. A single pointer would make the two the same
// value and every C function written to take a length would have to guess — so the
// declaration says two arguments and the call sends two.
func TestAStringCrossesAsAPointerAndALength(t *testing.T) {
	ir, err := compileForTest(`
		extern "C" fn strlen_of(text: String) -> Int;
		extern "C" fn greet(name: String) -> String;

		strlen_of(greet("world"));
	`)
	if err != nil {
		t.Fatalf("emitting failed: %v", err)
	}

	// The declaration is the C signature.
	if !strings.Contains(ir, "declare i64 @strlen_of(ptr, i64)") {
		t.Errorf("a string parameter is not declared as a pointer and a length:\n%s", ir)
	}
	// And the call sends a pointer and a length, read out of a struct the caller
	// allocated.
	if !strings.Contains(ir, "@strlen_of(ptr %") {
		t.Errorf("the call does not send a pointer first:\n%s", ir)
	}
	if !strings.Contains(ir, "extractvalue %nilffi.str") {
		t.Errorf("the string was not taken apart into two arguments:\n%s", ir)
	}

	// A string return is filled into a struct the caller owns, and the answer is
	// then copied into a Nilang string.
	if !strings.Contains(ir, "declare void @greet(ptr, i64, ptr)") {
		t.Errorf("a string return does not take a caller's struct:\n%s", ir)
	}
	if !strings.Contains(ir, "@nil_string_copy(ptr %") {
		t.Errorf("a string return is not copied into a Nilang string:\n%s", ir)
	}
}

// TestNothingInAModuleIsReturnedAsAStruct is the rule that replaced a struct
// return, and it is a rule about *every* function rather than about strings.
//
// Whether a 16-byte struct comes back in registers or through a hidden pointer is
// not a convention two compilers share: clang assembling a module with an
// `x86_64-pc-windows-msvc` triple returns `{ptr, i64}` in RAX:RDX, while GCC for
// `x86_64-w64-mingw32` returns it through a pointer in RCX. Both are correct, both
// appear in the same link, and the call site cannot tell which one it got.
//
// That is not a hypothetical. The first version of the string crossing did exactly
// this, linked cleanly on a machine holding both compilers, and answered every
// string with length zero.
//
// So the check is structural rather than per-type: no `declare` in a module may
// return a named struct. A test that listed today's offenders would pass the day a
// second one was added.
func TestNothingInAModuleIsReturnedAsAStruct(t *testing.T) {
	ir, err := compileForTest(`
		extern "C" fn greet(name: String) -> String;
		extern "C" fn measure(text: String) -> Int;

		measure(greet("world"));
	`)
	if err != nil {
		t.Fatalf("emitting failed: %v", err)
	}

	for _, line := range strings.Split(ir, "\n") {
		if !strings.HasPrefix(line, "declare ") {
			continue
		}
		result := line[len("declare "):]
		if fields := strings.Fields(result); len(fields) > 0 && strings.HasPrefix(fields[0], "%") {
			t.Errorf("a declaration returns a struct, whose meaning depends on which "+
				"compiler produced the other half:\n%s", line)
		}
	}
}

// TestAStringCrossingIsWrittenFromOneAnswer guards a split that already happened.
//
// The declaration and the call site are written separately, and they once disagreed:
// the call went to `nil_string_copy` — the *conversion* — where the declaration said
// `@nilffi_take`. It linked, because the symbol existed, and answered nothing at all.
//
// Both are now written from the same signature, so a function that answers with a
// string and one that takes one are the same shape to this emitter. The test that
// catches it is a *run*, not an inspection — this one only pins the shape.
func TestAStringCrossingIsWrittenFromOneAnswer(t *testing.T) {
	ir, err := compileForTest(`
		extern "C" fn take(text: String, count: Int) -> String;

		take("hello", 2);
	`)
	if err != nil {
		t.Fatalf("emitting failed: %v", err)
	}

	// Exactly one call to the foreign function, and exactly one to the copy.
	if got := strings.Count(ir, "call void @take("); got != 1 {
		t.Errorf("the foreign function was called %d times, wanted 1:\n%s", got, ir)
	}
	if got := strings.Count(ir, "call void @nil_string_copy("); got != 0 {
		t.Errorf("the conversion was called as if it were the function (%d times):\n%s", got, ir)
	}
	if !strings.Contains(ir, "call i64 @nil_string_copy(ptr %") {
		t.Errorf("the answer was not copied into a Nilang string:\n%s", ir)
	}
}

// TestAModuleWithNoStringsCarriesNoStringStruct keeps the change local.
//
// A program that crosses no string must produce byte-identical IR to one written
// before the string crossing existed. Otherwise every existing module changes and
// "did adding the FFI change this program?" stops being a question a diff can
// answer.
func TestAModuleWithNoStringsCarriesNoStringStruct(t *testing.T) {
	ir, err := compileForTest(`let a = 3; let b = 4; a * b;`)
	if err != nil {
		t.Fatalf("emitting failed: %v", err)
	}
	for _, unwanted := range []string{"%nilffi.str", "nil_string_unbox", "nil_string_copy"} {
		if strings.Contains(ir, unwanted) {
			t.Errorf("a module crossing no string mentions %s:\n%s", unwanted, ir)
		}
	}
}

// TestAModuleWithAStringDeclaresItsStruct is the other side.
//
// The struct is what the caller allocates to hand a string to or receive one from.
// A module that omits it is rejected for a type nobody declared — pointing at a
// line that reads perfectly ordinarily.
func TestAModuleWithAStringDeclaresItsStruct(t *testing.T) {
	ir, err := compileForTest(`
		extern "C" fn takes(text: String) -> Void;
		takes("hello");
	`)
	if err != nil {
		t.Fatalf("emitting failed: %v", err)
	}
	if !strings.Contains(ir, "%nilffi.str = type { ptr, i64 }") {
		t.Errorf("the string struct is not declared:\n%s", ir)
	}
	for _, symbol := range []string{"nil_string_unbox"} {
		if strings.Contains(ir, symbol) && !strings.Contains(ir, "declare void @"+symbol) {
			t.Errorf("%s is used but never declared:\n%s", symbol, ir)
		}
	}
}

// TestAModuleWithNoExternMentionsNoFFI keeps the change local.
//
// A program that calls no C function should produce byte-identical IR to one that
// never learned about FFI. Otherwise every existing module changes, every diff
// becomes unreadable, and "did adding the FFI change this program?" stops being a
// question a diff can answer.
func TestAModuleWithNoExternMentionsNoFFI(t *testing.T) {
	ir, err := compileForTest(`let a = 3; let b = 4; a * b;`)
	if err != nil {
		t.Fatalf("emitting failed: %v", err)
	}
	for _, symbol := range []string{"nil_int_unbox", "nil_int_box", "nil_float_box", "nilffi"} {
		if strings.Contains(ir, symbol) {
			t.Errorf("a module with no extern mentions %s:\n%s", symbol, ir)
		}
	}
}

// TestAModuleWithAnExternDeclaresItsConversions is the other side of that.
//
// A module that calls `nil_int_unbox` must declare it. Missing the declaration
// produces a module that assembles on a machine where the symbol happens to exist
// and fails to link on one where it does not — the worst place for it to fail,
// because those two machines are the developer's and the user's.
func TestAModuleWithAnExternDeclaresItsConversions(t *testing.T) {
	ir, err := compileForTest(`
		extern "C" fn nilffi_double(value: Int) -> Int;
		nilffi_double(21);
	`)
	if err != nil {
		t.Fatalf("emitting failed: %v", err)
	}

	for _, symbol := range []string{"nil_int_unbox", "nil_int_box"} {
		if !strings.Contains(ir, "declare i64 @"+symbol) {
			t.Errorf("the module calls %s but does not declare it:\n%s", symbol, ir)
		}
	}
}

// TestTheDeclarationIsTheCOne says what the linker has to match, and what the
// call has to send.
//
// The declaration is written in C's terms and the conversion happens at the call.
// The two halves have to agree, and nothing checks that but running the program:
// a declaration in Nilang's terms with a converted call would be a module that
// assembles, links, and passes the wrong bytes.
func TestTheDeclarationIsTheCOne(t *testing.T) {
	ir, err := compileForTest(`
		extern "C" fn nilffi_double(value: Int) -> Int;
		nilffi_double(21);
	`)
	if err != nil {
		t.Fatalf("emitting failed: %v", err)
	}

	if !strings.Contains(ir, "declare i64 @nilffi_double(i64)") {
		t.Errorf("the declaration is not the C signature:\n%s", ir)
	}
	// The call converts on the way out and the result on the way in. Both are
	// visible in the text, which is what makes a wrong conversion findable without
	// running the program.
	if !strings.Contains(ir, "call i64 @nil_int_unbox(i64 ") {
		t.Errorf("the argument is not converted on the way out:\n%s", ir)
	}
	if !strings.Contains(ir, "call i64 @nil_int_box(i64 ") {
		t.Errorf("the result is not converted on the way in:\n%s", ir)
	}
}
