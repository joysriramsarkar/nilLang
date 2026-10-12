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

	BIT_AND  = "&"
	BIT_OR   = "|"
	QUESTION = "?"

	// QUESTION_QUESTION is `??`, null coalescing: `a ?? b` is `b` when `a` is
	// null and `a` otherwise.
	//
	// QUESTION_DOT is `?.`, safe navigation: `a?.b` is null when `a` is null
	// and `a.b` otherwise.
	//
	// Both are single tokens so the parser never has to decide whether a second
	// `?` is an operator or the start of another construct. That ambiguity is
	// real: `a ?? b ? c : d` has both readings, and only the token form settles
	// it in the lexer's favour rather than by a special case later.
	QUESTION_QUESTION = "??"
	QUESTION_DOT      = "?."

	// FARROW is `=>`, the arm separator in a `match`. It is lexed as one token
	// so that the parser cannot mistake `=` plus a stray `>` for an assignment
	// followed by a comparison.
	FARROW = "=>"

	// ARROW is `->`, the return-type separator.
	//
	// Without it `fn f() -> Int` would lex as a minus followed by a greater-than,
	// and a negative return type would be indistinguishable from the separator.
	ARROW = "->"

	// PIPE separates the alternatives of a single match pattern.
	//
	// It shares its spelling with bitwise-or, which the language does not yet
	// implement, so the two constants are aliases rather than distinct types.
	// When a bitwise-or operator is added it must get its own token, because
	// `1 | 2` and `1 | 2 | 3 => ...` will then need different precedences.
	PIPE = "|"

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
	BREAK  = "BREAK"
	CONT   = "CONT"
	IN     = "IN"
	RANGE  = "RANGE"
	TASK   = "TASK"
	AWAIT  = "AWAIT"
	IMPORT = "IMPORT"
	EXPORT = "EXPORT"
	AS     = "AS"
	FROM   = "FROM"

	// UI & Declarative Keywords
	COMPONENT = "COMPONENT" // component
	STATE     = "STATE"     // state
	RENDER    = "RENDER"    // render
	EMIT      = "EMIT"      // emit
	ON        = "ON"        // on
	BUILD     = "BUILD"     // build
	STYLE     = "STYLE"     // style
	ENTITY    = "ENTITY"    // entity (web-implications.md Section 26)

	// Declarative type keywords.
	TYPE   = "TYPE"   // type alias
	STRUCT = "STRUCT" // struct declaration
	ENUM   = "ENUM"   // enum declaration
	MATCH  = "MATCH"  // match expression / statement
	TRAIT  = "TRAIT"  // trait declaration
	IMPL   = "IMPL"   // impl block
	SELF   = "SELF"   // self

	// Function modifiers. Each is a prefix on `fn`, so `pure fn f()` reads the
	// way it is written rather than as a modifier keyword placed after the name.
	PURE     = "PURE"     // pure fn
	UNSAFE   = "UNSAFE"   // unsafe fn
	REQUIRES = "REQUIRES" // requires clause

	// PRIVATE is the explicit form of leaving a declaration out of a module's
	// interface. It says the same thing as omitting `export` in a file that uses
	// it, and exists so the intent is visible at the declaration rather than
	// inferred from its absence.
	PRIVATE = "PRIVATE"

	// EXTERN declares a function implemented outside the language, in C.
	//
	// `extern "C" fn sin(x: Float) -> Float` names a symbol the linker resolves
	// rather than one this program defines. The ABI string is written out rather
	// than implied, because "C" is a claim about how the callee is called and a
	// future ABI would otherwise be indistinguishable from a mistake.
	EXTERN = "EXTERN"
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
	"break":     BREAK,
	"continue":  CONT,
	"in":        IN,
	"task":      TASK,
	"await":     AWAIT,
	"import":    IMPORT,
	"export":    EXPORT,
	"as":        AS,
	"from":      FROM,
	"component": COMPONENT,
	"state":     STATE,
	"render":    RENDER,
	"emit":      EMIT,
	"on":        ON,
	"build":     BUILD,
	"style":     STYLE,
	"entity":    ENTITY,
	"type":      TYPE,
	"struct":    STRUCT,
	"enum":      ENUM,
	"match":     MATCH,
	"trait":     TRAIT,
	"impl":      IMPL,
	"self":      SELF,
	"pure":      PURE,
	"unsafe":    UNSAFE,
	"requires":  REQUIRES,
	"private":   PRIVATE,
	"extern":    EXTERN,
}

func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
