// Lowering for the declarative type constructs: enum variants, struct literals,
// and `match`.
//
// The theme matches the rest of this package. None of these have a bytecode of
// their own, and giving each one an opcode would mean writing it twice, once per
// engine, with the two free to disagree. Desugaring instead keeps them correct in
// both engines because there is only one implementation.
//
// # Representation
//
// An enum value is a hash with a `variant` key and, when the variant carries a
// payload, a `payload` key:
//
//	Circle(3)   →   { "variant": "Circle", "payload": 3 }
//	Square      →   { "variant": "Square" }
//
// A struct value is a hash keyed by field name, which is exactly what field
// access already expects: `u.name` compiles to an index, so no new opcode is
// needed for reading or writing a field either.
//
// The trade-off is honest and worth stating. `u["name"]` works as well as
// `u.name`, and a plain hash with the same keys matches the same struct
// patterns. Struct identity is checked statically, by the type checker, where
// it belongs. A tagged runtime representation would fix the dynamic half at the
// cost of a distinct object type, a new opcode, and special cases in every
// operation that walks a hash.
//
// # What `match` becomes
//
// A nested if/else chain:
//
//	let __nil_subject = <subject>;
//	if (<test 1>) { <bindings 1>; <body 1> }
//	else if (<test 2>) { <bindings 2>; <body 2> }
//	else { unreachable("no match arm applied to ...") }
//
// The chain needs no "already matched" flag: a nested if/else stops at the
// first branch taken, which is precisely match's semantics. The final `else` is
// the non-exhaustive case, and the compiler reports that statically whenever the
// subject's type is known, so reaching it at run time means the program reached
// a state its type did not describe.
//
// # Guards
//
// A guard can use the pattern's bindings, but those bindings are `let`
// statements in the arm's body, which runs after the guard. Substituting them
// into the guard solves it, and is safe here precisely because a guard is a
// predicate: it cannot declare a variable, define a closure, or capture
// anything, so a name in it can only mean the pattern binding. The body keeps
// real `let` statements, so a closure inside an arm captures the binding the way
// a developer expects rather than a rewritten expression.
package lower

import (
	"fmt"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/token"
)

// Keys of the hash an enum value is represented by.
const (
	// EnumVariantKey names the variant.
	//
	// A program that builds a hash with this key by hand is matched as an enum
	// value. That is a documented permissiveness of the representation, not an
	// oversight: the alternative is a distinct object type and a new opcode for
	// every construction site.
	EnumVariantKey = "variant"

	// EnumPayloadKey holds the variant's value, and is absent when the variant
	// carries none.
	EnumPayloadKey = "payload"
)

// builtinsAsVariants are the built-in constructors that `match` treats as enum
// variants, because they produce values the language already understands as
// having cases.
//
// Matching `Ok(v)` is by far the most common thing a developer wants to write,
// so treating it as anything else would leave the feature half-built.
var builtinsAsVariants = map[string]variantAccessor{
	"Ok":   {predicate: "isOk", payload: "unwrapOr", payloadNeedsDefault: true},
	"Err":  {predicate: "isErr", payload: "errValue"},
	"Some": {predicate: "isSome", payload: "unwrapOr", payloadNeedsDefault: true},
}

// variantAccessor says how to test for a built-in case and how to read its value.
type variantAccessor struct {
	// predicate returns true when the value is this case.
	predicate string

	// payload returns the case's value.
	payload string

	// payloadNeedsDefault reports that the payload accessor takes a fallback for
	// the other case, which it needs because `unwrapOr` is written to succeed
	// rather than to fail.
	payloadNeedsDefault bool
}

