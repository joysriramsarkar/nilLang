package parser

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/diagnostics"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/token"
)

const (
	_ int = iota
	LOWEST
	RANGE_PREC  // .. and ..=
	TERNARY     // ? :
	OR          // ||
	COALESCE    // ??
	AND         // &&
	EQUALS      // == or !=
	LESSGREATER // > or < or >= or <=
	SUM         // + or -
	PRODUCT     // * or / or %
	PREFIX      // -X or !X
	CALL        // myFunction(X)
	INDEX       // array[index]
)

var precedences = map[token.TokenType]int{
	token.RANGE:             RANGE_PREC,
	token.QUESTION:          TERNARY,
	token.QUESTION_QUESTION: COALESCE,
	token.OR:                OR,
	token.AND:               AND,
	token.EQ:                EQUALS,
	token.NOT_EQ:            EQUALS,
	token.LT:                LESSGREATER,
	token.GT:                LESSGREATER,
	token.LTE:               LESSGREATER,
	token.GTE:               LESSGREATER,
	token.PLUS:              SUM,
	token.MINUS:             SUM,
	token.SLASH:             PRODUCT,
	token.ASTERISK:          PRODUCT,
	token.MODULO:            PRODUCT,
	token.LPAREN:            CALL,
	token.LBRACKET:          INDEX,
	token.DOT:               INDEX,
	token.QUESTION_DOT:      INDEX,
}

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(ast.Expression) ast.Expression
)

type Parser struct {
	l      *lexer.Lexer
	errors []string

	// diagnostics carries the structured, coded form of every parse failure.
	// It is the preferred machine-readable output; errors is the legacy string
	// form kept for CLI text and existing conformance assertions.
	diagnostics []*diagnostics.Diagnostic

	// filename and sourceLines enrich diagnostic spans with file name and the
	// quoted offending line. Both are optional.
	filename    string
	sourceLines []string

	// inTraitBody is set while a trait or impl block is being parsed.
	//
	// Inside one, a method may be written without a body, because which of the
	// two it is depends on the block rather than on the signature: a trait holds
	// signatures, an impl holds bodies. Everywhere else a missing body is an
	// error, and the flag is cleared before a body is parsed so that a function
	// declared inside a method still needs its own.
	inTraitBody bool

	curToken  token.Token
	peekToken token.Token

	// conditionDepth counts the enclosing positions where a `{` after an
	// expression must be read as the start of a body rather than as a struct
	// literal.
	//
	// `while x { }` and `for i in xs { }` would otherwise parse `x` as the name
	// of a struct literal and swallow the loop body. The grammar genuinely
	// cannot tell the two apart from the tokens alone, so the constructs that
	// introduce a body say which reading they mean. It is a counter rather than
	// a flag because a condition can contain another condition.
	conditionDepth int

	prefixParseFns map[token.TokenType]prefixParseFn
	infixParseFns  map[token.TokenType]infixParseFn
}

// parseCondition parses an expression in a position where a following `{` opens
// a body rather than a struct literal.
func (p *Parser) parseCondition() ast.Expression {
	p.conditionDepth++
	expr := p.parseExpression(LOWEST)
	p.conditionDepth--
	return expr
}

// skipToBrace advances past the end of a brace-delimited construct.
//
// A structural error inside `struct`, `enum`, or a `match` arm list otherwise
// leaves the parser standing on a field separator or a stray operator, and every
// following field produces another error. The developer sees a page of noise
// where one mistake was written, and the first message is no longer the one that
// explains it.
//
// Resynchronising at the closing brace is the usual choice: every construct that
// uses it is either fully inside or fully outside, so the brace is always the
// first token that means the same thing to the parser again.
func (p *Parser) skipToBrace() {
	depth := 0
	for !p.curTokenIs(token.EOF) {
		switch p.curToken.Type {
		case token.LBRACE:
			depth++
		case token.RBRACE:
			depth--
			if depth <= 0 {
				return
			}
		}
		p.nextToken()
	}
}

// parseSoftKeywordStatement handles a statement that begins with a soft keyword.
//
// The keyword is an ordinary name in every position except the one that makes
// it a declaration, so unsafe = 1 is an assignment and a bare unsafe is a
// read.
func (p *Parser) parseSoftKeywordStatement() ast.Statement {
	if p.peekTokenIs(token.ASSIGN) || p.peekTokenIs(token.PLUS_ASSIGN) || p.peekTokenIs(token.MINUS_ASSIGN) {
		return p.parseAssignStatement()
	}
	return p.parseExpressionStatement()
}

// softKeywords are the reserved words that are still usable as ordinary names.
//
// `type` is the only one today: it introduces a type alias when a name follows
// it, and is otherwise an identifier, so `let type = "widget"` keeps working. A
// hard keyword would take the name away from every developer who already uses
// it, in exchange for disambiguation the parser does not actually need.
var softKeywords = map[token.TokenType]bool{
	token.TYPE: true,

	// The function modifiers are soft for the same reason. unsafe is a
	// plausible variable name in a systems-adjacent language, and a hard keyword
	// would take the name away for no benefit.
	token.PURE:     true,
	token.UNSAFE:   true,
	token.REQUIRES: true,

	// `extern` is soft for the same reason: `extern "C" fn ...` is a declaration
	// and `extern = 1` is an assignment to a variable that happens to be named
	// `extern`. Requiring a string after the keyword is what tells them apart, so
	// reserving the word would take a usable name away and disambiguate nothing.
	token.EXTERN: true,

	// `self` is soft for the same reason. A method's receiver is written as an
	// ordinary first parameter, so `fn area(self: Box)` parses like any other
	// signature and `self` stays available as a name outside a method. It was a
	// hard keyword only in anticipation of a receiver form; now that the
	// receiver *is* a parameter, there is no syntax left for it to disambiguate.
	token.SELF: true,
}

// expectPeekName expects an identifier in a position where a new name is being
// introduced: a `let` binding, a loop variable, a parameter.
//
// The error names the keyword so that `let type = 1;` says what went wrong
// rather than reporting that an identifier was expected and found a TYPE, which
// reads as though the developer had never seen the word.
func (p *Parser) expectPeekName() bool {
	if p.peekTokenIs(token.IDENT) || softKeywords[p.peekToken.Type] {
		p.nextToken()
		return true
	}
	p.peekError(token.IDENT)
	return false
}

func New(l *lexer.Lexer) *Parser {
	return NewWithSource(l, "")
}

