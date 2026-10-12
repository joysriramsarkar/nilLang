package ffi

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/llvm"
	"github.com/joysriramsarkar/nilLang/compiler/mir"
)

// refusedByCompile returns the error the *emitter* produces, or "" when the program
// compiles.
//
// # Why the emitter and not the checker
//
// Because a struct crossing is a question about C, not about the language. `struct
// Point { x: Int }` is a perfectly good Nilang struct whether or not C will take it,
// and refusing it in the checker would mean a program could not *hold* a struct it
// has every right to hold. The question "can this go to C" is asked when the program
// asks to send it.
//
// So a test that checked `frontend.Check` would have been checking that the
// program is refused for being a struct — and it passed for the wrong reason, on a
// program the checker rejected for an unrelated reason.
func refusedByCompile(t *testing.T, source string) string {
	t.Helper()

	checked := frontend.Check(source, frontend.Options{})
	if !checked.OK {
		t.Fatalf("the program was rejected before the emitter ever saw it:\n%s", checked.ErrSummary())
	}
	optimized := hir.NewOptimizer().Optimize(hir.NewLowerer().LowerProgram(checked.Lowered))
	program := mir.NewLowerer().LowerHIR(optimized)

	_, err := llvm.NewEmitter(llvm.TargetHost).Emit(program)
	if err == nil {
		return ""
	}
	return err.Error()
}

// L5.7: a struct crosses into C as a pointer to a buffer the caller fills.
//
// The gate is the same one every FFI test uses and the one that has caught
// everything: write a C file against the *generated* header, compile it with every
// warning an error, link it against the program, and run it.
//
// What is new here is what the offsets have to be. Every other crossing converts one
// value and the two halves are agreed by construction. A struct is several values at
// offsets, and a wrong offset produces a program that runs and answers with a number
// assembled out of the wrong bytes.

// TestAStructCrossesWithTheOffsetsCComputes is the gate.
//
// `distance` reads three integers out of a struct it did not write and adds them.
// A wrong offset is not a wrong sum, it is a *plausible* wrong sum, which is why the
// fields are distinct rather than equal — a layout that shifted everything by eight
// still produces a number.
func TestAStructCrossesWithTheOffsetsCComputes(t *testing.T) {
	source := `
		struct Point { x: Int, y: Int, z: Int }
		extern "C" fn distance(p: Point) -> Int;

		let p = Point { x: 100, y: 20, z: 3 };
		distance(p);
	`
	got := runAgainstGeneratedHeader(t, source, "point", `
#include "point.h"
int64_t distance(const Point_t *p) { return p->x + p->y + p->z; }
`, "123")
	if !strings.Contains(got, "123") {
		t.Errorf("C read the fields at the wrong offsets: printed %q, wanted 123", strings.TrimSpace(got))
	}
}

// TestAStringFieldCrossesInsideAStruct is the two shapes at once.
//
// A struct field is not always one word. A `String` is a pointer and a length — two
// C members — and a crossing that handled the struct but not the field inside it
// would store an unboxed tagged word where C expects a pointer, and C would read the
// pointer's low bits as a length.
func TestAStringFieldCrossesInsideAStruct(t *testing.T) {
	source := `
		struct Named { label: String, count: Int }
		extern "C" fn label_length(n: Named) -> Int;

		let n = Named { label: "measuring", count: 9 };
		label_length(n);
	`
	// "measuring" is nine bytes. A length read from a tagged word rather than a
	// pointer would be some other number entirely, and `count` at the next offset
	// would land on the padding if the layout disagreed.
	got := runAgainstGeneratedHeader(t, source, "named", `
#include "named.h"
int64_t label_length(const Named_t *n) { return (int64_t)n->label_len; }
`, "9")
	if !strings.Contains(got, "9") {
		t.Errorf("a string field inside a struct crossed wrongly: printed %q, wanted 9", strings.TrimSpace(got))
	}
}

// TestThePaddingIsWhereCWouldPutIt is the case that a naive layout gets wrong.
//
// A `Byte` after an `Int` leaves seven bytes of padding, and a struct whose size is
// not rounded up to its alignment puts the *next* field somewhere a C compiler
// would never put it. Three bytes and three integers in one struct make both
// mistakes visible at once.
func TestThePaddingIsWhereCWouldPutIt(t *testing.T) {
	source := `
		struct Sparse { a: Bool, x: Int, b: Bool, y: Int }
		extern "C" fn spread(s: Sparse) -> Int;

		let s = Sparse { a: true, x: 100, b: false, y: 200 };
		spread(s);
	`
	got := runAgainstGeneratedHeader(t, source, "sparse", `
#include "sparse.h"
int64_t spread(const Sparse_t *s) {
  return (int64_t)s->a * 1000000 + s->x * 1000 + (int64_t)s->b * 100 + s->y;
}
`, "1100200")
	if !strings.Contains(got, "1100200") {
		t.Errorf("the padding is not where C puts it: printed %q, wanted 1100200", strings.TrimSpace(got))
	}
}

