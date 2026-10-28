package compiler

import (
	"fmt"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

// TestCompileRecoversFromOperandOverflow pins the compile-path recovery:
// a function with more locals than a one-byte operand can name makes
// code.Make panic on operand overflow. That panic used to escape
// Compile entirely and crash the embedding host; Compile must now
// surface it as an error. (If the panic escaped, this test process would
// die instead of failing, which is the assertion.)
func TestCompileRecoversFromOperandOverflow(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("let f = fn() {\n")
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&sb, "let v%d = %d;\n", i, i)
	}
	sb.WriteString("return v0;\n};\n")

	p := parser.New(lexer.New(sb.String()))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}

	comp := New()
	err := comp.Compile(program)
	if err == nil {
		t.Fatal("compiling a function with 300 locals must return an error, got nil")
	}
	if !strings.Contains(err.Error(), "internal compiler error") {
		t.Fatalf("error = %q, want the recovered internal compiler error", err)
	}
}

// TestCompileOrdinaryProgramStillSucceeds guards the wrapper itself: the
// recovery must not turn ordinary compilations into errors.
func TestCompileOrdinaryProgramStillSucceeds(t *testing.T) {
	p := parser.New(lexer.New("let x = 1 + 2; let f = fn(a) { a + x; }; f(3);"))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	comp := New()
	if err := comp.Compile(program); err != nil {
		t.Fatalf("ordinary program failed to compile: %v", err)
	}
	if comp.Bytecode() == nil {
		t.Fatal("Bytecode() returned nil after a successful Compile")
	}
}
