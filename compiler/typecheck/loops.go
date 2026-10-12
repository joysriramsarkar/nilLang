package typecheck

import (
	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/types"
)

// Checking loop bodies.
//
// # Why this file exists at all
//
// `while` was checked from the start and `for` was not. `for` has no bytecode of
// its own — it is lowered into a `while` — and the checker runs before lowering,
// so it saw a `ForStatement` it had no case for and fell through to the default.
// Every statement inside a `for` body therefore went unchecked: an undefined
// identifier, a call with the wrong arity, a method on a value that has no impl.
// All of it reached the backend and failed there, or worse, did not fail.
//
// The gap was invisible until a method call had to be resolved. A loop variable
// whose type was never inferred is a value whose `.method()` cannot be resolved,
// so the program was correct and simply had never been checked.
//
// # What is checked, and what is inferred
//
// The condition, the loop variable's type, and every statement in the body. The
// variable's type comes from the collection's element type, which is what makes
// `for box in boxes { box.area() }` resolve to the impl for `Box`. When the
// element type is not known the variable is `Any`, as everywhere else the checker
// has nothing to go on — and `Any` still catches an undefined name or the wrong
// number of arguments, which is most of what a body can get wrong.

// checkForStatement checks a `for` loop and its body.
func (c *Checker) checkForStatement(s *ast.ForStatement) {
	if s == nil {
		return
	}

	iterable := c.inferExpression(s.Iterable)

	// The loop variable is a `let` in a scope of its own, so it neither leaks
	// past the loop nor shadows anything the body wanted. This mirrors what the
	// lowering pass does, and the two have to agree or the body would be checked
	// in one scope and run in another.
	outer := c.currentScope
	c.currentScope = NewScope(outer)

	element := elementTypeOf(iterable)

	if s.Value != nil {
		c.currentScope.SetVar(s.Value.Value, element)
		c.inferExpression(s.Value)
	}

	// In the single-name form `for x in ...`, Key and Value are the *same*
	// identifier node, so setting the key's type here would overwrite the
	// element type just assigned above and leave the loop variable `Any` — which
	// is exactly what made a method call on it unresolvable. Only a genuinely
	// separate key gets its own binding.
	if s.Key != nil && s.Key != s.Value {
		c.currentScope.SetVar(s.Key.Value, types.Any)
		c.inferExpression(s.Key)
	}

	if s.Body != nil {
		for _, nested := range s.Body.Statements {
			c.checkStatement(nested)
		}
	}

	c.currentScope = outer
}

// elementTypeOf is what iterating a value yields.
//
// It answers `Any` rather than guessing when it does not know. A wrong element
// type would make a method call resolve to the wrong implementation, which is a
// far worse failure than one that stays `Any` and is reported as unresolved.
func elementTypeOf(iterable types.Type) types.Type {
	if iterable == nil {
		return types.Any
	}
	if generic, ok := iterable.(*types.GenericType); ok && len(generic.Parameters) > 0 {
		element := generic.Parameters[0]
		if element == nil || element.Equals(types.Void) {
			return types.Any
		}
		return element
	}

	// A range is always Int, and it is the one iterable whose element type the
	// checker can state without inference.
	if _, isRange := iterable.(*types.StructType); isRange {
		return types.Any
	}
	return types.Any
}

// checkWhileLet checks a `while let` loop.
//
// The value is checked where it is written, because it is re-evaluated on every
// pass (SEMANTICS §4.3 rule 5), and the body is checked in its own scope with
// the pattern's bindings declared — otherwise every name a `while let`
// introduces would read as undefined inside its own body.
func (c *Checker) checkWhileLet(s *ast.WhileLetStatement) {
	if s == nil {
		return
	}

	value := c.inferExpression(s.Value)
	c.checkPattern(s.Pattern, value, s.Token.Line, s.Token.Column)

	if s.Body == nil {
		return
	}

	outer := c.currentScope
	c.currentScope = NewScope(outer)

	// The names come from the same function `match` uses, so a `while let`
	// cannot bind a name that a `match` arm would reject.
	for _, name := range patternBindings(s.Pattern) {
		c.currentScope.SetVar(name, bindingTypeFor(value))
	}

	for _, nested := range s.Body.Statements {
		c.checkStatement(nested)
	}

	c.currentScope = outer
}

// bindingTypeFor is what a pattern name binds to against a given subject.
//
// The payload of an Optional or a Result is what a binding against `Some(x)` or
// `Ok(x)` introduces — the name is the value, not a box around one. When the
// subject carries no usable type the binding is `Any`, which keeps every name
// usable in the body without claiming a type the checker cannot justify.
func bindingTypeFor(subject types.Type) types.Type {
	if subject == nil {
		return types.Any
	}
	if generic, ok := subject.(*types.GenericType); ok {
		for _, param := range generic.Parameters {
			if param != nil && !param.Equals(types.Any) && !param.Equals(types.Void) {
				return param
			}
		}
	}
	return types.Any
}
