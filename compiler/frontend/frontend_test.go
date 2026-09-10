package frontend

import (
	"testing"
)

func TestParseAndCheck_Valid(t *testing.T) {
	src := `
	let x: int = 10;
	let y: int = 20;
	let z: int = x + y;
	`
	res := ParseAndCheck(src)
	if !res.OK {
		t.Fatalf("expected OK=true, got false (parseErrors=%v, diagnostics=%v)", res.ParseErrors, res.Diagnostics)
	}
	if res.Program == nil {
		t.Fatal("expected non-nil Program")
	}
}

func TestParseAndCheck_ParseError(t *testing.T) {
	src := `let = ;`
	res := ParseAndCheck(src)
	if res.OK {
		t.Fatal("expected OK=false for syntax error")
	}
	if len(res.ParseErrors) == 0 {
		t.Fatal("expected ParseErrors to be populated")
	}
}

func TestParseAndCheck_TypeCheckError(t *testing.T) {
	src := `let x: int = "not an int";`
	res := ParseAndCheck(src)
	if res.OK {
		t.Fatal("expected OK=false for type error")
	}
	if len(res.Diagnostics) == 0 {
		t.Fatal("expected Diagnostics to contain type error")
	}
}

func TestParseAndCheckWithCapabilities(t *testing.T) {
	src := `let x: int = 42;`
	res := ParseAndCheckWithCapabilities(src, []string{"io"})
	if !res.OK {
		t.Fatalf("expected OK=true, got false: %v", res.Diagnostics)
	}
}
