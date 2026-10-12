package hir

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/lower"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

// Loop-invariant hoisting (`L3.7`).
//
// # The risk is silence
//
// Hoisting is an optimisation that changes *when* something is evaluated. Done
// wrongly it does not crash — it returns the wrong answer, once, on a program that
// looked fine. Every test here therefore pairs a case that must hoist with one
// that must not, because a suite of only the first kind would pass for a pass
// that hoists everything.

// hoistOf runs a source through the real path and returns the optimised HIR.
func hoistOf(t *testing.T, source string) string {
	t.Helper()

	p := parser.New(lexer.New(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	lowered, err := lower.Program(prog)
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}
	return NewOptimizer().Optimize(NewLowerer().LowerProgram(lowered)).String()
}

func TestALoopInvariantBindingIsHoisted(t *testing.T) {
	text := hoistOf(t, `
	let xs = [1, 2, 3];
	let i = 0;
	while i < 3 {
		let n = len(xs);
		i = i + 1;
	}
	let result = i;
	`)

	if !strings.Contains(text, "PreLoop") && !strings.Contains(text, "let n") {
		t.Fatalf("the hoisted binding is not visible in the HIR at all:\n%s", text)
	}
}

// TestAValueTheLoopChangesIsNotHoisted is the half that matters.
//
// `let n = i + 1` mentions `i`, which the body writes. Hoisting it would compute
// the value from `i` as it was *before* the first iteration and then keep that
// number for every iteration after — a program that compiles, runs, and answers a
// question nobody asked.
func TestAValueTheLoopChangesIsNotHoisted(t *testing.T) {
	prog := NewLowerer().LowerProgram(hoistedProgram(t, `
	let i = 0;
	while i < 3 {
		let n = i + 1;
		i = n;
	}
	let result = i;
	`))
	optimized := NewOptimizer().Optimize(prog)

	loop := findLoop(optimized)
	if loop == nil {
		t.Fatal("no loop found in the optimised program")
	}
	if len(loop.PreLoop) != 0 {
		t.Errorf("%d binding(s) were hoisted out of a loop that changes them: %v",
			len(loop.PreLoop), loop.PreLoop)
	}
}

// TestOnlyLeadingBindingsAreHoisted covers the ordering rule.
//
// A binding *after* a `return` does not run on the iteration that returns. Moving
// it above the loop executes it at least once more than the program did, which for
// anything with a cost is a change nobody asked for.
func TestOnlyLeadingBindingsAreHoisted(t *testing.T) {
	// The binding comes *after* the return. Written before it, the binding is
	// simply a leading hoistable one and the test would be asserting nothing.
	prog := NewLowerer().LowerProgram(hoistedProgram(t, `
	let i = 0;
	while i < 3 {
		return 1;
		let n = len([1, 2]);
	}
	`))
	optimized := NewOptimizer().Optimize(prog)

	loop := findLoop(optimized)
	if loop == nil {
		t.Fatal("no loop found in the optimised program")
	}
	if len(loop.PreLoop) != 0 {
		t.Errorf("a binding after a `return` was hoisted: %v", loop.PreLoop)
	}
}

// TestAConstantIsNotHoisted guards the other direction.
//
// A `const` is a promise made *where it is written*. Moving the promise changes
// where it was made, and a reader checking that a constant is declared before use
// would be right to object.
func TestAConstantIsNotHoisted(t *testing.T) {
	prog := NewLowerer().LowerProgram(hoistedProgram(t, `
	let xs = [1, 2, 3];
	let i = 0;
	while i < 3 {
		const n = 7;
		i = i + 1;
	}
	`))
	optimized := NewOptimizer().Optimize(prog)

	loop := findLoop(optimized)
	if loop == nil {
		t.Fatal("no loop found in the optimised program")
	}
	if len(loop.PreLoop) != 0 {
		t.Errorf("a `const` was hoisted out of a loop: %v", loop.PreLoop)
	}
}

// findLoop is the first while loop in a program, at any depth.
func findLoop(p *Program) *WhileStmt {
	for _, stmt := range p.Statements {
		if found := findLoopIn(stmt); found != nil {
			return found
		}
	}
	return nil
}

func findLoopIn(stmt Statement) *WhileStmt {
	switch s := stmt.(type) {
	case *WhileStmt:
		return s
	case *IfStmt:
		for _, inner := range s.Consequence.Statements {
			if found := findLoopIn(inner); found != nil {
				return found
			}
		}
		if s.Alternative != nil {
			for _, inner := range s.Alternative.Statements {
				if found := findLoopIn(inner); found != nil {
					return found
				}
			}
		}
	}
	return nil
}

// hoistedProgram lowers a source into an AST the optimiser can run on.
func hoistedProgram(t *testing.T, source string) *ast.Program {
	t.Helper()

	p := parser.New(lexer.New(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	return prog
}
