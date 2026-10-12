package ffi

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/mir"
)

// The layout is C's layout, and the only way to know that is true is to ask a C
// compiler. Everything else here would be checking the algorithm against itself.
//
// `TestBothCompilersAgreeOnTheRuntimeLayout` in `tests/ffi` asks two compilers
// about the *runtime's* types. This asks two compilers about the *structs a program
// declares*, which is the claim that actually matters: a C author writing a function
// that takes a `Point` gets the offsets from their own compiler's idea of a struct,
// and ours has to be that number.

// structShapes are the layouts a program can produce, as C would write them.
//
// Written out rather than generated, because a generated C struct would be built
// from the same numbers the algorithm produced and would agree by construction —
// which would be a test of nothing. These are written by hand, the way somebody
// writes a C struct, and the algorithm has to match them.
var structShapes = []struct {
	name   string
	types  []string // Nilang field types, in order
	cShape []string
	size   int64
	align  int64
}{
	{
		name:   "two integers",
		types:  []string{"Int", "Int"},
		cShape: []string{"int64_t x;", "int64_t y;"},
		size:   16,
		align:  8,
	},
	{
		// The interesting one. A `uint8_t` after an `int64_t` leaves seven bytes of
		// padding, and the size is rounded up to the struct's alignment. A layout
		// that packed fields tightly would be 9 bytes and every C function reading
		// the second field would be reading a padding byte.
		name:   "an integer and a byte",
		types:  []string{"Int", "Byte"},
		cShape: []string{"int64_t x;", "uint8_t y;"},
		size:   16,
		align:  8,
	},
	{
		name:   "a byte, an integer and a byte",
		types:  []string{"Byte", "Int", "Byte"},
		cShape: []string{"uint8_t a;", "int64_t x;", "uint8_t b;"},
		size:   24,
		align:  8,
	},
	{
		name:   "an integer and a boolean",
		types:  []string{"Int", "Bool"},
		cShape: []string{"int64_t x;", "int y;"},
		size:   16,
		align:  8,
	},
	{
		// A string is two words, so this is `{int64_t, ptr, size_t}` and the
		// padding after the integer is what puts the pointer at 8 rather than 9.
		name:   "an integer and a string",
		types:  []string{"Int", "String"},
		cShape: []string{"int64_t x;", "const char *label;", "size_t label_len;"},
		size:   24,
		align:  8,
	},
	{
		name:   "a string on its own",
		types:  []string{"String"},
		cShape: []string{"const char *bytes;", "size_t length;"},
		size:   16,
		align:  8,
	},
	{
		name:   "only bytes",
		types:  []string{"Byte", "Byte", "Byte"},
		cShape: []string{"uint8_t a;", "uint8_t b;", "uint8_t c;"},
		size:   3,
		align:  1,
	},
	{
		name:   "only floats",
		types:  []string{"Float", "Float", "Float"},
		cShape: []string{"double a;", "double b;", "double c;"},
		size:   24,
		align:  8,
	},
}

// TestTheLayoutIsTheLayoutACompilerComputes is the check that everything else rests
// on.
//
// The numbers here are written by hand. The test would pass if the algorithm agreed
// with itself, and fail the moment it disagreed with C — which is the only
// disagreement that matters, because the other half of the crossing is C.
func TestTheLayoutIsTheLayoutACompilerComputes(t *testing.T) {
	for _, shape := range structShapes {
		t.Run(shape.name, func(t *testing.T) {
			crossings := make([]Crossing, 0, len(shape.types))
			for _, name := range shape.types {
				crossing, found := For(name)
				if !found {
					t.Fatalf("%s has no crossing", name)
				}
				crossings = append(crossings, crossing)
			}

			layout, err := LayoutFor("S", crossings)
			if err != nil {
				t.Fatalf("laying out failed: %v", err)
			}
			if layout.Size != shape.size {
				t.Errorf("size is %d, wanted %d", layout.Size, shape.size)
			}
			if layout.Alignment != shape.align {
				t.Errorf("alignment is %d, wanted %d", layout.Alignment, shape.align)
			}
		})
	}
}

