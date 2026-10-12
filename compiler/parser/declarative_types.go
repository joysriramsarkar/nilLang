package parser

import (
	"fmt"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/token"
)

// ─── TYPE ALIASES ────────────────────────────────────────────────────────────

// parseTypeAlias parses `type Name = Target`.
//
// The target is read as raw identifier text rather than as an expression, because
// a type is resolved against the file's scope and never evaluated.
func (p *Parser) parseTypeAlias() *ast.TypeAliasDecl {
	decl := &ast.TypeAliasDecl{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		p.skipToBrace()
		return nil
	}
	decl.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.ASSIGN) {
		p.recordError(CodeSyntaxError, p.curToken,
			fmt.Sprintf("a type alias needs `=`, got %s", p.curToken.Type),
			"write `type Name = ExistingType`")
		p.skipToBrace()
		return nil
	}

	decl.Target = p.parseTypeName()
	if decl.Target == "" {
		p.peekError(token.IDENT)
		p.skipToBrace()
		return nil
	}

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}
	return decl
}

// parseTypeName reads a single type reference: a name, optionally with
// generic arguments.
//
// It deliberately does not use parseTypeAnnotation. That function runs until it
// meets a token that cannot continue a type, which is the right rule where an
// annotation is followed by `=` or `,`. An alias is followed by the next
// statement instead, and there is no token that marks the end — so it would read
// the rest of the file as part of the type.
func (p *Parser) parseTypeName() string {
	if !p.peekTokenIs(token.IDENT) && !softKeywords[p.peekToken.Type] {
		return ""
	}
	p.nextToken()
	name := p.curToken.Literal

	if !p.peekTokenIs(token.LT) {
		return name
	}

	// `List<String>` and `Map<String, Int>`.
	parts := []string{name}
	depth := 0
	for !p.peekTokenIs(token.EOF) {
		p.nextToken()
		parts = append(parts, p.curToken.Literal)

		if p.curTokenIs(token.LT) {
			depth++
			continue
		}
		if p.curTokenIs(token.GT) {
			depth--
			if depth == 0 {
				return strings.Join(parts, "")
			}
			continue
		}
		if p.curTokenIs(token.GTE) {
			depth--
			if depth <= 0 {
				return strings.Join(parts, "")
			}
		}
	}
	return strings.Join(parts, "")
}

// ─── STRUCTS ─────────────────────────────────────────────────────────────────

// parseStructDecl parses `struct Name { field: Type, ... }`.
//
// A trailing separator is accepted, because a struct with many fields reads far
// better when every line ends in a comma.
func (p *Parser) parseStructDecl() *ast.StructDecl {
	decl := &ast.StructDecl{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		p.skipToBrace()
		return nil
	}
	decl.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.LBRACE) {
		p.skipToBrace()
		return nil
	}

	seen := make(map[string]bool)
	p.nextToken()

	closed := false
	afterSeparator := true

	for !p.curTokenIs(token.EOF) {
		// A closing brace is only this list's own when the parser arrived here by
		// stepping over a separator. After a field, cur sits on the field's last
		// token, which for a nested literal is *its* closing brace, and treating
		// that as the end here would stop one level early.
		if afterSeparator && p.curTokenIs(token.RBRACE) {
			closed = true
			break
		}
		afterSeparator = false

		if p.curTokenIs(token.SEMICOLON) || p.curTokenIs(token.COMMA) {
			p.nextToken()
			afterSeparator = true
			continue
		}

		if !p.curTokenIs(token.IDENT) {
			p.recordError(CodeSyntaxError, p.curToken,
				fmt.Sprintf("expected a field name, got %s", p.curToken.Type),
				"a struct field is written `name: Type`, separated by commas")
			p.skipToBrace()
			return nil
		}

		field := ast.StructFieldDecl{
			Token: p.curToken,
			Name:  &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal},
		}

		if seen[field.Name.Value] {
			p.recordError(CodeDuplicateField, p.curToken,
				fmt.Sprintf("duplicate field %q in struct %q", field.Name.Value, decl.Name.Value),
				fmt.Sprintf("%q is declared twice; each field name may appear once", field.Name.Value))
			p.skipToBrace()
			return nil
		}
		seen[field.Name.Value] = true

		if !p.expectPeek(token.COLON) {
			p.skipToBrace()
			return nil
		}

		// parseTypeAnnotation advances onto the type itself, so cur must be left
		// on the `:`. Advancing first would put it on the type's first token and
		// the annotation would come back empty.
		field.TypeAnnotation = p.parseTypeAnnotation()
		if field.TypeAnnotation == "" {
			p.peekError(token.IDENT)
			p.skipToBrace()
			return nil
		}

		decl.Fields = append(decl.Fields, field)

		switch {
		case p.peekTokenIs(token.COMMA) || p.peekTokenIs(token.SEMICOLON):
			p.nextToken()
			afterSeparator = true
		case p.peekTokenIs(token.RBRACE):
			p.nextToken()
			closed = true
		default:
			p.recordError(CodeSyntaxError, p.curToken,
				fmt.Sprintf("expected a separator or } after field %q, got %s",
					field.Name.Value, p.peekToken.Type),
				"separate struct fields with commas")
			p.skipToBrace()
			return nil
		}

		if closed {
			break
		}
	}

	if !closed {
		p.skipToBrace()
		return nil
	}
	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}
	return decl
}

