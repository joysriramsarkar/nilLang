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
			tok = newToken(token.QUESTION_QUESTION, l.ch, curLine, curCol)
		case '.':
			l.readChar()
			tok = newToken(token.QUESTION_DOT, l.ch, curLine, curCol)
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
				sb.WriteRune('\\')
			case '"':
				sb.WriteRune('"')
			case '(':
				sb.WriteString("\\(")
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
						if l.ch == 'x' {
							sb.WriteByte(byte(value))
						} else {
							sb.WriteRune(rune(value))
						}
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
	if l.ch == '0' && isRadixPrefix(l.peekChar()) {
		l.readChar() // consume '0'
		l.readChar() // consume the radix marker
		for isHexDigit(l.ch) || l.ch == '_' {
			l.readChar()
		}
		return l.input[position:l.position], false
	}

	isFloat := false
	for isDigit(l.ch) || l.ch == '_' ||
		(l.ch == '.' && isDigit(l.peekChar())) {
		if l.ch == '.' {
			isFloat = true
		}
		l.readChar()
	}
	return l.input[position:l.position], isFloat
}

// isRadixPrefix reports whether r introduces a base-prefixed integer.
func isRadixPrefix(r rune) bool {
	switch r {
	case 'x', 'X', 'b', 'B', 'o', 'O':
		return true
	}
	return false
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
