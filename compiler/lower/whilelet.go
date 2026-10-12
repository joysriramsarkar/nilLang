package lower

import (
	"fmt"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
)

// whileLet lowers `while let <pattern> = <value> { body }` into a `while` loop
// whose test is the pattern and whose body begins by binding the pattern's names.
//
// # The value is re-evaluated on every pass
//
// That is what makes the form useful. `while let Some(x) = items[i]` walks a
// collection because the body advances `i` and the next pass therefore reads a
// different element; binding the value once before the loop would read
// `items[0]` forever and never terminate. Rust's `while let` behaves the same way.
//
// The cost is that a value with side effects is evaluated once for the test and
// again for each binding the pattern introduces — two or three times per pass
// rather than once. That is the price of not needing a separate pop-style
// operation to advance a cursor, which the language does not have. A value whose
// evaluation is expensive or observable should be bound to a variable first:
//
//	let head = items[0];
//	while let Some(x) = head { ... }
//
// # Bindings
//
// The names a pattern introduces are bound as ordinary `let` statements at the
// top of the body, so they are scoped to one pass, are fresh each time, and a
// closure created in the body captures them the way a developer expects.
func (l *lowerer) whileLet(node *ast.WhileLetStatement) (ast.Statement, error) {
	if node.Value == nil {
		return nil, fmt.Errorf("line %d: `while let` has no value to match", node.Token.Line)
	}

	tok := node.Token

	lowered, err := l.expression(node.Value)
	if err != nil {
		return nil, err
	}

	b := &patternBuilder{lowerer: l, subject: lowered}
	test, bindings, err := b.build(node.Pattern)
	if err != nil {
		return nil, err
	}

	bodyStatements := make([]ast.Statement, 0, len(bindings)+1)
	bodyStatements = append(bodyStatements, bindings...)
	if node.Body != nil {
		bodyStatements = append(bodyStatements, node.Body)
	}

	loop := &ast.WhileStatement{
		Token:     tok,
		Condition: test,
		Body:      &ast.BlockStatement{Token: tok, Statements: bodyStatements},
	}

	return &ast.BlockStatement{Token: tok, Statements: []ast.Statement{loop}}, nil
}