// NewWithSource builds a parser that also knows the original source text, so
// diagnostics can quote the offending line. Front ends should prefer this over
// New so that rendered errors include context.
func NewWithSource(l *lexer.Lexer, src string) *Parser {
	p := &Parser{
		l:           l,
		errors:      []string{},
		diagnostics: []*diagnostics.Diagnostic{},
	}
	p.captureSource(src)

	p.prefixParseFns = make(map[token.TokenType]prefixParseFn)
	p.registerPrefix(token.IDENT, p.parseIdentifier)
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.FLOAT, p.parseFloatLiteral)
	p.registerPrefix(token.STRING, p.parseStringLiteral)
	p.registerPrefix(token.BANG, p.parsePrefixExpression)
	p.registerPrefix(token.MINUS, p.parsePrefixExpression)
	p.registerPrefix(token.TRUE, p.parseBoolean)
	p.registerPrefix(token.FALSE, p.parseBoolean)
	p.registerPrefix(token.NULL, p.parseNullLiteral)
	p.registerPrefix(token.LPAREN, p.parseGroupedExpression)
	p.registerPrefix(token.IF, p.parseIfExpression)
	p.registerPrefix(token.FN, p.parseFunctionLiteral)
	p.registerPrefix(token.TASK, p.parseTaskExpression)
	p.registerPrefix(token.AWAIT, p.parseAwaitExpression)
	p.registerPrefix(token.LBRACKET, p.parseArrayLiteral)
	p.registerPrefix(token.LBRACE, p.parseHashLiteral)
	p.registerPrefix(token.PURE, p.parseModifiedFunction)
	p.registerPrefix(token.UNSAFE, p.parseModifiedFunction)
	p.registerPrefix(token.MATCH, p.parseMatchExpression)
	p.registerPrefix(token.TYPE, p.parseKeywordIdentifier)
	p.registerPrefix(token.REQUIRES, p.parseKeywordIdentifier)
	// `extern` reaches an expression position only when it is not followed by a
	// string, which `parseStatement` has already decided by the time this is
	// reached. Registering it as an identifier is what lets `let extern = 1`
	// and a bare `extern;` read as the ordinary names they are.
	p.registerPrefix(token.EXTERN, p.parseKeywordIdentifier)
	p.registerPrefix(token.SELF, p.parseSelf)
	p.registerPrefix(token.COMPONENT, p.parseComponentLiteralExpression)
	p.registerPrefix(token.EMIT, p.parseKeywordIdentifier)

	p.infixParseFns = make(map[token.TokenType]infixParseFn)
	p.registerInfix(token.PLUS, p.parseInfixExpression)
	p.registerInfix(token.MINUS, p.parseInfixExpression)
	p.registerInfix(token.SLASH, p.parseInfixExpression)
	p.registerInfix(token.ASTERISK, p.parseInfixExpression)
	p.registerInfix(token.MODULO, p.parseInfixExpression)
	p.registerInfix(token.EQ, p.parseInfixExpression)
	p.registerInfix(token.NOT_EQ, p.parseInfixExpression)
	p.registerInfix(token.LT, p.parseInfixExpression)
	p.registerInfix(token.GT, p.parseInfixExpression)
	p.registerInfix(token.LTE, p.parseInfixExpression)
	p.registerInfix(token.GTE, p.parseInfixExpression)
	p.registerInfix(token.AND, p.parseInfixExpression)
	p.registerInfix(token.OR, p.parseInfixExpression)
	p.registerInfix(token.QUESTION, p.parseTernaryExpression)
	p.registerInfix(token.QUESTION_QUESTION, p.parseCoalesceExpression)
	p.registerInfix(token.QUESTION_DOT, p.parseSafeAccess)
	p.registerInfix(token.LPAREN, p.parseCallExpression)
	p.registerInfix(token.LBRACKET, p.parseIndexExpression)
	p.registerInfix(token.DOT, p.parseDotExpression)
	p.registerInfix(token.RANGE, p.parseRangeExpression)

	// Read two tokens, so curToken and peekToken are both set
	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) peekError(t token.TokenType) {
	msg := fmt.Sprintf("expected next token to be %s, got %s instead", t, p.peekToken.Type)
	if p.peekToken.Type == token.EOF {
		p.recordError(CodeUnexpectedEOF, p.peekToken, msg,
			fmt.Sprintf("the parser reached the end of the file while expecting %s", t))
		return
	}
	p.recordError(CodeSyntaxError, p.peekToken, msg,
		fmt.Sprintf("insert %s here, or remove the unexpected %s", t, p.peekToken.Type))
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) curTokenIs(t token.TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) expectPeek(t token.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.peekError(t)
	return false
}

func (p *Parser) peekPrecedence() int {
	if prec, ok := precedences[p.peekToken.Type]; ok {
		return prec
	}
	return LOWEST
}

func (p *Parser) curPrecedence() int {
	if prec, ok := precedences[p.curToken.Type]; ok {
		return prec
	}
	return LOWEST
}

func (p *Parser) registerPrefix(tokenType token.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

func (p *Parser) registerInfix(tokenType token.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	program.Statements = []ast.Statement{}

	for !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		p.appendStatement(&program.Statements, stmt)
		p.nextToken()
	}

	return program
}

// appendStatement collects one parsed statement, dropping nils.
//
// A failed declaration often comes back as a *typed* nil —
// (*ast.StructDecl)(nil) stored in an ast.Statement — which compares unequal
// to nil and would otherwise flow downstream, where the first method call on
// it panics far from the malformed source. Reflection is deliberate here: a
// type switch would have to name every statement type, and the next node kind
// someone adds would reopen the hole silently.
//
// A dropped statement with no recorded error becomes one: losing source
// quietly is worse than noise.
func (p *Parser) appendStatement(dst *[]ast.Statement, stmt ast.Statement) {
	if stmt == nil || isNilStatement(stmt) {
		if len(p.Errors()) == 0 {
			p.recordError(CodeSyntaxError, p.curToken,
				"could not parse a statement here",
				"the surrounding code is incomplete")
		}
		return
	}
	*dst = append(*dst, stmt)
}

// isNilStatement reports whether s is nil or a typed-nil pointer, which is
// what a recovery path returns when it gives up mid-declaration.
func isNilStatement(s ast.Statement) bool {
	if s == nil {
		return true
	}
	v := reflect.ValueOf(s)
	return v.Kind() == reflect.Ptr && v.IsNil()
}

func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
	case token.EXPORT, token.PRIVATE:
		return p.parseExportStatement()
	case token.IMPORT:
		return p.parseImportStatement()
	case token.STATE:
		return p.parseStateDeclaration()
	case token.LET, token.CONST:
		return p.parseLetStatement()
	case token.RETURN:
		return p.parseReturnStatement()
	case token.WHILE:
		return p.parseWhileStatement()
	case token.TYPE:
		// `type` is a soft keyword: it introduces an alias only when a name
		// follows. `let type = "widget"` and `type = "widget"` stay valid,
		// because naming a variable `type` is ordinary and a hard keyword would
		// take that name away for no benefit.
		if p.peekTokenIs(token.IDENT) {
			return p.parseTypeAlias()
		}
		return p.parseSoftKeywordStatement()

	case token.PURE, token.UNSAFE, token.REQUIRES:
		// The same reasoning as `type`: `pure fn f()` is a declaration, while
		// `unsafe = 1` is an assignment to a variable that happens to be named
		// `unsafe`.
		return p.parseSoftKeywordStatement()

	case token.EXTERN:
		// `extern` is soft for the same reason. `extern "C" fn ...` is a
		// declaration; `extern = 1` is an assignment to a variable that happens to
		// be named `extern`, and `let extern = "libc"` is a perfectly ordinary
		// binding. Requiring a string after the keyword is what separates them, and
		// a name worth taking away from every program is not worth one keyword's
		// worth of disambiguation.
		if p.peekTokenIs(token.STRING) {
			return p.parseExternStatement()
		}
		return p.parseSoftKeywordStatement()

	case token.STRUCT:
		return p.parseStructDecl()
	case token.TRAIT:
		return p.parseTraitDecl()
	case token.IMPL:
		return p.parseImplDecl()
	case token.ENUM:
		return p.parseEnumDecl()
	case token.FOR:
		return p.parseForStatement()
	case token.BREAK:
		return p.parseBreakStatement()
	case token.CONT:
		return p.parseContinueStatement()
	case token.COMPONENT:
		if p.peekTokenIs(token.IDENT) {
			return p.parseComponentDeclaration()
		}
		return p.parseExpressionStatement()
	case token.ENTITY:
		return p.parseEntityStatement()
	case token.STYLE:
		return p.parseStyleStatement()
	case token.IDENT:
		if p.curToken.Literal == "app" && (p.peekTokenIs(token.LBRACE) || p.peekTokenIs(token.IDENT)) {
			return p.parseAppStatement()
		}
		if p.peekTokenIs(token.ASSIGN) || p.peekTokenIs(token.PLUS_ASSIGN) || p.peekTokenIs(token.MINUS_ASSIGN) {
			return p.parseAssignStatement()
		}
		return p.parseExpressionStatement()
	default:
		return p.parseExpressionStatement()
	}
}

