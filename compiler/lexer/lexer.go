package lexer

import (
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
		tok = newToken(token.QUESTION, l.ch, curLine, curCol)
	case ';':
		tok = newToken(token.SEMICOLON, l.ch, curLine, curCol)
	case ':':
		tok = newToken(token.COLON, l.ch, curLine, curCol)
	case ',':
		tok = newToken(token.COMMA, l.ch, curLine, curCol)
	case '.':
		tok = newToken(token.DOT, l.ch, curLine, curCol)
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

func (l *Lexer) readIdentifier() string {
	position := l.position
	for isIdentPart(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

func (l *Lexer) readNumber() (string, bool) {
	position := l.position
	isFloat := false
	for isDigit(l.ch) || (l.ch == '.' && isDigit(l.peekChar())) {
		if l.ch == '.' {
			isFloat = true
		}
		l.readChar()
	}
	return l.input[position:l.position], isFloat
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
