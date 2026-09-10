package conformance

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/compiler/typecheck"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
)

type ConformanceCase struct {
	Name        string
	Source      string
	Expected    any    // int64, float64, bool, string, or nil
	ExpectError string // non-empty if both engines must error with this substring
}

func TestDualEngineConformance(t *testing.T) {
	testCases := []ConformanceCase{
		{
			Name:     "Integer Arithmetic Precedence",
			Source:   "1 + 2 * 3;",
			Expected: int64(7),
		},
		{
			Name:     "Complex Parenthesized Arithmetic",
			Source:   "(10 - 4) / 2 + 5 * 3;",
			Expected: int64(18),
		},
		{
			Name:     "Integer Negation",
			Source:   "-42 + 50;",
			Expected: int64(8),
		},
		{
			Name:     "Integer Modulo",
			Source:   "17 % 5;",
			Expected: int64(2),
		},
		{
			Name:     "Floating Point Operations",
			Source:   "1.5 * 2.0 + 0.5;",
			Expected: float64(3.5),
		},
		{
			Name:     "Boolean Relational Greater",
			Source:   "10 > 5;",
			Expected: true,
		},
		{
			Name:     "Boolean Relational Less Equal",
			Source:   "5 <= 5;",
			Expected: true,
		},
		{
			Name:     "Equality Check",
			Source:   "42 == 42;",
			Expected: true,
		},
		{
			Name:     "Inequality Check",
			Source:   "10 != 20;",
			Expected: true,
		},
		{
			Name:     "Boolean Negation",
			Source:   "!false;",
			Expected: true,
		},
		{
			Name:     "Short Circuit And",
			Source:   "true && false;",
			Expected: false,
		},
		{
			Name:     "Short Circuit Or",
			Source:   "false || true;",
			Expected: true,
		},
		{
			Name:     "If-Else Consequence",
			Source:   "if (10 > 5) { 100 } else { 200 };",
			Expected: int64(100),
		},
		{
			Name:     "If-Else Alternative",
			Source:   "if (2 > 5) { 100 } else { 200 };",
			Expected: int64(200),
		},
		{
			Name: "While Loop Accumulator",
			Source: `
			let i = 0;
			let sum = 0;
			while (i < 5) {
				sum = sum + i;
				i = i + 1;
			}
			sum;
			`,
			Expected: int64(10),
		},
		{
			Name: "Function Declaration and Invocation",
			Source: `
			fn multiply(a, b) {
				return a * b;
			}
			multiply(6, 7);
			`,
			Expected: int64(42),
		},
		{
			Name: "Recursive Fibonacci",
			Source: `
			fn fib(n) {
				if (n < 2) {
					return n;
				} else {
					return fib(n - 1) + fib(n - 2);
				}
			}
			fib(7);
			`,
			Expected: int64(13),
		},
		{
			Name: "Lexical Closure Capturing Variable",
			Source: `
			fn makeAdder(x) {
				return fn(y) {
					return x + y;
				};
			}
			let addTen = makeAdder(10);
			addTen(32);
			`,
			Expected: int64(42),
		},
		{
			Name: "Array Literal Indexing",
			Source: `
			let arr = [10, 20, 30, 40];
			arr[2];
			`,
			Expected: int64(30),
		},
		{
			Name: "Hash Literal Lookup",
			Source: `
			let record = {"name": "Nilang", "version": 1};
			record["version"];
			`,
			Expected: int64(1),
		},
		{
			Name:     "String Concatenation",
			Source:   `"Hello, " + "Nilang!";`,
			Expected: "Hello, Nilang!",
		},
		{
			Name:        "Integer Division By Zero Exception",
			Source:      "10 / 0;",
			ExpectError: "division by zero",
		},
		{
			Name:        "Modulo By Zero Exception",
			Source:      "10 % 0;",
			ExpectError: "division by zero",
		},
		{
			Name: "Nested While Loops Accumulator",
			Source: `
			let sum = 0;
			let i = 0;
			while (i < 3) {
				let j = 0;
				while (j < 3) {
					sum = sum + (i * j);
					j = j + 1;
				}
				i = i + 1;
			}
			sum;
			`,
			Expected: int64(9),
		},
		{
			Name: "Multi-Dimensional Array Indexing",
			Source: `
			let grid = [[10, 20], [30, 40]];
			grid[1][0];
			`,
			Expected: int64(30),
		},
		{
			Name: "Builtin Array Length",
			Source: `
			let items = [1, 2, 3, 4, 5];
			len(items);
			`,
			Expected: int64(5),
		},
		{
			Name: "Builtin String Length",
			Source: `
			len("Nilang");
			`,
			Expected: int64(6),
		},
		{
			Name: "Builtin Array First and Last",
			Source: `
			let arr = [100, 200, 300];
			first(arr) + last(arr);
			`,
			Expected: int64(400),
		},
		{
			Name: "Higher-Order Function Passing",
			Source: `
			fn applyTwice(f, val) {
				return f(f(val));
			}
			fn double(n) {
				return n * 2;
			}
			applyTwice(double, 5);
			`,
			Expected: int64(20),
		},
		{
			Name: "Closure Isolation Between Instances",
			Source: `
			fn makeMultiplier(factor) {
				return fn(x) {
					return x * factor;
				};
			}
			let mult3 = makeMultiplier(3);
			let mult7 = makeMultiplier(7);
			mult3(5) + mult7(5);
			`,
			Expected: int64(50),
		},
		{
			Name: "Function Scoping Shadowing Preserves Outer Binding",
			Source: `
			let x = 100;
			fn getLocal() {
				let x = 999;
				return x;
			}
			getLocal() + x;
			`,
			Expected: int64(1099),
		},
		{
			Name: "String Equality Comparison",
			Source: `
			"nilang" == "nilang";
			`,
			Expected: true,
		},
		{
			Name: "String Inequality Comparison",
			Source: `
			"alpha" == "beta";
			`,
			Expected: false,
		},
		{
			Name: "Compound Assignment In Loop",
			Source: `
			let sum = 0;
			let k = 1;
			while (k <= 5) {
				sum += k;
				k += 1;
			}
			sum;
			`,
			Expected: int64(15),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// Step 1: Lex and Parse
			l := lexer.New(tc.Source)
			p := parser.New(l)
			program := p.ParseProgram()
			if len(p.Errors()) > 0 {
				t.Fatalf("[%s] Parser errors: %v", tc.Name, p.Errors())
			}

			// Step 2: Static Verification Gate
			checker := typecheck.NewChecker()
			if !checker.CheckProgram(program) {
				// If we expect an error and typecheck caught it, that's fine
				if tc.ExpectError == "" {
					t.Fatalf("[%s] Typecheck rejected valid program: %v", tc.Name, checker.Diagnostics)
				}
			}

			// Step 3: Run in Tree-Walking Evaluator
			evalEnv := object.NewEnvironment()
			evalObj := evaluator.Eval(program, evalEnv)

			// Step 4: Run in Stack VM
			comp := compiler.New()
			compErr := comp.Compile(program)
			var vmObj object.Object
			var vmErr error

			if compErr == nil {
				machine := vm.New(comp.Bytecode())
				vmErr = machine.Run()
				if vmErr == nil {
					vmObj = machine.LastPoppedStackElem()
				}
			}

			// Step 5: Assert Expectations
			if tc.ExpectError != "" {
				// Both engines must produce an error
				evalErrorMsg := ""
				if errObj, ok := evalObj.(*object.Error); ok {
					evalErrorMsg = errObj.Message
				}

				if evalErrorMsg == "" {
					t.Errorf("[%s] Evaluator did not produce expected error %q. Got=%v", tc.Name, tc.ExpectError, evalObj)
				}
				if compErr == nil && vmErr == nil {
					t.Errorf("[%s] VM did not produce expected error %q. Got=%v", tc.Name, tc.ExpectError, vmObj)
				}
				return
			}

			// Normal execution path: both must succeed and agree
			if compErr != nil {
				t.Fatalf("[%s] Compiler error: %s", tc.Name, compErr)
			}
			if vmErr != nil {
				t.Fatalf("[%s] VM execution error: %s", tc.Name, vmErr)
			}
			if evalObj == nil {
				t.Fatalf("[%s] Evaluator returned nil object", tc.Name)
			}
			if vmObj == nil {
				t.Fatalf("[%s] VM returned nil object", tc.Name)
			}

			if errObj, ok := evalObj.(*object.Error); ok {
				t.Fatalf("[%s] Evaluator unexpected error: %s", tc.Name, errObj.Message)
			}

			// Convert objects to Go native values
			evalVal := objectToNative(evalObj)
			vmVal := objectToNative(vmObj)

			// Compare Evaluator vs VM output: they MUST agree!
			if !valuesEqual(evalVal, vmVal) {
				t.Fatalf("[%s] Engine Disagreement! Evaluator=%v (%T), VM=%v (%T)",
					tc.Name, evalVal, evalVal, vmVal, vmVal)
			}

			// Compare against expected value
			if !valuesEqual(evalVal, tc.Expected) {
				t.Fatalf("[%s] Output mismatch! Got=%v (%T), Expected=%v (%T)",
					tc.Name, evalVal, evalVal, tc.Expected, tc.Expected)
			}
		})
	}
}

func objectToNative(obj object.Object) any {
	if obj == nil {
		return nil
	}
	switch o := obj.(type) {
	case *object.Integer:
		return o.Value
	case *object.Float:
		return o.Value
	case *object.Boolean:
		return o.Value
	case *object.String:
		return o.Value
	case *object.Null:
		return nil
	default:
		return o.Inspect()
	}
}

func valuesEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if fa, ok := a.(float64); ok {
		if fb, ok := b.(float64); ok {
			return math.Abs(fa-fb) < 1e-9
		}
	}
	return reflect.DeepEqual(a, b) || fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}