func (p *Parser) parseAppStatement() *ast.AppStatement {
	stmt := &ast.AppStatement{Token: p.curToken}
	if p.peekTokenIs(token.IDENT) {
		p.nextToken()
		stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}
	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	stmt.Body = p.parseBlockStatement()
	return stmt
}

func (p *Parser) parseStateDeclaration() *ast.StateDeclaration {
	stmt := &ast.StateDeclaration{Token: p.curToken}

	if !p.expectPeekName() {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// Optional type: state count: i32 = 0
	if p.peekTokenIs(token.COLON) {
		p.nextToken() // cur is :
		if p.peekTokenIs(token.ASSIGN) || p.peekTokenIs(token.SEMICOLON) {
			p.peekError(token.IDENT)
			return nil
		}
		stmt.Type = p.parseTypeAnnotation()
		if stmt.Type == "" {
			p.peekError(token.IDENT)
			return nil
		}
	}

	if p.peekTokenIs(token.ASSIGN) {
		p.nextToken() // cur is =
		p.nextToken() // start of expr
		stmt.Value = p.parseExpression(LOWEST)
	}

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseEntityStatement() *ast.EntityStatement {
	stmt := &ast.EntityStatement{Token: p.curToken, Fields: []ast.EntityField{}}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	seenFields := make(map[string]bool)

	p.nextToken() // move inside {

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.SEMICOLON) {
			p.nextToken()
			continue
		}

		if p.curTokenIs(token.IDENT) {
			fieldName := p.curToken.Literal
			if seenFields[fieldName] {
				p.recordError(CodeEntityDuplicateField, p.curToken,
					fmt.Sprintf("duplicate field %q in entity %q", fieldName, stmt.Name.Value),
					fmt.Sprintf("an entity may declare each field name once; %q is declared twice", fieldName))
			}
			seenFields[fieldName] = true

			field := ast.EntityField{Name: fieldName}

			if p.peekTokenIs(token.COLON) {
				p.nextToken() // cur is :
				if p.peekTokenIs(token.IDENT) {
					p.nextToken() // cur is type
					field.Type = p.curToken.Literal
					if p.peekTokenIs(token.LT) {
						p.nextToken()
						genericInner := ""
						for !p.peekTokenIs(token.GT) && !p.peekTokenIs(token.EOF) {
							p.nextToken()
							genericInner += p.curToken.Literal
						}
						if genericInner == "" {
							p.recordError(CodeSyntaxError, p.curToken,
								fmt.Sprintf("empty generic type parameter for %q", field.Type),
								"write a type name inside the angle brackets, for example List<String>")
							return nil
						}
						if !p.expectPeek(token.GT) {
							return nil
						}
						field.Type += "<" + genericInner + ">"
					}
				}
			}

			// Parse optional field modifiers: primary, required, unique, relation -> Target
			for (p.peekTokenIs(token.IDENT) && isEntityFieldModifier(p.peekToken.Literal)) || p.peekTokenIs(token.MINUS) {
				p.nextToken()
				switch p.curToken.Literal {
				case "primary":
					field.IsPrimary = true
				case "required":
					field.IsRequired = true
				case "unique":
					field.IsUnique = true
				case "relation":
					if p.peekTokenIs(token.MINUS) {
						p.nextToken()
						if p.peekTokenIs(token.GT) {
							p.nextToken()
							if p.peekTokenIs(token.IDENT) {
								p.nextToken()
								field.TargetEntity = p.curToken.Literal
							}
						}
					} else if p.peekTokenIs(token.IDENT) {
						p.nextToken()
						field.TargetEntity = p.curToken.Literal
					}
				}
			}

			stmt.Fields = append(stmt.Fields, field)
		}

		p.nextToken()
	}

	return stmt
}

func isEntityFieldModifier(lit string) bool {
	return lit == "primary" || lit == "required" || lit == "unique" || lit == "relation"
}

func (p *Parser) parseTypeAnnotation() string {
	var parts []string
	parenDepth := 0
	angleDepth := 0

	for !p.peekTokenIs(token.EOF) {
		if parenDepth == 0 && angleDepth == 0 {
			if p.peekTokenIs(token.ASSIGN) || p.peekTokenIs(token.SEMICOLON) ||
				p.peekTokenIs(token.COMMA) || p.peekTokenIs(token.RBRACE) ||
				p.peekTokenIs(token.RPAREN) || p.peekTokenIs(token.LBRACE) ||
				p.peekTokenIs(token.REQUIRES) ||
				(p.peekTokenIs(token.IDENT) && isEntityFieldModifier(p.peekToken.Literal)) {
				break
			}
		}
		p.nextToken()
		if p.curTokenIs(token.LPAREN) {
			parenDepth++
		} else if p.curTokenIs(token.RPAREN) {
			if parenDepth > 0 {
				parenDepth--
			}
		} else if p.curTokenIs(token.LT) {
			angleDepth++
		} else if p.curTokenIs(token.GT) {
			if angleDepth > 0 {
				angleDepth--
			}
		}
		parts = append(parts, p.curToken.Literal)
		if parenDepth == 0 && angleDepth == 0 {
			if p.peekTokenIs(token.ASSIGN) || p.peekTokenIs(token.SEMICOLON) ||
				p.peekTokenIs(token.COMMA) || p.peekTokenIs(token.RBRACE) ||
				p.peekTokenIs(token.RPAREN) || p.peekTokenIs(token.LBRACE) ||
				p.peekTokenIs(token.REQUIRES) ||
				(p.peekTokenIs(token.IDENT) && isEntityFieldModifier(p.peekToken.Literal)) {
				break
			}
		}
	}
	res := strings.Join(parts, "")
	if strings.Contains(res, "<>") {
		p.recordError(CodeSyntaxError, p.curToken,
			fmt.Sprintf("invalid empty generic type parameter in %q", res),
			"write a type name inside the angle brackets, for example List<String>")
		return ""
	}
	return res
}

