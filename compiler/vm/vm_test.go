package vm

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

func runVm(input string) (object.Object, error) {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()

	comp := compiler.New()
	err := comp.Compile(program)
	if err != nil {
		return nil, err
	}

	vm := New(comp.Bytecode())
	err = vm.Run()
	if err != nil {
		return nil, err
	}

	return vm.LastPoppedStackElem(), nil
}

func TestIntegerArithmetic(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"1", 1},
		{"2", 2},
		{"1 + 2", 3},
		{"1 - 2", -1},
		{"1 * 2", 2},
		{"4 / 2", 2},
		{"50 / 2 * 2 + 10 - 5", 55},
		{"5 + 5 + 5 + 5 - 10", 10},
		{"2 * 2 * 2 * 2 * 2", 32},
		{"5 * 2 + 10", 20},
		{"5 + 2 * 10", 25},
		{"5 * (2 + 10)", 60},
	}

	for _, tt := range tests {
		stackElem, err := runVm(tt.input)
		if err != nil {
			t.Fatalf("runVm failed on %q: %s", tt.input, err)
		}

		result, ok := stackElem.(*object.Integer)
		if !ok {
			t.Fatalf("object is not Integer. got=%T (%+v)", stackElem, stackElem)
		}

		if result.Value != tt.expected {
			t.Errorf("wrong integer value on %q. got=%d, want=%d",
				tt.input, result.Value, tt.expected)
		}
	}
}

func TestBooleanExpressions(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"false", false},
		{"1 < 2", true},
		{"1 > 2", false},
		{"1 < 1", false},
		{"1 > 1", false},
		{"1 == 1", true},
		{"1 != 1", false},
		{"1 == 2", false},
		{"1 != 2", true},
		{"true == true", true},
		{"false == false", true},
		{"true == false", false},
		{"true != false", true},
		{"false != true", true},
		{"!true", false},
		{"!false", true},
		{"!5", false},
		{"!!true", true},
		{"!!false", false},
		{"!!5", true},
	}

	for _, tt := range tests {
		stackElem, err := runVm(tt.input)
		if err != nil {
			t.Fatalf("runVm failed on %q: %s", tt.input, err)
		}

		result, ok := stackElem.(*object.Boolean)
		if !ok {
			t.Fatalf("object is not Boolean. got=%T (%+v)", stackElem, stackElem)
		}

		if result.Value != tt.expected {
			t.Errorf("wrong boolean value on %q. got=%t, want=%t",
				tt.input, result.Value, tt.expected)
		}
	}
}

func TestConditionals(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"if (true) { 10 }", 10},
		{"if (true) { 10 } else { 20 }", 10},
		{"if (false) { 10 } else { 20 }", 20},
		{"if (1 < 2) { 10 }", 10},
		{"if (1 < 2) { 10 } else { 20 }", 10},
		{"if (1 > 2) { 10 } else { 20 }", 20},
	}

	for _, tt := range tests {
		stackElem, err := runVm(tt.input)
		if err != nil {
			t.Fatalf("runVm failed on %q: %s", tt.input, err)
		}

		result, ok := stackElem.(*object.Integer)
		if !ok {
			t.Fatalf("object is not Integer. got=%T (%+v)", stackElem, stackElem)
		}

		if result.Value != tt.expected {
			t.Errorf("wrong integer value on %q. got=%d, want=%d",
				tt.input, result.Value, tt.expected)
		}
	}
}

func TestGlobalLetStatements(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"let one = 1; one", 1},
		{"let one = 1; let two = 2; one + two", 3},
		{"let one = 1; let two = one + one; one + two", 3},
	}

	for _, tt := range tests {
		stackElem, err := runVm(tt.input)
		if err != nil {
			t.Fatalf("runVm failed on %q: %s", tt.input, err)
		}

		result, ok := stackElem.(*object.Integer)
		if !ok {
			t.Fatalf("object is not Integer. got=%T (%+v)", stackElem, stackElem)
		}

		if result.Value != tt.expected {
			t.Errorf("wrong integer value on %q. got=%d, want=%d",
				tt.input, result.Value, tt.expected)
		}
	}
}
