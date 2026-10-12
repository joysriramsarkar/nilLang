package llvm

import (
	"fmt"
	"math"
	"strings"
)

// Type is an LLVM type.
//
// # Why this is a type and not a string
//
// A type is written in a lot of places — `alloca`, `load`, `store`, `call`, a
// function signature — and the *same* type has to be spelled the same way each
// time. LLVM accepts aliases, so `%string = type { i8*, i64 }` and the expansion
// both work; a build that mixes them produces module text that is valid and a
// backend that rejects it.
//
// Naming each type once, at the top of the module, means a change to how a string
// is represented is a change in one place.
type Type struct {
	// Name is the LLVM type text, possibly referring to a module-level alias.
	Name string
}

// The primitive types. They are values rather than functions because a type is
// read far more often than it is written.
var (
	TypeI8     = Type{"i8"}
	TypeI32    = Type{"i32"}
	TypeI64    = Type{"i64"}
	TypeDouble = Type{"double"}
	TypeBool   = Type{"i1"}
	TypeVoid   = Type{"void"}
)

// String is Nilang's string: a byte pointer and a length in characters.
//
// # Why the length is in characters and the pointer is to bytes
//
// They are not the same, and getting it wrong is silent: `len("héllo")` becomes 6
// instead of 5, and `substr` slices at a byte offset that lands mid-rune. The
// interpreter counts runes; this has to agree with it, or the two engines disagree
// on any non-ASCII string.
type String struct{ Type }

// NilangString is the module-level name for the string type.
const NilangString = "%nil.string"

func (t Type) String() string { return t.Name }

// moduleTypes are the type aliases every module declares.
//
// They are collected rather than written inline because a module that omits one it
// uses is rejected, and a module that declares an unused one is accepted but
// noisy. Listing them centrally means a new type is added once.
func moduleTypes() []string {
	return []string{
		"%" + "nil.string = type { i8*, i64 }",
		"%" + "nil.list = type { ptr, i64, i64 }",
		"%" + "nil.map = type { ptr, i64 }",
	}
}

// escapeLLVMString renders a Go string as an LLVM constant.
//
// LLVM's `c"..."` syntax has no escape for a NUL and no way to end early, so a
// string containing one cannot be written as a C-style literal at all — it has to
// be a byte array. Doing that check here means the emitter never produces text that
// looks fine and assembles wrong.
func escapeLLVMString(s string) string {
	var b strings.Builder
	b.WriteString(`c"`)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			b.WriteString(`\"`)
		case c == '\\':
			b.WriteString(`\\`)
		case c == '\n':
			b.WriteString(`\0A`)
		case c == '\t':
			b.WriteString(`\09`)
		case c == 0:
			// A NUL would terminate the literal early, silently truncating the
			// string. Refusing here is better than emitting text that assembles to
			// the wrong answer.
			panic("llvm: a string constant containing NUL cannot be written as c\"...\"")
		default:
			if c < 0x20 || c >= 0x7f {
				fmt.Fprintf(&b, `\%02X`, c)
			} else {
				b.WriteByte(c)
			}
		}
	}
	b.WriteString(`"`)
	return b.String()
}

// floatLiteral renders a float64 as LLVM text.
//
// # Why the special cases
//
// LLVM parses `1.0` but not `1`, and it parses `inf` and `nan` only as keywords.
// A value that happens to be exactly zero, or infinite, or not-a-number — all of
// which a program can produce — has to be written in the form LLVM will read, or
// the module fails to parse at a place that has nothing to do with the arithmetic
// that produced it.
func floatLiteral(f float64) string {
	switch {
	case f != f:
		return "0x7FF8000000000000" // NaN
	case f > 1.7976931348623157e308:
		return "0x7FF0000000000000" // +inf
	case f < -1.7976931348623157e308:
		return "0xFFF0000000000000" // -inf
	case f == 0:
		// Negative zero is a distinct value, and `0x0` is not it. A program that
		// compares `1/x == -inf` can tell the difference.
		if signbit(f) {
			return "0x8000000000000000"
		}
		return "0x0"
	}
	// The hexadecimal form is exact: decimal notation rounds, and a rounded
	// literal is a different constant than the one the program computed.
	return "0x" + hexFloat64(f)
}

func signbit(f float64) bool {
	return math.Signbit(f)
}

// hexFloat64 is the bit pattern of a float, as hexadecimal.
//
// A decimal literal is rounded when it is parsed, so `0.1` written out in decimal
// is not the value the program computed. The bit pattern is exact, and it is also
// what LLVM's own documentation recommends for a value that must round-trip.
func hexFloat64(f float64) string {
	bits := math.Float64bits(f)
	return fmt.Sprintf("%016X", bits)
}
