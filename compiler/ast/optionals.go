package ast

import (
	"bytes"

	"github.com/joysriramsarkar/nilLang/compiler/token"
)

// Optional access and null coalescing.
//
// # Both are lowering, not new opcodes
//
// `a?.b` and `a ?? b` are rewritten into `if`/`else` before any backend sees the
// program, for the same reason `for` and `match` are (ADR-0007): one
// implementation means the two engines run the same code and cannot disagree
// about what an operator means.
//
// # Why the receiver is evaluated once
//
// A naive `if (a == null) { null } else { a.b }` evaluates `a` twice, which is
// wrong for any receiver with a side effect and wrong for a receiver that is
// expensive. Both lowerings bind the receiver to a hidden local first, exactly as
// a `match` subject is bound.

// CoalesceExpression is `left ?? right`: `left` unless it is null, then `right`.
type CoalesceExpression struct {
	Token token.Token // token.QUESTION_QUESTION
	Left  Expression
	Right Expression
}

func (ce *CoalesceExpression) expressionNode()      {}
func (ce *CoalesceExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CoalesceExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	if ce.Left != nil {
		out.WriteString(ce.Left.String())
	}
	out.WriteString(" ?? ")
	if ce.Right != nil {
		out.WriteString(ce.Right.String())
	}
	out.WriteString(")")
	return out.String()
}

// SafeAccessExpression is `left?.member`.
//
// It is a distinct node rather than a flag on DotExpression because the two mean
// different things and every consumer has to act on the difference. A flag would
// push the branch into each consumer, and the branch is the whole point.
type SafeAccessExpression struct {
	Token  token.Token // token.QUESTION_DOT
	Left   Expression
	Member *Identifier
}

func (se *SafeAccessExpression) expressionNode()      {}
func (se *SafeAccessExpression) TokenLiteral() string { return se.Token.Literal }
func (se *SafeAccessExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	if se.Left != nil {
		out.WriteString(se.Left.String())
	}
	out.WriteString("?.")
	if se.Member != nil {
		out.WriteString(se.Member.String())
	}
	out.WriteString(")")
	return out.String()
}
