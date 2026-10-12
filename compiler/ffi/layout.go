package ffi

import (
	"fmt"
	"sort"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/mir"
)

// A struct crosses the boundary through memory the caller owns, and this is where
// that memory's shape is decided.
//
// # Why by pointer and never by value
//
// Because a struct's *size and alignment* are agreed — every compiler targeting an
// ABI computes them the same way, and `nil_ffi_sizeof` exists so that is checkable
// rather than assumed. What is **not** agreed is how a struct return is classified:
// whether it comes back in registers or through a hidden pointer is a per-compiler
// decision, and two compilers in one link will disagree.
//
// clang assembling a module with an `x86_64-pc-windows-msvc` triple returns a
// `{ptr, i64}` in RAX:RDX. GCC for `x86_64-w64-mingw32` returns it through a pointer
// the caller passes in RCX. Both correct. Both present. The call site cannot tell
// which one it received.
//
// So nothing here returns an aggregate. A struct goes out as a pointer to a buffer
// the caller owns and fills; a function that answers with one takes a pointer and
// writes into it. That has one meaning on every toolchain.
//
// # Why the layout is C's, not ours
//
// Because the point is to hand a C function something it can declare. A field of
// `Int` is `int64_t` at the same offset a C `int64_t` field would be, and a
// `String` field is two words where a C `struct { const char *p; size_t n; }` would
// be — so a C author writes the struct they expected and it works.
//
// The alternative — one tagged word per field, uniform and simple — would give C
// something with no C spelling at all. Every field would have to be unboxed by hand,
// which is the conversion the crossing exists to perform.

// LayoutOf returns a struct's crossing layout, read from the program's own
// declarations.
//
// From `program.Types` rather than from the source text, because that map is what
// every other stage reads, and a second reader is a second opinion — and two
// opinions about what `Point` contains is the failure this whole exercise is about.
//
// The error names the field that could not cross, because a struct whose third
// field is a list is not a struct with two usable fields: the layout would leave a
// hole, and C would read the fourth field's bytes as the third's.
func LayoutOf(program *mir.Program, name string) (Layout, error) {
	declaration, declared := program.Types[name]
	if !declared {
		return Layout{}, fmt.Errorf(
			"%s is not a type this program declared, so there is nothing to lay out "+
				"(a field must be one of: %s)", name, strCrossable())
	}
	if declaration.Kind != mir.KindStruct {
		return Layout{}, fmt.Errorf(
			"%s is a %s, and only a struct has fields to lay out", name, declaration.Kind)
	}

	crossings := make([]Crossing, 0, len(declaration.Fields))
	names := make([]string, 0, len(declaration.Fields))
	for _, field := range declaration.Fields {
		crossing, found := For(field.TypeString)
		if !found || crossing.Size == 0 {
			return Layout{}, fmt.Errorf(
				"struct %s cannot cross: its field %s is a %s and has no crossing, "+
					"so there would be a hole where the value belongs (a field must be "+
					"one of: %s)",
				name, field.Name, DescribeMissing(field.TypeString), strCrossable())
		}
		crossings = append(crossings, crossing)
		names = append(names, field.Name)
	}

	layout, err := LayoutFor(name, crossings)
	if err != nil {
		return Layout{}, err
	}
	return layout.WithNames(names), nil
}

// IsStructName reports whether a written annotation names a struct this program
// declared.
//
// A lookup in the *program's* table rather than a test on the name, because a name
// is not a type: `Point` could be an entity, a component, or nothing at all, and
// only one of those has fields laid out at offsets.
func IsStructName(program *mir.Program, annotation string) bool {
	name := annotation
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		name = name[dot+1:]
	}
	declaration, declared := program.Types[name]
	return declared && declaration.Kind == mir.KindStruct
}

