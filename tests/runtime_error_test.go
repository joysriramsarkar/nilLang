package tests

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/capability"
	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
)

// TestBothEnginesReportTheSameErrorCategory is the ERROR_MODEL.md §4 contract
// stated as an executable property.
//
// The two engines report failures through different channels — the evaluator
// returns an Object, the VM returns a Go error — so the category is the only
// thing a host can rely on to make the same decision for both. Before the
// categorised error type existed, the only shared signal was English prose.
func TestBothEnginesReportTheSameErrorCategory(t *testing.T) {
	cases := []struct {
		name     string
		source   string
		category object.ErrorCategory
	}{
		{"integer division by zero", `10 / 0;`, object.CategoryDivisionByZero},
		{"modulo by zero", `10 % 0;`, object.CategoryDivisionByZero},
		{"unbounded recursion", `fn f(n) { return f(n + 1); } f(0);`, object.CategoryFrameOverflow},
		// Note: `let x = 5; x();` is deliberately absent here, because the type
		// checker catches it as E0106 before either engine runs. See
		// TestTypeCheckerCatchesWhatTheRuntimeWould.
		{"index assignment out of bounds", `let a = [1]; a[9] = 1;`, object.CategoryIndexOutOfBounds},
		{"property access on an integer", `let x = 5; x.field;`, object.CategoryTypeError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evalCategory := evaluatorErrorCategory(t, tc.source)
			if evalCategory != tc.category {
				t.Errorf("evaluator category = %q, want %q", evalCategory, tc.category)
			}

			vmCategory := vmErrorCategory(t, tc.source)
			if vmCategory != tc.category {
				t.Errorf("VM category = %q, want %q", vmCategory, tc.category)
			}
		})
	}
}

// TestTypeCheckerCatchesWhatTheRuntimeWould reports the cases where the static
// gate refuses a program before either engine runs.
//
// This is the reason the "calling a non-function" runtime case above exists at
// all: `let x = 5; x();` is caught as E0106, so the runtime path is only
// reachable through a construct the checker cannot resolve statically. Having
// the runtime category defined for it anyway means the two engines stay in
// agreement if the checker ever stops catching it.
func TestTypeCheckerCatchesWhatTheRuntimeWould(t *testing.T) {
	cases := []struct {
		name   string
		source string
		code   string
	}{
		{"calling an integer", `let x = 5; x();`, "E0106"},
		{"type mismatch on init", `let x: Int = "text";`, "E0101"},
		{"undefined identifier", `let x = missing;`, "E0102"},
		{"mutating a const", `const c = 1; c = 2;`, "E0104"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := evalSource(t, tc.source)
			if res.OK {
				t.Fatalf("the gate should have rejected: %s", tc.source)
			}
			for _, d := range res.Diagnostics {
				if d.Code == tc.code {
					return
				}
			}
			t.Errorf("expected diagnostic %s, got %v", tc.code, res.Diagnostics)
		})
	}
}

// TestRuntimeErrorCategorySetIsClosed guards the closed set. A category outside
// it would be unmatchable by any consumer, so the constructor refusing is
// better than a silent typo.
func TestRuntimeErrorCategorySetIsClosed(t *testing.T) {
	valid := []object.ErrorCategory{
		object.CategoryDivisionByZero,
		object.CategoryStackOverflow,
		object.CategoryFrameOverflow,
		object.CategoryIndexOutOfBounds,
		object.CategoryInvalidBytecode,
		object.CategoryCapabilityDenied,
		object.CategoryTaskCancelled,
		object.CategoryTaskFaulted,
		object.CategoryTypeError,
		object.CategoryUnknownOperation,
		object.CategoryUnsupported,
	}
	for _, category := range valid {
		if !object.IsValidCategory(category) {
			t.Errorf("category %q should be valid", category)
		}
	}

	if object.IsValidCategory("SomethingElse") {
		t.Error("an unknown category must not report itself valid")
	}
}

// TestRuntimeErrorIsUsableAsBothObjectAndGoError verifies the dual interface
// that lets one type serve both engines.
func TestRuntimeErrorIsUsableAsBothObjectAndGoError(t *testing.T) {
	err := object.NewRuntimeError(object.CategoryDivisionByZero, "division by zero")

	// As an Object, for the evaluator.
	var asObject object.Object = err
	if asObject.Type() != object.ERROR_OBJ {
		t.Errorf("Type() = %v, want ERROR_OBJ", asObject.Type())
	}
	if !strings.Contains(asObject.Inspect(), "DivisionByZero") {
		t.Errorf("Inspect() should name the category: %s", asObject.Inspect())
	}

	// As a Go error, for the VM.
	var asError error = err
	if !strings.Contains(asError.Error(), "division by zero") {
		t.Errorf("Error() should carry the message: %s", asError.Error())
	}
}

