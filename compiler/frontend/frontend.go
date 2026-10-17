// Package frontend is the single Parse→Typecheck gate for the Nilang compiler.
//
// Every execution path (nil run, nil build, nil check, REPL, LSP, nilc,
// module imports, evaluator, VM, HIR, MIR, WASM, UI) MUST call ParseAndCheck
// before any backend receives source.
//
// Rule: if ParseAndCheck returns Result{OK: false}, no backend may proceed.
//
// # Exemptions
//
// A small, explicitly enumerated set of packages may construct a lexer or
// parser directly. Each exemption exists for a concrete reason and is enforced
// by TestFrontendGateIsSingle in tests/architecture_test.go:
//
//   - compiler/parser    — internal re-entry for interpolation sub-parsing.
//   - compiler/formatter — must format syntactically invalid source; running
//     the type checker would reject code the formatter is meant to repair.
//   - pkg/lsp, cmd/nil   — IDE front ends must tolerate incomplete buffers, so
//     they parse without type checking and surface diagnostics independently.
//   - bootstrap          — the self-hosting compiler image deliberately
//     reimplements the lexer/parser in Nilang itself.
//   - compiler/oracle    — the Verified-Novelty oracle must be able to inspect
//     and diff candidate sources, including ones that fail to parse.
//
// # The REPL specifically
//
// cmd/nil's REPL sets SkipTypeCheck, and that is a different reason from the
// IDE exemption above: a REPL checks each line as a whole program, so `x + 1`
// typed after `let x = 1` names a binding that does not exist in *that line*.
// Type-checking per line would reject the second line of every ordinary session.
//
// The REPL therefore type-checks nothing, and its acceptance is therefore
// strictly wider than `nil run`'s. That is a deliberate divergence and it is
// bounded: a REPL session still goes through the gate for parsing and lowering,
// and anything the session defines is checked as part of a whole program the
// moment it is run through `nil run`. What the REPL does not do is report a
// type error at the line that introduced it.
//
// Closing the gap means checking the session rather than the line, which is a
// change to how the REPL accumulates a program — recorded in
// CORE_STABILIZATION.md P0.2, not done here.
package frontend

import (
	"fmt"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/diagnostics"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/lower"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/compiler/typecheck"
)

// Result holds the outcome of a full Parse+Typecheck pipeline run.
type Result struct {
	// Program is the parsed AST, always populated if parsing succeeded.
	//
	// This is the source as written, and is what tooling should show: a
	// formatter or an LSP must present `for i in 0..n { }`, not the lowered
	// `while` loop a backend executes.
	Program *ast.Program

	// Lowered is Program after the lowering pass: `for` loops and ranges
	// replaced by the `while` loops and index expressions the engines
	// implement.
	//
	// Backends must compile Lowered, never Program. Doing so once, here, is what
	// makes the two execution engines agree structurally rather than by
	// duplicated work.
	//
	// Lowered equals Program when lowering was skipped or produced nothing to
	// change; it is nil when lowering failed.
	Lowered *ast.Program

	// MethodResolutions maps each trait method call to the generated function it
	// compiles to, as decided by the checker.
	//
	// The lowering pass is its only consumer, and it reads them rather than
	// finding them in the tree: rewriting `d.greet()` into a call to a mangled
	// name in place would leave a formatter rendering the mangled name, since
	// the formatter reads the same Program this is derived from.
	MethodResolutions map[ast.Expression]string

	// Diagnostics contains all structured errors/warnings from parsing and
	// type-checking. May be non-empty even when OK is true (e.g. warnings).
	Diagnostics []*diagnostics.Diagnostic

	// ParseErrors contains raw parser error strings. These are present when
	// the source could not be parsed at all.
	ParseErrors []string

	// SourceDir is the directory imports in Source are resolved against.
	// Empty means the process working directory.
	SourceDir string

	// OK is true only when parsing succeeded AND typechecking produced no
	// errors. Backends must not proceed unless OK is true.
	OK bool
}

