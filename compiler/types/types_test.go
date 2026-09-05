package types

import (
	"testing"
)

func TestPrimitiveTypes(t *testing.T) {
	if !Int.Equals(Int) {
		t.Errorf("Int should equal Int")
	}
	if Int.Equals(Float) {
		t.Errorf("Int should not equal Float")
	}
	if !Int.AssignableTo(Any) {
		t.Errorf("Int should be assignable to Any")
	}
}

func TestUnionTypes(t *testing.T) {
	u := NewUnion(Int, String)
	if !u.Contains(Int) || !u.Contains(String) {
		t.Errorf("Union should contain Int and String")
	}
	if u.Contains(Float) {
		t.Errorf("Union should not contain Float")
	}
	if !Int.AssignableTo(u) {
		t.Errorf("Int should be assignable to Int | String")
	}
}

func TestOptionalTypes(t *testing.T) {
	opt := &OptionalType{Base: Int}
	if opt.String() != "?Int" {
		t.Errorf("Expected ?Int, got %s", opt.String())
	}
	if !Int.AssignableTo(opt) {
		t.Errorf("Int should be assignable to ?Int")
	}
	if !Null.AssignableTo(opt) {
		t.Errorf("Null should be assignable to ?Int")
	}
}

func TestResultTypes(t *testing.T) {
	res := &ResultType{Ok: Int, Err: String}
	if res.String() != "Result<Int, String>" {
		t.Errorf("Expected Result<Int, String>, got %s", res.String())
	}
}

func TestParseTypes(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Int", "Int"},
		{"Float", "Float"},
		{"String", "String"},
		{"?Int", "?Int"},
		{"Int | String", "Int | String"},
		{"List<Int>", "List<Int>"},
		{"Result<Int, String>", "Result<Int, String>"},
		{"fn(Int, String) -> Bool", "fn(Int, String) -> Bool"},
	}

	for _, tt := range tests {
		parsed, err := Parse(tt.input)
		if err != nil {
			t.Fatalf("Parse error for %q: %v", tt.input, err)
		}
		if parsed.String() != tt.expected {
			t.Errorf("For %q, expected %q, got %q", tt.input, tt.expected, parsed.String())
		}
	}
}