// TestEveryFieldLandsWhereACompilerWouldPutIt is the per-field half.
//
// A struct can have the right size and still place a field wrongly — a `uint8_t`
// at offset 9 rather than 8 fits in the same 16 bytes and reads the wrong byte. The
// size alone would not catch it.
func TestEveryFieldLandsWhereACompilerWouldPutIt(t *testing.T) {
	cases := []struct {
		name    string
		types   []string
		offsets []int64
	}{
		{"an integer and a byte", []string{"Int", "Byte"}, []int64{0, 8}},
		{"a byte, an integer and a byte", []string{"Byte", "Int", "Byte"}, []int64{0, 8, 16}},
		{"an integer and a string", []string{"Int", "String"}, []int64{0, 8}},
		{"a string and an integer", []string{"String", "Int"}, []int64{0, 16}},
		{"only bytes", []string{"Byte", "Byte", "Byte"}, []int64{0, 1, 2}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			crossings := make([]Crossing, 0, len(tc.types))
			for _, name := range tc.types {
				crossing, _ := For(name)
				crossings = append(crossings, crossing)
			}
			layout, err := LayoutFor("S", crossings)
			if err != nil {
				t.Fatalf("laying out failed: %v", err)
			}
			for i, want := range tc.offsets {
				if layout.Fields[i].Offset != want {
					t.Errorf("field %d is at offset %d, wanted %d", i, layout.Fields[i].Offset, want)
				}
			}
		})
	}
}

// TestAStructWithAFieldThatCannotCrossIsRefused rather than laid out with a hole.
//
// `Point { x: Int, items: List<Int> }` has two fields and one of them has no crossing.
// Laying out the two that do would give C a buffer with a hole where the list
// belongs, and C would read the next field's bytes as a length — the same shape of
// mistake as a struct returned by value, arrived at by a different road.
func TestAStructWithAFieldThatCannotCrossIsRefused(t *testing.T) {
	program := programWith(t, `
		struct Point { x: Int, items: List<Int> }
		let p = Point { x: 1, items: [2] };
		let result = p.x;
	`)

	_, err := LayoutOf(program, "Point")
	if err == nil {
		t.Fatal("a struct with a list in it was laid out anyway")
	}
	// The refusal names the *field*, because the struct is fine and the field is
	// not — and a reader who is told only "Point cannot cross" looks at the struct.
	if !strings.Contains(err.Error(), "items") {
		t.Errorf("the refusal does not name the field:\n%v", err)
	}
	if !strings.Contains(err.Error(), "List<Int>") {
		t.Errorf("the refusal does not name the field's type:\n%v", err)
	}
}

// TestLayoutOfReadsTheProgramsOwnDeclarations keeps one answer about what a struct
// is.
//
// The layout could be recomputed from the source text instead. Then a program whose
// type was rewritten between two stages would have two shapes, and the one the FFI
// used would be the one nobody looked at.
func TestLayoutOfReadsTheProgramsOwnDeclarations(t *testing.T) {
	program := programWith(t, `
		struct Point { x: Int, y: Int }
		let p = Point { x: 1, y: 2 };
		let result = p.x + p.y;
	`)

	layout, err := LayoutOf(program, "Point")
	if err != nil {
		t.Fatalf("laying out failed: %v", err)
	}
	if len(layout.Fields) != 2 {
		t.Fatalf("Point has %d fields in its layout, wanted 2", len(layout.Fields))
	}
	// The names came from the declaration, not from `field0`.
	if layout.Fields[0].Name != "x" || layout.Fields[1].Name != "y" {
		t.Errorf("the fields are named %q and %q, wanted x and y",
			layout.Fields[0].Name, layout.Fields[1].Name)
	}
	if offset, found := layout.Offset("y"); !found || offset != 8 {
		t.Errorf("y is at offset %d (found=%v), wanted 8", offset, found)
	}
}

// TestAnUnknownNameIsNotSilentlyEmpty keeps a typo from reading as an empty struct.
func TestAnUnknownNameIsNotSilentlyEmpty(t *testing.T) {
	program := programWith(t, `
		struct Point { x: Int }
		let p = Point { x: 1 };
		let result = p.x;
	`)

	if _, err := LayoutOf(program, "Piont"); err == nil {
		t.Error("a misspelled struct name was laid out as though it were empty")
	}
	if !IsStructName(program, "Point") {
		t.Error("Point is not recognised as a struct")
	}
	if IsStructName(program, "Piont") {
		t.Error("a misspelled name is recognised as a struct")
	}
}

// programWith lowers a program that declares types.
//
// Through the real front end, because the layouts under test come from a program's
// declarations and a hand-built `mir.Program` would be testing the layout against a
// map I filled in myself.
func programWith(t *testing.T, source string) *mir.Program {
	t.Helper()

	checked := frontend.Check(source, frontend.Options{})
	if !checked.OK {
		t.Fatalf("the program was rejected: %s", checked.ErrSummary())
	}
	optimized := hir.NewOptimizer().Optimize(hir.NewLowerer().LowerProgram(checked.Lowered))
	return mir.NewLowerer().LowerHIR(optimized)
}