// TestAStructIsFreedWhenTheCallReturns keeps the buffer honest.
//
// The struct's bytes come from the call's scope, which is freed when the call is
// done. A loop is where that shows: an `alloca` at the call site would grow the
// stack by the struct's size on every iteration, and a program that loops long
// enough would die of a stack overflow rather than of anything anyone could read off
// the crash.
func TestAStructIsFreedWhenTheCallReturns(t *testing.T) {
	source := `
		struct Point { x: Int, y: Int }
		extern "C" fn total(p: Point) -> Int;
		extern "C" fn nil_ffi_buffer_outstanding() -> Int;

		let sum = 0;
		let i = 0;
		while i < 5000 {
			sum = sum + total(Point { x: i, y: 1 });
			i = i + 1;
		}
		nil_ffi_buffer_outstanding() * 1000000 + sum;
	`
	got := runAgainstGeneratedHeader(t, source, "loop", `
#include "loop.h"
int64_t total(const Point_t *p) { return p->x + p->y; }
`, "12502500")
	// Nothing outstanding, and the sum of i+1 for i in 0..4999 — which is
	// 5000*5001/2, not what it looks like at a glance.
	if !strings.Contains(got, "12502500") {
		t.Errorf("a struct buffer outlived its call: printed %q, wanted 12502500", strings.TrimSpace(got))
	}
}

// TestTheGeneratedHeaderDeclaresTheStructItNeeds is the header's half.
//
// A prototype saying `const Point_t *` and no declaration of `Point_t` is a header
// that does not compile — and it is what a generator emits if it writes prototypes
// and forgets the types, which is the obvious way to add structs to a generator that
// previously had no types to declare.
func TestTheGeneratedHeaderDeclaresTheStructItNeeds(t *testing.T) {
	source := `
		struct Point { x: Int, y: Int }
		extern "C" fn measure(p: Point) -> Int;
		measure(Point { x: 1, y: 2 });
	`
	runAgainstGeneratedHeader(t, source, "declares", `
#include "declares.h"
int64_t measure(const Point_t *p) { return p->x + p->y; }
`, "3")

	header := lastGeneratedHeader(t)
	for _, want := range []string{
		"typedef struct Point_t {",
		"int64_t x;",
		"int64_t y;",
		"} Point_t;",
		"int64_t measure(const Point_t *p);",
	} {
		if !strings.Contains(header, want) {
			t.Errorf("the header does not declare %q:\n%s", want, header)
		}
	}
}

// TestAStringFieldIsTwoMembers is what a C author has to be told.
//
// One field, two C members, and the `_len` suffix is a name the reader has to guess
// otherwise. A prototype whose struct has a member the header never mentions is a
// header C cannot compile.
func TestAStringFieldIsTwoMembers(t *testing.T) {
	source := `
		struct Named { label: String }
		extern "C" fn label(n: Named) -> Int;
		label(Named { label: "x" });
	`
	runAgainstGeneratedHeader(t, source, "members", `
#include "members.h"
int64_t label(const Named_t *n) { return (int64_t)n->label_len; }
`, "1")

	header := lastGeneratedHeader(t)
	for _, want := range []string{"const char *label;", "size_t label_len;"} {
		if !strings.Contains(header, want) {
			t.Errorf("the header does not declare %q:\n%s", want, header)
		}
	}
}

// TestAStructWithAFieldThatCannotCrossIsRefused keeps the hole from forming.
//
// `Point { x: Int, items: List<Int> }` has two fields and one of them has no
// conversion. Laying out the two that do would leave a hole where the list belongs,
// and C would read the following field's bytes as its length — the same shape of
// mistake as a struct returned by value, reached by a different road.
func TestAStructWithAFieldThatCannotCrossIsRefused(t *testing.T) {
	source := `
		struct Point { x: Int, items: List<Int> }
		extern "C" fn measure(p: Point) -> Int;
		measure(Point { x: 1, items: [2] });
	`

	refusal := refusedByCompile(t, source)
	if refusal == "" {
		t.Fatal("a struct with a list in it was accepted")
	}
}

// TestAStructCannotComeBackYet is the direction that is not implemented.
//
// A struct *parameter* is a buffer the caller fills, and the runtime can fill one. A
// struct *return* would need the runtime to build a Nilang struct out of C's bytes,
// and nothing does that. Answering with a pointer would hand the program a tagged
// word pointing at memory it does not own.
func TestAStructCannotComeBackYet(t *testing.T) {
	source := `
		struct Point { x: Int, y: Int }
		extern "C" fn origin() -> Point;
		origin();
	`

	refusal := refusedByCompile(t, source)
	if refusal == "" {
		t.Fatal("a struct return was accepted")
	}
	// It has to say which direction is missing, or the reader thinks structs simply
	// do not cross.
	if !strings.Contains(refusal, "cannot cross back") {
		t.Errorf("the refusal does not say the direction is missing:\n%v", refusal)
	}
	if !strings.Contains(refusal, "one direction only") {
		t.Errorf("the refusal does not say that going in works:\n%v", refusal)
	}
}

// TestAnUnknownTypeIsStillRefusedByName is the ordinary case, unchanged.
//
// Adding struct resolution must not have made anything else permissive — a name that
// is neither a built-in nor a struct is still refused, and still names itself.
func TestAnUnknownTypeIsStillRefusedByName(t *testing.T) {
	source := `
		extern "C" fn sort(xs: List<Int>) -> Int;
		sort([1]);
	`

	refusal := refusedByCompile(t, source)
	if refusal == "" {
		t.Fatal("a list parameter was accepted")
	}
	if !strings.Contains(refusal, "List<Int>") {
		t.Errorf("the refusal does not name the type:\n%v", refusal)
	}
}