// StructNames lists the structs a program declared, sorted.
//
// Sorted so that anything generated from them — a header, a module's type list — is
// the same twice, which is the same reason the emitter sorts its functions.
func StructNames(program *mir.Program) []string {
	var names []string
	for name, declaration := range program.Types {
		if declaration.Kind == mir.KindStruct {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// strCrossable lists the types a field may have, for a diagnostic.
func strCrossable() string {
	names := make([]string, 0, len(Types))
	for name := range Types {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// Field is one field of a struct, as it crosses.
type Field struct {
	Name string

	// Crossing is how the field's type crosses.
	//
	// A field with no crossing makes the struct uncrossable rather than partially
	// crossed. A struct where the third field is a list is not a struct with three
	// fields; it is a struct the boundary cannot represent, and half of one would be
	// a buffer C reads with the wrong layout.
	Crossing Crossing

	// Offset is the field's byte offset within the struct.
	Offset int64
}

// Layout is a struct's shape at the boundary: where each field is, and how big the
// whole thing is.
type Layout struct {
	Name   string
	Fields []Field

	// Size is the whole struct's size, including the padding at the end.
	Size int64

	// Alignment is the strictest alignment any field requires.
	Alignment int64
}

// Offset returns where a field sits, and whether there is such a field.
func (l Layout) Offset(name string) (int64, bool) {
	for _, field := range l.Fields {
		if field.Name == name {
			return field.Offset, true
		}
	}
	return 0, false
}

// LayoutFor computes a struct's layout from its fields, in order.
//
// # Why this is C's algorithm and not a simpler one
//
// Because the output is compared against what clang does with the same struct, and
// that comparison is the test. A layout that packed fields tightly would be shorter
// and would not match, so every C function written against the program's own header
// would read the wrong bytes at some offset — silently, because both sides
// compiled.
//
// So: each field goes at the next multiple of its own alignment, the struct is as
// aligned as its strictest field, and the size is rounded up to that alignment. That
// is the rule C has, and the rule the ABI has, and they are the same rule.
//
// # The widths come from the crossing, not from here
//
// `Size` and `Alignment` are read off each `Crossing`. A second table of "how wide
// is an Int" in this file would be one more place for a new type to be forgotten,
// and a struct laid out wrongly is a buffer C reads with the wrong offsets.
func LayoutFor(name string, crossings []Crossing) (Layout, error) {
	layout := Layout{Name: name}
	offset := int64(0)

	for index, crossing := range crossings {
		size := crossing.Size
		alignment := crossing.Alignment

		// `void` is the only crossing with no width, and it is the only one that
		// cannot be a field. Refused rather than skipped: a struct with a hole where
		// a value should be is not a struct with a missing field, it is a buffer C
		// reads with offsets that do not line up.
		if size == 0 {
			return Layout{}, fmt.Errorf(
				"struct %s cannot cross: field %d is a %s and has no size, so there is "+
					"nothing to put at an offset", name, index+1, crossing.C)
		}

		// Rounded up before placing, which is the whole of the padding rule. A field
		// of one byte after an `int64_t` leaves seven bytes the C side has to agree
		// about as well, and it does — because the C side's compiler applied the same
		// rule to the same struct.
		if remainder := offset % alignment; remainder != 0 {
			offset += alignment - remainder
		}

		layout.Fields = append(layout.Fields, Field{
			Name:     crossingName(index),
			Crossing: crossing,
			Offset:   offset,
		})
		offset += size

		if alignment > layout.Alignment {
			layout.Alignment = alignment
		}
	}

	// A struct with no fields has alignment 1 and size 0. One with fields is
	// rounded up to its own alignment, so an array of them has no gaps a C
	// compiler would not also have put there.
	if layout.Alignment == 0 {
		layout.Alignment = 1
	}
	if remainder := offset % layout.Alignment; remainder != 0 {
		offset += layout.Alignment - remainder
	}
	layout.Size = offset

	return layout, nil
}

// crossingName is the placeholder for a field whose name was not supplied.
//
// A field is looked up by name when C asks about it, and by position when the
// crossing is built — so the name is documentation here rather than part of the
// contract. It is still filled in rather than left blank, because a layout printed
// with `Fields: [ {Offset:0}, {Offset:8} ]` is harder to read than one with names,
// and a reader debugging a crossing is reading exactly that.
func crossingName(index int) string {
	return fmt.Sprintf("field%d", index)
}

// WithNames replaces the positional placeholders with the struct's real field names.
//
// Done as a separate step rather than passed in, because the layout is computed from
// *crossings* — what each type becomes — while the names come from the declaration.
// Two inputs that are usually the same and are not always, which is exactly the
// shape of a bug.
func (l Layout) WithNames(names []string) Layout {
	if len(names) != len(l.Fields) {
		return l
	}
	out := l
	out.Fields = append([]Field(nil), l.Fields...)
	for i, name := range names {
		out.Fields[i].Name = name
	}
	return out
}