// parseStructLiteral parses `Name { field: value, ... }` in expression position.
//
// The name must already be a declared struct; the type checker enforces that, so
// this function only has to recognise the shape.
func (p *Parser) parseStructLiteral() ast.Expression {
	literal := &ast.StructLiteral{
		Token: p.curToken,
		Name:  &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal},
	}

	if !p.expectPeek(token.LBRACE) {
		p.skipToBrace()
		return nil
	}

	seen := make(map[string]bool)
	p.nextToken()

	closed := false
	afterSeparator := true

	for !p.curTokenIs(token.EOF) {
		// A closing brace is only this list's own when the parser arrived here by
		// stepping over a separator. After a field, cur sits on the field's last
		// token, which for a nested literal is *its* closing brace, and treating
		// that as the end here would stop one level early.
		if afterSeparator && p.curTokenIs(token.RBRACE) {
			closed = true
			break
		}
		afterSeparator = false

		if p.curTokenIs(token.SEMICOLON) || p.curTokenIs(token.COMMA) {
			p.nextToken()
			afterSeparator = true
			continue
		}

		if !p.curTokenIs(token.IDENT) {
			p.recordError(CodeSyntaxError, p.curToken,
				fmt.Sprintf("expected a field name, got %s", p.curToken.Type),
				"a struct literal field is written `name: value`")
			p.skipToBrace()
			return nil
		}

		fieldName := p.curToken.Literal
		if seen[fieldName] {
			p.recordError(CodeDuplicateField, p.curToken,
				fmt.Sprintf("duplicate field %q in a %s literal", fieldName, literal.Name.Value),
				fmt.Sprintf("%q is given twice; each field may be set once", fieldName))
			p.skipToBrace()
			return nil
		}
		seen[fieldName] = true

		field := ast.StructFieldExpr{
			Token: p.curToken,
			Name:  &ast.Identifier{Token: p.curToken, Value: fieldName},
		}

		if !p.expectPeek(token.COLON) {
			p.skipToBrace()
			return nil
		}

		p.nextToken()
		field.Value = p.parseExpression(LOWEST)
		if field.Value == nil {
			p.skipToBrace()
			return nil
		}
		literal.Fields = append(literal.Fields, field)

		switch {
		case p.peekTokenIs(token.COMMA) || p.peekTokenIs(token.SEMICOLON):
			p.nextToken()
			afterSeparator = true
		case p.peekTokenIs(token.RBRACE):
			p.nextToken()
			closed = true
		default:
			p.recordError(CodeSyntaxError, p.curToken,
				fmt.Sprintf("expected a separator or } after field %q, got %s",
					field.Name.Value, p.peekToken.Type),
				"separate struct literal fields with commas")
			p.skipToBrace()
			return nil
		}

		if closed {
			break
		}
	}

	if !closed {
		p.skipToBrace()
		return nil
	}

	// The loop normally stops with the closing brace on peek, but a trailing
	// comma leaves it on cur instead. Accepting both is what lets
	// { x: Int, } and { x: Int } mean the same thing.
	if !p.curTokenIs(token.RBRACE) && !p.expectPeek(token.RBRACE) {
		p.skipToBrace()
		return nil
	}
	return literal
}

// ─── ENUMS ───────────────────────────────────────────────────────────────────

