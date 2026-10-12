package parser

import (
	"fmt"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/token"
)

// Parsing for `trait` declarations and `impl` blocks.
//
// # Why a method is not parsed as a FunctionLiteral
//
// A method is not a function: its first parameter is the receiver, it has no
// value of its own, and it is reachable only through a receiver. Reusing
// FunctionLiteral would let a method be called as a plain function by every
// consumer that does not know better, which is exactly the mistake the lowering
// pass exists to prevent. A separate node keeps the distinction in the tree
// rather than in a comment.

// parseTraitDecl parses `trait Name { fn m(a: T) -> R ... }`.
func (p *Parser) parseTraitDecl() *ast.TraitDecl {
	decl := &ast.TraitDecl{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	decl.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.LBRACE) {
		p.recordError(CodeSyntaxError, p.curToken,
			fmt.Sprintf("a trait body must be a `{ ... }` block, got %s", p.curToken.Type),
			"write it as trait Name { fn method(...) ... }")
		return nil
	}

	// A method in this block may omit its body. A trait holds signatures; the
	// bodies live in the impl blocks.
	p.inTraitBody = true
	defer func() { p.inTraitBody = false }()

	decl.Body = p.parseBlockStatement()
	if decl.Body == nil {
		return nil
	}

	for _, stmt := range decl.Body.Statements {
		lit, ok := traitMethodLiteral(stmt)
		if !ok {
			p.recordError(CodeSyntaxError, p.curToken,
				"a trait body may contain only `fn` signatures",
				"a trait declares what an implementation owes, so it has signatures and no bodies")
			return nil
		}
		if lit.Body != nil {
			p.recordError(CodeSyntaxError, lit.Token,
				fmt.Sprintf("trait method %q has a body", lit.Name),
				"give the body in the `impl` block; a trait says what an implementation must provide")
			return nil
		}

		decl.Methods = append(decl.Methods, &ast.TraitMethod{
			Token:      lit.Token,
			Name:       lit.Name,
			Parameters: lit.Parameters,
			ReturnType: lit.ReturnType,
		})
	}

	if len(decl.Methods) == 0 {
		p.recordError(CodeSyntaxError, decl.Token,
			fmt.Sprintf("trait %q declares no methods", decl.Name.Value),
			"a trait with nothing in it constrains nothing")
		return nil
	}

	return decl
}

// parseImplDecl parses `impl Trait for Type { fn m(self, ...) { ... } }`.
func (p *Parser) parseImplDecl() *ast.ImplDecl {
	decl := &ast.ImplDecl{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	decl.Trait = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	// `for` is a keyword here, not a soft one: the position is unambiguous —
	// an impl always names a trait before `for` — and allowing `for` to be read
	// as an identifier in this one place would buy nothing.
	if !p.expectPeek(token.FOR) {
		p.recordError(CodeSyntaxError, p.curToken,
			fmt.Sprintf("an impl block needs `for` between the trait and the type, got %s", p.curToken.Type),
			"write it as impl TraitName for TypeName { ... }")
		return nil
	}

	if !p.expectPeek(token.IDENT) {
		p.recordError(CodeSyntaxError, p.curToken,
			"an impl block must name the type it implements the trait for",
			"write it as impl TraitName for TypeName { ... }")
		return nil
	}
	decl.Type = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.LBRACE) {
		p.recordError(CodeSyntaxError, p.curToken,
			fmt.Sprintf("an impl body must be a `{ ... }` block, got %s", p.curToken.Type),
			"write it as impl TraitName for TypeName { fn method(self) { ... } }")
		return nil
	}

	// An impl holds bodies, and may also hold a bare signature — which is
	// reported as a missing body below. Both are accepted by the parser so that
	// the diagnostic names the method rather than the grammar.
	p.inTraitBody = true
	defer func() { p.inTraitBody = false }()

	body := p.parseBlockStatement()
	if body == nil {
		return nil
	}

	seen := make(map[string]bool, len(body.Statements))
	for _, stmt := range body.Statements {
		lit, ok := traitMethodLiteral(stmt)
		if !ok {
			p.recordError(CodeSyntaxError, p.curToken,
				"an impl body may contain only `fn` methods",
				"write the method as fn name(self, args) { ... }")
			return nil
		}
		if lit.Body == nil {
			p.recordError(CodeSyntaxError, lit.Token,
				fmt.Sprintf("impl method %q has no body", lit.Name),
				"a trait holds the signature; an impl holds the body")
			return nil
		}
		if seen[lit.Name] {
			p.recordError(CodeSyntaxError, lit.Token,
				fmt.Sprintf("impl for %q defines method %q twice", decl.Type.Value, lit.Name),
				"a type has one implementation of a trait, so each method appears once")
			return nil
		}
		seen[lit.Name] = true

		decl.Methods = append(decl.Methods, &ast.TraitMethod{
			Token:      lit.Token,
			Name:       lit.Name,
			Parameters: lit.Parameters,
			ReturnType: lit.ReturnType,
			Body:       lit.Body,
		})
	}

	return decl
}

// traitMethodLiteral unwraps a statement into the function literal a trait or
// impl method is written as.
//
// The receiver is spelled `self` as an ordinary parameter rather than as a
// keyword. Writing `fn area(self: Dog)` out makes it obvious that the method
// takes the receiver like anything else, which is what it does after lowering,
// and it leaves `self` available as a name.
func traitMethodLiteral(stmt ast.Statement) (*ast.FunctionLiteral, bool) {
	expr, ok := stmt.(*ast.ExpressionStatement)
	if !ok {
		return nil, false
	}
	lit, ok := expr.Expression.(*ast.FunctionLiteral)
	if !ok {
		return nil, false
	}
	if lit.Name == "" {
		return nil, false
	}
	return lit, true
}
