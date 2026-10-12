package hir

import (
	"github.com/joysriramsarkar/nilLang/compiler/types"
)

// Loop-invariant hoisting.
//
// # What moves, and what may not
//
// A `let` whose value depends on nothing the loop body changes is computed once
// before the loop instead of on every iteration. That is worth doing because a
// loop body that touches a collection almost always touches its length first.
//
// The rule for "nothing the body changes" is deliberately narrow: the binding is
// hoistable only when the body's free variables do not intersect the names the
// body writes. A looser rule — "the bound expression is a call to a pure-looking
// function" — is what a developer reaches for, and it is wrong: `now()` is pure by
// every syntactic test and produces a different answer on every call, so hoisting
// it changes the program.
//
// # Where the hoisted statements go
//
// `WhileStmt.PreLoop` holds them, and MIR lowers them into the block before the
// loop header. Keeping them on the loop rather than mutating the enclosing block
// is what lets the same optimisation run on a loop nested anywhere without the
// caller having to know where the loop was.

// hoistLoopInvariants separates a body into what can move above the loop and what
// must stay.
func hoistLoopInvariants(body *BlockStmt) ([]Statement, *BlockStmt) {
	if body == nil || len(body.Statements) == 0 {
		return nil, body
	}

	written := writtenNames(body)
	candidates := make([]Statement, 0, len(body.Statements))
	remaining := make([]Statement, 0, len(body.Statements))

	// Only the *leading run* of hoistable bindings is considered, and the scan
	// stops at the first statement that is not one.
	//
	// Stopping matters for two reasons. A binding after a `return` does not run on
	// the iteration that returns, so hoisting it executes it at least once more
	// than the program did. And once a binding is left behind because it *is* loop
	// variant, every binding after it is usually variant too — they were written
	// in that order for a reason, and continuing past it hoists a value out of the
	// middle of a computation whose earlier parts stayed behind.
	for idx, stmt := range body.Statements {
		let, isLet := stmt.(*LetStmt)
		if !isLet || let.Name == "" || !isHoistable(let, written) ||
			usedByNestedLoop(body, idx+1, let.Name) {
			remaining = append(remaining, stmt)
			if idx == 0 || terminatesOrBranches(stmt) {
				// The first statement, or one after which control may not reach
				// the next iteration's start, ends the hoistable run. Everything
				// from here on stays where it is.
				remaining = append(remaining, body.Statements[idx+1:]...)
				break
			}
			continue
		}
		candidates = append(candidates, stmt)
	}

	if len(candidates) == 0 {
		return nil, body
	}
	// Re-append the tail in order, so the body keeps its original statement order.
	return candidates, &BlockStmt{Statements: remaining}
}

// terminatesOrBranches reports whether a statement may stop the current
// iteration from reaching the next one.
//
// A `return` obviously does. A nested loop does not terminate, but everything
// after it in the body is written as though the loop had run, and hoisting across
// it would move a binding above a computation the author put there for a reason.
// Treating both as a boundary is the conservative reading, and conservatism is
// what this pass must be: the cost of missing a hoist is one repeated call, and
// the cost of performing a wrong one is a wrong answer.
func terminatesOrBranches(stmt Statement) bool {
	switch stmt.(type) {
	case *ReturnStmt, *WhileStmt, *IfStmt:
		return true
	}
	return false
}

