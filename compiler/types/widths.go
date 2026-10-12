package types

import "strings"

// Numeric widths: spelling, ranges, and the relations between them.
//
// # Why each width is a distinct type
//
// `i8` and `Int` being the same type is what made `let x: i8 = 300;` compile. A
// width that changes nothing is worse than no width: a developer who writes it
// reasonably believes a range is being checked, and nothing tells them otherwise.
// So each is its own `PrimitiveType`, and `Int` is not assignable to any of them
// and vice versa. That is what "no implicit numeric coercion" means in practice
// — there is no implicit step anywhere.
//
// # The one concession: an integer literal
//
// `let x: u8 = 5;` has to work, or no numeric code could ever declare a width.
// An integer literal is therefore assignable to any integer width whose range it
// fits, and to none whose range it does not. That is context sensitivity, not
// coercion: the literal has no width of its own until the declaration gives it
// one. A *variable* of type `Int` still cannot go into a `u8`, which is the case
// that would need a conversion somebody did not write.
//
// # The run-time representation is still 64-bit
//
// A width here is a static constraint, not a storage size. Literals are checked
// against it and a width carries through arithmetic, but a value computed at run
// time that exceeds its declared width is not caught — catching that would mean a
// narrowing instruction in both engines, and a check that only sometimes fires is
// worse than one documented as compile-time.

// WidthRange is the inclusive range a width can hold.
//
// It is written as a pair rather than a struct with a signedness flag because a
// caller almost always wants both ends, and unpacking a flag is the kind of thing
// that gets one arm of the comparison wrong.
type WidthRange struct {
	Min int64
	Max int64
}

// IsFloat reports whether the range belongs to a floating-point width, where
// `Min` and `Max` carry no meaning.
func (r WidthRange) IsFloat() bool { return r.Max == 0 && r.Min == 0 }

// widthSpecs maps each width's spelling to its type and range.
//
// The spelled names are lower-case because that is how Nilang writes them
// (`u8`, `i32`); the type names are capitalised because that is how every other
// primitive is named. Both spellings of the type are accepted too, so a
// declaration may say `Uint8` if it prefers.
var widthSpecs = map[string]struct {
	Type  PrimitiveType
	Range WidthRange
}{
	"i8":  {Int8, WidthRange{Min: -128, Max: 127}},
	"i16": {Int16, WidthRange{Min: -32768, Max: 32767}},
	"i32": {Int32, WidthRange{Min: -2147483648, Max: 2147483647}},
	"i64": {Int64, WidthRange{Min: -9223372036854775808, Max: 9223372036854775807}},
	"u8":  {Uint8, WidthRange{0, 255}},
	"u16": {Uint16, WidthRange{0, 65535}},
	"u32": {Uint32, WidthRange{0, 4294967295}},
	"u64": {Uint64, WidthRange{0, 9223372036854775807}},
	"f32": {Float32, WidthRange{}},
	"f64": {Float64, WidthRange{}},
}

// NumericWidths lists every width spelling, in a fixed order so a diagnostic that
// names one does not vary between runs.
var NumericWidths = []string{"i8", "i16", "i32", "i64", "u8", "u16", "u32", "u64", "f32", "f64"}

// IsNumericWidth reports whether a type name is a numeric width.
//
// It accepts both spellings, because both are written: `u8` in source and
// `Uint8` after resolution. A lookup that only knew the first would make every
// width annotation fail at the point where it was being *used* rather than where
// it was written — which reads as a bug in the assignment, not in the name.
func IsNumericWidth(name string) bool {
	_, _, ok := WidthOf(name)
	return ok
}

// WidthOf returns the type and range a width spelling names.
//
// `Uint8` and `u8` are the same width written two ways, and the canonicalised
// spelling is the one in the table.
func WidthOf(name string) (PrimitiveType, WidthRange, bool) {
	spec, ok := widthSpecs[canonicalWidthName(name)]
	return spec.Type, spec.Range, ok
}

// canonicalWidthName reduces a width's spelling to the table's key.
func canonicalWidthName(name string) string {
	lowered := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.HasPrefix(lowered, "uint"):
		return "u" + strings.TrimPrefix(lowered, "uint")
	case strings.HasPrefix(lowered, "int"):
		return "i" + strings.TrimPrefix(lowered, "int")
	case strings.HasPrefix(lowered, "float"):
		return "f" + strings.TrimPrefix(lowered, "float")
	}
	return lowered
}

// RangeOf returns the range a numeric *type* holds, and whether it is numeric.
func RangeOf(t Type) (WidthRange, bool) {
	switch v := t.(type) {
	case PrimitiveType:
		switch v {
		case Int:
			return WidthRange{Min: -9223372036854775808, Max: 9223372036854775807}, true
		case Float:
			return WidthRange{}, true
		}
		// The table is keyed by spelling, so the lookup goes through the type's
		// own name — which is the capitalised spelling — and canonicalises it.
		for _, spec := range widthSpecs {
			if spec.Type == v {
				return spec.Range, true
			}
		}
	case *PrimitiveType:
		if v != nil {
			return RangeOf(*v)
		}
	}
	return WidthRange{}, false
}

// IsSigned reports whether a width can hold a negative number.
func IsSigned(t Type) bool {
	r, ok := RangeOf(t)
	if !ok || r.IsFloat() {
		return true
	}
	return r.Min < 0
}

// IsFloatWidth reports whether a width is floating point.
func IsFloatWidth(t Type) bool {
	r, ok := RangeOf(t)
	return ok && r.IsFloat()
}

// LiteralFits reports whether a constant can be held by a width.
//
// It is the only place a number crosses from "no width of its own" to "has
// this one", which is why it takes the value rather than a type.
func LiteralFits(width Type, value int64) bool {
	r, ok := RangeOf(width)
	if !ok || r.IsFloat() {
		return true
	}
	return value >= r.Min && value <= r.Max
}

// ArithmeticResult is the type of `left op right` for a numeric operator.
//
// Two identical widths give that width back — the property that makes `let x:
// u8 = 5; let y: u8 = x + 1;` work without a single conversion. Anything else
// falls back to `Int` or `Float`, which is the widest thing the language has and
// therefore the only answer that cannot be too narrow.
func ArithmeticResult(left, right Type) Type {
	if IsFloatWidth(left) || IsFloatWidth(right) {
		return Float
	}
	if left == nil || right == nil {
		return Any
	}

	leftWidth, _, leftIsWidth := WidthOf(typeName(left))
	rightWidth, _, rightIsWidth := WidthOf(typeName(right))

	switch {
	case leftIsWidth && rightIsWidth && leftWidth == rightWidth:
		return leftWidth
	case leftIsWidth && isPlainInt(right):
		return Int
	case rightIsWidth && isPlainInt(left):
		return Int
	case isPlainInt(left) && isPlainInt(right):
		return Int
	case left.Equals(Float) || right.Equals(Float):
		return Float
	}

	// A mix of widths, or a width and `Int` where a value must be read as a
	// number: `Int` is the honest answer, and it is wider than every width in
	// the language.
	return Int
}

func isPlainInt(t Type) bool {
	p, ok := t.(PrimitiveType)
	return ok && p == Int
}

func typeName(t Type) string {
	if t == nil {
		return ""
	}
	p, ok := t.(PrimitiveType)
	if !ok {
		return ""
	}
	return p.Name
}