// enumVariant lowers an enum variant expression to the hash that represents it.
func (l *lowerer) enumVariant(node *ast.EnumVariantExpr) ast.Expression {
	tok := node.Token

	pairs := []ast.HashPairExpr{
		{
			Key:   &ast.StringLiteral{Token: tok, Value: EnumVariantKey},
			Value: &ast.StringLiteral{Token: tok, Value: node.Name.Value},
		},
	}
	if node.Payload != nil {
		pairs = append(pairs, ast.HashPairExpr{
			Key:   &ast.StringLiteral{Token: tok, Value: EnumPayloadKey},
			Value: node.Payload,
		})
	}

	return hashLiteral(tok, pairs)
}

// hashLiteral builds a hash from pairs, keeping their order.
func hashLiteral(tok token.Token, pairs []ast.HashPairExpr) *ast.HashLiteral {
	byKey := make(map[ast.Expression]ast.Expression, len(pairs))
	for _, pair := range pairs {
		byKey[pair.Key] = pair.Value
	}
	return &ast.HashLiteral{Token: tok, Pairs: byKey, OrderedPairs: pairs}
}

// matchExpression rewrites a match into a let-bound subject followed by an
// if/else chain.
func (l *lowerer) matchExpression(node *ast.MatchExpression) (ast.Expression, error) {
	if node.Subject == nil {
		return nil, fmt.Errorf("line %d: `match` has no subject", node.Token.Line)
	}
	if len(node.Arms) == 0 {
		return nil, fmt.Errorf("line %d: `match` has no arms", node.Token.Line)
	}

	tok := node.Token
	subject := l.identifier(tok, l.unique("subject"))

	// The subject is lowered like any other expression, so `match Circle(7)`
	// builds the enum value rather than trying to call a name that is not a
	// function. The original is kept for the diagnostic, where the text the
	// developer wrote is more useful than the text after lowering.
	loweredSubject, err := l.expression(node.Subject)
	if err != nil {
		return nil, err
	}

	type loweredArm struct {
		test     ast.Expression
		bindings []ast.Statement
		guard    ast.Expression
		body     ast.Expression
	}

	var arms []loweredArm

	for _, arm := range node.Arms {
		if arm.Body == nil {
			return nil, fmt.Errorf("line %d: a match arm has no body", tok.Line)
		}

		body, err := l.expression(arm.Body)
		if err != nil {
			return nil, err
		}

		// Each alternative of an or-pattern becomes its own arm, because the
		// alternatives can bind different names. Folding them into one test with
		// one set of bindings would bind a name that one alternative never
		// introduced.
		for _, alternative := range patternAlternatives(arm.Pattern) {
			b := &patternBuilder{lowerer: l, subject: subject}

			test, bindings, err := b.build(alternative)
			if err != nil {
				return nil, err
			}

			guard := arm.Guard
			if guard != nil {
				lowered, err := l.expression(guard)
				if err != nil {
					return nil, err
				}
				// The guard runs before the arm's bindings exist, so they are
				// substituted into it. See the package comment for why that is
				// safe for a guard and not for a body.
				guard = substitute(lowered, bindingsOf(bindings))
			}

			arms = append(arms, loweredArm{
				test:     test,
				bindings: bindings,
				guard:    guard,
				body:     body,
			})
		}
	}

	// The chain is built from the last arm backwards, so each step's `else` is
	// the rest of the chain, and the innermost `else` — the one with nothing
	// after it — is the non-exhaustive case.
	var chain *ast.IfExpression
	for i := len(arms) - 1; i >= 0; i-- {
		arm := arms[i]

		condition := arm.test
		if arm.guard != nil {
			condition = &ast.InfixExpression{
				Token:    tok,
				Left:     condition,
				Operator: "&&",
				Right:    arm.guard,
			}
		}

		statements := make([]ast.Statement, 0, len(arm.bindings)+1)
		statements = append(statements, arm.bindings...)
		statements = append(statements, &ast.ExpressionStatement{Token: tok, Expression: arm.body})

		current := &ast.IfExpression{
			Token:       tok,
			Condition:   condition,
			Consequence: &ast.BlockStatement{Token: tok, Statements: statements},
		}

		switch {
		case chain != nil:
			current.Alternative = &ast.BlockStatement{
				Token:      tok,
				Statements: []ast.Statement{&ast.ExpressionStatement{Token: tok, Expression: chain}},
			}
		case i == 0:
			// Reached only when there is exactly one arm; with more, the last
			// arm is the one that has nothing after it.
			current.Alternative = nonExhaustive(tok, node, l)
		}

		chain = current
	}

	// The last arm is the innermost node. Attaching the non-exhaustive case to
	// the outermost node instead would silently discard every arm after the
	// first, which is a bug that only shows up at run time.
	if len(arms) > 1 {
		inmost := chain
		for {
			inner, ok := firstAlternativeExpression(inmost)
			if !ok {
				break
			}
			inmost = inner
		}
		inmost.Alternative = nonExhaustive(tok, node, l)
	}

	return &ast.LetExpression{
		Token: tok,
		Name:  subject,
		Value: loweredSubject,
		Body:  chain,
	}, nil
}

