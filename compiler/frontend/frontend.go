// Package frontend is the single Parse→Typecheck gate for the Nilang compiler.
//
// Every execution path (nil run, nil build, nil check, REPL, LSP, nilc,
// module imports, evaluator, VM, HIR, MIR, WASM, UI) MUST call ParseAndCheck
// before any backend receives source.
//
// Rule: if ParseAndCheck returns Result{OK: false}, no backend may proceed.
package frontend

import (
	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/diagnostics"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/compiler/typecheck"
)

// Result holds the outcome of a full Parse+Typecheck pipeline run.
type Result struct {
	// Program is the parsed AST, always populated if parsing succeeded.
	Program *ast.Program

	// Diagnostics contains all structured errors/warnings from parsing and
	// type-checking. May be non-empty even when OK is true (e.g. warnings).
	Diagnostics []*diagnostics.Diagnostic

	// ParseErrors contains raw parser error strings. These are present when
	// the source could not be parsed at all.
	ParseErrors []string

	// OK is true only when parsing succeeded AND typechecking produced no
	// errors. Backends must not proceed unless OK is true.
	OK bool
}

// ParseAndCheck parses source and runs the type checker.
// It always returns a Result; callers must check Result.OK before using
// Result.Program for compilation or evaluation.
func ParseAndCheck(source string) Result {
	return ParseAndCheckWithCapabilities(source, nil)
}

// ParseAndCheckWithCapabilities is like ParseAndCheck but pre-enables the
// given capability strings on the checker (e.g. "io", "filesystem").
func ParseAndCheckWithCapabilities(source string, capabilities []string) Result {
	// ── 1. Lex & Parse ───────────────────────────────────────────────────────
	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()

	parseErrors := p.Errors()
	if len(parseErrors) > 0 {
		return Result{
			Program:     program,
			ParseErrors: parseErrors,
			OK:          false,
		}
	}

	// ── 2. Type-check ────────────────────────────────────────────────────────
	checker := typecheck.NewChecker()
	for _, cap := range capabilities {
		checker.EnableCapability(cap)
	}
	ok := checker.CheckProgram(program)

	return Result{
		Program:     program,
		Diagnostics: checker.Diagnostics,
		OK:          ok,
	}
}
