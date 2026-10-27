package parser

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
)

// TestBanglaDigitLiterals verifies that Bengali numerals parse to the values a
// Bengali-first program expects. Before this, `let x = ৪২;` was rejected even
// though the lexer accepted the token, because strconv only understands ASCII.
func TestBanglaDigitLiterals(t *testing.T) {
	cases := []struct {
		source string
		want   int64
	}{
		{"let x = ৪২;", 42},
		{"let x = ০;", 0},
		{"let x = ৯;", 9},
		{"let x = ১২৩;", 123},
		{"let x = ১২৩৪৫৬৭৮৯০;", 1234567890},
		{"let x = ০০১;", 1},
	}

	for _, tc := range cases {
		prog := mustParse(t, tc.source)
		if len(prog.Statements) != 1 {
			t.Fatalf("%q: expected 1 statement, got %d", tc.source, len(prog.Statements))
		}
		let, ok := prog.Statements[0].(*ast.LetStatement)
		if !ok {
			t.Fatalf("%q: expected a let statement, got %T", tc.source, prog.Statements[0])
		}
		lit, ok := let.Value.(*ast.IntegerLiteral)
		if !ok {
			t.Fatalf("%q: expected an integer literal, got %T", tc.source, let.Value)
		}
		if lit.Value != tc.want {
			t.Errorf("%q = %d, want %d", tc.source, lit.Value, tc.want)
		}
	}
}

// TestDigitSeparators covers the readability separators the specification
// promises for large literals.
func TestDigitSeparators(t *testing.T) {
	cases := []struct {
		source string
		want   int64
	}{
		{"let x = 1_000;", 1000},
		{"let x = 1_000_000;", 1000000},
		{"let x = 9_999_999_999;", 9999999999},
		{"let x = ১_০০০;", 1000},
	}

	for _, tc := range cases {
		prog := mustParse(t, tc.source)
		let := prog.Statements[0].(*ast.LetStatement)
		lit, ok := let.Value.(*ast.IntegerLiteral)
		if !ok {
			t.Fatalf("%q: expected an integer literal, got %T", tc.source, let.Value)
		}
		if lit.Value != tc.want {
			t.Errorf("%q = %d, want %d", tc.source, lit.Value, tc.want)
		}
	}
}

// TestRadixPrefixedLiterals covers hex, octal, and binary integers.
func TestRadixPrefixedLiterals(t *testing.T) {
	cases := []struct {
		source string
		want   int64
	}{
		{"let x = 0xFF;", 255},
		{"let x = 0x0;", 0},
		{"let x = 0xdeadBEEF;", 0xdeadBEEF},
		{"let x = 0b1010;", 10},
		{"let x = 0b1111_1111;", 255},
		{"let x = 0o777;", 511},
		{"let x = 0o17;", 15},
	}

	for _, tc := range cases {
		prog := mustParse(t, tc.source)
		let := prog.Statements[0].(*ast.LetStatement)
		lit, ok := let.Value.(*ast.IntegerLiteral)
		if !ok {
			t.Fatalf("%q: expected an integer literal, got %T", tc.source, let.Value)
		}
		if lit.Value != tc.want {
			t.Errorf("%q = %d, want %d", tc.source, lit.Value, tc.want)
		}
	}
}

// TestBanglaFloatLiterals verifies decimal points with Bengali digits.
func TestBanglaFloatLiterals(t *testing.T) {
	cases := []struct {
		source string
		want   float64
	}{
		{"let x = ১.৫;", 1.5},
		{"let x = ০.২৫;", 0.25},
		{"let x = ১০০.৫০;", 100.5},
		{"let x = 1.5;", 1.5},
	}

	for _, tc := range cases {
		prog := mustParse(t, tc.source)
		let := prog.Statements[0].(*ast.LetStatement)
		lit, ok := let.Value.(*ast.FloatLiteral)
		if !ok {
			t.Fatalf("%q: expected a float literal, got %T", tc.source, let.Value)
		}
		if lit.Value != tc.want {
			t.Errorf("%q = %v, want %v", tc.source, lit.Value, tc.want)
		}
	}
}

// TestMalformedNumericLiteralsAreRejected ensures folding does not accept junk.
func TestMalformedNumericLiteralsAreRejected(t *testing.T) {
	sources := []string{
		"let x = 1__2__3.5.;",
		"let x = 1.2.3;",
	}

	for _, src := range sources {
		p := NewWithSource(lexer.New(src), src)
		p.ParseProgram()
		if len(p.Errors()) == 0 {
			t.Errorf("%q should have been rejected", src)
		}
		if len(p.Diagnostics()) == 0 {
			t.Errorf("%q should have produced a structured diagnostic", src)
		}
	}
}

