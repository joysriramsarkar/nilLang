package main

import (
	"strings"
	"testing"
)

func TestCompileSourceAcceptsAnnotatedLet(t *testing.T) {
	bytecode, err := compileSource(`let count: i32 = 0; count;`)
	if err != nil {
		t.Fatalf("expected annotated let to compile: %v", err)
	}
	if bytecode == nil || len(bytecode.Instructions) == 0 {
		t.Fatal("expected annotated let to produce bytecode")
	}
}

func TestCompileSourceRejectsTypeErrors(t *testing.T) {
	bytecode, err := compileSource(`let count: i32 = "zero";`)
	if err == nil || !strings.Contains(err.Error(), "type checking failed") {
		t.Fatalf("expected type checking error, got bytecode=%v err=%v", bytecode, err)
	}
	if bytecode != nil {
		t.Fatal("type errors must not produce bytecode")
	}
}

func TestCompileSourceRejectsSyntaxErrors(t *testing.T) {
	bytecode, err := compileSource(`let x = ;`)
	if err == nil || !strings.Contains(err.Error(), "parser errors") {
		t.Fatalf("expected parser error, got %v", err)
	}
	if bytecode != nil {
		t.Fatal("syntax errors must not produce bytecode")
	}
}

func TestCompileSourceRejectsUndefinedVariableAssignment(t *testing.T) {
	bytecode, err := compileSource(`undeclared = 42;`)
	if err == nil {
		t.Fatal("expected undefined variable assignment to be rejected")
	}
	if bytecode != nil {
		t.Fatal("undefined variable must not produce bytecode")
	}
}

func TestCompileSourceRejectsConstantReassignment(t *testing.T) {
	bytecode, err := compileSource(`const PI = 3; PI = 4;`)
	if err == nil {
		t.Fatal("expected constant reassignment to be rejected")
	}
	if bytecode != nil {
		t.Fatal("constant reassignment must not produce bytecode")
	}
}

