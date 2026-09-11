package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/token"
)

const (
	_ int = iota
	LOWEST
	TERNARY     // ? :
	OR          // ||
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
	token.QUESTION: TERNARY,
	token.OR:       OR,
	token.AND:      AND,
	token.EQ:       EQUALS,
	token.NOT_EQ:   EQUALS,
	token.LT:       LESSGREATER,
	token.GT:       LESSGREATER,
	token.LTE:      LESSGREATER,
	token.GTE:      LESSGREATER,
	token.PLUS:     SUM,
	token.MINUS:    SUM,
	token.SLASH:    PRODUCT,
	token.ASTERISK: PRODUCT,
	token.MODULO:   PRODUCT,
	token.LPAREN:   CALL,
	token.LBRACKET: INDEX,
	token.DOT:      INDEX,
}

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(ast.Expression) ast.Expression
)

type Parser struct {
	l      *lexer.Lexer
	errors []string

	curToken  token.Token
	peekToken token.Token

	prefixParseFns map[token.TokenType]prefixParseFn
	infixParseFns  map[token.TokenType]infixParseFn
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
	}

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
	p.registerInfix(token.LPAREN, p.parseCallExpression)
	p.registerInfix(token.LBRACKET, p.parseIndexExpression)
	p.registerInfix(token.DOT, p.parseDotExpression)

	// Read two tokens, so curToken and peekToken are both set
	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) peekError(t token.TokenType) {
	msg := fmt.Sprintf("line %d:%d: expected next token to be %s, got %s instead",
		p.peekToken.Line, p.peekToken.Column, t, p.peekToken.Type)
	p.errors = append(p.errors, msg)
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
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}

	return program
}

func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
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

	if !p.expectPeek(token.IDENT) {
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
				msg := fmt.Sprintf("line %d:%d: duplicate field %q in entity %q",
					p.curToken.Line, p.curToken.Column, fieldName, stmt.Name.Value)
				p.errors = append(p.errors, msg)
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
							msg := fmt.Sprintf("line %d:%d: empty generic type parameter for %q",
								p.curToken.Line, p.curToken.Column, field.Type)
							p.errors = append(p.errors, msg)
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
				(p.peekTokenIs(token.IDENT) && isEntityFieldModifier(p.peekToken.Literal)) {
				break
			}
		}
	}
	res := strings.Join(parts, "")
	if strings.Contains(res, "<>") {
		msg := fmt.Sprintf("line %d:%d: invalid empty generic type parameter in %q",
			p.curToken.Line, p.curToken.Column, res)
		p.errors = append(p.errors, msg)
		return ""
	}
	return res
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

	if !p.expectPeek(token.IDENT) {
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

func (p *Parser) parseWhileStatement() *ast.WhileStatement {
	stmt := &ast.WhileStatement{Token: p.curToken}

	if p.peekTokenIs(token.LPAREN) {
		p.nextToken()
		p.nextToken()
		stmt.Condition = p.parseExpression(LOWEST)
		if !p.expectPeek(token.RPAREN) {
			return nil
		}
	} else {
		p.nextToken()
		stmt.Condition = p.parseExpression(LOWEST)
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
		msg := fmt.Sprintf("line %d:%d: invalid assignment target",
			p.curToken.Line, p.curToken.Column)
		p.errors = append(p.errors, msg)
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
	msg := fmt.Sprintf("line %d:%d: no prefix parse function for %s found",
		p.curToken.Line, p.curToken.Column, t)
	p.errors = append(p.errors, msg)
}

func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseKeywordIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	lit := &ast.IntegerLiteral{Token: p.curToken}

	value, err := strconv.ParseInt(p.curToken.Literal, 0, 64)
	if err != nil {
		msg := fmt.Sprintf("line %d:%d: could not parse %q as integer",
			p.curToken.Line, p.curToken.Column, p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}

	lit.Value = value
	return lit
}

func (p *Parser) parseFloatLiteral() ast.Expression {
	lit := &ast.FloatLiteral{Token: p.curToken}

	value, err := strconv.ParseFloat(p.curToken.Literal, 64)
	if err != nil {
		msg := fmt.Sprintf("line %d:%d: could not parse %q as float",
			p.curToken.Line, p.curToken.Column, p.curToken.Literal)
		p.errors = append(p.errors, msg)
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
		expression.Condition = p.parseExpression(LOWEST)
		if !p.expectPeek(token.RPAREN) {
			return nil
		}
	} else {
		p.nextToken()
		expression.Condition = p.parseExpression(LOWEST)
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
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}

	return block
}

func (p *Parser) parseFunctionLiteral() ast.Expression {
	lit := &ast.FunctionLiteral{Token: p.curToken}

	if p.peekTokenIs(token.IDENT) {
		p.nextToken()
		lit.Name = p.curToken.Literal
	}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	lit.Parameters = p.parseFunctionParameters()

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	lit.Body = p.parseBlockStatement()

	return lit
}

func (p *Parser) parseFunctionParameters() []*ast.Identifier {
	identifiers := []*ast.Identifier{}

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return identifiers
	}

	p.nextToken()

	ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	identifiers = append(identifiers, ident)

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		if p.peekTokenIs(token.RPAREN) {
			p.peekError(token.IDENT)
			return nil
		}
		p.nextToken()
		ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		identifiers = append(identifiers, ident)
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return identifiers
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
	exp := &ast.DotExpression{Token: p.curToken, Left: left}

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

	exp.Member = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	return exp
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

		hash.Pairs[key] = value

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
				p.errors = append(p.errors, fmt.Sprintf("line %d:%d: component may only declare one render block", p.curToken.Line, p.curToken.Column))
			}
			comp.Render = p.parseComponentBlock()
		case token.BUILD:
			if comp.Build != nil {
				p.errors = append(p.errors, fmt.Sprintf("line %d:%d: component may only declare one build block", p.curToken.Line, p.curToken.Column))
			}
			comp.Build = p.parseComponentBlock()
		case token.ON:
			handler := p.parseEventHandler()
			if handler != nil {
				comp.Handlers = append(comp.Handlers, handler)
			}
		default:
			stmt := p.parseStatement()
			if stmt != nil {
				comp.Body.Statements = append(comp.Body.Statements, stmt)
			}
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
		handler.Parameters = p.parseFunctionParameters()
		if len(handler.Parameters) > 1 {
			p.errors = append(p.errors, fmt.Sprintf("line %d:%d: event handler accepts at most one payload parameter", handler.Token.Line, handler.Token.Column))
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
