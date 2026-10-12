package ffi

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/mir"
)

// L5.7's non-C half: what a struct annotation resolves to, and what it does not.
//
// These are the checks that run in a millisecond and say something, where the
// end-to-end tests in `tests/ffi` need clang, gcc and a link. The rule they exist for:
// **a refusal must happen here, by name.** A crossing that is discovered to be missing
// at the call site — or, worse, not discovered — is a wrong answer rather than a
// message.
//
// Every program below is built through the real front end rather than a hand-filled
// `mir.Program`. A map I wrote myself would agree with the layout code by
// construction, and the whole question is whether a *declaration* survives lowering.

// structCrossing resolves a struct annotation and fails the test if it does not.
func structCrossing(t *testing.T, source, annotation string) Crossing {
	t.Helper()

	crossing, err := ForIn(programWith(t, source), annotation)
	if err != nil {
		t.Fatalf("%s did not resolve: %v", annotation, err)
	}
	return crossing
}

// TestAStructResolvesToAPointerToItsOwnLayout is the whole of the crossing, in one
// assertion set.
//
// A struct crosses as `ptr` in LLVM and `const Name *` in C, and it carries the layout
// that says where each field goes. The third part is what the first two cannot express:
// `ptr` says only *something* is a pointer, and the C spelling says only that C will
// read one. The layout is why the buffer is the right size.
func TestAStructResolvesToAPointerToItsOwnLayout(t *testing.T) {
	crossing := structCrossing(t, `
		struct Point { x: Int, y: Int }
	`, "Point")

	if crossing.LLVM != "ptr" {
		t.Errorf("the LLVM crossing is %q, want %q", crossing.LLVM, "ptr")
	}
	if crossing.C != "const Point *" {
		t.Errorf("the C crossing is %q, want %q", crossing.C, "const Point *")
	}
	if crossing.Params != 1 {
		t.Errorf("a struct is one parameter, got %d", crossing.Params)
	}
	if crossing.Struct == nil {
		t.Fatal("the crossing carries no layout, so nothing can be stored at an offset")
	}
	if crossing.Size != 16 || crossing.Alignment != 8 {
		t.Errorf("two 64-bit fields are 16 bytes at 8-byte alignment, got %d at %d",
			crossing.Size, crossing.Alignment)
	}
	if len(crossing.Struct.Fields) != 2 {
		t.Fatalf("the layout has %d fields, want 2", len(crossing.Struct.Fields))
	}
	if crossing.Struct.Fields[1].Offset != 8 {
		t.Errorf("the second field is at %d, want 8", crossing.Struct.Fields[1].Offset)
	}
}

// TestAPaddingByteIsStillALaidOutField is the case a table of sizes would miss.
//
// Three `Byte`s are three bytes, aligned at one, and a C `struct` of them is three
// bytes too. Nothing rounds it up, because there is no eight-byte field asking for
// it. A layout that assumed every struct ended at an eight-byte boundary would say
// eight, and the next field of a containing struct would land eight bytes late.
func TestAPaddingByteIsStillALaidOutField(t *testing.T) {
	crossing := structCrossing(t, `
		struct Triple { a: Byte, b: Byte, c: Byte }
	`, "Triple")

	if crossing.Size != 3 {
		t.Errorf("three bytes are three bytes, got %d", crossing.Size)
	}
	if crossing.Alignment != 1 {
		t.Errorf("a struct of bytes is aligned at one, got %d", crossing.Alignment)
	}
	if crossing.Struct.Fields[2].Offset != 2 {
		t.Errorf("the third byte is at %d, want 2", crossing.Struct.Fields[2].Offset)
	}
}

