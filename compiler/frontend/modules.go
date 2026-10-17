package frontend

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/diagnostics"
)

// ModuleSource is an imported module's text and the identity to report its
// diagnostics under.
type ModuleSource struct {
	// Source is the module's text.
	Source string
	// Filename is what diagnostics inside the module are attributed to. It must
	// be the module's own path, never the importing file's: a type error three
	// files deep is reported against the file that contains it, or the reader
	// goes looking in the wrong place.
	Filename string
	// Canonical is an identity used to break import cycles. Two spellings of the
	// same module must produce the same value or the cycle check does not see
	// the cycle.
	Canonical string
}

// ModuleResolver loads an imported module.
//
// It is supplied by the caller because only the caller knows how paths resolve:
// `nil build` resolves against the project root and the embedded std library,
// `nilc` against the file's directory, an embedder against its own loader.
type ModuleResolver func(importPath, sourceDir string) (ModuleSource, error)

// checkImports type-checks every module the program imports, transitively,
// appending their diagnostics to result.
//
// # Why this exists
//
// The checker has always bound an imported name to `Any` and stopped there: it
// never opened the module. The claim in the module loaders — "modules are type
// checked as part of the importing program" — was not true, and nothing
// contradicted it, because a module with a type error in it compiled cleanly.
//
// That is a validation hole, not a style difference. A module whose `get()`
// declares `-> Int` and returns a string produced no diagnostic from any entry
// point, because no entry point ever looked at it.
//
// # What this does not do
//
// It does not unify a module's scope with its importer's, and it does not infer
// a module's signatures from the call site. Each module is checked as the
// self-contained unit it is written as, which is what the specification already
// describes. Imported names stay `Any` at the use site; that is a separate
// question, recorded in CORE_STABILIZATION.md P0.3 rather than settled here.
func checkImports(program *ast.Program, opts *Options, result *Result) {
	if opts.ResolveModule == nil || program == nil {
		return
	}

	sourceDir := opts.SourceDir
	if sourceDir == "" {
		sourceDir = "."
	}

	visited := opts.checkedModules
	if visited == nil {
		// Seed with the entry point, so a module that imports the file that
		// imports it is recognised as a cycle rather than as a new file.
		visited = map[string]bool{canonicalModuleKey(sourceDir, sourceDir): true}
		opts.checkedModules = visited
	}

	checkImportsInto(program, sourceDir, opts.ResolveModule, result, visited, 0)
}

// maxImportDepth bounds the module graph walk.
//
// The visited set is what actually terminates a cycle. This limit is the
// backstop for the case where a resolver hands back a different Canonical each
// time — a generator, a virtual filesystem — where no visited set can converge
// and stopping with a diagnostic is the only outcome left.
const maxImportDepth = 64