// TestZeroIsStillAPlainDecimal guards the radix-prefix branch against
// swallowing a lone zero or a zero followed by a dot.
func TestZeroIsStillAPlainDecimal(t *testing.T) {
	cases := []struct {
		source string
		want   float64
	}{
		{"let x = 0;", 0},
		{"let x = 0.5;", 0.5},
		{"let x = 0.0;", 0},
		{"let x = ০;", 0},
		{"let x = ০.৫;", 0.5},
	}

	for _, tc := range cases {
		prog := mustParse(t, tc.source)
		let := prog.Statements[0].(*ast.LetStatement)
		switch lit := let.Value.(type) {
		case *ast.IntegerLiteral:
			if float64(lit.Value) != tc.want {
				t.Errorf("%q = %d, want %v", tc.source, lit.Value, tc.want)
			}
		case *ast.FloatLiteral:
			if lit.Value != tc.want {
				t.Errorf("%q = %v, want %v", tc.source, lit.Value, tc.want)
			}
		default:
			t.Errorf("%q: unexpected literal type %T", tc.source, let.Value)
		}
	}
}

// TestBanglaNumbersInExpressions verifies folding survives arithmetic, not just
// a bare literal.
func TestBanglaNumbersInExpressions(t *testing.T) {
	prog := mustParse(t, "let x = ১০ + ২ * ৩;")
	let := prog.Statements[0].(*ast.LetStatement)
	bin, ok := let.Value.(*ast.InfixExpression)
	if !ok {
		t.Fatalf("expected an infix expression, got %T", let.Value)
	}
	if bin.Operator != "+" {
		t.Errorf("operator = %q, want +", bin.Operator)
	}
	left, ok := bin.Left.(*ast.IntegerLiteral)
	if !ok {
		t.Fatalf("expected an integer on the left, got %T", bin.Left)
	}
	if left.Value != 10 {
		t.Errorf("left = %d, want 10", left.Value)
	}
}

// TestFoldDigits covers the folding helper directly, including the "no change
// needed" fast path.
func TestFoldDigits(t *testing.T) {
	cases := []struct {
		in     string
		want   string
		folded bool
	}{
		{"123", "123", false},
		{"1.5", "1.5", false},
		{"৪২", "42", true},
		{"১_০০০", "1000", true},
		{"１２３", "123", true},
		{"0xFF", "0xFF", false},
		{"০xFF", "0xFF", true},
		{"1e10", "1e10", false},
		{"0b1010", "0b1010", false},
	}

	for _, tc := range cases {
		got, folded := foldDigits(tc.in)
		if got != tc.want {
			t.Errorf("foldDigits(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if folded != tc.folded {
			t.Errorf("foldDigits(%q) folded = %v, want %v", tc.in, folded, tc.folded)
		}
	}
}

// mustParse parses source and fails the test if it produced errors.
func mustParse(t *testing.T, source string) *ast.Program {
	t.Helper()
	p := NewWithSource(lexer.New(source), source)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("unexpected parse errors for %q: %v", source, p.Errors())
	}
	return prog
}

// TestScientificNotation pins that `1e10` is one float literal, not an integer
// followed by an identifier, and that the value survives evaluation scale.
func TestScientificNotation(t *testing.T) {
	cases := []struct {
		source string
		want   float64
	}{
		{"let x = 1e10;", 1e10},
		{"let x = 1.5e-3;", 1.5e-3},
		{"let x = 2E+10;", 2e10},
		{"let x = ১e২;", 100},
	}
	for _, tc := range cases {
		prog := mustParse(t, tc.source)
		let := prog.Statements[0].(*ast.LetStatement)
		lit, ok := let.Value.(*ast.FloatLiteral)
		if !ok {
			t.Fatalf("%q: expected a float literal, got %T", tc.source, let.Value)
		}
		if lit.Value != tc.want {
			t.Errorf("%q = %v, want %v", tc.source, lit.Value, tc.want)
		}
	}
}

// TestRadixDigitsValidatedPerBase pins that a digit outside the base ends the
// literal: `0b102` is `0b10` followed by a stray `2`, which the parser rejects
// rather than silently accepting binary 102.
func TestRadixDigitsValidatedPerBase(t *testing.T) {
	for _, src := range []string{"let x = 0b102;", "let x = 0o89;"} {
		p := NewWithSource(lexer.New(src), src)
		p.ParseProgram()
		if len(p.Errors()) == 0 {
			t.Errorf("%q should have been rejected", src)
		}
	}
}

// TestUnicodeDecimalDigitsBeyondBengali pins that decimal digits from other
// scripts convert to their values instead of failing after the lexer accepted
// them: Devanagari ४२ is 42, Arabic-Indic ٤٢ is 42.
func TestUnicodeDecimalDigitsBeyondBengali(t *testing.T) {
	cases := []struct {
		source string
		want   int64
	}{
		{"let x = ४२;", 42},
		{"let x = ٤٢;", 42},
		{"let x = ๔๒;", 42},
		{"let x = ০b১০১০;", 10},
	}
	for _, tc := range cases {
		prog := mustParse(t, tc.source)
		let := prog.Statements[0].(*ast.LetStatement)
		lit, ok := let.Value.(*ast.IntegerLiteral)
		if !ok {
			t.Fatalf("%q: expected an integer literal, got %T", tc.source, let.Value)
		}
		if lit.Value != tc.want {
			t.Errorf("%q = %d, want %d", tc.source, lit.Value, tc.want)
		}
	}
}
