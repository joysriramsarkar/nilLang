package lexer

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/token"
)

// TestCoalesceTokenLiterals pins the exact source text of the two-character
// `?` tokens: they used to carry a one-character literal, which made error
// messages and source echoes show `?` for `??` and `?.`.
func TestCoalesceTokenLiterals(t *testing.T) {
	l := New(`a ?? b?.c ? d`)
	want := []struct {
		typ token.TokenType
		lit string
	}{
		{token.IDENT, "a"},
		{token.QUESTION_QUESTION, "??"},
		{token.IDENT, "b"},
		{token.QUESTION_DOT, "?."},
		{token.IDENT, "c"},
		{token.QUESTION, "?"},
		{token.IDENT, "d"},
		{token.EOF, ""},
	}
	for i, w := range want {
		tok := l.NextToken()
		if tok.Type != w.typ || tok.Literal != w.lit {
			t.Fatalf("token %d = (%s, %q), want (%s, %q)", i, tok.Type, tok.Literal, w.typ, w.lit)
		}
	}
}

// TestScientificNotationTokens pins that exponents stay inside the number.
func TestScientificNotationTokens(t *testing.T) {
	l := New(`1e10 1.5e-3 2E+10 1e 1em`)
	want := []struct {
		typ token.TokenType
		lit string
	}{
		{token.FLOAT, "1e10"},
		{token.FLOAT, "1.5e-3"},
		{token.FLOAT, "2E+10"},
		{token.INT, "1"},
		{token.IDENT, "e"},
		{token.INT, "1"},
		{token.IDENT, "em"},
		{token.EOF, ""},
	}
	for i, w := range want {
		tok := l.NextToken()
		if tok.Type != w.typ || tok.Literal != w.lit {
			t.Fatalf("token %d = (%s, %q), want (%s, %q)", i, tok.Type, tok.Literal, w.typ, w.lit)
		}
	}
}

// TestRadixTokenBoundaries pins per-base scanning: the literal ends where the
// base's digits end.
func TestRadixTokenBoundaries(t *testing.T) {
	l := New(`0b10 2 0o17 8 0x1F g`)
	want := []string{"0b10", "2", "0o17", "8", "0x1F", "g"}
	for i, w := range want {
		tok := l.NextToken()
		if tok.Literal != w {
			t.Fatalf("token %d = %q, want %q", i, tok.Literal, w)
		}
	}
}

// TestHexEscapeIsACodePoint pins that \xHH denotes a Unicode code point, not
// a raw byte: \xE9 is U+00E9, the same character \u00E9 names.
func TestHexEscapeIsACodePoint(t *testing.T) {
	l := New(`"caf\xE9tcaf\u00E9te"`)
	tok := l.NextToken()
	if tok.Type != token.STRING {
		t.Fatalf("expected a string, got %s", tok.Type)
	}
	if tok.Literal != "cafétcaféte" {
		t.Fatalf(`got %q, want "caf\xc3\xa9tcaf\xc3\xa9te"`, tok.Literal)
	}
	// c a f é t c a f é t e: eleven code points. A byte-emitting \xE9 would
	// have produced twelve bytes with an invalid sequence in the middle.
	if got, want := len([]rune(tok.Literal)), 11; got != want {
		t.Fatalf("escape produced %d runes, want %d", got, want)
	}
}
