package typecheck

import (
	"fmt"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/types"
)

// Literal narrowing and numeric-width arithmetic.
//
// # What is checked
//
// A width is a distinct type, so `Int` is not assignable to `u8` and no
// conversion is inserted anywhere — that is what "no implicit numeric coercion"
// means in practice. The single concession is that an **integer literal** takes
// the width of its context when it fits:
//
//	let x: u8 = 5;      // accepted — the literal had no width of its own
//	let x: u8 = 300;    // E0110 — it does not fit
//	let y: Int = x;     // E0101 — a width is not Int, and nothing converts it
//
// Arithmetic gives back the width when both sides have it and the same one, and
// falls back to `Int` otherwise — the widest thing the language has, so the
// answer cannot be too narrow.
//
// # What is not checked
//
// The run-time representation is still a 64-bit integer. A value computed at run
// time that exceeds its declared width is not caught: `let a: u8 = 200;
// let b: u8 = a + a;` is 400 and stays 400. Catching that needs a narrowing
// instruction in both engines, and a check that fires only for some values is
// worse than one documented as compile-time.

// CodeLiteralOutOfRange is reported when an integer literal does not fit the
// width it was written against.
const CodeLiteralOutOfRange = "E0110"

// assignNumericValue checks a value being stored into a declared numeric type.
//
// It is the one place a literal takes on a width, so it is also the one place a
// range has to be checked. A value that is not a literal goes through the
// ordinary assignability rule, where a width simply is not `Int`.
func (c *Checker) assignNumericValue(value ast.Expression, actual types.Type, declared types.Type, line, col int) bool {
	if declared == nil || actual == nil {
		return false
	}
	if !types.IsNumericWidth(declaredTypeName(declared)) {
		return false
	}

	number, isLiteral := c.literalValue(value)
	if !isLiteral {
		// Not a literal. The width is a real type and `Int` is a different one, so
		// this fails on its own — with the message naming which two.
		return false
	}

	if types.LiteralFits(declared, number) {
		return true
	}

	r, _ := types.RangeOf(declared)
	c.report(CodeLiteralOutOfRange,
		fmt.Sprintf("%d does not fit %s, which holds %d to %d",
			number, declaredTypeName(declared), r.Min, r.Max),
		line, col)
	return true
}

// literalValue is an integer expression's constant value, if it has one.
//
// A negative literal arrives as a negation of a literal rather than as a
// literal with a sign, because that is how the parser reads `-128`. Recognising
// it here is what lets `i8: -128` work while `i8: -129` does not — the minimum
// of a signed width is the one value a signedness check is most likely to get
// wrong.
func (c *Checker) literalValue(expr ast.Expression) (int64, bool) {
	switch v := expr.(type) {
	case *ast.IntegerLiteral:
		return v.Value, true

	case *ast.PrefixExpression:
		if v.Operator != "-" {
			return 0, false
		}
		inner, ok := c.literalValue(v.Right)
		if !ok {
			return 0, false
		}
		return -inner, true

	case *ast.CoalesceExpression:
		// `payload["step"] ?? 1` is 1 whenever the payload has nothing, so the
		// fallback is its constant. That is what lets it take a width from its
		// context the same way a bare literal does — without it, reading a number
		// out of a payload and adding it to an `i32` would have no answer.
		if v.Left != nil {
			if leftType := c.inferExpression(v.Left); leftType != nil && !leftType.Equals(types.Any) {
				return 0, false
			}
		}
		return c.literalValue(v.Right)

	case *ast.InfixExpression:
		// Handled because a `-` in an infix position reads as a subtraction of a
		// literal from nothing. It only arises if a caller has already folded a
		// negation this way; the parser produces a PrefixExpression instead.
		if v.Operator != "-" {
			return 0, false
		}
		if v.Left != nil {
			if zero, isZero := c.literalValue(v.Left); !isZero || zero != 0 {
				return 0, false
			}
		}
		inner, ok := c.literalValue(v.Right)
		if !ok {
			return 0, false
		}
		return -inner, true
	}
	return 0, false
}

// declaredTypeName renders a declared type for a message.
func declaredTypeName(t types.Type) string {
	switch v := t.(type) {
	case types.PrimitiveType:
		return v.Name
	case *types.PrimitiveType:
		if v != nil {
			return v.Name
		}
	}
	if t == nil {
		return ""
	}
	return t.String()
}