// TestCategoryOfDistinguishesLegacyErrors records that a bare *object.Error
// reports "no category" rather than being assigned one. Inventing a category
// for a legacy error would make the two indistinguishable.
func TestCategoryOfDistinguishesLegacyErrors(t *testing.T) {
	if _, ok := object.CategoryOf(&object.Error{Message: "legacy"}); ok {
		t.Error("a bare *object.Error must not report a category")
	}
	if _, ok := object.CategoryOf(nil); ok {
		t.Error("nil must not report a category")
	}

	categorised := object.NewRuntimeError(object.CategoryTypeError, "bad type")
	category, ok := object.CategoryOf(categorised)
	if !ok || category != object.CategoryTypeError {
		t.Errorf("CategoryOf(categorised) = %q, %v; want TypeError, true", category, ok)
	}
}

// TestCapabilityDenialKeepsItsStructureThroughTheCategoryWrapper guards the
// layering: the VM reports a refusal with a category so the cross-engine
// contract holds, while the wrapped cause keeps the operation name reachable.
func TestCapabilityDenialKeepsItsStructureThroughTheCategoryWrapper(t *testing.T) {
	dir := t.TempDir()
	source := `writeFile("` + strings.ReplaceAll(dir, `\`, "/") + `/x.txt", "y");`

	err := runWithCapabilities(t, source, nil)
	if err == nil {
		t.Fatal("expected a refusal")
	}

	category, ok := object.CategoryOf(err)
	if !ok {
		t.Fatalf("refusal carries no category: %v", err)
	}
	if category != object.CategoryCapabilityDenied {
		t.Errorf("category = %q, want %q", category, object.CategoryCapabilityDenied)
	}

	denied, ok := capability.AsDenied(err)
	if !ok {
		t.Fatalf("the *DeniedError must remain reachable through the wrapper: %v", err)
	}
	if denied.Operation != "writeFile" {
		t.Errorf("Operation = %q, want writeFile", denied.Operation)
	}
	if len(denied.Required) == 0 || denied.Required[0] != capability.Filesystem {
		t.Errorf("Required = %v, want [%s]", denied.Required, capability.Filesystem)
	}
}

// TestDeepRecursionReportsFrameOverflowNotStackOverflow pins which limit binds.
//
// A program that recurses without bound should be told it ran out of frames in
// both engines. When the VM's operand stack bound first, the two engines
// disagreed on the category for the same program.
func TestDeepRecursionReportsFrameOverflowNotStackOverflow(t *testing.T) {
	source := `fn f(n) { return f(n + 1); } f(0);`

	if got := evaluatorErrorCategory(t, source); got != object.CategoryFrameOverflow {
		t.Errorf("evaluator category = %q, want %q", got, object.CategoryFrameOverflow)
	}
	if got := vmErrorCategory(t, source); got != object.CategoryFrameOverflow {
		t.Errorf("VM category = %q, want %q", got, object.CategoryFrameOverflow)
	}
}

// TestBoundedRecursionStillSucceeds is the negative control: the depth bound
// must not reject programs that terminate normally.
func TestBoundedRecursionStillSucceeds(t *testing.T) {
	source := `
fn count(n) {
  if (n <= 0) { return 0; }
  return count(n - 1) + 1;
}
let result = count(500);
`
	res := evalSource(t, source)
	if !res.OK {
		t.Fatalf("source rejected: %s", res.ErrSummary())
	}
	if out, err := evaluatorEval(res.Program, "result"); err != nil {
		t.Fatalf("evaluator: %v", err)
	} else if out != "500" {
		t.Errorf("evaluator produced %q, want 500", out)
	}
	if out, err := vmEval(res.Program, "result"); err != nil {
		t.Fatalf("VM: %v", err)
	} else if out != "500" {
		t.Errorf("VM produced %q, want 500", out)
	}
}

// compileToVM compiles an already gated program into a fresh VM.
func compileToVM(prog *ast.Program) (*vm.VM, error) {
	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		return nil, err
	}
	return vm.New(comp.Bytecode()), nil
}

// evaluatorErrorCategory runs source in the tree-walking evaluator and returns
// the runtime error category, or "" when it does not fail.
func evaluatorErrorCategory(t *testing.T, source string) object.ErrorCategory {
	t.Helper()
	res := evalSource(t, source)
	if !res.OK {
		t.Fatalf("source was rejected by the gate: %s", res.ErrSummary())
	}
	evaluated := evaluator.Eval(res.Program, newEvaluatorEnv())
	category, _ := object.CategoryOf(evaluated)
	return category
}

// vmErrorCategory runs source in the bytecode VM and returns the runtime error
// category, or "" when it does not fail.
func vmErrorCategory(t *testing.T, source string) object.ErrorCategory {
	t.Helper()
	res := evalSource(t, source)
	if !res.OK {
		t.Fatalf("source was rejected by the gate: %s", res.ErrSummary())
	}
	comp := compiler.New()
	if err := comp.Compile(res.Program); err != nil {
		t.Fatalf("compile: %v", err)
	}
	machine := vm.New(comp.Bytecode())
	err := machine.Run()
	if err == nil {
		return ""
	}
	category, _ := object.CategoryOf(err)
	return category
}