// parseEnumDecl parses `enum Name { Variant, Variant(Type), ... }`.
func (p *Parser) parseEnumDecl() *ast.EnumDecl {
	decl := &ast.EnumDecl{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		p.skipToBrace()
		return nil
	}
	decl.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.LBRACE) {
		p.skipToBrace()
		return nil
	}

	seen := make(map[string]bool)
	p.nextToken()

	closed := false
	afterSeparator := true

	for !p.curTokenIs(token.EOF) {
		// A closing brace is only this list's own when the parser arrived here by
		// stepping over a separator. After a field, cur sits on the field's last
		// token, which for a nested literal is *its* closing brace, and treating
		// that as the end here would stop one level early.
		if afterSeparator && p.curTokenIs(token.RBRACE) {
			closed = true
			break
		}
		afterSeparator = false

		if p.curTokenIs(token.SEMICOLON) || p.curTokenIs(token.COMMA) {
			p.nextToken()
			afterSeparator = true
			continue
		}

		if !p.curTokenIs(token.IDENT) {
			p.recordError(CodeSyntaxError, p.curToken,
				fmt.Sprintf("expected a variant name, got %s", p.curToken.Type),
				"an enum variant is written `Name` or `Name(Type)`")
			p.skipToBrace()
			return nil
		}

		variant := ast.EnumVariantDecl{
			Token: p.curToken,
			Name:  &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal},
		}

		if seen[variant.Name.Value] {
			p.recordError(CodeDuplicateField, p.curToken,
				fmt.Sprintf("duplicate variant %q in enum %q", variant.Name.Value, decl.Name.Value),
				fmt.Sprintf("%q is declared twice; each variant name may appear once", variant.Name.Value))
			p.skipToBrace()
			return nil
		}
		seen[variant.Name.Value] = true

		// An optional parenthesised payload type. cur is left on the `(` so
		// that parseTypeAnnotation, which advances onto the type itself, starts
		// from the right place.
		if p.peekTokenIs(token.LPAREN) {
			p.nextToken()
			variant.PayloadType = p.parseTypeAnnotation()
			if variant.PayloadType == "" {
				p.peekError(token.IDENT)
				p.skipToBrace()
				return nil
			}
			if !p.expectPeek(token.RPAREN) {
				p.skipToBrace()
				return nil
			}
		}

		decl.Variants = append(decl.Variants, variant)

		switch {
		case p.peekTokenIs(token.COMMA) || p.peekTokenIs(token.SEMICOLON):
			p.nextToken()
			afterSeparator = true
		case p.peekTokenIs(token.RBRACE):
			p.nextToken()
			closed = true
		default:
			p.recordError(CodeSyntaxError, p.curToken,
				fmt.Sprintf("expected a separator or `}` after variant %q, got %s",
					variant.Name.Value, p.peekToken.Type),
				"separate enum variants with commas")
			p.skipToBrace()
			return nil
		}

		if closed {
			break
		}
	}

	if !closed {
		p.skipToBrace()
		return nil
	}
	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}
	return decl
}

// parseEnumVariant parses `Name` or `Name(payload)` in expression position.
func (p *Parser) parseEnumVariant() ast.Expression {
	variant := &ast.EnumVariantExpr{
		Token: p.curToken,
		Name:  &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal},
	}

	if !p.peekTokenIs(token.LPAREN) {
		return variant
	}

	p.nextToken() // cur is '('
	p.nextToken() // cur is the first payload token
	variant.Payload = p.parseExpression(LOWEST)
	if variant.Payload == nil {
		p.skipToBrace()
		return nil
	}
	if !p.expectPeek(token.RPAREN) {
		p.skipToBrace()
		return nil
	}
	return variant
}

// ─── MATCH ───────────────────────────────────────────────────────────────────

// parseMatchExpression parses `match subject { pattern => body, ... }`.
//
// The subject is parsed as a condition: a `{` following it opens the arm list,
// so `match v { ... }` must not read `v` as the start of a struct literal.
func (p *Parser) parseMatchExpression() ast.Expression {
	match := &ast.MatchExpression{Token: p.curToken}

	p.nextToken()
	match.Subject = p.parseCondition()
	if match.Subject == nil {
		p.skipToBrace()
		return nil
	}

	if !p.expectPeek(token.LBRACE) {
		p.skipToBrace()
		return nil
	}

	p.nextToken()
	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.SEMICOLON) || p.curTokenIs(token.COMMA) {
			p.nextToken()
			continue
		}

		arm, ok := p.parseMatchArm()
		if !ok {
			p.skipToBrace()
			return nil
		}
		match.Arms = append(match.Arms, arm)

		// An arm may be followed by a comma, a semicolon, or nothing at all.
		// The separator is optional, so the parser cannot tell an arm's end from
		// the next arm's start without being told where to move to: a separator
		// means two tokens forward, and its absence means one.
		if p.peekTokenIs(token.COMMA) || p.peekTokenIs(token.SEMICOLON) {
			p.nextToken()
		}
		p.nextToken()
	}

	// The loop normally stops with the closing brace on peek, but a trailing
	// comma leaves it on cur instead. Accepting both is what lets
	// { x: Int, } and { x: Int } mean the same thing.
	if !p.curTokenIs(token.RBRACE) && !p.expectPeek(token.RBRACE) {
		p.skipToBrace()
		return nil
	}
	return match
}

