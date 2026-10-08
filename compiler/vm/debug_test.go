package vm

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

func compileForDebug(t *testing.T, file, src string) *compiler.Bytecode {
	t.Helper()
	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	c := compiler.New()
	c.SourceFile = file
	if err := c.Compile(program); err != nil {
		t.Fatalf("compile: %v", err)
	}
	bc := c.Bytecode()
	if len(bc.Positions) == 0 {
		t.Fatal("compiler recorded no source positions")
	}
	return bc
}

func TestSourcePositionTable(t *testing.T) {
	bc := compileForDebug(t, "test.nil", "let a = 1;\nlet b = 2;\nlet c = a + b;\nc;")
	lines := map[int]bool{}
	for _, p := range bc.Positions {
		lines[p.Line] = true
	}
	for _, want := range []int{1, 2, 3, 4} {
		if !lines[want] {
			t.Errorf("line %d missing from position table: %+v", want, bc.Positions)
		}
	}
}

func TestBreakpointHaltsAtSourceLine(t *testing.T) {
	bc := compileForDebug(t, "test.nil", "let a = 1;\nlet b = 2;\nlet c = a + b;\nc;")
	machine := New(bc)
	ds := NewDebugState()
	ds.SetBreakpoints("test.nil", []int{3})
	machine.Debug = ds

	err := machine.Run()
	if err != ErrPaused {
		t.Fatalf("expected ErrPaused at breakpoint, got %v", err)
	}
	loc := machine.Location()
	if loc.File != "test.nil" || loc.Line != 3 {
		t.Fatalf("paused at wrong location: %s:%d", loc.File, loc.Line)
	}
	if loc.Function != "<main>" {
		t.Fatalf("expected <main>, got %q", loc.Function)
	}

	// Globals set before the breakpoint must be inspectable by name.
	if v, ok := machine.EvaluateName("a"); !ok || v.Value != "1" {
		t.Fatalf("global a not inspectable: %+v (ok=%v)", v, ok)
	}
	if v, ok := machine.EvaluateName("b"); !ok || v.Value != "2" {
		t.Fatalf("global b not inspectable: %+v (ok=%v)", v, ok)
	}
}

func TestStepAdvancesSourceLines(t *testing.T) {
	bc := compileForDebug(t, "test.nil", "let a = 1;\nlet b = 2;\nlet c = a + b;\nc;")
	machine := New(bc)
	ds := NewDebugState()
	ds.SetBreakpoints("test.nil", []int{1})
	machine.Debug = ds

	if err := machine.Run(); err != ErrPaused {
		t.Fatalf("expected pause at line 1, got %v", err)
	}
	if l := machine.Location().Line; l != 1 {
		t.Fatalf("expected line 1, got %d", l)
	}

	ds.ClearBreakpoints()
	startLine := machine.Location().Line
	sawLater := false
	for i := 0; i < 200; i++ {
		if err := machine.Step(); err != nil {
			break
		}
		if machine.Location().Line > startLine {
			sawLater = true
			break
		}
	}
	if !sawLater {
		t.Fatal("stepping never advanced to a later source line")
	}
}

func TestBreakpointInsideFunction(t *testing.T) {
	src := "fn inc(x) {\n" +
		"  let y = x + 1;\n" +
		"  return y;\n" +
		"}\n" +
		"let r = inc(41);\n" +
		"r;"
	bc := compileForDebug(t, "func.nil", src)
	machine := New(bc)
	ds := NewDebugState()
	// The function body line 2 must be hit when inc() is called.
	ds.SetBreakpoints("func.nil", []int{2})
	machine.Debug = ds

	if err := machine.Run(); err != ErrPaused {
		t.Fatalf("expected pause inside function, got %v", err)
	}
	stack := machine.CallStack()
	if len(stack) < 2 {
		t.Fatalf("expected a nested call stack, got %+v", stack)
	}
	if stack[0].Function != "inc" {
		t.Fatalf("expected innermost frame to be inc, got %+v", stack[0])
	}
	if stack[0].Line != 2 {
		t.Fatalf("expected to stop at line 2 in inc, got %d", stack[0].Line)
	}
	// Parameter x must be visible as a local.
	if v, ok := machine.EvaluateName("x"); !ok || v.Value != "41" {
		t.Fatalf("parameter x not inspectable: %+v (ok=%v)", v, ok)
	}

	ds.ClearBreakpoints()
	if err := machine.Continue(); err != nil {
		t.Fatalf("continue to end: %v", err)
	}
}
