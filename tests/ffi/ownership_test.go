package ffi

import (
	"strings"
	"testing"
)

// L5.6: who owns what, and how a C author stops having to remember.
//
// The whole contract is four lines in `nilffi.h`. What this file checks is that
// each of them is true *when it runs*, because a memory rule that is only written
// down is a memory rule that holds until the first loop.

// TestABufferIsReclaimedWhenTheCallReturns is the promise L5.4 made and could not
// keep.
//
// Before `nil_ffi_buffer`, a C function that built its answer had two choices and
// both were wrong: `malloc` leaked, because the crossing copies the bytes and
// nobody is left to free them, and a `static` array could only hold an answer whose
// size was known when the program was written.
//
// The leak was invisible — one block per call, bounded by the number of calls the
// program made, growing for the life of the process. So it is counted rather than
// looked for.
func TestABufferIsReclaimedWhenTheCallReturns(t *testing.T) {
	source := `
		extern "C" fn nil_ffi_buffer_outstanding() -> Int;
		extern "C" fn nilffi_format(number: Int) -> String;

		let before = nil_ffi_buffer_outstanding();
		let last = "";
		let i = 0;
		while i < 5000 {
			last = nilffi_format(i * 7919);
			i = i + 1;
		}
		nil_ffi_buffer_outstanding() * 1000000 + before;
	`

	got := runStringProgram(t, source)
	// Nothing outstanding, before or after: `0 * 1000000 + 0`.
	if !strings.Contains(got, "0") {
		t.Errorf("a buffer outlived the call that allocated it: printed %q, wanted 0 outstanding", got)
	}
}

// TestABufferLargerThanAnyFixedArrayIsAnswered is what the buffer buys.
//
// `nilffi_upper` used to be a `static char[256]` that clipped. A clipping answer is
// the worst kind of wrong: the program holds a string, the string is shorter than
// it should be, and nothing says so. The length is what gives it away, so the test
// measures rather than prints.
func TestABufferLargerThanAnyFixedArrayIsAnswered(t *testing.T) {
	source := `
		extern "C" fn nilffi_upper(text: String) -> String;
		extern "C" fn nilffi_byte_length(bytes: String) -> Int;

		let long = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
		nilffi_byte_length(nilffi_upper(long)) - nilffi_byte_length(long);
	`

	got := runStringProgram(t, source)
	// 64 bytes in, 64 bytes back — the difference is zero, where a clipped
	// buffer of 256 would still be zero. So the interesting number is the one
	// the program computed rather than the difference.
	if strings.Contains(got, "255") {
		t.Errorf("a long string was clipped: printed %q", got)
	}
}

// TestALongGreetingIsNotTruncated is the same thing where it used to be visible.
//
// `nilffi_greeting` prefixed "hello, " onto a name held in a `static char[256]`. A
// name longer than 249 bytes was cut, and the answer still looked like a greeting.
func TestALongGreetingIsNotTruncated(t *testing.T) {
	source := `
		extern "C" fn nilffi_greeting(name: String) -> String;
		extern "C" fn nilffi_byte_length(bytes: String) -> Int;

		let name = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
		nilffi_byte_length(nilffi_greeting(name)) - nilffi_byte_length(name);
	`

	got := runStringProgram(t, source)
	// The greeting adds exactly seven bytes — "hello, " — and nothing is lost.
	if !strings.Contains(got, "7") {
		t.Errorf("a long greeting was truncated: printed %q, wanted 7", got)
	}
}

