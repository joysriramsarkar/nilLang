package lexer

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/joysriramsarkar/nilLang/compiler/token"
)

type Lexer struct {
	input        string
	position     int  // current position in input (points to current rune start)
	readPosition int  // current reading position in input (after current rune)
	ch           rune // current rune under examination
	line         int
	col          int
}

func New(input string) *Lexer {
	input = strings.TrimPrefix(input, "\xef\xbb\xbf")
	l := &Lexer{input: input, line: 1, col: 0}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
		l.position = l.readPosition
	} else {
		r, size := utf8.DecodeRuneInString(l.input[l.readPosition:])
		l.ch = r
		l.position = l.readPosition
		l.readPosition += size
	}
	l.col++
}

func (l *Lexer) peekChar() rune {
	if l.readPosition >= len(l.input) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.input[l.readPosition:])
	return r
}

// peekCharAt returns the rune n-1 positions past the current one: peekCharAt(1)
// is peekChar. Short reads at the end of input report 0.
func (l *Lexer) peekCharAt(n int) rune {
	offset := l.readPosition
	var r rune
	for i := 0; i < n; i++ {
		if offset >= len(l.input) {
			return 0
		}
		var size int
		r, size = utf8.DecodeRuneInString(l.input[offset:])
		if size == 0 {
			return 0
		}
		offset += size
	}
	return r
}

