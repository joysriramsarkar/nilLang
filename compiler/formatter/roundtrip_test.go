package formatter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

// The formatter tests in formatter_test.go assert golden output and
// idempotency — Format(Format(x)) == Format(x). A regrouping bug defeats
// both: once `(1..5) + 2` has been printed as `1..5 + 2`, the second format
// faithfully reproduces the wrong program, so the suite stays green while
// the formatted code means something else. This harness asserts the property
// that actually matters: parse → Format → re-parse must yield the same AST,
// compared through the AST's own String form.
//
// The comparison is String-based rather than a deep structural walk, so it
// cannot distinguish tree shapes String prints identically — `(1..2)..3`
// and `1..(2..3)` both print as `1..2..3`. Those cases are still listed:
// the formatter preserves the first one's parens, and pinning the String
// keeps the harness honest about what it can and cannot see.

// astString parses src and renders its AST, failing the test on parse errors.
func astString(t *testing.T, src string) string {
	t.Helper()
	rendered, ok := tryASTString(src)
	if !ok {
		p := parser.New(lexer.New(src))
		p.ParseProgram()
		t.Fatalf("source does not parse: %v\n%s", p.Errors(), src)
	}
	return rendered
}

// tryASTString is astString for corpus files, where a source that does not
// parse is a skip rather than a failure.
func tryASTString(src string) (string, bool) {
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return "", false
	}
	return prog.String(), true
}

func assertASTRoundTrip(t *testing.T, src string) {
	t.Helper()
	before := astString(t, src)

	formatted, err := Format(src)
	if err != nil {
		t.Fatalf("Format failed: %v\nsource:\n%s", err, src)
	}
	if after := astString(t, formatted); after != before {
		t.Errorf("format changed the program:\nsource:\n%s\nformatted:\n%s\nAST before: %s\nAST after:  %s",
			src, formatted, before, after)
	}

	again, err := Format(formatted)
	if err != nil {
		t.Fatalf("re-Format failed: %v\nformatted:\n%s", err, formatted)
	}
	if again != formatted {
		t.Errorf("format is not idempotent:\nfirst:\n%s\nsecond:\n%s", formatted, again)
	}
}

func TestFormatterASTRoundTrip(t *testing.T) {
	sources := []string{
		// Ranges: the shape that motivated this harness. A range's End is
		// parsed at LOWEST precedence, so a range used as an operand binds
		// looser than everything around it and must keep its parens.
		`let x = (1..5) + 2;`,
		`let x = y + (1..5);`,
		`let x = (1..3) * 2;`,
		`let x = -(1..3);`,
		`let r = (1..5);`,
		`let r = 1..5;`,
		`let r = 1..=5;`,
		`let v = (1..=5)[0];`,
		`let r = 1..n - 1;`,
		`let r = 1..(2..3);`,
		`let r = (1..2)..3;`,
		`let r = a + b..c;`,
		`for i in 1..10 { puts(i); }`,
		`for i in 0..=n { puts(i); }`,

		// Nested infix and prefix grouping.
		`let x = !(a && b) || -c * (d + e);`,
		`let x = a * (b + c) - d / (e - f);`,
		`let x = a == (b == c);`,
		`let y = a ?? b;`,

		// Collections, access, calls.
		`let h = {"a": 1, "b": [1, 2], "c": {"d": true}};`,
		`let a = [1, 2, 3]; let b = a[0] + obj.field;`,
		`let add = fn(a, b) { return a + b; }; let z = add(1, 2);`,

		// Control flow, including the ternary that desugars to an if.
		`let x = a > 0 ? 1 : 2;`,
		`if (x > 10) { return 1; } else { return 0; }`,
		`while (i < 10) { i = i + 1; }`,
		`for ch in text { puts(ch); }`,
		`let m = match v { Ok(x) if x > 0 => 1, Err(e) => 0, _ => 2 };`,

		// String interpolation and concurrency wrappers.
		`let s = "hi \(name), total \(a + b)";`,
		`let f = task { return g(1); }; let w = await f;`,
	}

	for _, src := range sources {
		t.Run(src, func(t *testing.T) {
			assertASTRoundTrip(t, src)
		})
	}
}

// TestFormatterASTRoundTripExamples runs the same property over the shipped
// example snippets, so a construct the hand-written corpus forgot is still
// covered if a snippet uses it. A snippet that does not parse is skipped —
// parsing it is the parser's tests' concern, not this harness's — but the
// counts are asserted so the sweep cannot silently shrink to nothing.
func TestFormatterASTRoundTripExamples(t *testing.T) {
	files, err := filepath.Glob("../../examples/snippets/*.nil")
	if err != nil {
		t.Fatalf("glob snippets: %v", err)
	}
	if len(files) < 100 {
		t.Fatalf("expected the shipped snippets to sweep, found %d files", len(files))
	}

	parsed, withRange := 0, 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		src := string(data)
		before, ok := tryASTString(src)
		if !ok {
			continue
		}
		parsed++
		if strings.Contains(src, "..") {
			withRange++
		}

		formatted, err := Format(src)
		if err != nil {
			t.Errorf("%s: Format failed: %v", filepath.Base(file), err)
			continue
		}
		if after, ok := tryASTString(formatted); !ok || after != before {
			t.Errorf("%s: format changed the program\nformatted:\n%s", filepath.Base(file), formatted)
		}
	}

	if parsed < 100 {
		t.Fatalf("only %d of %d snippets parsed; the sweep is not sweeping", parsed, len(files))
	}
	if withRange == 0 {
		t.Fatal("no snippet contains a range; the sweep no longer covers the construct this harness exists for")
	}
}