// nonExhaustive builds the block that reports a match with no applicable arm.
func nonExhaustive(tok token.Token, node *ast.MatchExpression, l *lowerer) *ast.BlockStatement {
	return &ast.BlockStatement{
		Token: tok,
		Statements: []ast.Statement{&ast.ExpressionStatement{Token: tok, Expression: l.unreachable(
			tok, fmt.Sprintf("no match arm applied to %s; the arms were %s",
				describeSubject(node.Subject), describeArms(node.Arms)))}},
	}
}

// firstAlternativeExpression returns the if/else chain nested in an if
// expression's alternative block.
func firstAlternativeExpression(node *ast.IfExpression) (*ast.IfExpression, bool) {
	if node.Alternative == nil || len(node.Alternative.Statements) == 0 {
		return nil, false
	}
	stmt, ok := node.Alternative.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		return nil, false
	}
	inner, ok := stmt.Expression.(*ast.IfExpression)
	return inner, ok
}

// unreachable builds the call that reports a non-exhaustive match.
func (l *lowerer) unreachable(tok token.Token, message string) ast.Expression {
	return &ast.CallExpression{
		Token:     tok,
		Function:  &ast.Identifier{Token: tok, Value: "unreachable"},
		Arguments: []ast.Expression{&ast.StringLiteral{Token: tok, Value: message}},
	}
}

// describeSubject names the subject for a diagnostic.
func describeSubject(expr ast.Expression) string {
	if ident, ok := expr.(*ast.Identifier); ok {
		return "`" + ident.Value + "`"
	}
	return "the matched value"
}

// describeArms lists the arms a match declared, for the non-exhaustive error.
func describeArms(arms []ast.MatchArm) string {
	out := make([]string, 0, len(arms))
	for _, arm := range arms {
		out = append(out, arm.Pattern.String())
	}
	switch {
	case len(out) == 0:
		return "none"
	case len(out) == 1:
		return out[0]
	case len(out) <= 4:
		return strings.Join(out[:len(out)-1], ", ") + " and " + out[len(out)-1]
	default:
		return strings.Join(out[:3], ", ") + fmt.Sprintf(", and %d more", len(out)-3)
	}
}

// bindingsOf maps each binding statement's name to the expression it binds, for
// substitution into a guard.
func bindingsOf(bindings []ast.Statement) map[string]ast.Expression {
	out := make(map[string]ast.Expression, len(bindings))
	for _, stmt := range bindings {
		let, ok := stmt.(*ast.LetStatement)
		if !ok || let.Name == nil || let.Value == nil {
			continue
		}
		out[let.Name.Value] = let.Value
	}
	return out
}

