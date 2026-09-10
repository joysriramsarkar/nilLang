package main

import (
	"strings"
	"testing"
)

func TestParseAndCheckRejectsInvalidSourceBeforeEvaluation(t *testing.T) {
	_, syntaxErrors, typeErrors := parseAndCheck("missing = 1;")
	if len(syntaxErrors) != 0 {
		t.Fatalf("expected type error, got syntax errors: %v", syntaxErrors)
	}
	if len(typeErrors) == 0 {
		t.Fatal("expected undeclared assignment to fail type checking")
	}
	if !strings.Contains(typeErrors[0], "E0102") {
		t.Fatalf("expected E0102, got: %v", typeErrors)
	}
}

func TestParseAndCheckAcceptsValidSource(t *testing.T) {
	_, syntaxErrors, typeErrors := parseAndCheck("let answer = 40 + 2; answer;")
	if len(syntaxErrors) != 0 || len(typeErrors) != 0 {
		t.Fatalf("valid source rejected: syntax=%v type=%v", syntaxErrors, typeErrors)
	}
}