func (l *Lexer) NextToken() token.Token {
	var tok token.Token

	l.skipWhitespaceAndComments()

	curLine := l.line
	curCol := l.col

	switch l.ch {
	case '=':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.EQ, Literal: string(ch) + string(l.ch), Line: curLine, Column: curCol}
		} else if l.peekChar() == '>' {
			// `=>`, the match arm separator.
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.FARROW, Literal: string(ch) + string(l.ch), Line: curLine, Column: curCol}
		} else {
			tok = newToken(token.ASSIGN, l.ch, curLine, curCol)
		}
	case '+':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.PLUS_ASSIGN, Literal: string(ch) + string(l.ch), Line: curLine, Column: curCol}
		} else {
			tok = newToken(token.PLUS, l.ch, curLine, curCol)
		}
	case '-':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.MINUS_ASSIGN, Literal: string(ch) + string(l.ch), Line: curLine, Column: curCol}
		} else if l.peekChar() == '>' {
			// `->`, the return-type separator.
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.ARROW, Literal: string(ch) + string(l.ch), Line: curLine, Column: curCol}
		} else {
			tok = newToken(token.MINUS, l.ch, curLine, curCol)
		}
	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.NOT_EQ, Literal: string(ch) + string(l.ch), Line: curLine, Column: curCol}
		} else {
			tok = newToken(token.BANG, l.ch, curLine, curCol)
		}
	case '/':
		tok = newToken(token.SLASH, l.ch, curLine, curCol)
	case '*':
		tok = newToken(token.ASTERISK, l.ch, curLine, curCol)
	case '%':
		tok = newToken(token.MODULO, l.ch, curLine, curCol)
	case '<':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.LTE, Literal: string(ch) + string(l.ch), Line: curLine, Column: curCol}
		} else {
			tok = newToken(token.LT, l.ch, curLine, curCol)
		}
	case '>':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.GTE, Literal: string(ch) + string(l.ch), Line: curLine, Column: curCol}
		} else {
			tok = newToken(token.GT, l.ch, curLine, curCol)
		}
	case '&':
		if l.peekChar() == '&' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.AND, Literal: string(ch) + string(l.ch), Line: curLine, Column: curCol}
		} else {
			tok = newToken(token.BIT_AND, l.ch, curLine, curCol)
		}
	case '|':
		if l.peekChar() == '|' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.OR, Literal: string(ch) + string(l.ch), Line: curLine, Column: curCol}
		} else {
			tok = newToken(token.BIT_OR, l.ch, curLine, curCol)
		}
	case '?':
		// `?` alone is the ternary test and the optional-type marker. `??` and
		// `?.` are single tokens rather than two, because a parser that saw two
		// would have to decide whether the second `?` is an operator or the start
		// of another construct — and `a ?? b ? c : d` is ambiguous without one.
		switch l.peekChar() {
		case '?':
			l.readChar()
			tok = token.Token{Type: token.QUESTION_QUESTION, Literal: "??", Line: curLine, Column: curCol}
		case '.':
			l.readChar()
			tok = token.Token{Type: token.QUESTION_DOT, Literal: "?.", Line: curLine, Column: curCol}
		default:
			tok = newToken(token.QUESTION, l.ch, curLine, curCol)
		}
	case ';':
		tok = newToken(token.SEMICOLON, l.ch, curLine, curCol)
	case ':':
		tok = newToken(token.COLON, l.ch, curLine, curCol)
	case ',':
		tok = newToken(token.COMMA, l.ch, curLine, curCol)
	case '.':
		// `..` and `..=` form a range; a lone `.` is member access.
		//
		// Lexing the range as one token means a mistyped `0...5` reports one
		// precise error rather than a cascade from a member access on a number.
		if l.peekChar() == '.' {
			l.readChar() // consume the second '.'
			literal := ".."

			if l.peekChar() == '=' {
				l.readChar()
				literal = "..="
			}

			tok = token.Token{Type: token.RANGE, Literal: literal, Line: curLine, Column: curCol}
		} else {
			tok = newToken(token.DOT, l.ch, curLine, curCol)
		}
	case '(':
		tok = newToken(token.LPAREN, l.ch, curLine, curCol)
	case ')':
		tok = newToken(token.RPAREN, l.ch, curLine, curCol)
	case '{':
		tok = newToken(token.LBRACE, l.ch, curLine, curCol)
	case '}':
		tok = newToken(token.RBRACE, l.ch, curLine, curCol)
	case '[':
		tok = newToken(token.LBRACKET, l.ch, curLine, curCol)
	case ']':
		tok = newToken(token.RBRACKET, l.ch, curLine, curCol)
	case '"':
		strVal, ok := l.readString()
		if !ok {
			tok = token.Token{Type: token.ILLEGAL, Literal: "unterminated string literal", Line: curLine, Column: curCol}
			return tok
		}
		tok = token.Token{Type: token.STRING, Literal: strVal, Line: curLine, Column: curCol}
		l.readChar()
		return tok
	case 0:
		tok.Literal = ""
		tok.Type = token.EOF
		tok.Line = curLine
		tok.Column = curCol
	default:
		if isIdentStart(l.ch) {
			tok.Literal = l.readIdentifier()
			tok.Type = token.LookupIdent(tok.Literal)
			tok.Line = curLine
			tok.Column = curCol
			return tok
		} else if isDigit(l.ch) {
			numLit, isFloat := l.readNumber()
			if isFloat {
				tok.Type = token.FLOAT
			} else {
				tok.Type = token.INT
			}
			tok.Literal = numLit
			tok.Line = curLine
			tok.Column = curCol
			return tok
		} else {
			tok = newToken(token.ILLEGAL, l.ch, curLine, curCol)
		}
	}

	l.readChar()
	return tok
}

func (l *Lexer) skipWhitespaceAndComments() {
	for {
		// skip whitespace including BOM (\uFEFF)
		for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' || l.ch == '\uFEFF' {
			if l.ch == '\n' {
				l.line++
				l.col = 0
			}
			l.readChar()
		}

		// check single-line comment
		if l.ch == '/' && l.peekChar() == '/' {
			for l.ch != '\n' && l.ch != 0 {
				l.readChar()
			}
			continue
		}

		// check multi-line comment (supports nesting)
		if l.ch == '/' && l.peekChar() == '*' {
			l.readChar() // eat /
			l.readChar() // eat *
			depth := 1
			for depth > 0 && l.ch != 0 {
				if l.ch == '\n' {
					l.line++
					l.col = 0
				}
				if l.ch == '/' && l.peekChar() == '*' {
					l.readChar() // eat /
					l.readChar() // eat *
					depth++
					continue
				}
				if l.ch == '*' && l.peekChar() == '/' {
					l.readChar() // eat *
					l.readChar() // eat /
					depth--
					continue
				}
				l.readChar()
			}
			continue
		}

		break
	}
}

