package parser

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
)

// TestExternDeclaresAFunctionTheLinkerResolves is the shape the whole FFI rests on.
//
// A C function is compiled elsewhere. The declaration says it exists, names it,
// and gives the linker something to resolve — which is all a signature can be, and
// all it needs to be.
func TestExternDeclaresAFunctionTheLinkerResolves(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "the canonical form",
			source: `extern "C" fn sin(x: Float) -> Float;`,
			want:   `extern "C" fn sin(x: Float) -> Float;`,
		},
		{
			name:   "no parameters",
			source: `extern "C" fn getpid() -> Int;`,
			want:   `extern "C" fn getpid() -> Int;`,
		},
		{
			name:   "several parameters",
			source: `extern "C" fn atan2(y: Float, x: Float) -> Float;`,
			want:   `extern "C" fn atan2(y: Float, x: Float) -> Float;`,
		},
		{
			name:   "a void return",
			source: `extern "C" fn fflush(stream: Int) -> Void;`,
			want:   `extern "C" fn fflush(stream: Int) -> Void;`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prog := parseOK(t, tc.source)
			if len(prog.Statements) != 1 {
				t.Fatalf("wanted one statement, got %d", len(prog.Statements))
			}

			ext, ok := prog.Statements[0].(*ast.ExternStatement)
			if !ok {
				t.Fatalf("wanted an extern declaration, got %T", prog.Statements[0])
			}

			if ext.Abi != "C" {
				t.Errorf("the ABI is %q, wanted \"C\"", ext.Abi)
			}
			if ext.Fn == nil {
				t.Fatal("the declaration carries no signature")
			}
			if ext.Fn.Body != nil {
				t.Error("the declaration has a body; it is implemented in C, not here")
			}
			if got := ext.String(); got != tc.want {
				t.Errorf("printed as\n  %s\nwanted\n  %s", got, tc.want)
			}
		})
	}
}

// TestExternIsSoft is the reason `extern` is not a reserved word.
//
// A developer who names a variable `extern` is doing something ordinary, and
// taking that name away buys disambiguation the parser does not need: requiring a
// string after the keyword separates the two readings perfectly well.
func TestExternIsSoft(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{"a variable named extern", `let extern = "libc"; extern;`},
		{"an assignment to a variable named extern", `let extern = 1; extern = 2;`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parseOK(t, tc.source)
		})
	}
}

// TestExternRejectsWhatItCannotMean.
//
// A declaration with no body is the point; a declaration *with* one contradicts
// itself. So does a declaration with no name: there is nothing for the linker to
// resolve. Both are visible at the parse, and both are worth saying at the parse
// rather than letting the linker report a duplicate symbol much later.
func TestExternRejectsWhatItCannotMean(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		wantMsg string
	}{
		{
			name:    "a declaration with a body",
			source:  `extern "C" fn f() -> Int { return 1; }`,
			wantMsg: "cannot have a body",
		},
		{
			name:    "a declaration with no name",
			source:  `extern "C" fn () -> Int;`,
			wantMsg: "must be named",
		},
		{
			name:    "a declaration with no function",
			source:  `extern "C" let x = 1;`,
			wantMsg: "must be followed by a function declaration",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := New(lexer.New(tc.source))
			p.ParseProgram()
			if len(p.Errors()) == 0 {
				t.Fatalf("the parser accepted %s", tc.source)
			}
			joined := strings.Join(p.Errors(), "\n")
			if !strings.Contains(joined, tc.wantMsg) {
				t.Errorf("the diagnostic does not say %q:\n%v", tc.wantMsg, p.Errors())
			}
		})
	}
}

func parseOK(t *testing.T, source string) *ast.Program {
	t.Helper()
	p := New(lexer.New(source))
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	return prog
}