// isHoistable reports whether a binding can be computed before the loop.
//
// # The four conditions, and why each exists
//
//   - its value mentions no name the body writes — otherwise the loop could change
//     the answer;
//   - it is not a `const`, because a constant's declaration *is* its promise and
//     moving the promise changes where it was made;
//   - **it is not the initialiser of a counter an inner loop walks.** Hoisting
//     `let j = 0` out of a loop whose body contains `while j < 3 { … j = j + 1 }`
//     moves the reset *before* the outer loop instead of before the inner one, so
//     `j` is never reset again. The outer loop runs once and the inner loop runs
//     once, and the program counts N instead of N².
//   - every name it mentions is bound outside the loop — a name bound inside the
//     body does not exist yet where the hoist would put it.
//
// The third is not covered by the first. `let j = 0` mentions nothing the body
// writes *at the point of the binding*, because the inner loop that writes `j`
// comes after. Only a walk of the statements that follow it can see that.
func isHoistable(let *LetStmt, written map[string]bool) bool {
	if let.Constant || let.Value == nil {
		return false
	}

	free := freeNames(let.Value)
	for name := range free {
		if written[name] {
			return false
		}
	}
	return true
}

// usedByNestedLoop reports whether a nested loop in the block reads or writes a
// name.
//
// It exists because the leading-binding scan stops at the first non-hoistable
// statement, which means the statements it has *not* looked at yet may still refer
// to the name being hoisted. Moving a binding above a loop that consumes it is the
// classic hoist bug, and it is silent: the program runs and counts wrong.
//
// # Why a *nested loop* and not any later statement
//
// A later statement reading the name is not a reason to refuse: the binding is
// still in scope after the hoist, at the same point relative to that statement
// only if the hoist stays inside the same block — which it does. `let n = len(xs)`
// followed by `if n > 3` hoists perfectly well.
//
// What refuses is a *loop* that walks the name. `let j = 0` before
// `while j < 3 { … j = j + 1 }` is hoisted above the outer loop, so the reset
// happens once instead of once per outer iteration, and the inner loop runs once
// instead of N times. A loop changes when its counter is reset; an ordinary read
// does not.
func usedByNestedLoop(block *BlockStmt, from int, name string) bool {
	for idx := from; idx < len(block.Statements); idx++ {
		if loopTouchesName(block.Statements[idx], name) {
			return true
		}
	}
	return false
}

// loopTouchesName reports whether a statement contains a loop that reads or writes
// a name.
func loopTouchesName(stmt Statement, name string) bool {
	switch s := stmt.(type) {
	case *WhileStmt:
		if s.Condition != nil && namesMention(s.Condition)[name] {
			return true
		}
		if s.Body != nil {
			for _, inner := range s.Body.Statements {
				if namesMentionStatement(inner)[name] || writtenNamesOf(inner)[name] {
					return true
				}
			}
		}
	case *IfStmt:
		if s.Condition != nil && namesMention(s.Condition)[name] {
			return true
		}
		for _, inner := range s.Consequence.Statements {
			if loopTouchesName(inner, name) {
				return true
			}
		}
		if s.Alternative != nil {
			for _, inner := range s.Alternative.Statements {
				if loopTouchesName(inner, name) {
					return true
				}
			}
		}
	}
	return false
}

func namesMention(expr Expression) map[string]bool {
	names := make(map[string]bool)
	collectNames(expr, names)
	return names
}

func namesMentionStatement(stmt Statement) map[string]bool {
	names := make(map[string]bool)
	collectStatementNames(stmt, names)
	return names
}

// statementTouchesName reports whether a statement reads or writes a name.
//
// It is kept because `writtenNamesOf` is used elsewhere, but nothing calls it any
// more: the rule it served — "a later use of the name blocks the hoist" — was
// wrong, since a read after a hoist still finds the binding in scope. Only a *loop*
// consumes a binding, and only a loop changes when it is reset.
func statementTouchesName(stmt Statement, name string) bool {
	touched := make(map[string]bool)
	collectStatementNames(stmt, touched)
	if touched[name] {
		return true
	}
	return writtenNamesOf(stmt)[name]
}

// writtenNamesOf is the names one statement writes.
func writtenNamesOf(stmt Statement) map[string]bool {
	written := make(map[string]bool)
	switch s := stmt.(type) {
	case *LetStmt:
		written[s.Name] = true
	case *AssignStmt:
		// Only an assignment to a variable names a variable. `a[1] = 9` writes
		// into a container, which no loop-invariant can be hoisted across — the
		// value it reads may change between iterations.
		if name, ok := s.VarName(); ok {
			written[name] = true
		}
	}
	return written
}

