package compiler_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
	pkgcompiler "github.com/joysriramsarkar/nilLang/pkg/compiler"
)

// runEvaluator executes source through the tree-walking evaluator,
// mirroring how `nil run` (default) executes scripts.
func runEvaluator(t *testing.T, dir, source string) object.Object {
	t.Helper()

	l := lexer.New(source)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}

	env := object.NewEnvironment()
	for name, builtin := range evaluator.Builtins {
		env.Set(name, builtin)
	}

	evaluator.PushScriptDir(dir)
	defer evaluator.PopScriptDir()

	result := evaluator.Eval(prog, env)
	if result != nil && result.Type() == object.ERROR_OBJ {
		t.Fatalf("evaluator error: %s", result.Inspect())
	}
	return result
}

// runVM executes source through the bytecode compiler + VM,
// mirroring how `nil run -vm` and `nil build` artifacts execute.
func runVM(t *testing.T, dir, source string) object.Object {
	t.Helper()

	mainPath := filepath.Join(dir, "main.nil")
	if err := os.WriteFile(mainPath, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}

	pipeline, err := pkgcompiler.CompileFile(mainPath)
	if err != nil {
		t.Fatalf("CompileFile: %v", err)
	}

	machine := vm.New(pipeline.GetBytecode())
	if err := machine.Run(); err != nil {
		t.Fatalf("VM run: %v", err)
	}
	return machine.StackTop()
}

// TestImportEvaluatorVMParity verifies that a program using file-module
// imports, native-module imports, and cross-module function calls produces
// the same result through the tree-walking evaluator and the bytecode VM.
// This is the core evaluator/VM parity guarantee for `nil build` artifacts.
func TestImportEvaluatorVMParity(t *testing.T) {
	dir := t.TempDir()

	modSrc := `
let double = fn(n) { return n * 2; };
let triple = fn(n) { return n * 3; };
let label = "pos-mod";
`
	if err := os.WriteFile(filepath.Join(dir, "mod.nil"), []byte(modSrc), 0644); err != nil {
		t.Fatal(err)
	}

	// Exercises: file-module import with alias, native-module import
	// (implicit base-name binding), cross-module calls, hash indexing.
	mainSrc := `import "./mod.nil" as M;
import "money";
let a = M.double(21);
let b = M.triple(5);
let mm = money.ofMinor(a + b, "BDT");
return mm["minor"];
`

	expected := int64(42 + 15) // 57

	vmResult := runVM(t, dir, mainSrc)
	if vmResult == nil {
		t.Fatal("VM returned no result")
	}
	vmInt, ok := vmResult.(*object.Integer)
	if !ok {
		t.Fatalf("VM result: expected INTEGER, got %s (%v)", vmResult.Type(), vmResult.Inspect())
	}
	if vmInt.Value != expected {
		t.Fatalf("VM result: expected %d, got %d", expected, vmInt.Value)
	}

	evalResult := runEvaluator(t, dir, mainSrc)
	if evalResult == nil {
		t.Fatal("evaluator returned no result")
	}
	evalInt, ok := evalResult.(*object.Integer)
	if !ok {
		t.Fatalf("evaluator result: expected INTEGER, got %s (%v)", evalResult.Type(), evalResult.Inspect())
	}
	if evalInt.Value != expected {
		t.Fatalf("evaluator result: expected %d, got %d", expected, evalInt.Value)
	}
}

// TestNativeNamedImportParity verifies `import {a, b} from "mod"` binds the
// same symbols in both backends.
func TestNativeNamedImportParity(t *testing.T) {
	dir := t.TempDir()

	mainSrc := `import { ofMinor, format } from "money";
return format(ofMinor(12345, "BDT"));
`

	expected := "৳123.45"

	vmResult := runVM(t, dir, mainSrc)
	if vmResult == nil {
		t.Fatal("VM returned no result")
	}
	vmStr, ok := vmResult.(*object.String)
	if !ok {
		t.Fatalf("VM result: expected STRING, got %s (%v)", vmResult.Type(), vmResult.Inspect())
	}
	if vmStr.Value != expected {
		t.Fatalf("VM result: expected %q, got %q", expected, vmStr.Value)
	}

	evalResult := runEvaluator(t, dir, mainSrc)
	if evalResult == nil {
		t.Fatal("evaluator returned no result")
	}
	evalStr, ok := evalResult.(*object.String)
	if !ok {
		t.Fatalf("evaluator result: expected STRING, got %s (%v)", evalResult.Type(), evalResult.Inspect())
	}
	if evalStr.Value != expected {
		t.Fatalf("evaluator result: expected %q, got %q", expected, evalStr.Value)
	}
}

// TestCircularImportParity verifies both backends reject circular file
// imports with an error instead of hanging.
func TestCircularImportParity(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "a.nil"), []byte(`import "./b.nil" as B;
let x = 1;
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.nil"), []byte(`import "./a.nil" as A;
let y = 2;
`), 0644); err != nil {
		t.Fatal(err)
	}

	mainSrc := `import "./a.nil" as A;
return A.x;
`

	// The bytecode compiler must fail with a circular-dependency error
	mainPath := filepath.Join(dir, "main.nil")
	if err := os.WriteFile(mainPath, []byte(mainSrc), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := pkgcompiler.CompileFile(mainPath); err == nil {
		t.Fatal("expected circular dependency error from bytecode compiler")
	}

	// Evaluator path must also fail
	l := lexer.New(mainSrc)
	p := parser.New(l)
	prog := p.ParseProgram()
	env := object.NewEnvironment()
	for name, builtin := range evaluator.Builtins {
		env.Set(name, builtin)
	}
	evaluator.PushScriptDir(dir)
	defer evaluator.PopScriptDir()
	result := evaluator.Eval(prog, env)
	if result == nil || result.Type() != object.ERROR_OBJ {
		t.Fatal("expected circular dependency error from evaluator")
	}
}