func (l *Lexer) readString() (string, bool) {
	var sb strings.Builder

	for {
		l.readChar()
		if l.ch == 0 {
			return sb.String(), false
		}
		if l.ch == '\n' {
			l.line++
			l.col = 0
		}
		if l.ch == '"' {
			break
		}

		if l.ch == '\\' {
			l.readChar()
			if l.ch == 0 {
				return sb.String(), false
			}
			switch l.ch {
			case 'n':
				sb.WriteRune('\n')
			case 't':
				sb.WriteRune('\t')
			case 'r':
				sb.WriteRune('\r')
			case '0':
				sb.WriteRune(0)
			case 'a':
				sb.WriteRune('\a')
			case 'b':
				sb.WriteRune('\b')
			case 'f':
				sb.WriteRune('\f')
			case 'v':
				sb.WriteRune('\v')
			case '\\':
				// An escaped backslash before `(` must stay two chars:
				// collapsing it would turn source `\\(` into `\(` and
				// the parser could no longer tell an escaped backslash
				// plus a paren from a real interpolation opener. The
				// parser unescapes `\\` in template literal parts.
				if l.peekChar() == '(' {
					sb.WriteString("\\\\")
				} else {
					sb.WriteRune('\\')
				}
			case '"':
				sb.WriteRune('"')
			case '(':
				// `\(` opens an interpolation: consume through its matching
				// `)` so a quote inside — `\("a)b")` — does not end the
				// outer string early. The scan is string-aware for the same
				// reason the template splitter is.
				sb.WriteString("\\(")
				// ScanBalanced returns the absolute index of the
				// matching `)`, not a relative offset.
				end, ok := ScanBalanced(l.input, l.readPosition)
				if !ok {
					return sb.String(), false
				}
				parenPos := end
				// Step past the `\(` itself, which is already written: the
				// loop below consumes only the interpolation's contents.
				l.readChar()
				for l.position < parenPos && l.ch != 0 {
					if l.ch == '\n' {
						l.line++
						l.col = 0
					}
					sb.WriteRune(l.ch)
					l.readChar()
				}
				// l.ch is now the closing paren: write it and continue.
				// The loop head advances past it, exactly as it does for
				// every other rune handled above, so no step is skipped.
				sb.WriteRune(l.ch)
				continue
			case 'e':
				// \e — ESC (0x1B), the ANSI control introducer.
				sb.WriteRune('\x1b')
			case 'x', 'u', 'U':
				// \xHH, \uXXXX and \UXXXXXXXX code-point escapes. They make
				// terminal colour and cursor control reachable from source.
				width := 4
				if l.ch == 'x' {
					width = 2
				} else if l.ch == 'U' {
					width = 8
				}
				if text, ok := l.peekRunes(width); ok && allHex(text) {
					value, parseErr := strconv.ParseUint(string(text), 16, 32)
					if parseErr == nil && utf8.ValidRune(rune(value)) {
						// Every escape denotes a Unicode code point, including
						// \xHH: emitting the byte would splice raw non-UTF-8
						// into the string and corrupt every later operation on
						// it. \xE9 is therefore é, the same as \u00E9.
						sb.WriteRune(rune(value))
						l.skipRunes(width)
						continue
					}
				}
				// Not a well-formed escape: keep it literal, exactly as an
				// unknown escape such as \q already behaves.
				sb.WriteRune('\\')
				sb.WriteRune(l.ch)
			default:
				sb.WriteRune('\\')
				sb.WriteRune(l.ch)
			}
			continue
		}

		sb.WriteRune(l.ch)
	}

	return sb.String(), true
}