// parseExportStatement parses a visibility keyword followed by a declaration.
//
// Both forms wrap the declaration rather than changing it, so every consumer
// that wants the declaration reads it through ast.Unwrap and behaves as if the
// wrapper were not there.
func (p *Parser) parseExportStatement() *ast.ExportStatement {
	visibility := ast.VisibilityExported
	if p.curTokenIs(token.PRIVATE) {
		visibility = ast.VisibilityPrivate
	}

	stmt := &ast.ExportStatement{Token: p.curToken, Visibility: visibility}

	if !p.peekStartsDeclaration() {
		p.recordError(CodeSyntaxError, p.curToken,
			fmt.Sprintf("a visibility keyword must be followed by a declaration, got %s", p.peekToken.Type),
			"write it as export fn name(...), export let name = ..., or export struct Name { ... }")
		return nil
	}
	p.nextToken()

	stmt.Decl = p.parseStatement()
	if stmt.Decl == nil {
		return nil
	}

	// A declaration that failed to parse arrives here as a typed nil, which is
	// not `== nil` as an interface and so slips past the check above. It has
	// already produced a diagnostic that names the real problem; adding a second
	// one about visibility would point at the wrong mistake.
	if ast.DeclaredName(stmt.Decl) == "" {
		if len(p.Errors()) == 0 {
			p.recordError(CodeSyntaxError, p.curToken,
				fmt.Sprintf("a declaration needs a name to be marked %q, and this one has none", string(visibility)),
				"a module exports names, so only a named declaration can be marked")
		}
		return nil
	}

	return stmt
}

// peekStartsDeclaration reports whether the next token begins a declaration that
// can carry a visibility marker.
func (p *Parser) peekStartsDeclaration() bool {
	switch p.peekToken.Type {
	case token.FN, token.LET, token.CONST, token.STRUCT, token.ENUM, token.TYPE,
		token.PURE, token.UNSAFE:
		return true
	default:
		return false
	}
}

func (p *Parser) parseImportStatement() *ast.ImportStatement {
	stmt := &ast.ImportStatement{Token: p.curToken, Names: []*ast.Identifier{}}

	// Case 1: import { Button, Text } from "alap/web"
	if p.peekTokenIs(token.LBRACE) {
		p.nextToken() // cur is {
		for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
			p.nextToken()
			if p.curTokenIs(token.IDENT) {
				stmt.Names = append(stmt.Names, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})
			}
			if p.peekTokenIs(token.COMMA) {
				p.nextToken()
			}
		}
		if p.curTokenIs(token.RBRACE) {
			p.nextToken() // past }
		}
		if p.curTokenIs(token.FROM) || (p.curTokenIs(token.IDENT) && p.curToken.Literal == "from") {
			p.nextToken() // past from
		}
		if p.curTokenIs(token.STRING) {
			stmt.Path = &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
		}
	} else if p.peekTokenIs(token.STRING) {
		p.nextToken()
		stmt.Path = &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
	} else {
		p.peekError(token.STRING)
		return nil
	}

	// Optional: as <alias>
	if p.peekTokenIs(token.AS) || (p.peekTokenIs(token.IDENT) && p.peekToken.Literal == "as") {
		p.nextToken() // at "as"
		if p.peekTokenIs(token.IDENT) {
			p.nextToken()
			stmt.Alias = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		}
	}

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseLetStatement() *ast.LetStatement {
	stmt := &ast.LetStatement{Token: p.curToken, Constant: p.curTokenIs(token.CONST)}

	if !p.expectPeekName() {
		return nil
	}

	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if p.peekTokenIs(token.COLON) {
		p.nextToken()
		if p.peekTokenIs(token.ASSIGN) || p.peekTokenIs(token.SEMICOLON) {
			p.peekError(token.IDENT)
			return nil
		}
		stmt.Type = p.parseTypeAnnotation()
		if stmt.Type == "" {
			p.peekError(token.IDENT)
			return nil
		}
	}

	if p.peekTokenIs(token.ASSIGN) {
		p.nextToken()
		p.nextToken()
		stmt.Value = p.parseExpression(LOWEST)
	} else if stmt.Constant {
		p.peekError(token.ASSIGN)
		return nil
	}

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	stmt := &ast.ReturnStatement{Token: p.curToken}

	p.nextToken()

	stmt.ReturnValue = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseAssignStatement() *ast.AssignStatement {
	stmt := &ast.AssignStatement{Token: p.curToken}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.peekTokenIs(token.ASSIGN) && !p.peekTokenIs(token.PLUS_ASSIGN) && !p.peekTokenIs(token.MINUS_ASSIGN) {
		p.peekError(token.ASSIGN)
		return nil
	}
	p.nextToken()
	stmt.Operator = p.curToken.Literal

	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

// parseForStatement parses `for <ident> in <iterable> { <body> }` and the
// two-variable form `for <key>, <value> in <iterable> { <body> }`.
//
// The loop variables are declared by the loop, so they are scoped to the body
// and a fresh binding appears per iteration. That scoping is what lets the same
// closure capture be created on each pass without the previous iteration's value
// leaking into it.
func (p *Parser) parseForStatement() *ast.ForStatement {
	stmt := &ast.ForStatement{Token: p.curToken}

	if !p.expectPeekName() {
		return nil
	}
	first := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// A comma introduces the `for key, value in ...` form. The first name is the
	// key (the index) and the second is the value, matching the order they are
	// written in.
	if p.peekTokenIs(token.COMMA) {
		p.nextToken() // cur is ','
		if !p.expectPeekName() {
			return nil
		}
		stmt.Key = first
		stmt.Value = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	} else {
		// With one name, that name is the value and the key mirrors it, so a
		// consumer can always read both fields without a nil check.
		stmt.Value = first
		stmt.Key = first
	}

	if !p.expectPeek(token.IN) {
		p.recordError(CodeSyntaxError, p.curToken,
			fmt.Sprintf("expected `in` after the loop variable, got %s", p.curToken.Type),
			"write the loop as `for item in collection`, naming what to iterate")
		return nil
	}

	// The iterable comes next, then the body. Parentheses are optional so that
	// `for i in (0..n) { }` and `for i in 0..n { }` both work.
	p.nextToken()
	if p.curTokenIs(token.LPAREN) {
		p.nextToken()
		stmt.Iterable = p.parseExpression(LOWEST)
		if !p.expectPeek(token.RPAREN) {
			return nil
		}
	} else {
		// Not parenthesised, so the { that follows opens the body: or i in
		// xs { } must not read xs as the start of a struct literal.
		stmt.Iterable = p.parseCondition()
	}
	if stmt.Iterable == nil {
		return nil
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	stmt.Body = p.parseBlockStatement()

	return stmt
}

func (p *Parser) parseBreakStatement() *ast.BreakStatement {
	stmt := &ast.BreakStatement{Token: p.curToken}
	p.consumeOptionalSemicolon()
	return stmt
}

func (p *Parser) parseContinueStatement() *ast.ContinueStatement {
	stmt := &ast.ContinueStatement{Token: p.curToken}
	p.consumeOptionalSemicolon()
	return stmt
}

// consumeOptionalSemicolon advances past a trailing `;` when there is one.
//
// The statement list loop advances once after every statement, so leaving the
// semicolon unconsumed would make the loop see `;` as the next statement and try
// to parse it as an expression.
func (p *Parser) consumeOptionalSemicolon() {
	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}
}

// parseRangeExpression parses `start..end` and `start..=end`.
//
// It has the infix signature so that the range binds to the expression on its
// left, which is what lets `0..n - 1` mean `0..(n - 1)` rather than `(0..n) - 1`.
func (p *Parser) parseRangeExpression(left ast.Expression) ast.Expression {
	// The caller has already advanced onto the operator, so cur is '..' or
	// '..=' here. Advancing again would skip the operator entirely and land on
	// the right operand's first token.
	rangeToken := p.curToken

	p.nextToken()
	right := p.parseExpression(LOWEST)

	return &ast.RangeExpression{
		Token:     rangeToken,
		Start:     left,
		End:       right,
		Inclusive: strings.HasSuffix(rangeToken.Literal, "="),
	}
}

func (p *Parser) parseWhileStatement() ast.Statement {
	stmt := &ast.WhileStatement{Token: p.curToken}

	// `while let` binds a pattern and repeats for as long as it matches. It is
	// checked first because `let` is a keyword and would otherwise be read as
	// the start of a condition that does not exist.
	if p.peekTokenIs(token.LET) {
		return p.parseWhileLet()
	}

	if p.peekTokenIs(token.LPAREN) {
		p.nextToken()
		p.nextToken()
		stmt.Condition = p.parseCondition()
		if !p.expectPeek(token.RPAREN) {
			return nil
		}
	} else {
		p.nextToken()
		stmt.Condition = p.parseCondition()
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	stmt.Body = p.parseBlockStatement()

	return stmt
}

func (p *Parser) parseExpressionStatement() ast.Statement {
	stmt := &ast.ExpressionStatement{Token: p.curToken}

	expr := p.parseExpression(LOWEST)

	if p.peekTokenIs(token.ASSIGN) {
		p.nextToken() // cur is =
		p.nextToken() // cur is start of value
		val := p.parseExpression(LOWEST)
		if p.peekTokenIs(token.SEMICOLON) {
			p.nextToken()
		}
		if idxExpr, ok := expr.(*ast.IndexExpression); ok {
			return &ast.IndexAssignStatement{
				Token: p.curToken,
				Left:  idxExpr.Left,
				Index: idxExpr.Index,
				Value: val,
			}
		}
		if dotExpr, ok := expr.(*ast.DotExpression); ok {
			return &ast.IndexAssignStatement{
				Token: p.curToken,
				Left:  dotExpr.Left,
				Index: &ast.StringLiteral{
					Token: dotExpr.Member.Token,
					Value: dotExpr.Member.Value,
				},
				Value: val,
			}
		}
		if idExpr, ok := expr.(*ast.Identifier); ok {
			return &ast.AssignStatement{
				Token: idExpr.Token,
				Name:  idExpr,
				Value: val,
			}
		}
		p.recordError(CodeSyntaxError, p.curToken, "invalid assignment target",
			"the left side of = must be an identifier, an index expression such as a[i], or a member such as a.b")
		return nil
	}

	stmt.Expression = expr

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseExpression(precedence int) ast.Expression {
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}
	leftExp := prefix()

	for !p.peekTokenIs(token.SEMICOLON) && precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}

		p.nextToken()

		leftExp = infix(leftExp)
	}

	return leftExp
}

func (p *Parser) noPrefixParseFnError(t token.TokenType) {
	if t == token.EOF {
		p.recordError(CodeUnexpectedEOF, p.curToken, "unexpected end of file",
			"an expression was expected here but the file ended")
		return
	}
	p.recordError(CodeUnexpectedToken, p.curToken,
		fmt.Sprintf("no prefix parse function for %s found", t),
		fmt.Sprintf("%s cannot start an expression; remove it or place a value before it", t))
}

// parseIdentifier parses a name, or the value-producing construct that starts
// with one.
//
// An identifier followed by `{` is a struct literal and one followed by `(` may
// be an enum variant. Deciding here rather than in the infix chain keeps
// `User { ... }` from parsing as an identifier followed by a block, and keeps
// the call path for ordinary calls unchanged.
func (p *Parser) parseIdentifier() ast.Expression {
	if p.conditionDepth == 0 && p.peekTokenIs(token.LBRACE) {
		return p.parseStructLiteral()
	}
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

// parseSelf reads `self` as the receiver name.
//
// It is a soft keyword for the same reason `type` and `pure` are: a method's
// receiver is written as an ordinary first parameter, so `self` is an ordinary
// name and reading it as one costs nothing. Turning it back into a reserved word
// would buy a syntax that no longer exists to be disambiguated from.
func (p *Parser) parseSelf() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: "self"}
}