// parseMatchArm parses `pattern [if guard] => expression`.
func (p *Parser) parseMatchArm() (ast.MatchArm, bool) {
	arm := ast.MatchArm{Token: p.curToken}

	pattern, ok := p.parseMatchPattern()
	if !ok {
		return arm, false
	}
	arm.Pattern = pattern

	// An optional guard, distinguished from `=>` by its leading `if`.
	if p.peekTokenIs(token.IF) {
		p.nextToken()
		p.nextToken()
		arm.Guard = p.parseExpression(LOWEST)
		if arm.Guard == nil {
			return arm, false
		}
	}

	if !p.expectPeek(token.FARROW) {
		p.recordError(CodeSyntaxError, p.curToken,
			fmt.Sprintf("expected `=>` after a match pattern, got %s", p.curToken.Type),
			"a match arm reads `pattern => expression`")
		return arm, false
	}

	p.nextToken()
	arm.Body = p.parseExpression(LOWEST)
	if arm.Body == nil {
		return arm, false
	}
	return arm, true
}

// parseMatchPattern parses one arm's pattern, including `|` alternatives.
func (p *Parser) parseMatchPattern() (ast.MatchPattern, bool) {
	first, ok := p.parseSingleMatchPattern()
	if !ok {
		return ast.MatchPattern{}, false
	}

	if !p.peekTokenIs(token.PIPE) {
		return first, true
	}

	alternatives := []ast.MatchPattern{first}
	for p.peekTokenIs(token.PIPE) {
		p.nextToken() // cur is '|'
		p.nextToken() // cur is the next pattern's first token
		next, ok := p.parseSingleMatchPattern()
		if !ok {
			return ast.MatchPattern{}, false
		}
		alternatives = append(alternatives, next)
	}

	return ast.MatchPattern{Token: first.Token, Alternatives: alternatives}, true
}

// parseSingleMatchPattern parses one alternative of a match pattern.
func (p *Parser) parseSingleMatchPattern() (ast.MatchPattern, bool) {
	pattern := ast.MatchPattern{Token: p.curToken}

	// `_` is lexed as an identifier, so the wildcard is detected by name.
	if p.curTokenIs(token.IDENT) && p.curToken.Literal == "_" {
		pattern.Wildcard = true
		return pattern, true
	}

	// A negative literal: the `-` arrives before the number.
	if p.curTokenIs(token.MINUS) {
		p.nextToken()
		operand := p.parseExpression(PREFIX)
		if operand == nil {
			return pattern, false
		}
		pattern.Literal = &ast.PrefixExpression{
			Token:    p.curToken,
			Operator: "-",
			Right:    operand,
		}
		return pattern, true
	}

	switch p.curToken.Type {
	case token.INT, token.FLOAT, token.STRING, token.TRUE, token.FALSE, token.NULL:
		pattern.Literal = p.parseExpression(LOWEST)
		if pattern.Literal == nil {
			return pattern, false
		}
		return pattern, true

	case token.IDENT:
		name := p.curToken.Literal

		// A struct pattern is `Name {`; an enum pattern is `Name` or `Name(`.
		if p.peekTokenIs(token.LBRACE) {
			structPattern, ok := p.parseMatchStructPattern()
			if !ok {
				return pattern, false
			}
			pattern.StructPattern = structPattern
			return pattern, true
		}

		if p.peekTokenIs(token.LPAREN) {
			p.nextToken() // cur is '('
			enumPattern := &ast.MatchEnumPattern{
				Variant: &ast.Identifier{Token: p.curToken, Value: name},
			}

			p.nextToken() // cur is the binding or '_'
			if p.curTokenIs(token.IDENT) && p.curToken.Literal == "_" {
				enumPattern.DiscardPayload = true
			} else if p.curTokenIs(token.IDENT) {
				enumPattern.PayloadBinding = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
			} else {
				p.recordError(CodeSyntaxError, p.curToken,
					fmt.Sprintf("expected a binding name or `_` inside %s(...), got %s", name, p.curToken.Type),
					"an enum pattern payload binds a name or is discarded with `_`")
				return pattern, false
			}

			if !p.expectPeek(token.RPAREN) {
				return pattern, false
			}
			pattern.EnumVariant = enumPattern
			return pattern, true
		}

		// A bare identifier always binds. That is the rule that keeps
		// `match x { y => ... }` unambiguous: a bare name cannot be a constant
		// pattern, because there are no named constants to match against.
		pattern.Binding = &ast.Identifier{Token: p.curToken, Value: name}
		return pattern, true

	default:
		p.recordError(CodeSyntaxError, p.curToken,
			fmt.Sprintf("a match pattern cannot start with %s", p.curToken.Type),
			"a pattern is `_`, a literal, a name, `Variant(binding)`, or `Name { field: pattern }`")
		return pattern, false
	}
}