// TestABuiltInIsNeverShadowedByADeclaration is the ordering rule, tested.
//
// `ForIn` looks at the built-ins first. That order is load-bearing: a program that
// declared `struct Int { ... }` would otherwise have two different crossings for one
// name, and the difference is invisible — both are plausible, and only the bytes at
// the far end of a buffer tell the story.
//
// So the declaration is ignored for a built-in name, deliberately.
func TestABuiltInIsNeverShadowedByADeclaration(t *testing.T) {
	crossing := structCrossing(t, `
		struct Int { lo: Int, hi: Int }
	`, "Int")

	if crossing.Struct != nil {
		t.Error("a declaration shadowed a built-in, so one name now crosses two ways")
	}
	if crossing.Size != 8 {
		t.Errorf("the built-in Int is 8 bytes, got %d", crossing.Size)
	}
	if crossing.LLVM != "i64" {
		t.Errorf("the built-in Int crosses as i64, got %q", crossing.LLVM)
	}
}

// TestAnEnumIsNotAStruct says which way round the mistake goes.
//
// An enum's *payload* type crosses, so an enum name has no crossing of its own. The
// refusal has to say so — "neither a built-in nor a struct" would send a reader
// looking for a struct declaration that was never missing.
func TestAnEnumIsNotAStruct(t *testing.T) {
	_, err := ForIn(programWith(t, `
		enum Colour { Red, Green, Blue }
	`), "Colour")
	if err == nil {
		t.Fatal("an enum was given a crossing")
	}
	if !strings.Contains(err.Error(), "enum") {
		t.Errorf("the refusal does not say what kind of thing it was:\n%v", err)
	}
}

// TestAFieldWithNoCrossingIsRefusedByItsOwnName is the case a struct must not paper
// over.
//
// `struct Point { x: Int, items: List<Int> }` has two fields and one of them has no
// conversion. Laying out the field that does work would leave a hole where the list
// belongs, and C would read the following bytes as a length — the same shape of
// mistake as a struct returned by value, reached by a different road.
//
// So the refusal names the *field*: "Point cannot cross" sends a reader to the
// declaration, and "Point's field items is a List<Int>" sends them to the line.
func TestAFieldWithNoCrossingIsRefusedByItsOwnName(t *testing.T) {
	_, err := ForIn(programWith(t, `
		struct Point { x: Int, items: List<Int> }
	`), "Point")
	if err == nil {
		t.Fatal("a struct with a field that cannot cross was given a crossing")
	}
	message := err.Error()
	if !strings.Contains(message, "items") {
		t.Errorf("the refusal does not name the field at fault:\n%v", err)
	}
	if !strings.Contains(message, "List<Int>") {
		t.Errorf("the refusal does not name the field's type:\n%v", err)
	}
}

// TestAStringFieldCarriesBothHalves is the shape that is two words in one field.
//
// A `String` is a pointer and a length. Laying it out as one word would put the
// length where C reads the pointer's low bits — a plausible number, wrong, and only
// on the second crossing.
func TestAStringFieldCarriesBothHalves(t *testing.T) {
	crossing := structCrossing(t, `
		struct Named { label: String, count: Int }
	`, "Named")

	if crossing.Size != 24 {
		t.Errorf("a pointer, a length and an integer are 24 bytes, got %d", crossing.Size)
	}

	label := crossing.Struct.Fields[0]
	if !label.Crossing.Splat {
		t.Error("a string field is not marked as splatted, so it would be stored as one word")
	}
	if label.Offset != 0 {
		t.Errorf("the string field is at %d, want 0", label.Offset)
	}
	// The integer must land *after both halves*, not between them.
	if crossing.Struct.Fields[1].Offset != 16 {
		t.Errorf("the field after a string is at %d, want 16", crossing.Struct.Fields[1].Offset)
	}
}