// checkImportsInto is the breadth-first walk. It is a separate function so the
// queue and the depth are per-call rather than per-process: a check running
// inside a check must not see the outer walk's state.
func checkImportsInto(program *ast.Program, sourceDir string, resolve ModuleResolver, result *Result, visited map[string]bool, depth int) {
	if depth > maxImportDepth {
		result.Diagnostics = append(result.Diagnostics, &diagnostics.Diagnostic{
			Code:     "E0303",
			Severity: diagnostics.SeverityError,
			Message:  fmt.Sprintf("module graph is deeper than %d; imports are not terminating", maxImportDepth),
			Span:     diagnostics.Span{StartLine: 1},
		})
		result.OK = false
		return
	}

	var queue []*ast.ImportStatement
	collectImports(program, &queue)

	for len(queue) > 0 {
		stmt := queue[0]
		queue = queue[1:]
		if stmt == nil || stmt.Path == nil {
			continue
		}

		importPath := stmt.Path.Value
		speculativeKey := canonicalModuleKey(importPath, sourceDir)
		if visited[speculativeKey] {
			continue
		}

		module, err := resolve(importPath, sourceDir)
		if err != nil {
			// Mark it even on failure: a module that cannot be resolved will not
			// resolve on a second visit either, and reporting it once per
			// import statement would bury the real error under repeats.
			visited[speculativeKey] = true

			// A module that cannot be resolved is reported here. Skipping it
			// silently would let an unresolved import type-check clean, which is
			// how "it compiled" and "it runs" drift apart.
			result.Diagnostics = append(result.Diagnostics, &diagnostics.Diagnostic{
				Code:     "E0302",
				Severity: diagnostics.SeverityError,
				Message:  fmt.Sprintf("cannot resolve module %q: %v", importPath, err),
				Span:     diagnostics.Span{StartLine: stmt.Token.Line},
			})
			result.OK = false
			continue
		}

		// The resolver's own identity is authoritative: it knows the real path
		// after symlinks and `..` are folded, and two spellings of one module
		// collapse to one entry here, which is what breaks a cycle.
		//
		// It is checked before anything is marked visited. Marking the
		// speculative key first and then comparing Canonical would match
		// against itself and skip the module entirely — silently dropping the
		// check this function exists to perform.
		key := module.Canonical
		if key == "" {
			key = speculativeKey
		}
		if visited[key] {
			continue
		}
		visited[key] = true

		res := Check(module.Source, Options{
			Filename:       module.Filename,
			SourceDir:      sourceDirFor(module.Filename, sourceDir),
			ResolveModule:  resolve,
			checkedModules: visited,
			// A module is never lowered into its importer's tree: the backends
			// inline it from source, so lowering it here would be work thrown
			// away, and on a deeply nested import graph it is work thrown away
			// many times over.
			SkipLowering: true,
		})

		// The module's diagnostics carry their own spans and filenames, so they
		// are merged as they are. Wrapping them in the importer's context would
		// hide the file the error is in, which is the one thing a diagnostic
		// from another file must not lose.
		result.Diagnostics = append(result.Diagnostics, res.Diagnostics...)
		for _, msg := range res.ParseErrors {
			result.Diagnostics = append(result.Diagnostics, &diagnostics.Diagnostic{
				Code:     "E0000",
				Severity: diagnostics.SeverityError,
				Message:  msg,
				Span:     diagnostics.Span{Filename: module.Filename, StartLine: 1},
			})
		}
		if !res.OK {
			result.OK = false
		}

		if res.Program != nil {
			next := sourceDirFor(module.Filename, sourceDir)
			checkImportsInto(res.Program, next, resolve, result, visited, depth+1)
		}
	}
}

// sourceDirFor resolves the directory a module's own relative imports are
// measured from.
//
// A module that imports a sibling must resolve it next to itself, not next to
// the original entry point. Getting this wrong makes a module's own imports work
// or fail depending on who imported it.
func sourceDirFor(moduleFilename, fallback string) string {
	if moduleFilename == "" {
		return fallback
	}
	return filepath.Dir(moduleFilename)
}

func canonicalModuleKey(importPath, sourceDir string) string {
	if strings.HasPrefix(importPath, "std/") {
		return "std:" + importPath
	}
	cleaned := filepath.Clean(filepath.Join(sourceDir, importPath))
	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return cleaned
	}
	return abs
}

// collectImports gathers the import statements of a program in source order.
//
// It recurses through nested statements because an import inside a function is
// still an import, and a module that only ever imports its dependency from inside
// a function body would otherwise skip the check entirely.
func collectImports(program *ast.Program, out *[]*ast.ImportStatement) {
	if program == nil {
		return
	}
	for _, stmt := range program.Statements {
		if stmt == nil {
			continue
		}
		if imp, ok := stmt.(*ast.ImportStatement); ok {
			*out = append(*out, imp)
			continue
		}
		ast.Inspect(stmt, func(node ast.Node) bool {
			if imp, ok := node.(*ast.ImportStatement); ok {
				*out = append(*out, imp)
			}
			return true
		})
	}
}
