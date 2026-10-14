package lower

import (
	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/token"
)

// Optional access and null coalescing, lowered to `if`/`else`.
//
// # The shapes
//
//	a ?? b      →  let __nil_x = a; if (isNull(__nil_x)) { b } else { __nil_x }
//	a?.b        →  let __nil_x = a; if (isNull(__nil_x)) { null } else { __nil_x.b }
//	a?.b(args)  →  let __nil_x = a; if (isNull(__nil_x)) { null } else { __nil_x.b(args) }
//
// # Why the receiver is bound to a hidden local
//
// A naive `if (a == null) { null } else { a.b }` evaluates `a` twice. That is
// wrong for a receiver with a side effect and wasteful for an expensive one, and
// it is the same reason a `match` subject is bound before its arms run.
//
// # Why `a?.b(args)` is one construct rather than two
//
// Lowering the callee and letting the call follow would evaluate the safe
// navigation, get `null` for the empty case, and then call it. The call has to be
// inside the `else` branch, so the whole call is lowered together. Otherwise
// `a?.b()` on a null `a` would call null, and the "safe" in safe navigation would
// mean nothing at all.
//
// # Why a lowering
//
// For the same reason `for` and `match` are (ADR-0007): one implementation means
// the two engines run the same code and cannot disagree about what an operator
// means.

// coalesce lowers `left ?? right`.
func (l *lowerer) coalesce(node *ast.CoalesceExpression) (ast.Expression, error) {
	if node.Left == nil || node.Right == nil {
		return nil, &Error{Line: node.Token.Line, Msg: "`??` needs a value on both sides"}
	}

	tok := node.Token
	subject := l.identifier(tok, l.unique("value"))

	loweredLeft, err := l.expression(node.Left)
	if err != nil {
		return nil, err
	}
	loweredRight, err := l.expression(node.Right)
	if err != nil {
		return nil, err
	}

	// The fallback is evaluated only when it is needed. That is the whole point
	// of the operator: `findUser(id) ?? makeDefaultUser()` must not build a
	// default user when there is already one.
	return &ast.LetExpression{
		Token: tok,
		Name:  subject,
		Value: loweredLeft,
		Body: &ast.IfExpression{
			Token:     tok,
			Condition: call(tok, "isNull", subject),
			Consequence: &ast.BlockStatement{
				Token:      tok,
				Statements: []ast.Statement{&ast.ExpressionStatement{Token: tok, Expression: loweredRight}},
			},
			Alternative: &ast.BlockStatement{
				Token:      tok,
				Statements: []ast.Statement{&ast.ExpressionStatement{Token: tok, Expression: subject}},
			},
		},
	}, nil
}

// findSafeAccess walks a receiver spine looking for a `?.`.
//
// The spine is whatever a receiver is made of: the left of a `.` access, the
// function of a call. A `?.` anywhere along it means the *whole* expression that
// sits above it has to be guarded, because that is what a developer means:
// in `o?.i.v`, a null `o` yields null, not an attempt to read `.v` off a null.
func findSafeAccess(expr ast.Expression) *ast.SafeAccessExpression {
	switch n := expr.(type) {
	case *ast.SafeAccessExpression:
		return n
	case *ast.DotExpression:
		return findSafeAccess(n.Left)
	case *ast.CallExpression:
		return findSafeAccess(n.Function)
	}
	return nil
}

// replaceSafeAccess rebuilds a chain with one `?.` node swapped for a plain
// access on the guarded local.
//
// Only the spine is rebuilt. The replacement is a DotExpression on the local, so
// a second pass over the result finds no `?.` and the recursion terminates.
func (l *lowerer) replaceSafeAccess(expr ast.Expression, local ast.Expression, safe *ast.SafeAccessExpression) ast.Expression {
	switch n := expr.(type) {
	case *ast.SafeAccessExpression:
		if n == safe {
			return &ast.DotExpression{Token: n.Token, Left: local, Member: n.Member}
		}
		return expr
	case *ast.DotExpression:
		return &ast.DotExpression{
			Token:  n.Token,
			Left:   l.replaceSafeAccess(n.Left, local, safe),
			Member: n.Member,
		}
	case *ast.CallExpression:
		return &ast.CallExpression{
			Token:     n.Token,
			Function:  l.replaceSafeAccess(n.Function, local, safe),
			Arguments: n.Arguments,
		}
	}
	return expr
}