// Options configures a gated frontend run.
//
// The zero value is valid and corresponds to ParseAndCheck: no capabilities
// pre-enabled, no type checking.
type Options struct {
	// Filename is the source file name recorded in diagnostic spans.
	Filename string

	// Capabilities are capability names pre-enabled on the checker.
	Capabilities []string

	// SourceDir is the directory relative imports resolve against.
	SourceDir string

	// SkipTypeCheck parses only. Reserved for IDE front ends and formatters,
	// which must tolerate source that does not type check.
	SkipTypeCheck bool

	// ResolveModule loads an imported module so the gate can check it too.
	//
	// Without it, an imported module's body is never type checked: the checker
	// binds the imported names to `Any` and never opens the file. That was the
	// state of the world before this field existed, and it meant a module with
	// a type error compiled cleanly from every entry point.
	//
	// nil means "do not resolve imports", which is correct for a caller that
	// genuinely cannot (a formatter, an LSP working on a single buffer) and is
	// wrong for every backend. The backends supply one.
	ResolveModule ModuleResolver

	// SkipLowering leaves Result.Lowered nil. Only tooling that never executes
	// the program should set it; every backend needs the lowered form.
	SkipLowering bool

	// checkedModules is the set of module identities this gate run has already
	// validated, shared across the recursive Check calls the module walk makes.
	//
	// It is unexported because it is the gate's own bookkeeping, not something a
	// caller sets: a caller that pre-filled it would silently mark modules as
	// checked. Options is passed by value, so the copy a nested Check receives
	// carries the set forward while the caller's own Options stays untouched.
	checkedModules map[string]bool
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
	return Check(source, Options{Capabilities: capabilities})
}

// Check parses and type checks source under the given options. It is the
// canonical entry point every compilation path must use.
func Check(source string, opts Options) Result {
	result := Result{SourceDir: opts.SourceDir}

	// ── 1. Lex & Parse ───────────────────────────────────────────────────────
	l := lexer.New(source)
	p := parser.NewWithSource(l, source)
	if opts.Filename != "" {
		p.SetFilename(opts.Filename)
	}
	program := p.ParseProgram()
	result.Program = program

	// The parser records a structured diagnostic alongside every legacy string,
	// so the coded form is available without re-parsing the English message.
	if parseErrors := p.Errors(); len(parseErrors) > 0 {
		result.ParseErrors = parseErrors
		result.Diagnostics = p.Diagnostics()
		return result
	}

	if opts.SkipTypeCheck {
		result.Lowered = lowerProgram(&result, opts.Filename)
		attachSourceContext(source, opts.Filename, result.Diagnostics)
		return result
	}

	// ── 2. Type-check ────────────────────────────────────────────────────────
	checker := typecheck.NewChecker()
	for _, cap := range opts.Capabilities {
		checker.EnableCapability(cap)
	}
	result.OK = checker.CheckProgram(program)
	result.Diagnostics = checker.Diagnostics

	// Method-call resolutions come from the checker and are handed to the
	// lowering pass rather than written into the tree, so a formatter reading
	// this program still sees what was written.
	result.MethodResolutions = checker.MethodResolutions()

	// Imported modules are part of this program as far as validation goes. A
	// backend that receives a program whose dependency does not type check has
	// been handed something the gate was supposed to refuse, so this runs before
	// the result is handed back and can turn the check red.
	checkImports(program, &opts, &result)

	result.Lowered = lowerProgram(&result, opts.Filename)
	attachSourceContext(source, opts.Filename, result.Diagnostics)

	return result
}