func (p *Parser) parseKeywordIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	lit := &ast.IntegerLiteral{Token: p.curToken}

	// Bengali (০-৯) and fullwidth digits are accepted, as are '_' separators and
	// 0x/0o/0b radix prefixes. See compiler/parser/numerals.go.
	value, err := parseIntLiteral(p.curToken.Literal)
	if err != nil {
		p.recordError(CodeUnexpectedToken, p.curToken,
			fmt.Sprintf("could not parse %q as integer", p.curToken.Literal),
			"an integer literal may use Bengali or ASCII digits, '_' separators, and a 0x/0o/0b radix prefix")
		return nil
	}

	lit.Value = value
	return lit
}

func (p *Parser) parseFloatLiteral() ast.Expression {
	lit := &ast.FloatLiteral{Token: p.curToken}

	value, err := parseFloatLiteral(p.curToken.Literal)
	if err != nil {
		p.recordError(CodeUnexpectedToken, p.curToken,
			fmt.Sprintf("could not parse %q as float", p.curToken.Literal),
			"a float literal needs a fractional part after '.', for example 1.5 or ১.৫")
		return nil
	}

	lit.Value = value
	return lit
}

func (p *Parser) parseStringLiteral() ast.Expression {
	rawValue := p.curToken.Literal

	if !strings.Contains(rawValue, "\\(") {
		return &ast.StringLiteral{Token: p.curToken, Value: rawValue}
	}

	return p.parseStringTemplate(rawValue)
}

func (p *Parser) parseStringTemplate(raw string) ast.Expression {
	template := &ast.StringTemplate{Token: p.curToken, Parts: []ast.TemplatePart{}}

	i := 0
	for i < len(raw) {
		interpStart := strings.Index(raw[i:], "\\(")
		if interpStart == -1 {
			if i < len(raw) {
				template.Parts = append(template.Parts, ast.TemplatePart{
					IsExpression: false,
					Literal:      raw[i:],
				})
			}
			break
		}

		if interpStart > 0 {
			template.Parts = append(template.Parts, ast.TemplatePart{
				IsExpression: false,
				Literal:      raw[i : i+interpStart],
			})
		}

		exprStart := i + interpStart + 2
		parenDepth := 1
		exprEnd := exprStart

		for exprEnd < len(raw) && parenDepth > 0 {
			switch raw[exprEnd] {
			case '(':
				parenDepth++
			case ')':
				parenDepth--
			}
			if parenDepth > 0 {
				exprEnd++
			}
		}

		exprStr := raw[exprStart:exprEnd]
		exprAST := p.parseInterpolatedExpression(exprStr)
		if exprAST != nil {
			template.Parts = append(template.Parts, ast.TemplatePart{
				IsExpression: true,
				Expression:   exprAST,
			})
		}

		i = exprEnd + 1
	}

	return template
}

