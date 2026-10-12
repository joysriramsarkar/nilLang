package llvm

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ffi"
)

// The FFI's surface is a contract, and this is where it is written down.
//
// Every other test here asks whether something works today. This asks whether
// *tomorrow* — someone adds a function, changes a signature, changes what a type
// crosses as, changes who owns what — and a program built yesterday still does the
// right thing.
//
// It cannot: a compiled object holds the numbers and the layout it was built
// against. What it *can* do is fail loudly at build time instead of quietly at run
// time, and that is what the version guard buys. This file is the other half: it
// makes changing the surface a deliberate act rather than an accident.

// ffiSurfaceIsFrozen is the whole contract, as a list.
//
// A list, and a frozen one, on purpose. "Something changed" is the message every
// consumer of a shared library wants, and it has to arrive before the change ships
// rather than after somebody has linked against it. A test that merely checked
// "every declared symbol is implemented" would pass forever and say nothing about a
// function being *added* — which is the change that breaks a program most quietly,
// because adding is the one change that cannot fail to link.
//
// Bumping the version is what makes this test pass again. That is the point: the
// person deciding to change the boundary is the person deciding to break the
// programs built against the old one, and they should have to write that down.
// Sorted, because the order of a list is not a fact and an unsorted one produces a
// diff on every reformat. Read by eye rather than generated: the point of writing it
// out is that changing it is a deliberate act, and a list a program regenerates is a
// list nobody has decided anything about.
const ffiSurfaceIsFrozen = `nil_bool_box
nil_bool_unbox
nil_byte_box
nil_byte_unbox
nil_callback_arity
nil_callback_invoke
nil_callback_new
nil_callback_release
nil_callback_trampoline
nil_channel_close
nil_channel_new
nil_channel_outstanding
nil_channel_post
nil_channel_serve
nil_channel_take_reply
nil_decode
nil_encode
nil_encoded_size
nil_ffi_abi_v2
nil_ffi_alignof
nil_ffi_buffer
nil_ffi_buffer_outstanding
nil_ffi_can_cross
nil_ffi_scope_close
nil_ffi_scope_open
nil_ffi_sizeof
nil_ffi_version
nil_float_box
nil_float_unbox
nil_int_box
nil_int_unbox
nil_string_copy
nil_string_unbox`

