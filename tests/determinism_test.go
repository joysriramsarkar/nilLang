package tests

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/formatter"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/token"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
)

// TestHashLiteralEvaluatesInSourceOrder is the semantic requirement that the
// AST's ordered pair slice exists to satisfy.
//
// SEMANTICS.md §1 requires left-to-right lexical evaluation. When key or value
// expressions have side effects, map iteration made the result depend on Go's
// randomised hash seed, so the same program could print different things on
// two consecutive runs.
func TestHashLiteralEvaluatesInSourceOrder(t *testing.T) {
	// Each value is a call to a function that records when it ran, so the order
	// of evaluation is directly observable in the returned string.
	// Each value is a call to a function that appends its label to a module
	// level string, so the order in which values evaluate is directly
	// observable in the result.
	source := `
let order = "";
fn mark(n) {
  order = order + n;
  return n;
}
fn makeLiteral() {
  let h = {
    "a": mark("a"),
    "b": mark("b"),
    "c": mark("c"),
    "d": mark("d"),
    "e": mark("e"),
  };
  return order;
}
let result = makeLiteral();
`
	res := evalSource(t, source)
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}

	// Both engines must agree, and both must agree with the source order.
	const want = "abcde"

	out, err := evaluatorEval(res.Program, "result")
	if err != nil {
		t.Fatalf("evaluation error: %v", err)
	}
	if out != want {
		t.Errorf("evaluator evaluated hash values as %q, want %q", out, want)
	}

	vmOut, err := vmEval(res.Program, "result")
	if err != nil {
		t.Fatalf("vm error: %v", err)
	}
	if vmOut != want {
		t.Errorf("VM evaluated hash values as %q, want %q", vmOut, want)
	}
}

// TestHashLiteralOrderIsStableAcrossRuns guards the fix directly: the same
// program must produce the same trace every time, in one process and across
// repeated evaluation. Before the fix this was at the mercy of Go's per-map
// randomised iteration order.
func TestHashLiteralOrderIsStableAcrossRuns(t *testing.T) {
	source := `
let order = "";
fn mark(n) { order = order + n; return n; }
fn makeLiteral() {
  let h = {
    "k1": mark(1), "k2": mark(2), "k3": mark(3),
    "k4": mark(4), "k5": mark(5), "k6": mark(6),
    "k7": mark(7), "k8": mark(8),
  };
  return order;
}
let result = makeLiteral();
`
	first := ""
	for i := 0; i < 25; i++ {
		res := evalSource(t, source)
		if !res.OK {
			t.Fatalf("run %d: source rejected: %s", i, res.ErrSummary())
		}
		out, err := evaluatorEval(res.Program, "result")
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		if i == 0 {
			first = out
			continue
		}
		if out != first {
			t.Fatalf("run %d produced %q but run 0 produced %q; evaluation order is not stable",
				i, out, first)
		}
	}
	if first != "12345678" {
		t.Errorf("evaluation order = %q, want %q", first, "12345678")
	}
}

// TestHashLiteralDuplicateKeysResolveLastWins pins the behaviour for a repeated
// key, which was previously dependent on iteration order.
func TestHashLiteralDuplicateKeysResolveLastWins(t *testing.T) {
	source := `
let h = {"k": "first", "k": "second"};
let result = h["k"];
`
	res := evalSource(t, source)
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}
	out, err := evaluatorEval(res.Program, "result")
	if err != nil {
		t.Fatalf("evaluation error: %v", err)
	}
	if out != "second" {
		t.Errorf("duplicate key resolved to %q, want %q (the last written value)", out, "second")
	}

	vmOut, err := vmEval(res.Program, "result")
	if err != nil {
		t.Fatalf("vm error: %v", err)
	}
	if vmOut != "second" {
		t.Errorf("VM resolved the duplicate key to %q, want %q", vmOut, "second")
	}
}