func (p *Parser) parseInterpolatedExpression(exprStr string) ast.Expression {
	subLexer := lexer.New(exprStr)
	subParser := New(subLexer)

	expr := subParser.parseExpression(LOWEST)
	if len(subParser.Errors()) > 0 {
		p.errors = append(p.errors, subParser.Errors()...)
		// Re-anchor sub-parser diagnostics onto the template's opening token so
		// the reported span points at the interpolation site in the outer file
		// rather than at a synthetic offset inside the extracted fragment.
		for _, d := range subParser.Diagnostics() {
			merged := *d
			merged.Span = diagnostics.Span{
				Filename:  p.filename,
				StartLine: p.curToken.Line,
				StartCol:  p.curToken.Column,
				EndLine:   p.curToken.Line,
				EndCol:    p.curToken.Column + 1,
			}
			merged.ContextLine = p.lineFor(p.curToken.Line)
			p.diagnostics = append(p.diagnostics, &merged)
		}
		return nil
	}

	return expr
}

func (p *Parser) parsePrefixExpression() ast.Expression {
	expression := &ast.PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
	}

	p.nextToken()

	expression.Right = p.parseExpression(PREFIX)

	return expression
}

func (p *Parser) parseTaskExpression() ast.Expression {
	expression := &ast.TaskExpression{Token: p.curToken}
	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	expression.Body = p.parseBlockStatement()
	return expression
}

func (p *Parser) parseAwaitExpression() ast.Expression {
	expression := &ast.AwaitExpression{Token: p.curToken}
	p.nextToken()
	expression.Right = p.parseExpression(PREFIX)
	return expression
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	expression := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}

	precedence := p.curPrecedence()
	p.nextToken()
	expression.Right = p.parseExpression(precedence)

	return expression
}

func (p *Parser) parseTernaryExpression(condition ast.Expression) ast.Expression {
	tok := p.curToken // ?
	p.nextToken()     // past ?

	consequence := p.parseExpression(LOWEST)
	if consequence == nil {
		return nil
	}

	if !p.expectPeek(token.COLON) {
		return nil
	}

	p.nextToken() // past :

	alternative := p.parseExpression(TERNARY - 1)
	if alternative == nil {
		return nil
	}

	return &ast.IfExpression{
		Token:     tok,
		Condition: condition,
		Consequence: &ast.BlockStatement{
			Token: token.Token{Type: token.LBRACE, Literal: "{"},
			Statements: []ast.Statement{
				&ast.ExpressionStatement{
					Token:      tok,
					Expression: consequence,
				},
			},
		},
		Alternative: &ast.BlockStatement{
			Token: token.Token{Type: token.LBRACE, Literal: "{"},
			Statements: []ast.Statement{
				&ast.ExpressionStatement{
					Token:      tok,
					Expression: alternative,
				},
			},
		},
	}
}

func (p *Parser) parseBoolean() ast.Expression {
	return &ast.Boolean{Token: p.curToken, Value: p.curTokenIs(token.TRUE)}
}

func (p *Parser) parseNullLiteral() ast.Expression {
	return &ast.NullLiteral{Token: p.curToken}
}

func (p *Parser) parseGroupedExpression() ast.Expression {
	p.nextToken()

	exp := p.parseExpression(LOWEST)

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return exp
}

func (p *Parser) parseIfExpression() ast.Expression {
	expression := &ast.IfExpression{Token: p.curToken}

	if p.peekTokenIs(token.LPAREN) {
		p.nextToken()
		p.nextToken()
		expression.Condition = p.parseCondition()
		if !p.expectPeek(token.RPAREN) {
			return nil
		}
	} else {
		p.nextToken()
		expression.Condition = p.parseCondition()
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	expression.Consequence = p.parseBlockStatement()

	if p.peekTokenIs(token.ELSE) {
		p.nextToken()

		if p.peekTokenIs(token.IF) {
			p.nextToken()
			chainedIf := p.parseIfExpression()
			expression.Alternative = &ast.BlockStatement{
				Token: token.Token{Type: token.LBRACE, Literal: "{"},
				Statements: []ast.Statement{
					&ast.ExpressionStatement{
						Token:      token.Token{Type: token.IF, Literal: "if"},
						Expression: chainedIf,
					},
				},
			}
		} else {
			if !p.expectPeek(token.LBRACE) {
				return nil
			}
			expression.Alternative = p.parseBlockStatement()
		}
	}

	return expression
}

func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken, Statements: []ast.Statement{}}

	p.nextToken()

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		p.appendStatement(&block.Statements, stmt)
		p.nextToken()
	}

	return block
}

// parseModifiedFunction parses pure fn ... and unsafe fn ....
//
// The modifier is a prefix rather than a keyword placed after the name, so it
// reads in the order it is written and cannot be confused with a function named
// pure. Both modifiers may appear: pure unsafe fn is legal and means a pure
// function that performs capability-gated work, which is the combination that
// makes the exemption worth recording.
func (p *Parser) parseModifiedFunction() ast.Expression {
	// Only a `fn` makes this a modifier. Without the lookahead, a variable named
	// `pure` would be read as a modifier everywhere it appeared as an expression,
	// including inside a string interpolation, where it is most surprising.
	if !p.peekTokenIs(token.FN) {
		return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}

	isPure := p.curTokenIs(token.PURE)
	isUnsafe := p.curTokenIs(token.UNSAFE)

	if !p.expectPeek(token.FN) {
		p.recordError(CodeSyntaxError, p.curToken,
			fmt.Sprintf("%s must be followed by `fn`, got %s", p.curToken.Literal, p.peekToken.Type),
			"write it as `pure fn name(...)` or `unsafe fn name(...)`")
		return nil
	}

	lit, _ := p.parseFunctionLiteral().(*ast.FunctionLiteral)
	if lit == nil {
		return nil
	}
	lit.IsPure = isPure
	lit.IsUnsafe = isUnsafe
	return lit
}

// parseRequiresClause parses a requires clause after a function's signature.
//
// The clause is optional. Each capability is a string literal rather than an
// identifier because the set is closed: a bare identifier would read as a
// capability nobody has heard of, where a quoted name says it is being checked
// against a list.
func (p *Parser) parseRequiresClause() ([]string, bool) {
	if !p.peekTokenIs(token.REQUIRES) {
		return nil, true
	}

	p.nextToken() // cur is 'requires'
	var names []string

	for {
		if !p.peekTokenIs(token.STRING) {
			p.recordError(CodeSyntaxError, p.curToken,
				fmt.Sprintf("expected a capability name in quotes, got %s", p.peekToken.Type),
				"write the clause as requires \"filesystem\"")
			p.skipToBrace()
			return nil, false
		}
		p.nextToken()
		names = append(names, p.curToken.Literal)

		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
			continue
		}
		break
	}

	return names, true
}

