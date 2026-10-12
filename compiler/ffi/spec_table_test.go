package ffi

import (
	"testing"
)

// The layouts §9.16 of the language spec publishes as a table.
//
// # Why this exists
//
// Because a spec table is a **claim**, and a claim about offsets is the one kind that
// looks obviously true and is silently wrong more often than any other. When §9.16 was
// written the first example was given as 24 bytes and the real answer is 32: `Sparse`
// has eighteen bytes of fields, rounds each field to its own alignment, and rounds
// the whole to eight. Nothing about the number is derivable by eye.
//
// So the table is here, as data, and a reader who wants to change it has to change it
// in two places and run the tests. A spec that can be edited without failing anything
// is a spec nobody should rely on.

// documentedLayouts is §9.16's table, verbatim.
var documentedLayouts = []struct {
	name      string
	declaring string
	size      int64
	alignment int64
	offsets   map[string]int64
}{
	{
		name:      "P",
		declaring: "struct P { x: Int, y: Int }",
		size:      16,
		alignment: 8,
		offsets:   map[string]int64{"x": 0, "y": 8},
	},
	{
		// The one that was wrong. Eighteen bytes of fields, thirty-two of struct.
		name:      "Sparse",
		declaring: "struct Sparse { a: Bool, x: Int, b: Bool, y: Int }",
		size:      32,
		alignment: 8,
		offsets:   map[string]int64{"a": 0, "x": 8, "b": 16, "y": 24},
	},
	{
		name:      "Named",
		declaring: "struct Named { label: String, count: Int }",
		size:      24,
		alignment: 8,
		offsets:   map[string]int64{"label": 0, "count": 16},
	},
	{
		// Three bytes and no rounding, because nothing in it asks for eight.
		name:      "T",
		declaring: "struct T { a: Byte, b: Byte, c: Byte }",
		size:      3,
		alignment: 1,
		offsets:   map[string]int64{"a": 0, "b": 1, "c": 2},
	},
}

// TestTheLayoutsTheSpecPublishesAreTheRealOnes checks the table against the compiler.
//
// It builds each declaration through the real front end, so what it checks is the
// layout a program gets — not the layout of a struct this test assembled itself, which
// would agree with anything.
func TestTheLayoutsTheSpecPublishesAreTheRealOnes(t *testing.T) {
	for _, want := range documentedLayouts {
		layout, err := LayoutOf(programWith(t, want.declaring), want.name)
		if err != nil {
			t.Errorf("%s: %v", want.name, err)
			continue
		}

		if layout.Size != want.size {
			t.Errorf("%s is %d bytes; the spec says %d", want.name, layout.Size, want.size)
		}
		if layout.Alignment != want.alignment {
			t.Errorf("%s is aligned at %d; the spec says %d",
				want.name, layout.Alignment, want.alignment)
		}
		if len(layout.Fields) != len(want.offsets) {
			t.Errorf("%s has %d fields; the spec lists %d",
				want.name, len(layout.Fields), len(want.offsets))
			continue
		}
		for _, field := range layout.Fields {
			offset, listed := want.offsets[field.Name]
			if !listed {
				t.Errorf("%s has a field %q the spec's table does not list", want.name, field.Name)
				continue
			}
			if field.Offset != offset {
				t.Errorf("%s.%s is at %d; the spec says %d", want.name, field.Name, field.Offset, offset)
			}
		}
	}
}

// TestTheSpecTableHasNoDuplicates is the failure mode of a table in a test.
//
// Two rows with the same name would pass, and the second would simply never be read
// by anything — a specification that quietly disagrees with itself is worse than one
// that is absent, because it looks checked.
func TestTheSpecTableHasNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, row := range documentedLayouts {
		if seen[row.name] {
			t.Errorf("%q appears twice in the spec's layout table", row.name)
		}
		seen[row.name] = true
	}
}
