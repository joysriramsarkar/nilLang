package lexer

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/token"
)

func TestNextToken(t *testing.T) {
	input := `let five = 5;
let ten = 10.5;

let add = fn(x, y) {
  x + y;
};

let result = add(five, ten);
!- / * 5;
5 < 10 > 5;

if (5 < 10) {
	return true;
} else {
	return false;
}

10 == 10;
10 != 9;
"foobar"
"foo \(bar) baz"
[1, 2];
{"foo": "bar"}
while (x < 5) {
	let x = x + 1;
}
`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.LET, "let"},
		{token.IDENT, "five"},
		{token.ASSIGN, "="},
		{token.INT, "5"},
		{token.SEMICOLON, ";"},

		{token.LET, "let"},
		{token.IDENT, "ten"},
		{token.ASSIGN, "="},
		{token.FLOAT, "10.5"},
		{token.SEMICOLON, ";"},

		{token.LET, "let"},
		{token.IDENT, "add"},
		{token.ASSIGN, "="},
		{token.FN, "fn"},
		{token.LPAREN, "("},
		{token.IDENT, "x"},
		{token.COMMA, ","},
		{token.IDENT, "y"},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.IDENT, "x"},
		{token.PLUS, "+"},
		{token.IDENT, "y"},
		{token.SEMICOLON, ";"},
		{token.RBRACE, "}"},
		{token.SEMICOLON, ";"},

		{token.LET, "let"},
		{token.IDENT, "result"},
		{token.ASSIGN, "="},
		{token.IDENT, "add"},
		{token.LPAREN, "("},
		{token.IDENT, "five"},
		{token.COMMA, ","},
		{token.IDENT, "ten"},
		{token.RPAREN, ")"},
		{token.SEMICOLON, ";"},

		{token.BANG, "!"},
		{token.MINUS, "-"},
		{token.SLASH, "/"},
		{token.ASTERISK, "*"},
		{token.INT, "5"},
		{token.SEMICOLON, ";"},

		{token.INT, "5"},
		{token.LT, "<"},
		{token.INT, "10"},
		{token.GT, ">"},
		{token.INT, "5"},
		{token.SEMICOLON, ";"},

		{token.IF, "if"},
		{token.LPAREN, "("},
		{token.INT, "5"},
		{token.LT, "<"},
		{token.INT, "10"},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.RETURN, "return"},
		{token.TRUE, "true"},
		{token.SEMICOLON, ";"},
		{token.RBRACE, "}"},
		{token.ELSE, "else"},
		{token.LBRACE, "{"},
		{token.RETURN, "return"},
		{token.FALSE, "false"},
		{token.SEMICOLON, ";"},
		{token.RBRACE, "}"},

		{token.INT, "10"},
		{token.EQ, "=="},
		{token.INT, "10"},
		{token.SEMICOLON, ";"},

		{token.INT, "10"},
		{token.NOT_EQ, "!="},
		{token.INT, "9"},
		{token.SEMICOLON, ";"},

		{token.STRING, "foobar"},
		{token.STRING, "foo \\(bar) baz"},

		{token.LBRACKET, "["},
		{token.INT, "1"},
		{token.COMMA, ","},
		{token.INT, "2"},
		{token.RBRACKET, "]"},
		{token.SEMICOLON, ";"},

		{token.LBRACE, "{"},
		{token.STRING, "foo"},
		{token.COLON, ":"},
		{token.STRING, "bar"},
		{token.RBRACE, "}"},

		{token.WHILE, "while"},
		{token.LPAREN, "("},
		{token.IDENT, "x"},
		{token.LT, "<"},
		{token.INT, "5"},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.LET, "let"},
		{token.IDENT, "x"},
		{token.ASSIGN, "="},
		{token.IDENT, "x"},
		{token.PLUS, "+"},
		{token.INT, "1"},
		{token.SEMICOLON, ";"},
		{token.RBRACE, "}"},

		{token.EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestUnicodeIdentifiers(t *testing.T) {
	input := `let জয় = 100;
let নাম = "নীলাং";
let সংখ্যা১ = ১২৩;`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.LET, "let"},
		{token.IDENT, "জয়"},
		{token.ASSIGN, "="},
		{token.INT, "100"},
		{token.SEMICOLON, ";"},

		{token.LET, "let"},
		{token.IDENT, "নাম"},
		{token.ASSIGN, "="},
		{token.STRING, "নীলাং"},
		{token.SEMICOLON, ";"},

		{token.LET, "let"},
		{token.IDENT, "সংখ্যা১"},
		{token.ASSIGN, "="},
		{token.INT, "১২৩"},
		{token.SEMICOLON, ";"},
		{token.EOF, ""},
	}

	l := New(input)
	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("test[%d] wrong type. expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}
		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("test[%d] wrong literal. expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}

func TestUnterminatedString(t *testing.T) {
	input := `"unterminated string without closing quote`
	l := New(input)
	tok := l.NextToken()
	if tok.Type != token.ILLEGAL {
		t.Fatalf("expected ILLEGAL token for unterminated string, got=%q (%s)", tok.Type, tok.Literal)
	}
}

func TestNestedComments(t *testing.T) {
	input := `/* outer /* inner */ still comment */ let x = 42;`
	l := New(input)
	tok := l.NextToken()
	if tok.Type != token.LET {
		t.Fatalf("expected LET after nested comments, got=%q", tok.Type)
	}
	tok2 := l.NextToken()
	if tok2.Type != token.IDENT || tok2.Literal != "x" {
		t.Fatalf("expected IDENT x, got=%+v", tok2)
	}
}

func TestEscapeSequences(t *testing.T) {
	input := `"hello\nworld\t\"quote\"\\null\0"`
	l := New(input)
	tok := l.NextToken()
	if tok.Type != token.STRING {
		t.Fatalf("expected STRING, got=%q", tok.Type)
	}
	expected := "hello\nworld\t\"quote\"\\null\x00"
	if tok.Literal != expected {
		t.Fatalf("escapes mismatch. expected=%q, got=%q", expected, tok.Literal)
	}
}

func TestMultilineStringLineNumbers(t *testing.T) {
	input := "\"line1\nline2\nline3\";\nidentifier;"
	l := New(input)
	strTok := l.NextToken()
	if strTok.Type != token.STRING {
		t.Fatalf("expected STRING, got=%q", strTok.Type)
	}
	semiTok := l.NextToken()
	if semiTok.Type != token.SEMICOLON {
		t.Fatalf("expected SEMICOLON, got=%q", semiTok.Type)
	}
	idTok := l.NextToken()
	if idTok.Type != token.IDENT {
		t.Fatalf("expected IDENT, got=%q", idTok.Type)
	}
	if idTok.Line != 4 {
		t.Fatalf("expected line 4 for identifier after 3-line string, got line %d", idTok.Line)
	}
}