func (p *Parser) parseFunctionLiteral() ast.Expression {
	lit := &ast.FunctionLiteral{Token: p.curToken}

	if p.peekTokenIs(token.IDENT) {
		p.nextToken()
		lit.Name = p.curToken.Literal
	}

	// A generic function declares its type parameters between the name and the
	// parameter list: `fn map<T, U>(...)`.
	//
	// They are read here rather than as part of the name because they are a list,
	// and `f<T>` as a name would make a call to a generic function and a call to
	// a function-valued variable look alike in the grammar. The `<` after a name
	// has no other reading here.
	if p.peekTokenIs(token.LT) {
		params, ok := p.parseTypeParameters()
		if !ok {
			return nil
		}
		lit.TypeParams = params
	}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	lit.Parameters = p.parseFunctionParameters()

	returnType, ok := p.parseReturnType()
	if !ok {
		return nil
	}
	lit.ReturnType = returnType

	// The clause comes after the signature and before the body. Keeping it in
	// that one position leaves the grammar unambiguous: the keyword is reserved,
	// so accepting it anywhere else would mean a function could not be named
	// after it at all.

	requires, ok := p.parseRequiresClause()
	if !ok {
		return nil
	}
	lit.Requires = requires

	// Inside a `trait` or `impl` block, a body is optional: a trait holds
	// signatures and an impl holds bodies, and which one a given `fn` is
	// depends on the block it appears in. The flag is cleared before parsing the
	// body so that a function declared *inside* one still requires its own.
	wasInTraitBody := p.inTraitBody
	p.inTraitBody = false

	if p.peekTokenIs(token.LBRACE) {
		p.nextToken()
		lit.Body = p.parseBlockStatement()
		p.inTraitBody = wasInTraitBody
		return lit
	}

	if wasInTraitBody {
		// A signature: no body, and that is correct rather than a missing piece.
		p.inTraitBody = true
		return lit
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	lit.Body = p.parseBlockStatement()
	p.inTraitBody = wasInTraitBody

	return lit
}

// parseExternStatement reads `extern "C" fn name(params) -> Ret;`.
//
// The signature is parsed by the same code that parses a definition, with one
// difference: `parseFunctionLiteral` requires a body, so the body requirement is
// suspended here the way it already is inside a `trait` — a signature is a
// complete thing in both places.
//
// That reuse is the point. If extern had its own grammar, a change to how
// parameters or return types are written would have to be made twice, and the two
// would eventually disagree about what a program may declare.
func (p *Parser) parseExternStatement() ast.Statement {
	p.nextToken() // cur is the ABI string
	abi := p.curToken.Literal

	if !p.expectPeek(token.FN) {
		p.recordError(CodeSyntaxError, p.curToken, "extern must be followed by a function declaration",
			"write `extern \"C\" fn name(params) -> Type;` to declare a function implemented in C")
		return nil
	}

	// A signature has no body, and here that is correct rather than a missing piece
	// — so the same flag the `trait` path uses is set for the duration.
	wasInTraitBody := p.inTraitBody
	p.inTraitBody = true

	expr := p.parseFunctionLiteral()
	p.inTraitBody = wasInTraitBody

	if expr == nil {
		return nil
	}
	fn, ok := expr.(*ast.FunctionLiteral)
	if !ok {
		return nil
	}

	if fn.Name == "" {
		p.recordError(CodeSyntaxError, p.curToken, "an extern function must be named",
			"write `extern \"C\" fn name(params) -> Type;` - a declaration with no name has nothing for the linker to resolve")
		return nil
	}

	// `extern fn f() -> T { ... }` is a contradiction the parser can see: a body
	// would define the function here, and the declaration says it is defined
	// elsewhere. Saying so is better than accepting it and letting the linker
	// report a duplicate symbol.
	if fn.Body != nil {
		p.recordError(CodeSyntaxError, p.curToken, "an extern declaration cannot have a body",
			"remove the braces - the function is implemented in C and linked in, not defined here")
		return nil
	}

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return &ast.ExternStatement{Token: p.curToken, Abi: abi, Fn: fn}
}

// parseTypeParameters reads `<T, U>` after a function name.//
// A lone `<` is not enough to decide there are type parameters: it is also the
// less-than operator. It is checked here because this position follows a
// declaration's name, where no expression can continue, so the two readings
// cannot collide.
//
// Only one `>` closes the list, whatever follows. `fn f<T>(x: Array<Array<T>>)`
// ends its *annotation* with two, and the annotation parser counts those itself —
// so treating a `>>` here as two closings would consume the start of the
// parameter list.
func (p *Parser) parseTypeParameters() ([]string, bool) {
	p.nextToken() // cur is `<`

	var params []string
	seen := make(map[string]bool)

	for {
		if !p.expectPeek(token.IDENT) {
			p.recordError(CodeSyntaxError, p.curToken,
				"a type parameter list holds names, for example <T, U>",
				"write the parameters between angle brackets after the function name")
			return nil, false
		}

		name := p.curToken.Literal
		if seen[name] {
			p.recordError(CodeSyntaxError, p.curToken,
				fmt.Sprintf("type parameter %q is declared twice", name),
				"each type parameter is declared once")
			return nil, false
		}
		seen[name] = true
		params = append(params, name)

		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
			continue
		}
		break
	}

	if !p.expectPeek(token.GT) {
		p.recordError(CodeSyntaxError, p.curToken,
			fmt.Sprintf("a type parameter list must close with `>`, got %s", p.curToken.Type),
			"write it as fn name<T, U>(...)")
		return nil, false
	}

	return params, true
}

func (p *Parser) parseFunctionParameters() []*ast.Parameter {
	parameters := []*ast.Parameter{}

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return parameters
	}

	p.nextToken()
	param, ok := p.parseOneParameter()
	if !ok {
		return nil
	}
	parameters = append(parameters, param)

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		if p.peekTokenIs(token.RPAREN) {
			p.peekError(token.IDENT)
			return nil
		}
		p.nextToken()
		param, ok := p.parseOneParameter()
		if !ok {
			return nil
		}
		parameters = append(parameters, param)
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return parameters
}

// parseOneParameter parses `name` or `name: Type`.
//
// cur is the parameter's name. A missing annotation is left empty rather than
// defaulted to `Any`, so that printing a function round-trips what was written
// instead of inventing a type the developer never wrote.
func (p *Parser) parseOneParameter() (*ast.Parameter, bool) {
	if !p.curTokenIs(token.IDENT) && !softKeywords[p.curToken.Type] {
		p.recordError(CodeSyntaxError, p.curToken,
			fmt.Sprintf("expected a parameter name, got %s", p.curToken.Type),
			"a parameter is written `name` or `name: Type`")
		p.skipToBrace()
		return nil, false
	}

	param := &ast.Parameter{
		Token: p.curToken,
		Name:  &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal},
	}

	if !p.peekTokenIs(token.COLON) {
		return param, true
	}

	p.nextToken() // cur is ':'
	// parseTypeAnnotation advances onto the type itself, so cur stays on the
	// colon; advancing first would come back with an empty annotation.
	param.TypeAnnotation = p.parseTypeAnnotation()
	if param.TypeAnnotation == "" {
		p.peekError(token.IDENT)
		p.skipToBrace()
		return nil, false
	}

	return param, true
}

// parseReturnType parses the optional `-> Type` after a parameter list.
//
// The annotation is optional, so a function without one is not an error and its
// return type stays empty, meaning `Any`.
func (p *Parser) parseReturnType() (string, bool) {
	if !p.peekTokenIs(token.ARROW) {
		return "", true
	}

	p.nextToken() // cur is '->'
	if p.peekTokenIs(token.LBRACE) || p.peekTokenIs(token.SEMICOLON) {
		p.recordError(CodeSyntaxError, p.curToken,
			"`->` needs a type after it",
			"write the return type, as in `fn f(a: Int) -> Int`")
		return "", false
	}

	// cur stays on the `->`: parseTypeAnnotation advances onto the type itself,
	// so advancing first would leave it looking at the type's first token and
	// reading an empty annotation.
	returnType := p.parseTypeAnnotation()
	if returnType == "" {
		p.peekError(token.IDENT)
		return "", false
	}
	return returnType, true
}