// attachSourceContext fills Filename and ContextLine on diagnostics that
// lack them. The checker works on the AST without the text, so positions
// without file context arrive bare; the frontend owns the source, so it is
// the one place that completes them. End columns are left alone: a point is
// honest, a fabricated range is not.
func attachSourceContext(source, filename string, diags []*diagnostics.Diagnostic) {
	if source == "" {
		return
	}
	lines := strings.Split(source, "\n")
	for _, d := range diags {
		if d == nil {
			continue
		}
		if d.Span.Filename == "" {
			d.Span.Filename = filename
		}
		if d.ContextLine == "" && d.Span.StartLine >= 1 && d.Span.StartLine <= len(lines) {
			d.ContextLine = strings.TrimRight(lines[d.Span.StartLine-1], "\r\n")
		}
	}
}

// lowerProgram runs the lowering pass and turns a failure into a diagnostic.
//
// A lowering failure is a source error rather than an internal fault: `break`
// outside a loop is something a developer writes and needs told about. Routing
// it through the diagnostic channel means `nil check` reports it with a file
// name and a span, like any other mistake.
func lowerProgram(result *Result, filename string) *ast.Program {
	if result.Program == nil {
		return nil
	}

	lowered, err := lower.ProgramWithMethods(result.Program, result.MethodResolutions)
	if err != nil {
		result.OK = false
		// The lowerer reports its line; the column is genuinely unknown, so
		// the span names the line and lets the renderer mark it.
		span := diagnostics.Span{}
		if line := lower.ErrorLine(err); line > 0 {
			span = diagnostics.Span{Filename: filename, StartLine: line}
		}
		result.Diagnostics = append(result.Diagnostics, &diagnostics.Diagnostic{
			Code:     diagnostics.CodeLoweringFailed,
			Severity: diagnostics.SeverityError,
			Message:  lower.ErrorMessage(err),
			Span:     span,
			AIExplanation: "This construct cannot be compiled: `break` and " +
				"`continue` only make sense inside a loop, and `for` needs a " +
				"collection or a range to walk.",
		})
		return nil
	}
	return lowered
}

// ErrSummary renders a Result's diagnostics as a single human readable
// message suitable for a CLI error return value.
//
// Coded diagnostics are preferred over the parser's raw strings. Both describe
// the same failures — the parser records one alongside every legacy string —
// but only the coded form names a reason a caller can act on or match. Rendering
// the strings when both are available would make a syntax error the one class of
// refusal that arrives without a code, which is precisely the class a caller
// cannot handle programmatically.
func (r Result) ErrSummary() string {
	if r.OK {
		return ""
	}

	coded := r.errorDiagnostics()
	if len(coded) > 0 {
		errs := 0
		var b strings.Builder
		for _, d := range coded {
			if d.Severity != diagnostics.SeverityError {
				continue
			}
			errs++
			b.WriteString("\n  ")
			b.WriteString(d.String())
		}
		if errs > 0 {
			return fmt.Sprintf("compilation failed with %d error(s):%s", errs, b.String())
		}
	}

	if len(r.ParseErrors) > 0 {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("compilation failed with %d error(s):", len(r.ParseErrors)))
		for _, msg := range r.ParseErrors {
			b.WriteString("\n  ")
			b.WriteString(msg)
		}
		return b.String()
	}

	errs := 0
	var b strings.Builder
	for _, d := range r.Diagnostics {
		if d.Severity != diagnostics.SeverityError {
			continue
		}
		errs++
		b.WriteString("\n  ")
		b.WriteString(d.String())
	}
	return fmt.Sprintf("compilation failed with %d error(s):%s", errs, b.String())
}

// errorDiagnostics returns the diagnostics that name a problem.
//
// A diagnostic with no code is a message, not a refusal a caller can branch on,
// so it is not counted here: an uncoded parse failure falls through to the raw
// strings, which is the same information without the pretence of a code.
func (r Result) errorDiagnostics() []*diagnostics.Diagnostic {
	var out []*diagnostics.Diagnostic
	for _, d := range r.Diagnostics {
		if d != nil && d.Code != "" {
			out = append(out, d)
		}
	}
	return out
}