// TestHashLiteralPrinterIsDeterministic checks the AST pretty printer, which
// feeds the formatter and the HIR dump commands.
func TestHashLiteralPrinterIsDeterministic(t *testing.T) {
	res := evalSource(t, `let h = {"z": 1, "a": 2, "m": 3, "b": 4};`)
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}

	// Find the literal through its let binding, which is how a hash normally
	// appears in source. Collecting them all and comparing keeps the check
	// honest if a later change adds a second hash to the program.
	var literals []*ast.HashLiteral
	for _, stmt := range res.Program.Statements {
		let, ok := stmt.(*ast.LetStatement)
		if !ok {
			continue
		}
		if hl, ok := let.Value.(*ast.HashLiteral); ok {
			literals = append(literals, hl)
		}
	}
	if len(literals) == 0 {
		t.Fatal("no HashLiteral found among the top-level statements")
	}

	first := literals[0].String()
	if !strings.Contains(first, `"z"`) || !strings.Contains(first, `"a"`) {
		t.Errorf("printed hash %q is missing entries", first)
	}
	// Source order, not alphabetical order.
	if strings.Index(first, `"z"`) > strings.Index(first, `"a"`) {
		t.Errorf("hash printed as %q; expected source order with \"z\" before \"a\"", first)
	}

	// Repeated printing of the same node is stable.
	for i := 0; i < 20; i++ {
		if got := literals[0].String(); got != first {
			t.Fatalf("print %d produced %q, want %q", i, got, first)
		}
	}

	// A hash node assembled without an order slice still prints deterministically.
	// The entries are sorted by printed key, so the fallback is a stable order
	// rather than Go's randomised map order.
	sortedFallback := &ast.HashLiteral{Pairs: map[ast.Expression]ast.Expression{
		stringExpr("m", "3"): intExpr(3),
		stringExpr("a", "1"): intExpr(1),
		stringExpr("z", "2"): intExpr(2),
	}}
	// HashLiteral.String() renders as key:value with no space; the formatter is
	// what inserts the space, and it does so in its own code path.
	wantFallback := `{"a":1, "m":3, "z":2}`
	for i := 0; i < 20; i++ {
		if got := sortedFallback.String(); got != wantFallback {
			t.Fatalf("map fallback printed %q, want %q", got, wantFallback)
		}
	}
}

// TestHashFormatterPreservesOrder verifies `nil fmt` does not reshuffle code.
func TestHashFormatterPreservesOrder(t *testing.T) {
	src := `let m = { "z": 100, "a": 200, "m": 300 };` + "\n"
	want := `let m = {"z": 100, "a": 200, "m": 300};` + "\n"

	got, err := formatter.Format(src)
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	if got != want {
		t.Errorf("formatter changed hash order:\ngot:  %swant: %s", got, want)
	}
}

// TestHashLiteralOrderReachesHIR verifies the order survives lowering, so that
// MIR-based backends inherit it.
func TestHashLiteralOrderReachesHIR(t *testing.T) {
	res := evalSource(t, `let h = {"first": 1, "second": 2, "third": 3};`)
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}

	lowered := hir.NewLowerer().LowerProgram(res.Program)
	dump := lowered.String()

	firstIdx := strings.Index(dump, "first")
	secondIdx := strings.Index(dump, "second")
	thirdIdx := strings.Index(dump, "third")

	if firstIdx < 0 || secondIdx < 0 || thirdIdx < 0 {
		t.Fatalf("HIR dump is missing hash keys:\n%s", dump)
	}
	if !(firstIdx < secondIdx && secondIdx < thirdIdx) {
		t.Errorf("HIR lost source order; indices were first=%d second=%d third=%d in:\n%s",
			firstIdx, secondIdx, thirdIdx, dump)
	}
}

// TestBytecodeHashEmissionIsStable confirms the compiler emits the same
// instruction stream for the same hash literal across runs.
func TestBytecodeHashEmissionIsStable(t *testing.T) {
	res := evalSource(t, `let h = {"alpha": 1, "beta": 2, "gamma": 3, "delta": 4};`)
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}

	var first string
	for i := 0; i < 15; i++ {
		comp := compiler.New()
		if err := comp.Compile(res.Program); err != nil {
			t.Fatalf("compile: %v", err)
		}
		got := vm.Disassemble(comp.Bytecode().Instructions)
		if i == 0 {
			first = got
			continue
		}
		if got != first {
			t.Fatalf("bytecode differed between runs:\nfirst:\n%s\nrun %d:\n%s", first, i, got)
		}
	}
}

// zeroToken builds a placeholder token for AST nodes assembled by hand in
// tests, where position information is irrelevant.
func zeroToken(literal string) token.Token {
	return token.Token{Type: token.STRING, Literal: literal, Line: 1, Column: 1}
}