// peekRunes returns the n runes that follow the current one without consuming
// them. ok is false when the input ends first or a rune is truncated.
func (l *Lexer) peekRunes(n int) ([]rune, bool) {
	runes := make([]rune, 0, n)
	offset := l.readPosition
	for len(runes) < n {
		if offset >= len(l.input) {
			return runes, false
		}
		r, decoded := utf8.DecodeRuneInString(l.input[offset:])
		if decoded == 0 || (r == utf8.RuneError && decoded == 1) {
			return runes, false
		}
		runes = append(runes, r)
		offset += decoded
	}
	return runes, true
}

// skipRunes advances the lexer over the n runes following the current one.
func (l *Lexer) skipRunes(n int) {
	for i := 0; i < n; i++ {
		l.readChar()
	}
}

// allHex reports whether every rune is a hexadecimal digit.
func allHex(runes []rune) bool {
	for _, r := range runes {
		if !isHexDigit(r) {
			return false
		}
	}
	return len(runes) > 0
}

func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func (l *Lexer) readIdentifier() string {
	position := l.position
	for isIdentPart(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

func (l *Lexer) readNumber() (string, bool) {
	position := l.position

	// Radix-prefixed integers: 0x1F, 0o17, 0b1010. A prefix is only recognised
	// when followed by at least one valid digit for that base, so "0" and
	// "0.5" are unaffected and "0x" alone falls through as a decimal 0 plus an
	// identifier, which the parser then reports with a precise diagnostic.
	// Digits are validated per base: in `0b102` the `2` is not a binary digit,
	// so the literal ends at `0b10` and the parser reports the stray `2`.
	if zero, isZero := DigitValue(l.ch); isZero && zero == 0 && isRadixPrefix(l.peekChar()) {
		valid := validRadixDigit(l.peekChar())
		if !valid(l.peekCharAt(2)) && l.peekCharAt(2) != '_' {
			// No valid digit follows the prefix: not a radix literal at all.
			// Fall through to the decimal path, which yields a plain "0".
		} else {
			l.readChar() // consume '0'
			l.readChar() // consume the radix marker
			for valid(l.ch) || l.ch == '_' {
				l.readChar()
			}
			return l.input[position:l.position], false
		}
	}

	isFloat := false
	for isDigit(l.ch) || l.ch == '_' ||
		(l.ch == '.' && isDigit(l.peekChar())) {
		if l.ch == '.' {
			isFloat = true
		}
		l.readChar()
	}
	// Scientific notation: `1e10`, `1.5e-3`, `2E+10`. The exponent marker is
	// only consumed when a valid exponent follows, so `1e` alone still lexes
	// as `1` plus an identifier, exactly as before.
	if (l.ch == 'e' || l.ch == 'E') && isExponentStart(l.peekChar(), l.peekCharAt(2)) {
		isFloat = true
		l.readChar() // consume e/E
		if l.ch == '+' || l.ch == '-' {
			l.readChar()
		}
		for isDigit(l.ch) || l.ch == '_' {
			l.readChar()
		}
	}
	return l.input[position:l.position], isFloat
}

// isExponentStart reports whether the runes after an `e`/`E` form a valid
// exponent: a digit, or a sign followed by a digit.
func isExponentStart(next, after rune) bool {
	if isDigit(next) {
		return true
	}
	return (next == '+' || next == '-') && isDigit(after)
}

// FindInterpolationStart locates the next `\(` that opens an interpolation
// at or after from. A `\(` preceded by an even-length run of backslashes is
// an escaped backslash followed by a plain paren, not an interpolation:
// `\\(` stays literal while `\\\(` opens one after a literal backslash.
// Returns -1 when there is none.
func FindInterpolationStart(raw string, from int) int {
	i := from
	for i < len(raw) {
		if raw[i] != '\\' {
			i++
			continue
		}
		j := i
		for j < len(raw) && raw[j] == '\\' {
			j++
		}
		if j < len(raw) && raw[j] == '(' && (j-i)%2 == 1 {
			return j - 1
		}
		i = j
		if j < len(raw) && raw[j] == '(' {
			i++
		}
	}
	return -1
}

// ScanBalanced finds the `)` closing the interpolation whose contents start
// at start (just past `\(`). Parentheses inside string literals do not count
// — without that, `\("a)b")` ends at the string's paren and the expression
// is truncated. Backslash escapes inside strings are skipped so `\"` does
// not end the string early. ok is false when the input ends first.
// Byte offsets are safe here: every byte the scan compares against is ASCII,
// which never appears inside a multibyte rune.
func ScanBalanced(input string, start int) (end int, ok bool) {
	parenDepth := 1
	pos := start
	var inString byte
	for pos < len(input) {
		c := input[pos]
		if inString != 0 {
			if c == '\\' {
				pos += 2
				continue
			}
			if c == inString {
				inString = 0
			}
			pos++
			continue
		}
		switch c {
		case '"':
			inString = '"'
			pos++
		case '(':
			parenDepth++
			pos++
		case ')':
			parenDepth--
			if parenDepth == 0 {
				return pos, true
			}
			pos++
		default:
			pos++
		}
	}
	return pos, false
}

// isRadixPrefix reports whether r introduces a base-prefixed integer.
func isRadixPrefix(r rune) bool {
	switch r {
	case 'x', 'X', 'b', 'B', 'o', 'O':
		return true
	}
	return false
}

// validRadixDigit returns the digit predicate for a radix marker: hex digits
// for 0x, octal for 0o, binary for 0b. Scanning with the wrong predicate
// silently accepts digits the base cannot hold. Digits are Unicode decimal
// digits in any script — `০b১০১০` is binary ten — so the predicates fold
// through DigitValue rather than comparing ASCII ranges.
func validRadixDigit(marker rune) func(rune) bool {
	switch marker {
	case 'x', 'X':
		return func(r rune) bool {
			if isHexDigit(r) {
				return true
			}
			_, ok := DigitValue(r)
			return ok
		}
	case 'o', 'O':
		return func(r rune) bool {
			v, ok := DigitValue(r)
			return ok && v <= 7
		}
	default: // 'b', 'B'
		return func(r rune) bool {
			v, ok := DigitValue(r)
			return ok && v <= 1
		}
	}
}

// DigitValue returns the decimal value of any Unicode decimal digit (category
// Nd), not just ASCII. Every Nd digit belongs to a contiguous run of exactly
// ten, so scanning down to the run's start finds its zero without a
// per-script table, and scripts Unicode adds later work without code changes.
// The scan is bounded: more than nine steps means the rune is not in a
// decimal run after all.
func DigitValue(r rune) (int, bool) {
	if '0' <= r && r <= '9' {
		return int(r - '0'), true
	}
	if !unicode.IsDigit(r) {
		return 0, false
	}
	zero := r
	for i := 0; i < 9 && unicode.IsDigit(zero-1); i++ {
		zero--
	}
	v := int(r - zero)
	if v < 0 || v > 9 || !unicode.IsDigit(zero) {
		return 0, false
	}
	return v, true
}

func isIdentStart(ch rune) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_' || unicode.IsLetter(ch)
}

func isIdentPart(ch rune) bool {
	return isIdentStart(ch) || isDigit(ch) || unicode.Is(unicode.M, ch)
}

func isLetter(ch rune) bool {
	return isIdentStart(ch)
}

func isDigit(ch rune) bool {
	return '0' <= ch && ch <= '9' || unicode.IsDigit(ch)
}

func newToken(tokenType token.TokenType, ch rune, line, col int) token.Token {
	return token.Token{Type: tokenType, Literal: string(ch), Line: line, Column: col}
}
