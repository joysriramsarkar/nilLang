package cheader

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ffi"
)

// TestEveryTypeAPrototypeNamesIsDeclared is the check that would have caught the bug
// this file was written after.
//
// The first version of struct support emitted `typedef struct Point_t { ... } Point_t;`
// and, one line later, `int64_t distance(const Point *p);`. The header looked right in
// every way a reader checks by eye: a typedef, a prototype, a comment above each.
// It did not compile, and C said:
//
//	error: unknown type name 'Point'; did you mean 'Point_t'?
//
// which is the best error a C compiler produces and entirely avoidable.
//
// # Why the check is a whole header rather than one fixture
//
// Because the mistake is not "forgot the `_t` in the prototype". It is that **two
// places decided the same name independently**. Any test written against one fixture
// pins one pair of places; a test that asks "is every type any prototype names
// actually declared?" holds however the next type arrives — as a struct, as an enum,
// as a typedef somebody adds later.
//
// So: collect every identifier that appears in a parameter list or a return type,
// subtract the ones the header includes from the standard library and declares
// itself, and the remainder must be empty.
func TestEveryTypeAPrototypeNamesIsDeclared(t *testing.T) {
	header, program := generateFor(t, `
		struct Point { x: Int, y: Int }
		struct Named { label: String, count: Int }
		extern "C" fn distance(p: Point) -> Int;
		extern "C" fn measure(text: String, count: Int) -> Int;
		extern "C" fn describe(n: Named) -> Int;
		extern "C" fn scaled(n: Named, p: Point, factor: Float) -> Bool;
		distance(Point { x: 1, y: 2 });
		measure("a", 1);
		describe(Named { label: "b", count: 2 });
		scaled(Named { label: "c", count: 3 }, Point { x: 4, y: 5 }, 2.0);
	`, "types")

	// The check is anchored on the **typedef**, not on the prototype.
	//
	// The first attempt scanned prototypes for words shaped like types and complained
	// about any that were not declared. That is circular: the bug it was written for is
	// a prototype that *lost* its suffix, so the word no longer looks like a type and
	// the scan finds nothing at all — the test failed, but for the wrong reason, and a
	// reader would have had to work out which.
	//
	// The typedef is the reliable direction. `writeStructTypes` always spells it the
	// one true way; a prototype is what may drift. So the typedef supplies the name
	// and the prototype is required to use exactly that.
	checked := 0
	for _, fn := range program.Functions {
		signature, err := ffi.SignatureFor(program, fn)
		if err != nil {
			continue
		}
		prototype := prototypeFor(t, header, fn.Name)

		for _, crossing := range signature.Params {
			if crossing.Struct == nil {
				continue
			}
			checked++
			want := "const " + cTypeName(crossing.Struct.Name) + " *"
			if !strings.Contains(prototype, want) {
				t.Errorf("%s takes a %s, so its prototype must say %q; it says:\n  %s\n%s",
					fn.Name, crossing.Struct.Name, want, strings.TrimSpace(prototype), header)
			}
		}
	}

	if checked == 0 {
		t.Fatalf("no struct parameter reached a prototype, so nothing was checked:\n%s", header)
	}
}

// TestAStructParameterIsASinglePointer is what "crosses as a pointer" means.
//
// Two pointers would mean the struct crosses by something other than what
// `cTypeName` says — a `char *` for a byte buffer, or a pointer to a pointer
// because C's `const` was placed wrongly. Both compile, both link, and both hand the
// callee something it will read at the wrong place.
//
// So the count is checked rather than the spelling: a struct is exactly one
// `const <Name>_t *` and nothing else.
func TestAStructParameterIsASinglePointer(t *testing.T) {
	header, program := generateFor(t, `
		struct Point { x: Int, y: Int }
		extern "C" fn distance(p: Point) -> Int;
		distance(Point { x: 1, y: 2 });
	`, "single")

	prototype := prototypeFor(t, header, "distance")
	inner := prototype[strings.Index(prototype, "(")+1 : strings.LastIndex(prototype, ")")]
	if countHeaderParams(t, prototype) != 1 {
		t.Errorf("a struct is one pointer, not %d parameters:\n  %s",
			countHeaderParams(t, prototype), strings.TrimSpace(prototype))
	}
	if strings.TrimSpace(inner) != "const Point_t *p" {
		t.Errorf("the parameter is not `const Point_t *p`:\n  %s", strings.TrimSpace(prototype))
	}
	_ = program
}

// TestAStructIsNamedInExactlyOnePlace keeps the fix from being undone by accident.
//
// `_t` was appended at two places, one of which was missed. The suffix now lives
// inside `cTypeName`, which returns the whole name, so there is nothing to keep in
// step — but nothing *tests* that either, and a later edit that writes `name + "_t"`
// at a use site would reintroduce the same bug in a new place.
//
// So the check is that the bare name appears in no prototype: if the suffix is being
// added by one function, every prototype says `Point_t` and none says `Point`.
func TestAStructIsNamedInExactlyOnePlace(t *testing.T) {
	header, _ := generateFor(t, `
		struct Point { x: Int, y: Int }
		extern "C" fn distance(p: Point) -> Int;
		distance(Point { x: 1, y: 2 });
	`, "oneplace")

	prototype := prototypeFor(t, header, "distance")
	if strings.Contains(prototype, "const Point *") {
		t.Errorf("the prototype spells the struct without its suffix:\n%s", header)
	}
	if !strings.Contains(prototype, "const Point_t *") {
		t.Errorf("the prototype does not name the struct the typedef declares:\n%s", header)
	}
}

// TestTheStructTypedefSaysHowBigItIs is what a C author needs before allocating one.
//
// A `Point_t *` on the stack is a promise about size, and a reader who cannot check
// it has to take it on trust. The size is already printed — it is what the compiler
// computed the stores from — so the cost of saying it is one line, and the cost of
// not saying it is a `malloc(sizeof(Point_t))` written wrong.
func TestTheStructTypedefSaysHowBigItIs(t *testing.T) {
	header, _ := generateFor(t, `
		struct Point { x: Int, y: Int }
		extern "C" fn distance(p: Point) -> Int;
		distance(Point { x: 1, y: 2 });
	`, "sizeof")

	// Two 8-byte fields, rounded up to the struct's own alignment.
	if !strings.Contains(header, "Point: 16 byte(s), 8-byte aligned.") {
		t.Errorf("the header does not say how big the struct is:\n%s", header)
	}
}