// substitute replaces every identifier named in `names` within expr.
//
// It does not descend into function literals. A closure inside a guard would be
// an odd thing to write, and not descending keeps the rewrite from reaching into
// a scope where the name may mean something else.
func substitute(expr ast.Expression, names map[string]ast.Expression) ast.Expression {
	if expr == nil || len(names) == 0 {
		return expr
	}

	switch node := expr.(type) {
	case *ast.Identifier:
		if replacement, ok := names[node.Value]; ok {
			return replacement
		}
		return node

	case *ast.PrefixExpression:
		return &ast.PrefixExpression{
			Token:    node.Token,
			Operator: node.Operator,
			Right:    substitute(node.Right, names),
		}

	case *ast.InfixExpression:
		return &ast.InfixExpression{
			Token:    node.Token,
			Left:     substitute(node.Left, names),
			Operator: node.Operator,
			Right:    substitute(node.Right, names),
		}

	case *ast.IndexExpression:
		return &ast.IndexExpression{
			Token: node.Token,
			Left:  node.Left,
			Index: substitute(node.Index, names),
		}

	case *ast.CallExpression:
		args := make([]ast.Expression, 0, len(node.Arguments))
		for _, arg := range node.Arguments {
			args = append(args, substitute(arg, names))
		}
		return &ast.CallExpression{Token: node.Token, Function: node.Function, Arguments: args}

	case *ast.StringTemplate:
		parts := make([]ast.TemplatePart, 0, len(node.Parts))
		for _, part := range node.Parts {
			if part.IsExpression {
				parts = append(parts, ast.TemplatePart{
					IsExpression: true,
					Expression:   substitute(part.Expression, names),
				})
				continue
			}
			parts = append(parts, part)
		}
		return &ast.StringTemplate{Token: node.Token, Parts: parts}

	default:
		return expr
	}
}

// patternBuilder turns one pattern into a test expression plus the bindings it
// introduces.
type patternBuilder struct {
	lowerer *lowerer

	// subject is the expression being matched. A struct field pattern replaces
	// it with the field's value, so that a nested pattern reads the right place.
	subject ast.Expression
}

// withSubject returns a builder for a nested position.
func (b *patternBuilder) withSubject(expr ast.Expression) *patternBuilder {
	return &patternBuilder{lowerer: b.lowerer, subject: expr}
}

// build returns the test for a pattern and the `let` statements that bind the
// names it introduces.
func (b *patternBuilder) build(pattern ast.MatchPattern) (ast.Expression, []ast.Statement, error) {
	tok := pattern.Token

	switch {
	case pattern.Wildcard:
		return &ast.Boolean{Token: tok, Value: true}, nil, nil

	case pattern.Binding != nil:
		// A bare name normally binds the subject, which is what makes
		// `match x { y => ... }` say "call it y".
		//
		// The exception is a name that a declared enum already uses. There,
		// `Circle => ...` is the variant, not a binding named Circle: the
		// alternative would be a program where the only way to name a variant
		// in a pattern is to write `Circle()`, while `Circle => ` silently binds
		// a value to a variable that will hold something else. Rust, Swift, and
		// Kotlin all read it the same way.
		if b.lowerer.isVariant(pattern.Binding.Value, 0) {
			return b.enumVariant(&ast.MatchEnumPattern{Variant: pattern.Binding})
		}

		stmt := &ast.LetStatement{Token: tok, Name: pattern.Binding, Value: b.subject}
		return &ast.Boolean{Token: tok, Value: true}, []ast.Statement{stmt}, nil

	case pattern.Literal != nil:
		lowered, err := b.lowerer.expression(pattern.Literal)
		if err != nil {
			return nil, nil, err
		}
		return &ast.InfixExpression{
			Token:    tok,
			Left:     b.subject,
			Operator: "==",
			Right:    lowered,
		}, nil, nil

	case pattern.EnumVariant != nil:
		return b.enumVariant(pattern.EnumVariant)

	case pattern.StructPattern != nil:
		return b.structPattern(pattern.StructPattern)

	case len(pattern.Alternatives) > 0:
		// The caller flattens alternatives into separate arms, so reaching here
		// means an or-pattern was nested inside another pattern.
		return nil, nil, fmt.Errorf("line %d: a `|` pattern cannot be nested inside another pattern", tok.Line)

	default:
		return nil, nil, fmt.Errorf("line %d: empty match pattern", tok.Line)
	}
}