// numericArithmetic is the type of a binary arithmetic operator over two
// operands.
//
// A string operand makes the whole thing a string operation whatever the other
// side is, so it is asked about first: `"a" + 1` is a string, and a width on the
// other side does not change that.
func (c *Checker) numericArithmetic(e *ast.InfixExpression) types.Type {
	left := c.inferExpression(e.Left)
	right := c.inferExpression(e.Right)

	if left.Equals(types.String) || right.Equals(types.String) {
		return types.String
	}

	// `+` over something the checker cannot type is `Any`, not `Int`.
	//
	// Two untyped values may well be strings at run time — that is exactly what
	// `payload["name"] + payload["tag"]` is in the UI examples — and answering
	// `Int` would make assigning the result to a `String` a compile error in a
	// program that has always worked. The old code had the same answer by falling
	// through; it is written out here because `+` is the one operator where the
	// difference between `Any` and `Int` is observable.
	if e.Operator == "+" && !isKnownNumeric(left) && !isKnownNumeric(right) {
		return types.Any
	}

	_, leftIsLiteral := c.literalValue(e.Left)
	_, rightIsLiteral := c.literalValue(e.Right)

	// A bare literal takes the width of the other operand.
	//
	// Without this, `x + 1` where `x` is a `u8` would be `Int` — the literal has
	// no width of its own and nothing says which to pick. Adopting the operand's
	// width is the same context sensitivity as `let x: u8 = 5`, one level up, and
	// it is what makes a width survive arithmetic at all.
	//
	// It applies only when the *other* side is a width and the literal is the bare
	// `Int` a literal always has. A genuine `Int` variable beside a width falls
	// through to `ArithmeticResult`, which widens to `Int` — there is no width to
	// borrow from a value the checker has not narrowed.
	if leftIsLiteral && types.IsNumericWidth(otherTypeName(right)) {
		return right
	}
	if rightIsLiteral && types.IsNumericWidth(otherTypeName(left)) {
		return left
	}

	if types.IsFloatWidth(left) || types.IsFloatWidth(right) ||
		left.Equals(types.Float) || right.Equals(types.Float) {
		return types.Float
	}

	return types.ArithmeticResult(left, right)
}

// isKnownNumeric reports whether a type is a number the checker can reason about.
//
// `Any` is the interesting case: it is not numeric, and treating it as such is
// what would let a run-time string concatenation be typed as arithmetic.
func isKnownNumeric(t types.Type) bool {
	if t == nil {
		return false
	}
	if t.Equals(types.Int) || t.Equals(types.Float) {
		return true
	}
	return types.IsNumericWidth(otherTypeName(t))
}

// checkAssignable is the one assignment check, so every site narrows a literal
// the same way.
//
// It lives here rather than at the four call sites because `let`, `state`, an
// argument, and a `return` all need the same answer, and a rule implemented four
// times is a rule that four times disagrees with itself. The first version of this
// put the narrowing only on `let`, and `state count: Int32 = 0` in the shipped
// examples stopped compiling — which is the failure mode this function exists to
// remove.
func (c *Checker) checkAssignable(value ast.Expression, actual, declared types.Type, line, col int, message string) bool {
	if c.assignNumericValue(value, actual, declared, line, col) {
		return true
	}
	if actual.AssignableTo(declared) {
		return true
	}
	c.report("E0101", message, line, col)
	return false
}

// argLine is the position an argument argument's diagnostic should point at,
// falling back to the call when the argument itself has none.
func argLine(arg ast.Expression, call *ast.CallExpression) int {
	line, _ := exprPosition(arg)
	if line == 0 && call != nil {
		return call.Token.Line
	}
	return line
}

// exprPositionOr is the column that goes with argLine.
func exprPositionOr(arg ast.Expression, call *ast.CallExpression) int {
	_, col := exprPosition(arg)
	if col == 0 && call != nil {
		return call.Token.Column
	}
	return col
}

// otherTypeName is the type's own name, or "" when it has none.
func otherTypeName(t types.Type) string {
	if p, ok := t.(types.PrimitiveType); ok {
		return p.Name
	}
	return ""
}