// parseMatchStructPattern parses `Name { field: pattern, ... }`.
func (p *Parser) parseMatchStructPattern() (*ast.MatchStructPattern, bool) {
	structPattern := &ast.MatchStructPattern{
		Name: &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal},
	}

	if !p.expectPeek(token.LBRACE) {
		p.skipToBrace()
		return nil, false
	}

	seen := make(map[string]bool)
	p.nextToken()

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.SEMICOLON) || p.curTokenIs(token.COMMA) {
			p.nextToken()
			continue
		}

		if !p.curTokenIs(token.IDENT) {
			p.recordError(CodeSyntaxError, p.curToken,
				fmt.Sprintf("expected a field name in the %s pattern, got %s",
					structPattern.Name.Value, p.curToken.Type),
				"a struct pattern field is written `name: pattern`")
			p.skipToBrace()
			return nil, false
		}

		// The field's own token is captured before parsing moves on, so a
		// diagnostic pointing at the field points at the name the developer
		// wrote rather than at whatever token the parser has since reached.
		fieldToken := p.curToken
		fieldName := fieldToken.Literal
		if seen[fieldName] {
			p.recordError(CodeDuplicateField, fieldToken,
				fmt.Sprintf("duplicate field %q in the %s pattern", fieldName, structPattern.Name.Value),
				fmt.Sprintf("%q is matched twice; each field may appear once", fieldName))
			p.skipToBrace()
			return nil, false
		}
		seen[fieldName] = true

		if !p.expectPeek(token.COLON) {
			p.skipToBrace()
			return nil, false
		}

		p.nextToken()
		fieldPattern, ok := p.parseMatchPattern()
		if !ok {
			p.skipToBrace()
			return nil, false
		}
		structPattern.Fields = append(structPattern.Fields, ast.MatchStructFieldPattern{
			Name:    &ast.Identifier{Token: fieldToken, Value: fieldName},
			Pattern: fieldPattern,
		})

		if p.peekTokenIs(token.COMMA) || p.peekTokenIs(token.SEMICOLON) {
			p.nextToken()
		}
		p.nextToken()
	}

	if !p.curTokenIs(token.RBRACE) && !p.expectPeek(token.RBRACE) {
		p.skipToBrace()
		return nil, false
	}
	return structPattern, true
}

// parseWhileLet parses `while let <pattern> = <expr> { body }`.
//
// The pattern is the same grammar a `match` arm uses, so the two forms cannot
// drift apart: a pattern that compiles in one compiles in the other.
func (p *Parser) parseWhileLet() *ast.WhileLetStatement {
	stmt := &ast.WhileLetStatement{Token: p.curToken}

	if !p.expectPeek(token.LET) {
		p.skipToBrace()
		return nil
	}

	p.nextToken()
	pattern, ok := p.parseMatchPattern()
	if !ok {
		return nil
	}
	stmt.Pattern = pattern

	if !p.expectPeek(token.ASSIGN) {
		p.recordError(CodeSyntaxError, p.curToken,
			fmt.Sprintf("`while let` needs `=` between the pattern and the value, got %s", p.curToken.Type),
			"write it as `while let Some(x) = next()`")
		p.skipToBrace()
		return nil
	}

	p.nextToken()
	stmt.Value = p.parseCondition()
	if stmt.Value == nil {
		p.skipToBrace()
		return nil
	}

	if !p.expectPeek(token.LBRACE) {
		p.skipToBrace()
		return nil
	}
	stmt.Body = p.parseBlockStatement()

	return stmt
}