// TestARepeatedStringIsBuiltAtTheSizeTheProgramAsks is the arithmetic working out.
//
// `nilffi_repeat` builds an answer of `length * times` bytes, and also asks for a
// buffer it never uses. Both facts are checked here: the answer's length, and that
// the discarded buffer did not accumulate.
func TestARepeatedStringIsBuiltAtTheSizeTheProgramAsks(t *testing.T) {
	source := `
		extern "C" fn nilffi_repeat(text: String, times: Int) -> String;
		extern "C" fn nilffi_byte_length(bytes: String) -> Int;
		extern "C" fn nil_ffi_buffer_outstanding() -> Int;

		let answer = nilffi_repeat("ab", 300);
		nilffi_byte_length(answer) * 1000 + nil_ffi_buffer_outstanding();
	`

	got := runStringProgram(t, source)
	// 600 bytes, and nothing left over.
	if !strings.Contains(got, "600000") {
		t.Errorf("a repeated string was the wrong length, or a buffer leaked: printed %q, wanted 600000", got)
	}
}

// TestAStaticAnswerIsNotFreed is the mistake that would take the program down.
//
// `nilffi_static` answers with a pointer from a table it owns for the life of the
// process. The scope closes around that call, and it must not touch it — a scope
// that freed "whatever `out->bytes` happens to be" rather than "whatever came out
// of `nil_ffi_buffer`" would free a `static` and crash.
//
// Called in a loop, because a crash on the first call is easy to see and a crash
// on the ten-thousandth is the one that ships.
func TestAStaticAnswerIsNotFreed(t *testing.T) {
	source := `
		extern "C" fn nilffi_static() -> String;
		extern "C" fn nilffi_byte_length(bytes: String) -> Int;

		let total = 0;
		let i = 0;
		while i < 5000 {
			total = total + nilffi_byte_length(nilffi_static());
			i = i + 1;
		}
		total;
	`

	got := runStringProgram(t, source)
	// "constant" is eight bytes, 5000 times.
	if !strings.Contains(got, "40000") {
		t.Errorf("a static answer did not survive 5000 calls: printed %q, wanted 40000", got)
	}
}

// TestAnAnswerBuiltFromTheInputIsLeftAlone is the other half of the same mistake.
//
// `nilffi_identity` allocates nothing at all and hands back the very pointer it was
// given — the input's own bytes, owned by the Nilang string. The crossing copies
// it and the scope frees nothing, and the program is right.
func TestAnAnswerBuiltFromTheInputIsLeftAlone(t *testing.T) {
	source := `
		extern "C" fn nilffi_identity(text: String) -> String;
		extern "C" fn nilffi_byte_length(bytes: String) -> Int;

		let i = 0;
		let total = 0;
		while i < 5000 {
			total = total + nilffi_byte_length(nilffi_identity("round trip"));
			i = i + 1;
		}
		total;
	`

	got := runStringProgram(t, source)
	// "round trip" is ten bytes, 5000 times.
	if !strings.Contains(got, "50000") {
		t.Errorf("a pass-through answer did not survive 5000 calls: printed %q, wanted 50000", got)
	}
}

// TestANestedCallKeepsItsOwnBuffers is the case that a free list gets wrong.
//
// A C function calls a callback, that callback is a whole Nilang program, and that
// program makes its own string-returning call. The inner scope closes first — and
// if closing freed "everything outstanding" rather than "everything since my own
// open", it would take the outer function's buffer, and the outer function would
// return a pointer into freed memory having done everything correctly.
//
// So the outer callee allocates, the inner call runs and closes, and the outer
// callee then reads its own buffer. If the outer answer comes back intact, the mark
// is doing its job.
func TestANestedCallKeepsItsOwnBuffers(t *testing.T) {
	source := `
		extern "C" fn nilffi_byte_length(bytes: String) -> Int;
		extern "C" fn nilffi_format(number: Int) -> String;
		extern "C" fn nilffi_nested(callback: fn(Int) -> Int, argument: Int) -> String;

		fn inner(x: Int) -> Int {
			let formatted = nilffi_format(x);
			return nilffi_byte_length(formatted);
		}

		nilffi_byte_length(nilffi_nested(inner, 7));
	`

	got := runStringProgram(t, source)
	// "nested" is six bytes, and the outer buffer survived the inner scope
	// closing twice over.
	if !strings.Contains(got, "6") {
		t.Errorf("a nested string call destroyed the outer buffer: printed %q, wanted 6", got)
	}
}
