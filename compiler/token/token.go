package token

type TokenType string

const (
	ILLEGAL = "ILLEGAL"
	EOF     = "EOF"

	// Identifiers + literals
	IDENT  = "IDENT"  // add, foobar, x, y, ...
	INT    = "INT"    // 1343456
	FLOAT  = "FLOAT"  // 3.14159
	STRING = "STRING" // "hello world"

	// Operators
	ASSIGN   = "="
	PLUS     = "+"
	MINUS    = "-"
	BANG     = "!"
	ASTERISK = "*"
	SLASH    = "/"
	MODULO   = "%"

	LT     = "<"
	GT     = ">"
	LTE    = "<="
	GTE    = ">="
	EQ     = "=="
	NOT_EQ = "!="

	AND = "&&"
	OR  = "||"

	PLUS_ASSIGN  = "+="
	MINUS_ASSIGN = "-="

	// Delimiters
	COMMA     = ","
	SEMICOLON = ";"
	COLON     = ":"
	DOT       = "."

	LPAREN   = "("
	RPAREN   = ")"
	LBRACE   = "{"
	RBRACE   = "}"
	LBRACKET = "["
	RBRACKET = "]"

	// Keywords
	LET    = "LET"
	CONST  = "CONST"
	FN     = "FN"
	TRUE   = "TRUE"
	FALSE  = "FALSE"
	NULL   = "NULL"
	IF     = "IF"
	ELSE   = "ELSE"
	RETURN = "RETURN"
	WHILE  = "WHILE"
	FOR    = "FOR"
	IMPORT = "IMPORT"
	EXPORT = "EXPORT"

	// UI & Declarative Keywords
	COMPONENT = "COMPONENT" // component
	STATE     = "STATE"     // state
	RENDER    = "RENDER"    // render
	EMIT      = "EMIT"      // emit
	ON        = "ON"        // on
	BUILD     = "BUILD"     // build
	STYLE     = "STYLE"     // style
)

type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
}

var keywords = map[string]TokenType{
	"let":       LET,
	"const":     CONST,
	"fn":        FN,
	"true":      TRUE,
	"false":     FALSE,
	"null":      NULL,
	"if":        IF,
	"else":      ELSE,
	"return":    RETURN,
	"while":     WHILE,
	"for":       FOR,
	"import":    IMPORT,
	"export":    EXPORT,
	"component": COMPONENT,
	"state":     STATE,
	"render":    RENDER,
	"emit":      EMIT,
	"on":        ON,
	"build":     BUILD,
	"style":     STYLE,
}

func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
