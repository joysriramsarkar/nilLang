package typecheck

import (
	"fmt"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/types"
)

// Member access: `a.b`, `a.b.c`, and the `?.` forms.
//
// # Why this is its own file
//
// A member chain used to infer as `Any`, always. Every consumer that needed to
// know a field's type — `let x: Int = o.count`, a trait method called on a field,
// a `?.` whose receiver is a chain — was therefore working with no information,
// and a wrong member reached the backend and failed there instead of being
// reported where it was written.
//
// # What is and is not known
//
// A member's type is known when the receiver is a declared struct: the checker
// holds its field list. For every other receiver — a hash, a primitive, a value
// whose type could not be inferred — the member's type is `Any` and nothing is
// reported. That is deliberate. A hash has whatever keys were put in it, so
// refusing an unknown key would reject correct programs; and a receiver whose
// type the checker lost is not evidence of a mistake, so guessing would invent
// errors that are not there.
//
// A receiver whose declared type *lacks* the member is a fact rather than a
// guess, and `E0111` reports it.

// CodeUnknownMember is reported when a member chain names something the
// receiver's declared type does not have.
const CodeUnknownMember = "E0111"

// memberAccess infers `left.member` and checks that the member exists.
func (c *Checker) memberAccess(node *ast.DotExpression) types.Type {
	if node == nil || node.Member == nil {
		return types.Any
	}

	receiver := c.inferExpression(node.Left)
	return c.memberType(receiver, node.Member, node.Left)
}

// safeMemberAccess infers `left?.member`.
//
// It checks the same as a plain access, and for the same reason: `?.` changes
// what happens when the receiver is *null*, not what a member is called. A member
// that does not exist is a mistake either way.
func (c *Checker) safeMemberAccess(node *ast.SafeAccessExpression) types.Type {
	if node == nil || node.Member == nil {
		return types.Any
	}

	receiver := c.inferExpression(node.Left)
	return c.memberType(receiver, node.Member, node.Left)
}

// memberType resolves one member on a receiver, reporting `E0111` when the
// receiver's declared type does not have it.
func (c *Checker) memberType(receiver types.Type, member *ast.Identifier, written ast.Expression) types.Type {
	memberType, known, checkable := lookupMember(receiver, member.Value)
	if !checkable {
		return types.Any
	}
	if !known {
		c.report(CodeUnknownMember,
			fmt.Sprintf("%s has no member %q", describeReceiver(receiver, written), member.Value),
			member.Token.Line, member.Token.Column)
		return types.Any
	}
	return memberType
}

// lookupMember finds a named member on a receiver.
//
// The third result is the one that matters: it says whether the checker knows
// enough to answer. It is false for a hash — whose keys are whatever was put in
// it — and for any type it could not infer, so nothing is reported in either
// case.
func lookupMember(receiver types.Type, name string) (types.Type, bool, bool) {
	switch t := receiver.(type) {
	case *types.StructType:
		if t == nil {
			return nil, false, false
		}
		// A struct with no fields is a placeholder: the checker saw the name but
		// never a declaration for it. Reporting against it would blame the use
		// for a declaration that is missing.
		if len(t.Fields) == 0 {
			return nil, false, false
		}
		if fieldType, ok := t.GetField(name); ok {
			return fieldType, true, true
		}
		return nil, false, true

	case *types.EnumType:
		// An enum value is a variant, not a record. These are the two members it
		// has, which is the shape the lowering gives it at run time.
		if t == nil || len(t.Variants) == 0 {
			return nil, false, false
		}
		switch name {
		case "variant":
			return types.String, true, true
		case "payload":
			return types.Any, true, true
		}
		return nil, false, true
	}

	return nil, false, false
}

// describeReceiver renders a receiver for a diagnostic, preferring what the
// developer wrote over the type the checker inferred.
//
// The written text is what a reader recognises. `o.i.nosuch` is easier to act on
// than "struct Inner has no member nosuch", because it names the exact chain that
// failed.
func describeReceiver(receiver types.Type, written ast.Expression) string {
	if written != nil {
		if text := written.String(); text != "" {
			return text
		}
	}
	if receiver == nil {
		return "this value"
	}
	return receiver.String()
}
