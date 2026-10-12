package main

import (
	"fmt"
	"os"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
)

// backendProgram is the program a backend must compile.
//
// # Why this exists rather than `res.Lowered` at each call site
//
// `Result.Lowered` carries a contract in its own documentation: *backends must
// compile Lowered, never Program*. Three call sites read `res.Program` instead, and
// nothing failed when they did — the programs simply behaved differently, and
// differently in a way that took a coverage test to notice.
//
// The cost of that was twelve missing HIR translations. The HIR path was handed
// `for` loops, `match`, `struct`, `trait`, `?.` and `??` — constructs the lowering
// pass desugars into `while` loops, `if` chains and calls — and had no case for any
// of them. Each one then needed its own implementation *in HIR*, which is the one
// duplication the contract exists to prevent: two places that turn a `for` into
// something, free to disagree about what that something is.
//
// One function with one job makes the mistake visible. A future backend that
// reaches for `res.Program` compiles successfully and silently loses every
// desugared construct, and that is exactly what happened here for long enough.
//
// `Program` is still the right input for tooling — a formatter and an LSP must
// show `for i in 0..n { }`, not the `while` loop a backend runs.
func backendProgram(res frontend.Result) *ast.Program {
	if res.Lowered != nil {
		return res.Lowered
	}
	if res.Program == nil {
		fmt.Fprintln(os.Stderr, "internal error: neither a lowered nor a parsed program is available")
		os.Exit(1)
	}
	return res.Program
}