// TestTheFFISurfaceIsWhatWasPromised is the contract.
//
// Every name the frozen list names must exist in `nilffi.h`, and every name
// `nilffi.h` exports must be in the list. Both directions, because they fail
// differently: a name in the list but not in the header is a promise the runtime
// does not keep, and a name in the header but not the list is a promise nobody
// made.
func TestTheFFISurfaceIsWhatWasPromised(t *testing.T) {
	declared := ffiSurfaceSymbols(t)
	promised := map[string]bool{}
	for _, name := range strings.Split(strings.TrimSpace(ffiSurfaceIsFrozen), "\n") {
		promised[strings.TrimSpace(name)] = true
	}

	var missing, unexpected []string
	for name := range promised {
		if !declared[name] {
			missing = append(missing, name)
		}
	}
	for name := range declared {
		if !promised[name] {
			unexpected = append(unexpected, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(unexpected)

	for _, name := range missing {
		t.Errorf("%s is promised by the FFI's contract but nilffi.h does not declare it", name)
	}
	for _, name := range unexpected {
		t.Errorf("nilffi.h exports %s, which the FFI's contract does not mention.\n"+
			"Adding a function is a change to the boundary: every program compiled against "+
			"the old header was built against a contract this is no longer describing.\n"+
			"If that is intended, add it to ffiSurfaceIsFrozen and raise "+
			"NIL_FFI_ABI_VERSION in nilffi.h.", name)
	}

	if len(missing) == 0 && len(unexpected) == 0 {
		return
	}
	t.Errorf("the FFI's surface changed. If that is intended, update ffiSurfaceIsFrozen " +
		"in this file and NIL_FFI_ABI_VERSION in nilffi.h together — the version is what " +
		"turns a compiled object that disagrees into a build failure rather than a " +
		"program that answers wrongly")
}

// TestTheFrozenSurfaceIsNotEmpty guards the guard.
//
// A test that passes because a list is empty is worse than no test: it reports
// "the contract holds" about nothing. Two entries is the smallest list that could
// still be an accident.
func TestTheFrozenSurfaceIsNotEmpty(t *testing.T) {
	names := strings.Fields(ffiSurfaceIsFrozen)
	if len(names) < 2 {
		t.Fatalf("the frozen FFI surface lists %d names, which cannot be a contract", len(names))
	}
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			t.Errorf("the frozen FFI surface lists %q twice", name)
		}
		seen[name] = true
	}
}

// TestEveryCrossingTheEmitterUsesIsOnTheFrozenSurface ties the two halves together.
//
// The emitter declares the symbols a module calls. The frozen list is what the
// runtime promises to export. A symbol the emitter emits but the contract does not
// name is a symbol whose addition nobody noticed — and the one test that could catch
// it is this one, because it compares the two lists rather than either alone.
func TestEveryCrossingTheEmitterUsesIsOnTheFrozenSurface(t *testing.T) {
	promised := map[string]bool{}
	for _, name := range strings.Fields(ffiSurfaceIsFrozen) {
		promised[name] = true
	}

	// The conversions the crossing table can produce, taken from the table rather
	// than from a second list.
	for name, crossing := range ffi.Types {
		for _, symbol := range []string{crossing.Unbox, crossing.Box} {
			if symbol == "" {
				continue
			}
			if !promised[symbol] {
				t.Errorf("the %s crossing calls %s, which the FFI's contract does not "+
					"promise to export; a module using it would link or fail depending "+
					"on a list nobody wrote down", name, symbol)
			}
		}
	}
}

// ffiSurfaceSymbols is every `nil_*` function `nilffi.h` declares.
//
// Read with the same parser the ABI tests use, rather than a second reader: a
// second reader is a second opinion about what the header says, and this file's
// whole subject is two readers disagreeing quietly.
func ffiSurfaceSymbols(t *testing.T) map[string]bool {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot find this test's own path")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "runtime", "ffi", "include", "nilffi.h")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	names := map[string]bool{}
	for _, match := range cDeclaration.FindAllStringSubmatch(string(raw), -1) {
		names[match[2]] = true
	}
	if len(names) == 0 {
		t.Fatal("no declarations were read out of nilffi.h; the pattern is wrong")
	}

	// The same check the ABI tests make, for the same reason: a parser that skips a
	// form makes a symbol look absent rather than unread, and "absent" here reads
	// as "was never promised".
	for _, match := range nilDeclarationStart.FindAllStringSubmatch(string(raw), -1) {
		if !names[match[1]] {
			t.Errorf("nilffi.h declares %s in a form the header parser does not understand, "+
				"so the frozen surface cannot see it", match[1])
		}
	}
	return names
}

// TestEverySymbolIsPrefixed says the naming convention still holds at the surface.
//
// `nil_` for the runtime's own functions, and nothing else. A name that came to
// contain a dot would need an `__asm__` label in the C file, and `__asm__` does not
// exist in MSVC's C compiler — so the mistake costs portability before it costs
// correctness.
func TestEverySymbolIsPrefixed(t *testing.T) {
	for name := range ffiSurfaceSymbols(t) {
		if !strings.HasPrefix(name, "nil_") {
			t.Errorf("%s does not start with nil_", name)
		}
		if strings.Contains(name, ".") {
			t.Errorf("%s contains a dot, which no C symbol may", name)
		}
	}
}

// TestTheVersionGuardIsOnTheSurface is the one symbol the whole contract turns on.
//
// If the guard is not exported, a mismatched object fails to link — which is the
// good outcome, but only if the symbol is genuinely there. A guard the contract
// forgot to name is a guard nobody has promised to keep.
func TestTheVersionGuardIsOnTheSurface(t *testing.T) {
	// Matched by shape rather than written out as `nil_ffi_abi_v1`.
	//
	// The literal was correct until the channel work raised the ABI to 2, and then it
	// was a test that fails whenever the version is raised — including on the raise
	// that was the whole point. The property being checked is "a guard whose name
	// carries a version is on the surface", and a regexp is that property; a string
	// is one version of it.
	guard := regexp.MustCompile(`nil_ffi_abi_v(\d+)`)
	if !guard.MatchString(ffiSurfaceIsFrozen) {
		t.Error("the ABI guard is not on the frozen surface; a runtime could drop it and " +
			"every version check would stop happening at build time")
	}

	// And the guard the surface promises must be the guard the emitter writes. Those
	// are two constants in two places, which is one more place to forget — and it is
	// the same forgetting `TestTheVersionInTheGuardIsTheVersionInTheHeader` catches
	// from the other direction, so both are asked.
	if match := guard.FindStringSubmatch(ffiSurfaceIsFrozen); match != nil {
		if match[0] != ffiABIGuard {
			t.Errorf("the frozen surface promises %s but the emitter writes %s; "+
				"a module built by this compiler would not link against a runtime "+
				"that keeps the promise", match[0], ffiABIGuard)
		}
	}
}
