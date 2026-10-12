package parser

import (
	"errors"
	"strconv"
	"strings"
)

// errInvalidLiteral marks a numeric literal the parser cannot convert.
var errInvalidLiteral = errors.New("invalid numeric literal")

// Bengali and other Unicode decimal digit blocks that Nilang accepts in
// numeric literals.
//
// A Bengali-first language must let a program read naturally: `let মোট = ১২৩৪`
// has to mean the same as `let মোট = 1234`. Go's strconv understands only
// ASCII digits, so literals are normalised before conversion.
var digitFoldStart, digitFoldEnd = rune(0xFF10), rune(0xFF19)       // ０-９ fullwidth
var bengaliDigitStart, bengaliDigitEnd = rune(0x09E6), rune(0x09EF) // ০-৯ Bengali

// foldDigits rewrites every non-ASCII decimal digit in a numeric literal to
// its ASCII equivalent and removes digit separators ('_'), which the
// specification permits for readability (for example 1_000_000).
//
// Characters that are not digits and not separators are preserved so that a
// radix-prefixed literal keeps its meaning; base detection happens afterwards.
func foldDigits(s string) (string, bool) {
	if !needsFolding(s) {
		return s, false
	}

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= bengaliDigitStart && r <= bengaliDigitEnd:
			b.WriteRune('0' + (r - bengaliDigitStart))
		case r >= digitFoldStart && r <= digitFoldEnd:
			b.WriteRune('0' + (r - digitFoldStart))
		case r == '_':
			// Digit separator: dropped before conversion.
		default:
			b.WriteRune(r)
		}
	}
	return b.String(), true
}

func needsFolding(s string) bool {
	for _, r := range s {
		if r == '_' ||
			(r >= bengaliDigitStart && r <= bengaliDigitEnd) ||
			(r >= digitFoldStart && r <= digitFoldEnd) {
			return true
		}
	}
	return false
}

// parseIntLiteral converts an integer literal to its value.
//
// It accepts decimal, 0x hex, 0o octal, and 0b binary, in ASCII or Bengali or
// fullwidth digits, with optional '_' separators. A negative literal is formed
// by the parser's unary minus rather than by a sign inside the literal, which
// is why a leading '-' is rejected here.
func parseIntLiteral(literal string) (int64, error) {
	folded, _ := foldDigits(literal)
	if folded == "" {
		return 0, errInvalidLiteral
	}
	// strconv.ParseInt with base 0 accepts 0x/0o/0b prefixes and plain decimal.
	// A leading sign is rejected so that unary minus stays the only way to
	// write a negative literal, keeping the token grammar unambiguous.
	if folded[0] == '-' || folded[0] == '+' {
		return 0, errInvalidLiteral
	}
	return strconv.ParseInt(folded, 0, 64)
}

// parseFloatLiteral converts a floating point literal to its value. The
// decimal point is always '.', so only the digit characters are folded.
func parseFloatLiteral(literal string) (float64, error) {
	folded, _ := foldDigits(literal)
	if folded == "" {
		return 0, errInvalidLiteral
	}
	return strconv.ParseFloat(folded, 64)
}
