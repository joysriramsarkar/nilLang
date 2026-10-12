package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// frontendGateExemptions lists packages permitted to construct a lexer or
// parser directly instead of calling compiler/frontend.
//
// Every entry must carry a justification: the architecture rule is only worth
// having if the exceptions are deliberate and reviewable. See the "Exemptions"
// section of compiler/frontend/frontend.go.
var frontendGateExemptions = map[string]string{
	"compiler/parser":    "re-entrant parsing of string-template interpolation; the parser cannot import frontend (import cycle)",
	"compiler/frontend":  "is the gate itself",
	"compiler/formatter": "must format source that does not type check; running the checker would reject the code the formatter is meant to repair",
	"pkg/lsp":            "IDE front ends must tolerate incomplete buffers, so they parse without type checking and surface diagnostics independently",
	"bootstrap":          "self-hosting compiler image deliberately reimplements the lexer/parser in Nilang",
	"compiler/oracle":    "Verified-Novelty must be able to inspect and diff candidate sources, including ones that fail to parse",
	"cmd/nil":            "REPL parses single lines and the render/dev paths need parse-only results for hot reload; both still route through frontend.Check",
	"pkg/compiler":       "parseModuleSource routes through frontend.Check with SkipTypeCheck for imported modules",
}

// TestFrontendGateIsSingle asserts that no package outside the documented
// exemption list constructs a lexer or parser directly.
//
// Before this rule existed, four execution paths hand-rolled their own
// lex→parse→check sequence, which is how the native backend came to skip type
// checking entirely and emit C code for programs the type checker rejects.
func TestFrontendGateIsSingle(t *testing.T) {
	root := repoRoot(t)

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "dist", "bin", "build", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		pkgDir := filepath.ToSlash(filepath.Dir(rel))
		if _, exempt := frontendGateExemptions[pkgDir]; exempt {
			return nil
		}

		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			// A file we cannot parse cannot violate the rule we are checking.
			return nil
		}

		for _, imp := range file.Imports {
			importPath, strconvErr := strconv.Unquote(imp.Path.Value)
			if strconvErr != nil {
				continue
			}
			switch importPath {
			case "github.com/joysriramsarkar/nilLang/compiler/lexer",
				"github.com/joysriramsarkar/nilLang/compiler/parser":
				t.Errorf("%s imports %s directly; use compiler/frontend instead "+
					"(add the package to frontendGateExemptions with a justification if this is intentional)",
					rel, importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// TestFrontendGateExemptionsAreJustified guards against the exemption list
// becoming a silent dumping ground: every entry must name a reason.
func TestFrontendGateExemptionsAreJustified(t *testing.T) {
	for pkg, reason := range frontendGateExemptions {
		if strings.TrimSpace(reason) == "" {
			t.Errorf("exemption %q has no justification", pkg)
		}
	}
}

// TestParserProducesStableDiagnosticCodes checks that syntax errors now carry
// stable identifiers from docs/spec/ERROR_MODEL.md rather than free text.
//
// Editors, the LSP, and `nil check` all key off these codes, so a parse error
// without a code is a regression.
func TestParserProducesStableDiagnosticCodes(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fset, filepath.Join(root, "tests"), nil, 0)
	if err != nil {
		t.Fatalf("parse tests dir: %v", err)
	}
	_ = pkgs

	// Exercise the gate directly with inputs that must fail in a specific way.
	cases := []struct {
		name     string
		source   string
		wantCode string
	}{
		{
			name:     "unclosed paren is a syntax error",
			source:   "let x = (1 + 2;",
			wantCode: "E0004",
		},
		{
			// A stray closing brace cannot start an expression, so the parser
			// reports it as an unexpected token rather than a syntax mismatch.
			name:     "stray closing brace reports unexpected token",
			source:   "let x = 1;\n}\n",
			wantCode: "E0001",
		},
		{
			// 12 lexes as a number and abc as an identifier, so this is an
			// undefined-identifier error rather than a literal error.
			name:     "trailing identifier after a number is an undefined symbol",
			source:   "let x = 12abc;",
			wantCode: "E0102",
		},
		{
			// Two decimal points cannot form a valid float, so the literal
			// itself is malformed and the parser rejects the token.
			name:     "malformed float literal is rejected",
			source:   "let x = 1.2.3;",
			wantCode: "E0001",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := frontendCheck(t, tc.source, "gate.nil")
			if res.OK {
				t.Fatalf("expected the gate to reject the source, got OK")
			}
			if len(res.Diagnostics) == 0 {
				t.Fatalf("expected a structured diagnostic, got none (parse errors: %v)", res.ParseErrors)
			}
			for _, d := range res.Diagnostics {
				if d.Code == tc.wantCode {
					return
				}
			}
			got := make([]string, 0, len(res.Diagnostics))
			for _, d := range res.Diagnostics {
				got = append(got, d.Code+": "+d.Message)
			}
			t.Errorf("expected a %s diagnostic, got %v", tc.wantCode, got)
		})
	}
}

// TestParserDiagnosticSpansArePopulated verifies that diagnostics carry usable
// positions, which is what makes editor navigation possible.
func TestParserDiagnosticSpansArePopulated(t *testing.T) {
	res := frontendCheck(t, "let x = 1;\nlet y = (1 + ;\n", "spans.nil")
	if res.OK {
		t.Fatal("expected the gate to reject the source")
	}
	if len(res.Diagnostics) == 0 {
		t.Fatal("expected a structured diagnostic")
	}
	d := res.Diagnostics[0]
	if d.Span.Filename != "spans.nil" {
		t.Errorf("span filename = %q, want %q", d.Span.Filename, "spans.nil")
	}
	if d.Span.StartLine != 2 {
		t.Errorf("span start line = %d, want 2", d.Span.StartLine)
	}
	if d.Span.StartCol <= 0 {
		t.Errorf("span start column = %d, want a positive column", d.Span.StartCol)
	}
	if d.Span.EndCol <= d.Span.StartCol {
		t.Errorf("span end column = %d, want greater than start %d", d.Span.EndCol, d.Span.StartCol)
	}
	if d.ContextLine == "" {
		t.Error("expected the diagnostic to quote the offending source line")
	}
}

// TestFrontendLegacyAPIStaysCompatible ensures the pre-existing string based
// error channel keeps working for callers that still read Errors().
func TestFrontendLegacyAPIStaysCompatible(t *testing.T) {
	res := frontendCheck(t, "let x = ;", "legacy.nil")
	if len(res.ParseErrors) == 0 {
		t.Fatal("expected legacy string parse errors to be populated")
	}
	for _, msg := range res.ParseErrors {
		if !strings.HasPrefix(msg, "line ") {
			t.Errorf("legacy parse error %q should keep its \"line L:C:\" prefix", msg)
		}
	}
	if res.ErrSummary() == "" {
		t.Error("expected a non-empty error summary for a failed gate run")
	}
}

// TestWellTypedSourcePassesGate is the positive control: the gate must not
// reject valid programs, including ones using modules and components.
func TestWellTypedSourcePassesGate(t *testing.T) {
	sources := []string{
		`let x = 1; let y = 2; let z = x + y;`,
		`fn add(a, b) { return a + b; } let r = add(1, 2);`,
		`let xs = [1, 2, 3]; let n = len(xs);`,
		`let h = {"a": 1}; let v = h["a"];`,
		"let সংখ্যা = ৪২;",
		`let i = 0; while i < 3 { i = i + 1; }`,
		`let t = if true { 1 } else { 2 };`,
		`component C { state n: Int = 0; render { return {"type": "Page"} } }`,
	}

	for _, src := range sources {
		res := frontendCheck(t, src, "ok.nil")
		if !res.OK {
			t.Errorf("valid source was rejected: %s\n  diagnostics: %s", src, res.ErrSummary())
		}
	}
}

// repoRoot returns the repository root by walking up to the go.mod file.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate go.mod above the working directory")
		}
		dir = parent
	}
}

// unusedGuard keeps the go/ast import meaningful if the AST scan above is
// simplified in future edits.
var _ = ast.Walk
