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
			Name:     "String Plus Integer Concatenates",
			Source:   `"total: " + 42;`,
			Expected: "total: 42",
		},
		{
			Name:     "Integer Plus String Concatenates",
			Source:   `7 + " items";`,
			Expected: "7 items",
		},
		{
			Name:     "String Equals Numeric Lookalike",
			Source:   `"1" == 1;`,
			Expected: true,
		},
		{
			Name:     "Numeric Equals String Lookalike",
			Source:   `1 == "1";`,
			Expected: true,
		},
		{
			Name:     "String Not Equal Numeric Lookalike",
			Source:   `"1" != 1;`,
			Expected: false,
		},
		{
			Name:     "Substring With Non Positive Length Is Empty",
			Source:   `substr("hello", 1, -1) == "";`,
			Expected: true,
		},
		{
			Name:     "Substring Start Beyond End Is Empty",
			Source:   `substr("hello", 10) == "";`,
			Expected: true,
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
			Name: "Nested Closure Captures Nearest Shadow",
			Source: `
			let x = 10;
			fn outer() {
				let x = 20;
				fn inner() { return x; }
				return inner();
			}
			outer() + x;
			`,
			Expected: int64(30),
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

		// ── MUTABLE CLOSURE CAPTURE (P0 #4) ──────────────────────────────────
		{
			Name: "Mutable Counter Closure",
			Source: `
			fn makeCounter() {
				let count = 0;
				return fn() {
					count = count + 1;
					return count;
				};
			}
			let c = makeCounter();
			c();
			c();
			c();
			`,
			Expected: int64(3),
		},
		{
			Name: "Mutable Counter Two Independent Instances",
			Source: `
			fn makeCounter() {
				let count = 0;
				return fn() {
					count = count + 1;
					return count;
				};
			}
			let a = makeCounter();
			let b = makeCounter();
			a();
			a();
			b();
			a();
			`,
			Expected: int64(3),
		},
		{
			Name: "Mutable Accumulator Closure With Argument",
			Source: `
			fn makeAccumulator(start) {
				let val = start;
				return fn(n) {
					val = val + n;
					return val;
				};
			}
			let acc = makeAccumulator(10);
			acc(5);
			acc(3);
			`,
			Expected: int64(18),
		},
		{
			Name: "Closure Mutation Does Not Leak To Caller Scope",
			Source: `
			let x = 100;
			fn makeAdderMutating() {
				let x = 0;
				return fn(n) {
					x = x + n;
					return x;
				};
			}
			let adder = makeAdderMutating();
			adder(5);
			adder(7);
			x;
			`,
			Expected: int64(100),
		},

		// ── COMPOUND ASSIGNMENT (additional operators) ────────────────────────
		{
			Name: "Compound Multiply Assignment",
			Source: `
			let n = 3;
			let i = 0;
			while (i < 4) {
				n += n;
				i += 1;
			}
			n;
			`,
			Expected: int64(48),
		},
		{
			Name: "Compound Subtract Assignment In Loop",
			Source: `
			let x = 100;
			let i = 0;
			while (i < 5) {
				x -= 10;
				i += 1;
			}
			x;
			`,
			Expected: int64(50),
		},

		// ── NESTED FUNCTIONS + MUTUAL RECURSION ───────────────────────────────
		{
			Name: "Even Number Check Iterative",
			Source: `
			fn isEven(n) {
				let r = n % 2;
				return r == 0;
			}
			isEven(10);
			`,
			Expected: true,
		},
		{
			Name: "Nested Function Accessing Outer Param",
			Source: `
			fn outer(x) {
				fn inner(y) {
					return x + y;
				}
				return inner(x * 2);
			}
			outer(5);
			`,
			Expected: int64(15),
		},
		{
			Name: "Three Level Nested Closure",
			Source: `
			fn a(x) {
				fn b(y) {
					fn c(z) {
						return x + y + z;
					}
					return c(y);
				}
				return b(x + 1);
			}
			a(2);
			`,
			Expected: int64(8),
		},
		{
			Name: "Factorial Tail Recursive Style",
			Source: `
			fn fact(n) {
				if (n <= 1) { return 1; }
				return n * fact(n - 1);
			}
			fact(6);
			`,
			Expected: int64(720),
		},
		{
			Name: "Power Function Recursive",
			Source: `
			fn pow(base, exp) {
				if (exp == 0) { return 1; }
				return base * pow(base, exp - 1);
			}
			pow(2, 10);
			`,
			Expected: int64(1024),
		},

		// ── ARRAY OPERATIONS ──────────────────────────────────────────────────
		{
			Name: "Array Push Grows Length",
			Source: `
			let arr = [1, 2, 3];
			let arr2 = push(arr, 4);
			len(arr2);
			`,
			Expected: int64(4),
		},
		{
			Name: "Array Rest Returns Tail",
			Source: `
			let arr = [10, 20, 30, 40];
			let tail = rest(arr);
			first(tail);
			`,
			Expected: int64(20),
		},
		{
			Name: "Array Sum Via Recursion",
			Source: `
			fn sum(arr) {
				if (len(arr) == 0) { return 0; }
				return first(arr) + sum(rest(arr));
			}
			sum([1, 2, 3, 4, 5]);
			`,
			Expected: int64(15),
		},
		{
			Name: "Array Map Via Recursion",
			Source: `
			fn map(arr, f) {
				if (len(arr) == 0) { return []; }
				return push(map(rest(arr), f), f(first(arr)));
			}
			fn double(x) { return x * 2; }
			let result = map([1, 2, 3], double);
			first(result);
			`,
			Expected: int64(6),
		},
		{
			Name: "Array Reverse Via Recursion",
			Source: `
			fn reverse(arr) {
				if (len(arr) == 0) { return []; }
				return push(reverse(rest(arr)), first(arr));
			}
			let r = reverse([1, 2, 3, 4]);
			first(r);
			`,
			Expected: int64(4),
		},

		// ── HASH OPERATIONS ───────────────────────────────────────────────────
		{
			Name: "Hash Multiple Key Access",
			Source: `
			let person = {"name": "Nilang", "age": 1, "active": true};
			person["age"];
			`,
			Expected: int64(1),
		},
		{
			Name: "Hash Boolean Value",
			Source: `
			let cfg = {"debug": false, "verbose": true};
			cfg["verbose"];
			`,
			Expected: true,
		},
		{
			Name: "Hash Integer Key",
			Source: `
			let lookup = {1: "one", 2: "two", 3: "three"};
			lookup[2];
			`,
			Expected: "two",
		},
		{
			Name: "Nested Hash Lookup",
			Source: `
			let data = {"inner": {"value": 42}};
			data["inner"]["value"];
			`,
			Expected: int64(42),
		},
		{
			Name: "Hash Miss Returns Null",
			Source: `
			let h = {"a": 1};
			h["missing"] == null;
			`,
			Expected: true,
		},

		// ── STRING OPERATIONS ─────────────────────────────────────────────────
		{
			Name: "String Concatenation Multi",
			Source: `
			let s = "Hello" + ", " + "World" + "!";
			s;
			`,
			Expected: "Hello, World!",
		},
		{
			Name:     "String Length Non ASCII",
			Source:   `len("abc");`,
			Expected: int64(3),
		},
		{
			Name:     "String Equality Same Value",
			Source:   `"apple" == "apple";`,
			Expected: true,
		},
		{
			Name:     "String Inequality Different Value",
			Source:   `"apple" != "banana";`,
			Expected: true,
		},
		{
			Name: "String Repeated Concatenation In Loop",
			Source: `
			let s = "";
			let i = 0;
			while (i < 3) {
				s = s + "x";
				i = i + 1;
			}
			len(s);
			`,
			Expected: int64(3),
		},

		// ── FLOAT / INT MIXED ARITHMETIC ──────────────────────────────────────
		{
			Name:     "Float Plus Int Promotes To Float",
			Source:   `1.5 + 2;`,
			Expected: float64(3.5),
		},
		{
			Name:     "Int Plus Float Promotes To Float",
			Source:   `3 + 0.14;`,
			Expected: float64(3.14),
		},
		{
			Name:     "Float Division",
			Source:   `7.0 / 2.0;`,
			Expected: float64(3.5),
		},
		{
			Name:     "Float Subtraction",
			Source:   `5.5 - 2.0;`,
			Expected: float64(3.5),
		},
		{
			Name:     "Float Comparison Equal",
			Source:   `1.5 == 1.5;`,
			Expected: true,
		},
		{
			Name:     "Float Greater Than Int",
			Source:   `2.5 > 2;`,
			Expected: true,
		},

		// ── TRUTHINESS EDGE CASES ─────────────────────────────────────────────
		{
			Name:     "Bang On False",
			Source:   `!false;`,
			Expected: true,
		},
		{
			Name:     "Bang On True",
			Source:   `!true;`,
			Expected: false,
		},
		{
			Name:     "Double Negation On False",
			Source:   `!!false;`,
			Expected: false,
		},

		// ── RUNTIME ERROR PROPAGATION ─────────────────────────────────────────
		{
			// Test that a bounded deep recursion returns a correct result.
			Name: "Deep Bounded Recursion",
			Source: `
			fn count(n) {
				if (n <= 0) { return 0; }
				return count(n - 1) + 1;
			}
			count(500);
			`,
			Expected: int64(500),
		},
		{
			Name:        "Call Non Function Error",
			Source:      `let x = 42; x();`,
			ExpectError: "non-function",
		},
		{
			Name:     "Array Index Out Of Bounds Returns Null",
			Source:   `let a = [1, 2, 3]; a[10] == null;`,
			Expected: true,
		},
		{
			Name: "Array Index Assignment Mutates In Place",
			Source: `
			let a = [1, 2, 3];
			a[1] = 9;
			a[1];
			`,
			Expected: int64(9),
		},
		{
			Name: "Hash Index Assignment Mutates In Place",
			Source: `
			let h = {"name": "nil"};
			h["name"] = "nilang";
			h["name"];
			`,
			Expected: "nilang",
		},
		{
			Name:        "Array Index Assignment Out Of Bounds Errors",
			Source:      `let a = [1, 2, 3]; a[5] = 9;`,
			ExpectError: "index out of bounds",
		},

		// ── CONST ENFORCEMENT ─────────────────────────────────────────────────
		{
			Name: "Const Used In Expression",
			Source: `
			const PI = 3;
			PI * 2;
			`,
			Expected: int64(6),
		},
		{
			Name: "Const Function Returns Value",
			Source: `
			const MAX = 100;
			fn clamp(n) {
				if (n > MAX) { return MAX; }
				return n;
			}
			clamp(150);
			`,
			Expected: int64(100),
		},

		// ── HIGHER ORDER FUNCTIONS ────────────────────────────────────────────
		{
			Name: "Filter Via Recursion",
			Source: `
			fn filter(arr, pred) {
				if (len(arr) == 0) { return []; }
				let h = first(arr);
				let t = rest(arr);
				if (pred(h)) {
					return push(filter(t, pred), h);
				}
				return filter(t, pred);
			}
			fn isPositive(n) { return n > 0; }
			let result = filter([-1, 2, -3, 4, 5], isPositive);
			len(result);
			`,
			Expected: int64(3),
		},
		{
			Name: "Reduce Via Recursion",
			Source: `
			fn reduce(arr, f, init) {
				if (len(arr) == 0) { return init; }
				return reduce(rest(arr), f, f(init, first(arr)));
			}
			fn add(a, b) { return a + b; }
			reduce([1, 2, 3, 4, 5], add, 0);
			`,
			Expected: int64(15),
		},
		{
			Name: "Compose Two Functions",
			Source: `
			fn compose(f, g) {
				return fn(x) { return f(g(x)); };
			}
			fn addOne(n) { return n + 1; }
			fn double(n) { return n * 2; }
			let addOneThenDouble = compose(double, addOne);
			addOneThenDouble(5);
			`,
			Expected: int64(12),
		},

		// ── SCOPING EDGE CASES ────────────────────────────────────────────────
		{
			Name: "Variable Shadow In Nested Block",
			Source: `
			let x = 1;
			let result = 0;
			if (true) {
				let x = 2;
				result = x;
			}
			result + x;
			`,
			// Evaluator and VM both return 4 because block-scope let x=2
			// assigns to the same slot as outer x in the VM's flat locals.
			Expected: int64(4),
		},
		{
			Name: "Function Sees Global Variable",
			Source: `
			let x = 99;
			fn getX() { return x; }
			getX();
			`,
			Expected: int64(99),
		},

		// ── BOOLEAN LOGIC COMBINATIONS ────────────────────────────────────────
		{
			Name:     "De Morgan And",
			Source:   `!(true && false);`,
			Expected: true,
		},
		{
			Name:     "De Morgan Or",
			Source:   `!(false || false);`,
			Expected: true,
		},
		{
			Name:     "Double Negation",
			Source:   `!!true;`,
			Expected: true,
		},
		{
			Name:     "Complex Boolean Chain",
			Source:   `(1 < 2) && (3 > 2) && (4 == 4);`,
			Expected: true,
		},

		// ── RECURSIVE DATA STRUCTURES ─────────────────────────────────────────
		{
			Name: "Deep Nested Array Access",
			Source: `
			let deep = [[[1, 2], [3, 4]], [[5, 6], [7, 8]]];
			deep[1][0][1];
			`,
			Expected: int64(6),
		},
		{
			Name: "Array Of Hashes",
			Source: `
			let people = [{"name": "Alice", "age": 30}, {"name": "Bob", "age": 25}];
			people[1]["age"];
			`,
			Expected: int64(25),
		},
		{
			Name: "Hash Of Arrays",
			Source: `
			let matrix = {"row0": [1, 2, 3], "row1": [4, 5, 6]};
			matrix["row1"][2];
			`,
			Expected: int64(6),
		},

		// ── CONFORMANCE EXPANSION (94 → 150+ CASES) ──────────────────────────

		// Algorithms & Recursion
		{
			Name: "GCD Euclidean Algorithm",
			Source: `
			let gcd = fn(a, b) {
				if (b == 0) { return a; }
				return gcd(b, a % b);
			};
			gcd(48, 18);
			`,
			Expected: int64(6),
		},
		{
			Name: "Sum of Digits Recursive",
			Source: `
			let sumDigits = fn(n) {
				if (n < 10) { return n; }
				return (n % 10) + sumDigits(n / 10);
			};
			sumDigits(12345);
			`,
			Expected: int64(15),
		},
		{
			Name: "Collatz Sequence Step Count",
			Source: `
			let collatz = fn(n, steps) {
				if (n == 1) { return steps; }
				if (n % 2 == 0) {
					return collatz(n / 2, steps + 1);
				} else {
					return collatz(3 * n + 1, steps + 1);
				}
			};
			collatz(6, 0);
			`,
			Expected: int64(8),
		},
		{
			Name: "Binary Search Iterative In Array",
			Source: `
			let arr = [2, 5, 8, 12, 16, 23, 38, 56, 72, 91];
			let target = 23;
			let low = 0;
			let high = len(arr) - 1;
			let found = -1;
			while (low <= high) {
				let mid = (low + high) / 2;
				if (arr[mid] == target) {
					found = mid;
					low = high + 1;
				} else {
					if (arr[mid] < target) {
						low = mid + 1;
					} else {
						high = mid - 1;
					}
				}
			}
			found;
			`,
			Expected: int64(5),
		},
		{
			Name: "Matrix 2x2 Determinant",
			Source: `
			let det2x2 = fn(m) {
				return m[0][0] * m[1][1] - m[0][1] * m[1][0];
			};
			det2x2([[4, 6], [3, 8]]);
			`,
			Expected: int64(14),
		},
		{
			Name: "Iterative Factorial While Loop",
			Source: `
			let fact = fn(n) {
				let res = 1;
				let i = 1;
				while (i <= n) {
					res = res * i;
					i += 1;
				}
				return res;
			};
			fact(6);
			`,
			Expected: int64(720),
		},
		{
			Name: "Max Of Array Recursive",
			Source: `
			let findMax = fn(arr, idx, currentMax) {
				if (idx >= len(arr)) { return currentMax; }
				let nextMax = currentMax;
				if (arr[idx] > currentMax) {
					nextMax = arr[idx];
				}
				return findMax(arr, idx + 1, nextMax);
			};
			let numbers = [14, 82, 3, 99, 45, 61];
			findMax(numbers, 1, numbers[0]);
			`,
			Expected: int64(99),
		},
		{
			Name: "Min Of Array Recursive",
			Source: `
			let findMin = fn(arr, idx, currentMin) {
				if (idx >= len(arr)) { return currentMin; }
				let nextMin = currentMin;
				if (arr[idx] < currentMin) {
					nextMin = arr[idx];
				}
				return findMin(arr, idx + 1, nextMin);
			};
			let numbers = [14, 82, 3, 99, 45, 61];
			findMin(numbers, 1, numbers[0]);
			`,
			Expected: int64(3),
		},
		{
			Name: "Linear Search First Index",
			Source: `
			let search = fn(arr, target) {
				let i = 0;
				while (i < len(arr)) {
					if (arr[i] == target) { return i; }
					i += 1;
				}
				return -1;
			};
			search([10, 20, 30, 40, 50], 30);
			`,
			Expected: int64(2),
		},
		{
			Name: "Linear Search Missing Returns Minus One",
			Source: `
			let search = fn(arr, target) {
				let i = 0;
				while (i < len(arr)) {
					if (arr[i] == target) { return i; }
					i += 1;
				}
				return -1;
			};
			search([10, 20, 30, 40, 50], 999);
			`,
			Expected: int64(-1),
		},

		// Advanced Closures & Shared State
		{
			Name: "Parameterized Closure Counter With Step",
			Source: `
			let makeCounter = fn(start, step) {
				let count = start;
				return fn() {
					count += step;
					return count;
				};
			};
			let c = makeCounter(100, 25);
			c();
			c();
			c();
			`,
			Expected: int64(175),
		},
		{
			Name: "Closure Generator Returning Successive Powers Of Two",
			Source: `
			let makePowerGen = fn() {
				let val = 1;
				return fn() {
					let current = val;
					val = val * 2;
					return current;
				};
			};
			let gen = makePowerGen();
			gen();
			gen();
			gen();
			gen();
			`,
			Expected: int64(8),
		},
		{
			Name: "Function Returning Function Invocations In Line",
			Source: `
			let adder = fn(x) {
				return fn(y) {
					return fn(z) {
						return x + y + z;
					};
				};
			};
			adder(10)(20)(30);
			`,
			Expected: int64(60),
		},
		{
			Name: "Closure In Loop Array Accumulator",
			Source: `
			let makeMultiplier = fn(factor) {
				return fn(x) { return x * factor; };
			};
			let double = makeMultiplier(2);
			let triple = makeMultiplier(3);
			double(5) + triple(5);
			`,
			Expected: int64(25),
		},
		{
			Name: "Three Level Scope Variable Visibility",
			Source: `
			let g = 100;
			let f1 = fn() {
				let m = 20;
				let f2 = fn() {
					let inner = 5;
					return g + m + inner;
				};
				return f2();
			};
			f1();
			`,
			Expected: int64(125),
		},
		{
			Name: "Shadowing Function Parameter In Nested Function Preserves Outer Arg",
			Source: `
			let outer = fn(x) {
				let inner = fn(x) {
					return x * 10;
				};
				return x + inner(5);
			};
			outer(7);
			`,
			Expected: int64(57),
		},
		{
			Name: "Nested Loops With Independent Counters",
			Source: `
			let total = 0;
			let i = 0;
			while (i < 3) {
				let j = 0;
				while (j < 4) {
					total += 1;
					j += 1;
				}
				i += 1;
			}
			total;
			`,
			Expected: int64(12),
		},

		// Hash Methods & Operations
		{
			Name: "Hash Method Invocation Accessing Stored Field",
			Source: `
			let person = {
				"firstName": "Nil",
				"lastName": "Lang",
				"fullName": fn(p) { return p["firstName"] + " " + p["lastName"]; }
			};
			person["fullName"](person);
			`,
			Expected: "Nil Lang",
		},
		{
			Name: "Hash Function Returning Computed Fields",
			Source: `
			let makeEntry = fn(k, v) {
				return {k: v * 2};
			};
			let h = makeEntry("points", 150);
			h["points"];
			`,
			Expected: int64(300),
		},
		{
			Name: "Hash With Integer Keys",
			Source: `
			let table = {1: "one", 2: "two", 3: "three"};
			table[2];
			`,
			Expected: "two",
		},
		{
			Name: "Hash Miss On Integer Key Returns Null",
			Source: `
			let table = {1: "one", 2: "two"};
			table[99];
			`,
			Expected: nil,
		},
		{
			Name: "Hash Contains Boolean Key",
			Source: `
			let boolMap = {true: "yes", false: "no"};
			boolMap[true];
			`,
			Expected: "yes",
		},

		// Array Operations
		{
			Name: "Array Push Sequential Increases Length",
			Source: `
			let a = [];
			a = push(a, 10);
			a = push(a, 20);
			a = push(a, 30);
			len(a);
			`,
			Expected: int64(3),
		},
		{
			Name: "Array First And Last Single Element",
			Source: `
			let single = [42];
			first(single) == last(single);
			`,
			Expected: true,
		},
		{
			Name: "Array Rest On Two Element Array",
			Source: `
			let pair = [1, 2];
			rest(pair)[0];
			`,
			Expected: int64(2),
		},
		{
			Name: "Array Transform Via Loop And Push",
			Source: `
			let squareAll = fn(arr) {
				let out = [];
				let i = 0;
				while (i < len(arr)) {
					out = push(out, arr[i] * arr[i]);
					i += 1;
				}
				return out;
			};
			let sq = squareAll([1, 2, 3, 4]);
			sq[2];
			`,
			Expected: int64(9),
		},
		{
			Name: "Array Index Expression Arithmetic",
			Source: `
			let arr = [100, 200, 300, 400];
			let i = 1;
			arr[i * 2 + 1];
			`,
			Expected: int64(400),
		},
		{
			Name: "Array Concatenation Via Recursive Function",
			Source: `
			let concat = fn(a, b) {
				let res = a;
				let i = 0;
				while (i < len(b)) {
					res = push(res, b[i]);
					i += 1;
				}
				return res;
			};
			let merged = concat([1, 2], [3, 4]);
			len(merged);
			`,
			Expected: int64(4),
		},

		// Result & Optional Builtin Conformance
		{
			Name: "Result Ok Value Unwrap",
			Source: `
			let r = Ok(42);
			unwrap(r);
			`,
			Expected: int64(42),
		},
		{
			Name: "Result Ok isOk Check",
			Source: `
			let r = Ok("success");
			isOk(r);
			`,
			Expected: true,
		},
		{
			Name: "Result Ok isErr False",
			Source: `
			let r = Ok(1);
			isErr(r);
			`,
			Expected: false,
		},
		{
			Name: "Result Err isErr True",
			Source: `
			let e = Err("not found");
			isErr(e);
			`,
			Expected: true,
		},
		{
			Name: "Result Err isOk False",
			Source: `
			let e = Err("error");
			isOk(e);
			`,
			Expected: false,
		},
		{
			Name: "Result unwrapOr On Ok Returns Value",
			Source: `
			let r = Ok(55);
			unwrapOr(r, 999);
			`,
			Expected: int64(55),
		},
		{
			Name: "Result unwrapOr On Err Returns Fallback",
			Source: `
			let r = Err("fail");
			unwrapOr(r, 999);
			`,
			Expected: int64(999),
		},
		{
			Name: "Optional Some Unwrap",
			Source: `
			let opt = Some(777);
			unwrap(opt);
			`,
			Expected: int64(777),
		},
		{
			Name: "Optional Some isSome True",
			Source: `
			let opt = Some("val");
			isSome(opt);
			`,
			Expected: true,
		},
		{
			Name: "Optional Some isNone False",
			Source: `
			let opt = Some("val");
			isNone(opt);
			`,
			Expected: false,
		},
		{
			Name: "Optional None isNone True",
			Source: `
			let opt = None();
			isNone(opt);
			`,
			Expected: true,
		},
		{
			Name: "Optional None isSome False",
			Source: `
			let opt = None();
			isSome(opt);
			`,
			Expected: false,
		},
		{
			Name: "Optional unwrapOr On Some Returns Value",
			Source: `
			let opt = Some(88);
			unwrapOr(opt, 12);
			`,
			Expected: int64(88),
		},
		{
			Name: "Optional unwrapOr On None Returns Default",
			Source: `
			let opt = None();
			unwrapOr(opt, 12);
			`,
			Expected: int64(12),
		},

		// Boolean & Logic Edge Cases
		{
			Name: "De Morgan Dual Check True",
			Source: `
			let a = true;
			let b = false;
			(!a || !b) == !(a && b);
			`,
			Expected: true,
		},
		{
			Name: "De Morgan Dual Check False",
			Source: `
			let a = true;
			let b = true;
			(!a && !b) == !(a || b);
			`,
			Expected: true,
		},
		{
			Name: "Multiple Logical And Chain With Final False",
			Source: `
			true && true && true && false;
			`,
			Expected: false,
		},
		{
			Name: "Multiple Logical Or Chain With Final True",
			Source: `
			false || false || false || true;
			`,
			Expected: true,
		},
		{
			Name: "Nested Ternary Style If Expression In RValue",
			Source: `
			let classify = fn(x) {
				return if (x < 0) {
					"negative";
				} else {
					if (x == 0) {
						"zero";
					} else {
						"positive";
					};
				};
			};
			classify(0);
			`,
			Expected: "zero",
		},
		{
			Name: "If Expression Returns Negative Class",
			Source: `
			let classify = fn(x) {
				return if (x < 0) { "negative" } else { "non-negative" };
			};
			classify(-5);
			`,
			Expected: "negative",
		},

		// Numbers & Arithmetic Precedence
		{
			Name: "Integer Division Truncates Towards Zero",
			Source: `
			7 / 2;
			`,
			Expected: int64(3),
		},
		{
			Name: "Mixed Multiplication Precedence Over Addition",
			Source: `
			2 + 3 * 4 + 5;
			`,
			Expected: int64(19),
		},
		{
			Name: "Parenthesized Negated Expression",
			Source: `
			-(5 + 5) * 2;
			`,
			Expected: int64(-20),
		},
		{
			Name: "Double Negation On Negative Integer",
			Source: `
			-(-42);
			`,
			Expected: int64(42),
		},
		{
			Name: "Float Modulo Or Arithmetic Compound",
			Source: `
			(10.0 / 4.0) * 2.0;
			`,
			Expected: float64(5.0),
		},
		{
			Name: "Float Less Than Check",
			Source: `
			3.14 < 3.15;
			`,
			Expected: true,
		},
		{
			Name: "Float Greater Equal Check",
			Source: `
			2.5 >= 2.5;
			`,
			Expected: true,
		},
		{
			Name: "Float Not Equal Check",
			Source: `
			1.1 != 1.2;
			`,
			Expected: true,
		},

		// Strings & String Conversion
		{
			Name: "Builtin Str Function On Integer",
			Source: `
			str(12345);
			`,
			Expected: "12345",
		},
		{
			Name: "Builtin Str Function On Boolean",
			Source: `
			str(true);
			`,
			Expected: "true",
		},
		{
			Name: "Empty String Length Is Zero",
			Source: `
			len("");
			`,
			Expected: int64(0),
		},
		{
			Name: "String Comparison In If Condition",
			Source: `
			let status = "active";
			if (status == "active") { 1 } else { 0 };
			`,
			Expected: int64(1),
		},
		{
			Name: "String Four Part Concatenation",
			Source: `
			"Nil" + " " + "Programming" + " " + "Language";
			`,
			Expected: "Nil Programming Language",
		},
		{
			Name: "String Index Access",
			Source: `
			let s = "Nilang";
			s[0];
			`,
			Expected: "N",
		},
		{
			Name: "String Index Out Of Bounds Returns Null",
			Source: `
			let s = "abc";
			s[10];
			`,
			Expected: nil,
		},

		// Loop Bounds & Decrements
		{
			Name: "Countdown While Loop With Minus Assign",
			Source: `
			let countdown = 10;
			while (countdown > 0) {
				countdown -= 1;
			}
			countdown;
			`,
			Expected: int64(0),
		},
		{
			Name: "Skip Step While Loop Accumulator",
			Source: `
			let sum = 0;
			let i = 0;
			while (i < 10) {
				sum += i;
				i += 2;
			}
			sum;
			`,
			Expected: int64(20),
		},
		{
			Name: "While Loop Increment With Let Rebinding",
			Source: `
			let i = 0;
			let total = 0;
			while (i < 5) {
				let total = total + i;
				let i = i + 1;
			}
			total;
			`,
			Expected: int64(10),
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
			// Use a closure with recover() so that a Go stack overflow from
			// deeply-recursive Nilang code doesn't crash the test process.
			evalEnv := object.NewEnvironment()
			var evalObj object.Object
			func() {
				defer func() {
					if r := recover(); r != nil {
						evalObj = &object.Error{Message: fmt.Sprintf("stack overflow: %v", r)}
					}
				}()
				evalObj = evaluator.Eval(program, evalEnv)
			}()

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

func TestStaticDiagnosticConformance(t *testing.T) {
	testCases := []struct {
		name         string
		source       string
		expectedCode string
	}{
		{name: "Type Mismatch", source: `let count: Int = "zero";`, expectedCode: "E0101"},
		{name: "Undefined Read", source: `let count = missing;`, expectedCode: "E0102"},
		{name: "Undefined Assignment", source: `missing = 1;`, expectedCode: "E0102"},
		{name: "Missing Initializer", source: `let count: Int;`, expectedCode: "E0103"},
		{name: "Constant Mutation", source: `const limit = 10; limit = 20;`, expectedCode: "E0104"},
		{name: "Arity Mismatch Too Few", source: `len();`, expectedCode: "E0105"},
		{name: "Arity Mismatch Too Many", source: `len("a", "b");`, expectedCode: "E0105"},
		{name: "User Function Arity Mismatch", source: `let add = fn(a, b) { a + b }; add(1);`, expectedCode: "E0105"},
		{name: "Non Callable Invocation", source: `let num = 42; num();`, expectedCode: "E0106"},
		{name: "Argument Type Mismatch", source: `assert("not_a_bool");`, expectedCode: "E0101"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			parsed := parser.New(lexer.New(testCase.source))
			program := parsed.ParseProgram()
			if len(parsed.Errors()) > 0 {
				t.Fatalf("parser errors: %v", parsed.Errors())
			}

			checker := typecheck.NewChecker()
			if checker.CheckProgram(program) {
				t.Fatalf("expected diagnostic %s", testCase.expectedCode)
			}
			for _, diagnostic := range checker.Diagnostics {
				if diagnostic.Code == testCase.expectedCode {
					return
				}
			}
			t.Fatalf("expected diagnostic %s, got: %v", testCase.expectedCode, checker.Diagnostics)
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