// hoistDeclarations turns hoisted statements into the form the loop carries.
//
// They are not re-optimised here: the value was already optimised when the body
// was, and folding it a second time is a chance to fold something the first pass
// deliberately left.
func hoistDeclarations(stmts []Statement) []Statement {
	if len(stmts) == 0 {
		return nil
	}
	out := make([]Statement, 0, len(stmts))
	out = append(out, stmts...)
	return out
}

// writtenNames is every name the block assigns to.
func writtenNames(block *BlockStmt) map[string]bool {
	written := make(map[string]bool)
	for _, stmt := range block.Statements {
		switch s := stmt.(type) {
		case *LetStmt:
			written[s.Name] = true
		case *AssignStmt:
			// As above: a container write is not a variable write.
			if name, ok := s.VarName(); ok {
				written[name] = true
			}
		}
	}
	return written
}

// freeNames is every identifier an expression reads.
//
// It is a name walk rather than a type question on purpose: a name the body writes
// is disqualifying whether or not the checker could prove the two expressions are
// related. Proving things about aliases is exactly the analysis this pass must not
// depend on, because the pass runs on code that has already been checked by
// something else with different knowledge.
func freeNames(expr Expression) map[string]bool {
	names := make(map[string]bool)
	collectNames(expr, names)
	return names
}

func collectNames(expr Expression, into map[string]bool) {
	switch e := expr.(type) {
	case nil:
		return
	case *VarRef:
		into[e.Name] = true
	case *BinaryExpr:
		collectNames(e.Left, into)
		collectNames(e.Right, into)
	case *UnaryExpr:
		collectNames(e.Right, into)
	case *CallExpr:
		collectNames(e.Callee, into)
		for _, arg := range e.Args {
			collectNames(arg, into)
		}
	case *IndexExpr:
		collectNames(e.Left, into)
		collectNames(e.Index, into)
	case *DotExpr:
		collectNames(e.Left, into)
	case *IfExpr:
		collectNames(e.Condition, into)
		collectNames(e.WhenTrue, into)
		collectNames(e.WhenFalse, into)
	case *BlockExpr:
		for _, stmt := range e.Statements {
			collectStatementNames(stmt, into)
		}
	case *ListLit:
		for _, elem := range e.Elements {
			collectNames(elem, into)
		}
	case *MapLit:
		for idx := range e.Keys {
			collectNames(e.Keys[idx], into)
		}
		for idx := range e.Values {
			collectNames(e.Values[idx], into)
		}
	}
}

func collectStatementNames(stmt Statement, into map[string]bool) {
	switch s := stmt.(type) {
	case *LetStmt:
		collectNames(s.Value, into)
	case *AssignStmt:
		collectNames(s.Target, into)
		collectNames(s.Value, into)
	case *ReturnStmt:
		collectNames(s.Value, into)
	case *ExprStmt:
		collectNames(s.Expr, into)
	case *IfStmt:
		collectNames(s.Condition, into)
		for _, inner := range s.Consequence.Statements {
			collectStatementNames(inner, into)
		}
		if s.Alternative != nil {
			for _, inner := range s.Alternative.Statements {
				collectStatementNames(inner, into)
			}
		}
	case *WhileStmt:
		collectNames(s.Condition, into)
		if s.Body != nil {
			for _, inner := range s.Body.Statements {
				collectStatementNames(inner, into)
			}
		}
	}
}

// isPureType is a small guard used by tests and diagnostics.
//
// It exists so a caller asking "may this value be reused?" has one answer to read
// rather than a rule spread across three passes.
func isPureType(t types.Type) bool {
	return t != nil && !t.Equals(types.Any) && !t.Equals(types.Null)
}