// TestTwoLookupsOfOneStructAgree is about the pointer in `Crossing`.
//
// `Struct` is a `*Layout` rather than a `Layout`, for two reasons that have nothing to
// do with each other: a nil is how "this crossing is not a struct" is expressed, and a
// pointer keeps `Crossing` cheap to copy, since it is copied into a slice, a map
// lookup and a parameter list on every foreign call.
//
// It does **not** mean one layout per type. `ForIn` computes a fresh one per lookup, so
// a signature with two `Point` parameters carries two equal-but-distinct layouts.
//
// That is safe because nothing writes to a layout after `LayoutOf` returns it, and the
// test that would break if that ever stopped being true is this one: it compares the
// two field by field rather than comparing pointers. A pointer comparison would pass
// today and mean nothing.
//
// So the assertion is on the *contents*, which is the thing the emitter reads.
func TestTwoLookupsOfOneStructAgree(t *testing.T) {
	program := programWith(t, `
		struct Point { x: Int, y: Int }
	`)

	first, err := ForIn(program, "Point")
	if err != nil {
		t.Fatal(err)
	}
	second, err := ForIn(program, "Point")
	if err != nil {
		t.Fatal(err)
	}
	// Compared field by field: `Layout` holds a slice, so `==` is not available on
	// it, and a `reflect.DeepEqual` would hide *which* field disagreed.
	if first.Struct.Name != second.Struct.Name ||
		first.Struct.Size != second.Struct.Size ||
		first.Struct.Alignment != second.Struct.Alignment ||
		len(first.Struct.Fields) != len(second.Struct.Fields) {
		t.Fatalf("two lookups of one struct produced different layouts:\n%+v\n%+v",
			first.Struct, second.Struct)
	}
	for index, field := range first.Struct.Fields {
		other := second.Struct.Fields[index]
		if field.Name != other.Name || field.Offset != other.Offset {
			t.Errorf("field %d differs: %+v then %+v", index, field, other)
		}
		if field.Crossing.LLVM != other.Crossing.LLVM || field.Crossing.C != other.Crossing.C {
			t.Errorf("field %q crosses two ways: %q then %q",
				field.Name, field.Crossing.LLVM, other.Crossing.LLVM)
		}
		if field.Crossing.Splat != other.Crossing.Splat {
			t.Errorf("field %q is splatted in one layout and not the other", field.Name)
		}
	}
	if first.Size != second.Size || first.Alignment != second.Alignment {
		t.Errorf("the sizes disagree: %d/%d then %d/%d",
			first.Size, first.Alignment, second.Size, second.Alignment)
	}
}

// TestAStructReturnIsRefusedAndAStructParameterIsNot is the direction, both ways.
//
// Kept as one test because the risk is symmetry: someone reading one refusal and
// generalising it would refuse the direction that works, and a refusal of a working
// direction is worse than a missing feature — it tells a programmer the language
// cannot do a thing it does.
func TestAStructReturnIsRefusedAndAStructParameterIsNot(t *testing.T) {
	program := programWith(t, `
		struct Point { x: Int, y: Int }
	`)

	in := &mir.Function{Name: "distance", ParamTypes: []string{"Point"}, ReturnType: "Int"}
	if _, err := SignatureFor(program, in); err != nil {
		t.Errorf("a struct parameter was refused, and it is implemented: %v", err)
	}

	out := &mir.Function{Name: "origin", ReturnType: "Point"}
	_, err := SignatureFor(program, out)
	if err == nil {
		t.Fatal("a struct return was accepted")
	}
	// Both halves, because "structs do not cross" would be a lie a reader acts on.
	for _, want := range []string{"cannot cross back", "one direction only", "origin", "Point"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q:\n%v", want, err)
		}
	}
}

// TestAnUnknownTypeStillListsWhatIsAvailable keeps the older refusal from regressing.
//
// Struct resolution was added after the original message. It must not have made the
// message vaguer: "no crossing for `Array<Int>`" without the list leaves the reader to
// guess what they *could* have written.
func TestAnUnknownTypeStillListsWhatIsAvailable(t *testing.T) {
	_, err := ForIn(mir.NewProgram(), "Array<Int>")
	if err == nil {
		t.Fatal("a list was given a crossing")
	}
	message := err.Error()
	if !strings.Contains(message, "Array<Int>") {
		t.Errorf("the refusal does not name the type:\n%v", err)
	}
	if !strings.Contains(message, "defined crossings:") {
		t.Errorf("the refusal does not say what is available:\n%v", err)
	}
}