// stringExpr and intExpr build literals for hand-assembled AST nodes.
//
// IntegerLiteral.String() returns the token's literal text rather than the
// numeric value, so the literal text is kept in sync with the value here. That
// is precisely why the printer's map fallback had to sort by printed key.
func stringExpr(value, literal string) ast.Expression {
	return &ast.StringLiteral{Token: zeroToken(literal), Value: value}
}

func intExpr(value int64) ast.Expression {
	return &ast.IntegerLiteral{
		Token: token.Token{Type: token.INT, Literal: strconv.FormatInt(value, 10), Line: 1, Column: 1},
		Value: value,
	}
}

// mustEvaluatorEval is evaluatorEval for tests where the value is the point.
func mustEvaluatorEval(t *testing.T, prog *ast.Program, name string) string {
	t.Helper()
	out, err := evaluatorEval(prog, name)
	if err != nil {
		t.Fatalf("evaluator: %v", err)
	}
	return out
}

// mustVMEval is vmEval for tests where the value is the point.
func mustVMEval(t *testing.T, prog *ast.Program, name string) string {
	t.Helper()
	out, err := vmEval(prog, name)
	if err != nil {
		t.Fatalf("VM: %v", err)
	}
	return out
}

// mustRuntimeError runs a program in the named engine and returns the error it
// stopped with, requiring that it stopped.
//
// It exists for the tests about *failing* programs. A helper that reads a named
// global cannot express "this must not get that far", and a test that merely
// checked the global would pass for a program that silently carried on past a
// failed assertion.
func mustRuntimeError(t *testing.T, engine string, prog *ast.Program) error {
	t.Helper()

	switch engine {
	case "evaluator":
		env := object.NewEnvironment()
		for builtinName, builtin := range evaluator.Builtins {
			env.Set(builtinName, builtin)
		}
		evaluated := evaluator.Eval(prog, env)
		switch errObj := evaluated.(type) {
		case *object.Error:
			return errors.New(errObj.Message)
		case *object.RuntimeError:
			return errObj
		default:
			return nil
		}

	case "VM":
		comp := compiler.New()
		if err := comp.Compile(prog); err != nil {
			t.Fatalf("VM compile: %v", err)
		}
		return vm.New(comp.Bytecode()).Run()

	default:
		t.Fatalf("unknown engine %q", engine)
		return nil
	}
}

// evaluatorEval runs the tree-walking evaluator over an already gated program
// and returns the value bound to the global named name.
//
// Reading a named binding rather than the program's result matters: a program
// whose last statement is a declaration evaluates to nil, so the "value" of the
// program is not a reliable channel for observing effects.
func evaluatorEval(prog *ast.Program, name string) (string, error) {
	env := object.NewEnvironment()
	for builtinName, builtin := range evaluator.Builtins {
		env.Set(builtinName, builtin)
	}
	evaluated := evaluator.Eval(prog, env)
	if errObj, ok := evaluated.(*object.Error); ok {
		return "", fmt.Errorf("%s", errObj.Inspect())
	}
	bound, ok := env.Get(name)
	if !ok || bound == nil {
		return "", fmt.Errorf("global %q is not bound after evaluation", name)
	}
	return bound.Inspect(), nil
}

// vmEval runs the bytecode VM over an already gated program and returns the
// value bound to the global named name.
//
// Nilang globals live in the VM's global array, not on the operand stack, so an
// effect cannot be observed by popping a result. The slot is located through the
// compiler's debug name table rather than hard coded, which keeps the helper
// correct when a test program gains or reorders globals.
func vmEval(prog *ast.Program, name string) (string, error) {
	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		return "", err
	}
	bytecode := comp.Bytecode()

	slot := -1
	for i, globalName := range bytecode.GlobalNames {
		if globalName == name {
			slot = i
			break
		}
	}
	if slot < 0 {
		return "", fmt.Errorf("global %q not found; available slots: %v", name, bytecode.GlobalNames)
	}

	machine := vm.New(bytecode)
	if err := machine.Run(); err != nil {
		return "", err
	}
	bound := machine.GetGlobal(slot)
	if bound == nil {
		return "", fmt.Errorf("global %q (slot %d) is unbound after Run", name, slot)
	}
	return bound.Inspect(), nil
}