// enumVariant builds the test and bindings for `Variant` and `Variant(binding)`.
func (b *patternBuilder) enumVariant(variant *ast.MatchEnumPattern) (ast.Expression, []ast.Statement, error) {
	tok := variant.Variant.Token
	hasPayload := variant.PayloadBinding != nil && !variant.DiscardPayload

	// A built-in case, such as `Ok(v)`.
	if accessor, ok := builtinsAsVariants[variant.Variant.Value]; ok {
		var bindings []ast.Statement
		if hasPayload {
			value := call(tok, accessor.payload, b.subject)
			if accessor.payloadNeedsDefault {
				value = &ast.CallExpression{
					Token:    tok,
					Function: value.(*ast.CallExpression).Function,
					Arguments: []ast.Expression{
						value.(*ast.CallExpression).Arguments[0],
						&ast.NullLiteral{Token: tok},
					},
				}
			}
			bindings = append(bindings, &ast.LetStatement{Token: tok, Name: variant.PayloadBinding, Value: value})
		}
		return call(tok, accessor.predicate, b.subject), bindings, nil
	}

	var bindings []ast.Statement
	if hasPayload {
		bindings = append(bindings, &ast.LetStatement{
			Token: tok,
			Name:  variant.PayloadBinding,
			Value: indexString(tok, b.subject, EnumPayloadKey),
		})
	}

	return &ast.InfixExpression{
		Token:    tok,
		Left:     indexString(tok, b.subject, EnumVariantKey),
		Operator: "==",
		Right:    &ast.StringLiteral{Token: tok, Value: variant.Variant.Value},
	}, bindings, nil
}

// structPattern builds the test and bindings for `Name { field: pattern, ... }`.
//
// Every listed field must match, and fields not listed are ignored. That is the
// usual rule: a pattern states what it cares about, and the type checker is what
// notices a pattern that forgot a field it should have constrained.
func (b *patternBuilder) structPattern(pattern *ast.MatchStructPattern) (ast.Expression, []ast.Statement, error) {
	tok := pattern.Name.Token

	if len(pattern.Fields) == 0 {
		// `Name { }` matches any value of that shape.
		return &ast.Boolean{Token: tok, Value: true}, nil, nil
	}

	var test ast.Expression
	var bindings []ast.Statement

	for _, field := range pattern.Fields {
		fieldValue := indexString(tok, b.subject, field.Name.Value)

		fieldTest, fieldBindings, err := b.withSubject(fieldValue).build(field.Pattern)
		if err != nil {
			return nil, nil, err
		}
		bindings = append(bindings, fieldBindings...)

		if test == nil {
			test = fieldTest
			continue
		}
		test = &ast.InfixExpression{Token: tok, Left: test, Operator: "&&", Right: fieldTest}
	}

	return test, bindings, nil
}

// indexString reads `target[key]`.
func indexString(tok token.Token, target ast.Expression, key string) ast.Expression {
	return &ast.IndexExpression{
		Token: tok,
		Left:  target,
		Index: &ast.StringLiteral{Token: tok, Value: key},
	}
}

// call builds a one-argument call.
func call(tok token.Token, name string, arg ast.Expression) ast.Expression {
	return &ast.CallExpression{
		Token:     tok,
		Function:  &ast.Identifier{Token: tok, Value: name},
		Arguments: []ast.Expression{arg},
	}
}

// patternAlternatives flattens an or-pattern into its alternatives, so each can
// become its own arm with its own bindings.
func patternAlternatives(pattern ast.MatchPattern) []ast.MatchPattern {
	if len(pattern.Alternatives) == 0 {
		return []ast.MatchPattern{pattern}
	}
	var out []ast.MatchPattern
	for _, alternative := range pattern.Alternatives {
		out = append(out, patternAlternatives(alternative)...)
	}
	return out
}