func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {
	exp := &ast.CallExpression{Token: p.curToken, Function: function}
	exp.Arguments = p.parseExpressionList(token.RPAREN)
	return exp
}

func (p *Parser) parseExpressionList(end token.TokenType) []ast.Expression {
	list := []ast.Expression{}

	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}

	p.nextToken()
	list = append(list, p.parseExpression(LOWEST))

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		list = append(list, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(end) {
		return nil
	}

	return list
}

func (p *Parser) parseArrayLiteral() ast.Expression {
	array := &ast.ArrayLiteral{Token: p.curToken}
	array.Elements = p.parseExpressionList(token.RBRACKET)
	return array
}

func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	exp := &ast.IndexExpression{Token: p.curToken, Left: left}

	p.nextToken()
	exp.Index = p.parseExpression(LOWEST)

	if !p.expectPeek(token.RBRACKET) {
		return nil
	}

	return exp
}

func (p *Parser) parseDotExpression(left ast.Expression) ast.Expression {
	return p.parseMemberAccess(left, false)
}

// parseSafeAccess parses `left?.member`.
//
// It shares the member-name rules with `left.member` rather than having its own
// list: the set of names that may follow a dot is a property of member access,
// not of whether the access is safe, and two lists would drift.
func (p *Parser) parseSafeAccess(left ast.Expression) ast.Expression {
	return p.parseMemberAccess(left, true)
}

func (p *Parser) parseMemberAccess(left ast.Expression, safe bool) ast.Expression {
	tok := p.curToken

	if !p.peekTokenIs(token.IDENT) &&
		!p.peekTokenIs(token.STATE) &&
		!p.peekTokenIs(token.RENDER) &&
		!p.peekTokenIs(token.EMIT) &&
		!p.peekTokenIs(token.ON) &&
		!p.peekTokenIs(token.BUILD) {
		p.peekError(token.IDENT)
		return nil
	}
	p.nextToken()

	member := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	if safe {
		return &ast.SafeAccessExpression{Token: tok, Left: left, Member: member}
	}
	return &ast.DotExpression{Token: tok, Left: left, Member: member}
}

// parseCoalesceExpression parses `left ?? right`.
//
// The right side is parsed one precedence level below `??` so that a chain
// groups left to right, the way `a ?? b ?? c` reads. The single step down is
// also what keeps the ternary out of the middle of a chain: with `??` at the
// ternary's own level, `a ?? b ? c : d` would take the `?:` first and produce a
// different program than the same text read the other way.
func (p *Parser) parseCoalesceExpression(left ast.Expression) ast.Expression {
	tok := p.curToken
	p.nextToken()

	right := p.parseExpression(COALESCE - 1)
	if right == nil {
		return nil
	}

	return &ast.CoalesceExpression{Token: tok, Left: left, Right: right}
}

func (p *Parser) parseHashLiteral() ast.Expression {
	hash := &ast.HashLiteral{Token: p.curToken, Pairs: make(map[ast.Expression]ast.Expression)}

	for !p.peekTokenIs(token.RBRACE) {
		p.nextToken()
		key := p.parseExpression(LOWEST)

		if !p.expectPeek(token.COLON) {
			return nil
		}

		p.nextToken()
		value := p.parseExpression(LOWEST)

		// SetPair keeps the ordered slice and the lookup map in step, so
		// evaluation follows source order per SEMANTICS.md §1.
		hash.SetPair(key, value)

		if !p.peekTokenIs(token.RBRACE) && !p.expectPeek(token.COMMA) {
			return nil
		}
	}

	if !p.expectPeek(token.RBRACE) {
		return nil
	}

	return hash
}

func (p *Parser) parseComponentDeclaration() *ast.ComponentLiteral {
	comp := &ast.ComponentLiteral{
		Token:    p.curToken,
		States:   []*ast.StateDeclaration{},
		Handlers: []*ast.EventHandler{},
		Body:     &ast.BlockStatement{Statements: []ast.Statement{}},
	}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	comp.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	comp.Body.Token = p.curToken
	p.nextToken()
	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		switch p.curToken.Type {
		case token.STATE:
			state := p.parseStateDeclaration()
			if state != nil {
				comp.States = append(comp.States, state)
			}
		case token.RENDER:
			if comp.Render != nil {
				p.recordError(CodeComponentDuplicate, p.curToken,
					"component may only declare one render block",
					"merge the two render blocks into one, or move the extra content into a helper component")
			}
			comp.Render = p.parseComponentBlock()
		case token.BUILD:
			if comp.Build != nil {
				p.recordError(CodeComponentDuplicate, p.curToken,
					"component may only declare one build block",
					"merge the two build blocks into one, or move the extra content into a helper component")
			}
			comp.Build = p.parseComponentBlock()
		case token.ON:
			handler := p.parseEventHandler()
			if handler != nil {
				comp.Handlers = append(comp.Handlers, handler)
			}
		default:
			stmt := p.parseStatement()
			p.appendStatement(&comp.Body.Statements, stmt)
		}
		p.nextToken()
	}
	return comp
}

func (p *Parser) parseComponentBlock() *ast.RenderMethod {
	method := &ast.RenderMethod{Token: p.curToken}
	if !p.expectPeek(token.LBRACE) {
		return method
	}
	method.Body = p.parseBlockStatement()
	return method
}

func (p *Parser) parseEventHandler() *ast.EventHandler {
	handler := &ast.EventHandler{Token: p.curToken, Parameters: []*ast.Identifier{}}
	if !p.expectPeek(token.IDENT) {
		return nil
	}
	handler.Event = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	if p.peekTokenIs(token.LPAREN) {
		p.nextToken()
		// An event handler's payload parameter is a plain name: on tap(e) takes
		// the event and nothing else, so a type annotation on it would be
		// accepted by the function grammar and ignored here.
		if params := p.parseFunctionParameters(); params != nil {
			for _, param := range params {
				handler.Parameters = append(handler.Parameters, param.Name)
			}
		}
		if len(handler.Parameters) > 1 {
			p.recordError(CodeSyntaxError, handler.Token,
				"event handler accepts at most one payload parameter",
				"declare a single payload parameter, or none, for the handler")
		}
	}
	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	handler.Body = p.parseBlockStatement()
	return handler
}

func (p *Parser) parseComponentLiteralExpression() ast.Expression {
	return p.parseComponentDeclaration()
}

func (p *Parser) parseStyleStatement() *ast.StyleStatement {
	stmt := &ast.StyleStatement{Token: p.curToken, Properties: make(map[string]string)}

	if p.peekTokenIs(token.IDENT) {
		p.nextToken()
		stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.IDENT) {
			k := p.curToken.Literal
			if p.expectPeek(token.COLON) {
				p.nextToken()
				stmt.Properties[k] = p.curToken.Literal
			}
		}
		p.nextToken()
	}

	return stmt
}
