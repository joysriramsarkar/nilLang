package evaluator

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

func testEvalHelper(input string) object.Object {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	env := object.NewEnvironment()
	return Eval(program, env)
}

func TestResultOkAndErr(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"let res = Ok(42); res;", "Ok(42)"},
		{"let res = Err(\"connection failed\"); res;", "Err(connection failed)"},
		{"let res = Ok(100); isOk(res);", "true"},
		{"let res = Ok(100); isErr(res);", "false"},
		{"let res = Err(\"bad input\"); isOk(res);", "false"},
		{"let res = Err(\"bad input\"); isErr(res);", "true"},
		{"let res = Ok(99); unwrap(res);", "99"},
		{"let res = Err(\"err\"); unwrapOr(res, 555);", "555"},
		{"let res = Ok(777); unwrapOr(res, 555);", "777"},
	}

	for _, tt := range tests {
		evaluated := testEvalHelper(tt.input)
		if evaluated == nil {
			t.Fatalf("eval failed for %q, got nil", tt.input)
		}
		if evaluated.Inspect() != tt.expected {
			t.Errorf("for input %q: expected %q, got %q", tt.input, tt.expected, evaluated.Inspect())
		}
	}
}

func TestOptionalSomeAndNone(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"let opt = Some(\"nilang\"); opt;", "Some(nilang)"},
		{"let opt = None(); opt;", "None"},
		{"let opt = Some(10); isSome(opt);", "true"},
		{"let opt = Some(10); isNone(opt);", "false"},
		{"let opt = None(); isSome(opt);", "false"},
		{"let opt = None(); isNone(opt);", "true"},
		{"let opt = Some(\"found\"); unwrap(opt);", "found"},
		{"let opt = None(); unwrapOr(opt, \"default\");", "default"},
		{"let opt = Some(\"existing\"); unwrapOr(opt, \"default\");", "existing"},
	}

	for _, tt := range tests {
		evaluated := testEvalHelper(tt.input)
		if evaluated == nil {
			t.Fatalf("eval failed for %q, got nil", tt.input)
		}
		if evaluated.Inspect() != tt.expected {
			t.Errorf("for input %q: expected %q, got %q", tt.input, tt.expected, evaluated.Inspect())
		}
	}
}