// guardChain lowers an access whose receiver spine contains a `?.`, by putting
// the whole access inside the guard.
//
// The alternative — lowering each node on its own — is what makes `o?.i.v` read
// `.v` off a null: the `?.` guards only `o.i`, and `.v` then runs regardless.
func (l *lowerer) guardChain(tok token.Token, whole ast.Expression, safe *ast.SafeAccessExpression) (ast.Expression, bool, error) {
	if safe == nil || safe.Left == nil {
		return nil, false, nil
	}

	local := l.identifier(tok, l.unique("recv"))

	loweredReceiver, err := l.expression(safe.Left)
	if err != nil {
		return nil, true, err
	}

	inner, err := l.expression(l.replaceSafeAccess(whole, local, safe))
	if err != nil {
		return nil, true, err
	}

	return &ast.LetExpression{
		Token: tok,
		Name:  local,
		Value: loweredReceiver,
		Body: &ast.IfExpression{
			Token:     tok,
			Condition: call(tok, "isNull", local),
			Consequence: &ast.BlockStatement{
				Token:      tok,
				Statements: []ast.Statement{&ast.ExpressionStatement{Token: tok, Expression: &ast.NullLiteral{Token: tok}}},
			},
			Alternative: &ast.BlockStatement{
				Token:      tok,
				Statements: []ast.Statement{&ast.ExpressionStatement{Token: tok, Expression: inner}},
			},
		},
	}, true, nil
}

// safeAccess lowers `left?.member`.
func (l *lowerer) safeAccess(node *ast.SafeAccessExpression) (ast.Expression, error) {
	if node.Left == nil || node.Member == nil {
		return nil, &Error{Line: node.Token.Line, Msg: "`?.` needs a receiver and a member"}
	}
	return l.safeAccessBody(node, node.Token, node.Left, node.Member, nil)
}

// safeAccessCall lowers `left?.member(args)`.
func (l *lowerer) safeAccessCall(node *ast.CallExpression, access *ast.SafeAccessExpression) (ast.Expression, error) {
	if access.Left == nil || access.Member == nil {
		return nil, &Error{Line: access.Token.Line, Msg: "`?.` needs a receiver and a member"}
	}
	// The lookup key is the SafeAccessExpression, not the call. That is the node
	// the checker records against, because that is the node that names the
	// method — the call only supplies arguments.
	return l.safeAccessBody(access, access.Token, access.Left, access.Member, node.Arguments)
}

// safeAccessBody is the shared shape behind both `a?.b` and `a?.b(args)`.
//
// `callee` is the node the checker recorded a method resolution under. Passing it
// in rather than looking it up from a rebuilt node is what makes `d?.greet()`
// call the trait method: the node this pass constructs is new, so a lookup on it
// would find nothing and the call would fall back to a field access on a hash.
func (l *lowerer) safeAccessBody(callee ast.Expression, tok token.Token, receiver ast.Expression, member *ast.Identifier, args []ast.Expression) (ast.Expression, error) {
	subject := l.identifier(tok, l.unique("recv"))

	loweredReceiver, err := l.expression(receiver)
	if err != nil {
		return nil, err
	}

	var accessed ast.Expression

	if generated, ok := l.resolve(callee); ok {
		// A trait method: the generated function, with the receiver in front of
		// the written arguments, exactly as a plain `d.greet()` call lowers.
		callArgs := make([]ast.Expression, 0, len(args)+1)
		callArgs = append(callArgs, subject)
		for _, arg := range args {
			lowered, err := l.expression(arg)
			if err != nil {
				return nil, err
			}
			callArgs = append(callArgs, lowered)
		}
		accessed = &ast.CallExpression{
			Token:     tok,
			Function:  &ast.Identifier{Token: tok, Value: generated},
			Arguments: callArgs,
		}
	} else {
		dot := &ast.DotExpression{Token: tok, Left: subject, Member: member}
		accessed = dot
		if args != nil {
			loweredArgs := make([]ast.Expression, 0, len(args))
			for _, arg := range args {
				lowered, err := l.expression(arg)
				if err != nil {
					return nil, err
				}
				loweredArgs = append(loweredArgs, lowered)
			}
			accessed = &ast.CallExpression{Token: tok, Function: dot, Arguments: loweredArgs}
		}
	}

	return &ast.LetExpression{
		Token: tok,
		Name:  subject,
		Value: loweredReceiver,
		Body: &ast.IfExpression{
			Token:     tok,
			Condition: call(tok, "isNull", subject),
			Consequence: &ast.BlockStatement{
				Token:      tok,
				Statements: []ast.Statement{&ast.ExpressionStatement{Token: tok, Expression: &ast.NullLiteral{Token: tok}}},
			},
			Alternative: &ast.BlockStatement{
				Token:      tok,
				Statements: []ast.Statement{&ast.ExpressionStatement{Token: tok, Expression: accessed}},
			},
		},
	}, nil
}
