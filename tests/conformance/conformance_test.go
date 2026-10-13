package conformance

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/compiler/typecheck"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
)

type ConformanceCase struct {
	Name     string
	Source   string
	Expected any // int64, float64, bool, string, or nil

	// ExpectError is non-empty when both engines must fail, and its value is a
	// substring of the expected message.
	//
	// Substring matching is retained for human intent ("division by zero"), but
	// it is not the contract. ERROR_MODEL.md §4 requires both engines to report
	// the same *category*, which ExpectCategory asserts; message text is free to
	// change without breaking conformance.
	ExpectError string

	// ExpectCategory is the object.ErrorCategory both engines must report when
	// the case expects an error. Empty skips the cross-engine category check.
	ExpectCategory object.ErrorCategory

	// SkipAOT is non-empty when the ahead-of-time backend cannot run this
	// case yet; its value is the recorded reason, asserted by the ratchet in
	// threeengines_test.go. A case lands here for a missing backend
	// capability, never to hide a disagreement between the two Go engines.
	SkipAOT string
}

// errorMessageOf extracts the message from either error shape the evaluator can
// return: a categorised *object.RuntimeError or a legacy bare *object.Error.
func errorMessageOf(obj object.Object) string {
	switch err := obj.(type) {
	case *object.RuntimeError:
		return err.Message
	case *object.Error:
		return err.Message
	case nil:
		return ""
	default:
		return ""
	}
}

// conformanceCases is every program the suite runs through every engine.

// # Why it is a package variable rather than a local
//
// Because the two engine suites have to run the *same* programs. A list written
// inside one test cannot be reached from the other, and the obvious workaround —
// copy the list — is how two suites drift into testing different things while both
// report the same number of cases.
var conformanceCases = []ConformanceCase{
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
		Name:           "Integer Division By Zero Exception",
		Source:         "10 / 0;",
		ExpectError:    "division by zero",
		ExpectCategory: object.CategoryDivisionByZero,
	},
	{
		// Modulo by zero shares the DivisionByZero category with `/ 0`: a
		// program that divides by zero and one that takes a remainder by zero
		// have the same cause and the same remedy.
		Name:           "Modulo By Zero Exception",
		Source:         "10 % 0;",
		ExpectError:    "division by zero",
		ExpectCategory: object.CategoryDivisionByZero,
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

	// ── FOR LOOPS, BREAK, CONTINUE ───────────────────────────────────────
	// These reach both engines through the lowering pass, so a divergence
	// here means the engines are not running the same lowered program.
	{
		Name:     "For Over Exclusive Range",
		Source:   "let t = 0; for i in 0..5 { t = t + i; } t;",
		Expected: int64(10),
	},
	{
		Name:     "For Over Inclusive Range",
		Source:   "let t = 0; for i in 0..=4 { t = t + i; } t;",
		Expected: int64(10),
	},
	{
		Name:     "For Over Array",
		Source:   "let t = 0; for x in [10, 20, 30] { t = t + x; } t;",
		Expected: int64(60),
	},
	{
		Name:     "For Over Array With Index",
		Source:   "let t = 0; for i, x in [4, 5] { t = t + i * x; } t;",
		Expected: int64(5),
	},
	{
		Name:     "For Descending Range",
		Source:   "let t = 0; for i in 3..0 { t = t + i; } t;",
		Expected: int64(6),
	},
	{
		Name:     "Break Exits Loop",
		Source:   "let c = 0; for i in 0..100 { if (i == 4) { break; } c = c + 1; } c;",
		Expected: int64(4),
	},
	{
		Name:     "Continue Skips Body",
		Source:   "let t = 0; for i in 0..6 { if (i % 2 == 0) { continue; } t = t + i; } t;",
		Expected: int64(9),
	},
	{
		Name:     "Continue Advances The Loop",
		Source:   "let c = 0; for i in 0..10 { continue; c = c + 1; } c;",
		Expected: int64(0),
	},
	{
		Name:     "Break In Inner Loop Leaves Outer Running",
		Source:   "let t = 0; for i in 0..3 { for j in 0..3 { if (j == 1) { break; } t = t + 1; } t = t + 10; } t;",
		Expected: int64(33),
	},
	{
		Name:     "Nested For Over Same Range",
		Source:   "let t = 0; for i in 0..3 { for j in 0..3 { t = t + 1; } } t;",
		Expected: int64(9),
	},
	{
		Name:     "For Over Empty Array",
		Source:   "let c = 0; for x in [] { c = c + 1; } c;",
		Expected: int64(0),
	},
	{
		Name:     "While With Break",
		Source:   "let c = 0; let i = 0; while (i < 100) { if (i == 3) { break; } c = c + 1; i = i + 1; } c;",
		Expected: int64(3),
	},
	{
		Name:     "While With Continue",
		Source:   "let c = 0; let i = 0; while (i < 5) { i = i + 1; if (i % 2 == 0) { continue; } c = c + 1; } c;",
		Expected: int64(3),
	},
	{
		Name:     "For Over String Yields Characters",
		Source:   "let s = \"\"; for c in \"abc\" { s = s + c; } s;",
		Expected: "abc",
	},

	// ── STRUCT, ENUM, MATCH ──────────────────────────────────────────────
	// These reach both engines through the lowering pass, so a divergence
	// here means the engines are not running the same lowered program.
	{
		Name:     "Struct Field Access",
		Source:   "struct P { x: Int, y: String } let p = P { x: 1, y: \"a\" }; p.x;",
		Expected: int64(1),
	},
	{
		Name:     "Struct Field Write",
		Source:   "struct P { x: Int } let p = P { x: 1 }; p.x = 9; p.x;",
		Expected: int64(9),
	},
	{
		Name:     "Struct Keeps Declaration Order",
		Source:   "struct P { b: Int, a: Int } P { b: 1, a: 2 };",
		Expected: "{b: 1, a: 2}",
	},
	{
		// The order survives a rehash.
		//
		// A map's written order is kept in an array of *slot* indices, and every
		// index in it is wrong the moment the table grows and rehashes. So this case
		// is written to force a growth: the compiler pre-sizes a struct literal from
		// its field count, but a map built by repeated assignment starts at four
		// slots and doubles from there.
		//
		// Twelve entries in reverse-alphabetical order, so a table that came out in
		// hash order or in slot order could not produce the right answer by luck.
		Name: "Map Keeps Written Order Through A Rehash",
		Source: `
			let m = {};
			m["l"] = 1; m["k"] = 2; m["j"] = 3; m["i"] = 4; m["h"] = 5; m["g"] = 6;
			m["f"] = 7; m["e"] = 8; m["d"] = 9; m["c"] = 10; m["b"] = 11; m["a"] = 12;
			m;
		`,
		Expected: `{l: 1, k: 2, j: 3, i: 4, h: 5, g: 6, f: 7, e: 8, d: 9, c: 10, b: 11, a: 12}`,
	},
	{
		// Writing a key again replaces its value; it does not move it.
		//
		// `m["a"] = 1; m["b"] = 2; m["a"] = 3` wrote `b` second, so the answer is
		// `a, b`. A table that appended on every write would say `b, a`, and one that
		// rehashed on every write would say neither.
		Name:     "Rewriting A Key Keeps Its Place",
		Source:   "let m = {}; m[\"a\"] = 1; m[\"b\"] = 2; m[\"a\"] = 3; m;",
		Expected: "{a: 3, b: 2}",
	},
	{
		Name:     "Hash Literal Keeps Written Order",
		Source:   `{"z": 1, "y": 2, "x": 3};`,
		Expected: "{z: 1, y: 2, x: 3}",
	},
	{
		Name:     "Enum Variant With Payload",
		Source:   "enum E { A(Int) } A(3);",
		Expected: "{variant: A, payload: 3}",
	},
	{
		Name:     "Enum Variant Without Payload",
		Source:   "enum E { A, B } B;",
		Expected: "{variant: B}",
	},
	{
		Name:     "Match Literal",
		Source:   "match 2 { 1 => \"one\", 2 => \"two\", _ => \"many\" };",
		Expected: "two",
	},
	{
		Name:     "Match Binding",
		Source:   "let v = 5; match v { x => x * 2 };",
		Expected: int64(10),
	},
	{
		Name:     "Match Or Pattern",
		Source:   "let v = 3; match v { 1 | 2 | 3 => \"low\", _ => \"high\" };",
		Expected: "low",
	},
	{
		Name:     "Match Guard True",
		Source:   "match 7 { n if n > 5 => \"big\", _ => \"small\" };",
		Expected: "big",
	},
	{
		Name:     "Match Guard False",
		Source:   "match 3 { n if n > 5 => \"big\", _ => \"small\" };",
		Expected: "small",
	},
	{
		Name:     "Match Enum Payload",
		Source:   "enum E { A(Int), B } match A(4) { A(n) => n, B => 0 };",
		Expected: int64(4),
	},
	{
		Name:     "Match Struct Pattern",
		Source:   "struct P { x: Int, y: Int } let p = P { x: 1, y: 2 }; match p { P { x: a, y: b } => a + b };",
		Expected: int64(3),
	},
	{
		Name:     "Match Ok",
		Source:   "match Ok(5) { Ok(v) => v, Err(e) => 0 };",
		Expected: int64(5),
	},
	{
		Name:     "Match Err Payload",
		Source:   "match Err(\"bad\") { Ok(v) => \"ok\", Err(e) => e };",
		Expected: "bad",
	},
	{
		Name:     "Match Nested",
		Source:   "match Ok(2) { Ok(v) => match v { 1 => \"one\", 2 => \"two\", _ => \"many\" }, Err(_) => \"no\" };",
		Expected: "two",
	},
	{
		Name:     "Match Subject Evaluated Once",
		Source:   "let n = 0; fn next() { n = n + 1; return n; } match next() { 1 => \"one\", _ => \"many\" } + n;",
		Expected: "one1",
	},
	{
		Name:     "Match In A Function",
		Source:   "fn classify(n) { return match n { 0 => \"zero\", x if x < 0 => \"neg\", _ => \"pos\" }; } classify(0);",
		Expected: "zero",
	},
	{
		Name:     "Type Alias Annotation",
		Source:   "type Count = Int let n: Count = 5; n;",
		Expected: int64(5),
	},
	{
		Name:     "Type Alias Chain",
		Source:   "type A = Int type B = A type C = B let n: C = 7; n;",
		Expected: int64(7),
	},
	// ── BUILTIN SHADOWING ────────────────────────────────────────────────
	// A `let` that shadows a builtin is ordinary code. It used to compile to
	// a local write while reads went to the builtin, so the VM failed with a
	// type error naming BUILTIN and INTEGER.
	{
		Name:     "Shadow Len",
		Source:   "let len = 99; len + 1;",
		Expected: int64(100),
	},
	{
		Name:     "Shadow First",
		Source:   "let first = \"hello\"; first;",
		Expected: "hello",
	},
	{
		Name:     "Shadow Then Use Real Builtin",
		Source:   "let first = \"abc\"; len(first);",
		Expected: int64(3),
	},
	{
		Name:     "Shadow Push",
		Source:   "let push = 5; push * 2;",
		Expected: int64(10),
	},
	{
		Name:     "Shadow Type",
		Source:   "let type = \"widget\"; type;",
		Expected: "widget",
	},

	// ── WHILE LET ────────────────────────────────────────────────────────
	// The value is re-evaluated each pass, which is what lets the body
	// advance a cursor. A case that binds once would hang here rather than
	// fail, so these run in both engines to keep that honest.
	{
		Name:     "While Let Some",
		Source:   "let xs = [Some(1), Some(2), None]; let t = 0; let i = 0; while let Some(x) = xs[i] { t = t + x; i = i + 1; } t;",
		Expected: int64(3),
	},
	{
		Name:     "While Let Ok Result",
		Source:   "let rs = [Ok(1), Ok(2), Err(\"e\")]; let s = 0; let i = 0; while let Ok(v) = rs[i] { s = s + v; i = i + 1; } s;",
		Expected: int64(3),
	},
	{
		Name:     "While Let Empty Stops Immediately",
		Source:   "let xs = [None]; let n = 0; let i = 0; while let Some(x) = xs[i] { n = n + 1; i = i + 1; } n;",
		Expected: int64(0),
	},
	{
		Name:     "While Let With Break",
		Source:   "let xs = [Some(1), Some(2), Some(3)]; let n = 0; let i = 0; while let Some(x) = xs[i] { if (x == 2) { break; } n = n + 1; i = i + 1; } n;",
		Expected: int64(1),
	},
	{
		Name:     "While Let With Continue",
		Source:   "let xs = [Some(1), Some(2), Some(3)]; let n = 0; let i = 0; while let Some(x) = xs[i] { i = i + 1; if (x == 2) { continue; } n = n + 1; } n;",
		Expected: int64(2),
	},
	{
		Name:     "While Let Enum Pattern",
		Source:   "enum S { C(Int), Q } let xs = [C(1), C(2), Q]; let t = 0; let i = 0; while let C(r) = xs[i] { t = t + r; i = i + 1; } t;",
		Expected: int64(3),
	},

	// ── PURE, UNSAFE, REQUIRES ───────────────────────────────────────────
	// The modifiers are soft keywords, so a variable of the same name still
	// works — including inside an interpolation, which is where the parser
	// first read one as a modifier.
	{
		Name:     "Pure Function",
		Source:   "pure fn double(x: Int) -> Int { return x * 2; } double(21);",
		Expected: int64(42),
	},
	{
		Name:     "Shadow Pure",
		Source:   "let pure = 1; let unsafe = 2; let requires = 3; \"\\(pure) \\(unsafe) \\(requires)\";",
		Expected: "1 2 3",
	},
	{
		Name:     "Function Modifier Keywords Are Soft",
		Source:   "pure fn double(x: Int) -> Int { return x * 2; } let pure = 3; let unsafe = 4; let requires = 5; double(pure) + unsafe + requires;",
		Expected: int64(15),
	},

	// ── TRAITS ───────────────────────────────────────────────────────────
	// An impl lowers to ordinary functions and a rewritten call, so a trait
	// reaches both engines through exactly the path a plain function takes.
	// A divergence here would mean the lowering does not do what it claims.
	{
		Name:     "Trait Method Call",
		Source:   "trait Greeter { fn greet(self) -> String } struct Dog { name: String } impl Greeter for Dog { fn greet(self) -> String { return \"Woof\"; } } Dog { name: \"Rex\" }.greet();",
		Expected: "Woof",
	},
	{
		Name:     "Two Types One Trait",
		Source:   "trait Greeter { fn greet(self) -> String } struct Dog { name: String } struct Cat { name: String } impl Greeter for Dog { fn greet(self) -> String { return \"Woof\"; } } impl Greeter for Cat { fn greet(self) -> String { return \"Meow\"; } } Dog { name: \"R\" }.greet() + Cat { name: \"T\" }.greet();",
		Expected: "WoofMeow",
	},
	{
		Name:     "Method Uses Self",
		Source:   "trait Greeter { fn greet(self) -> String } struct Dog { name: String } impl Greeter for Dog { fn greet(self) -> String { return self.name; } } Dog { name: \"Rex\" }.greet();",
		Expected: "Rex",
	},
	{
		Name:     "Method Calls Method",
		Source:   "trait Chain { fn first(self) -> Int; fn twice(self) -> Int } struct N { v: Int } impl Chain for N { fn first(self) -> Int { return self.v; } fn twice(self) -> Int { return self.first() * 2; } } N { v: 5 }.twice();",
		Expected: int64(10),
	},
	{
		Name:     "Method Over A Loop Variable",
		Source:   "trait Sized { fn area(self) -> Int } struct Box { w: Int, h: Int } impl Sized for Box { fn area(self) -> Int { return self.w * self.h; } } let b = Box { w: 2, h: 3 }; let t = 0; for box in [b, b] { t = t + box.area(); } t;",
		Expected: int64(12),
	},
	{
		Name:     "Self Is An Ordinary Name",
		Source:   "let self = 7; self;",
		Expected: int64(7),
	},

	// ── OPTIONAL ACCESS ──────────────────────────────────────────────────
	// `??` and `?.` lower to if/else before either engine sees the program, so
	// they reach the engines through the same path `match` and `for` take.
	{
		Name:     "Coalesce Takes The Fallback",
		Source:   "let a = null; a ?? \"d\";",
		Expected: "d",
	},
	{
		Name:     "Coalesce Keeps A Present Value",
		Source:   "let a = \"v\"; a ?? \"d\";",
		Expected: "v",
	},
	{
		Name:     "Coalesce Keeps A Falsy Present Value",
		Source:   "let a = 0; a ?? 9;",
		Expected: int64(0),
	},
	{
		Name:     "Coalesce Chains Left To Right",
		Source:   "let a = null; let b = null; let c = \"x\"; a ?? b ?? c;",
		Expected: "x",
	},
	{
		Name:     "Safe Access On Null Is Null",
		Source:   "let a = null; \"\\(a?.name)\";",
		Expected: "null",
	},
	{
		Name:     "Safe Access Reads Through A Present Value",
		Source:   "struct P { name: String } let p = P { name: \"Rex\" }; p?.name;",
		Expected: "Rex",
	},
	{
		// The rule that is easy to get wrong: `?.` guards the whole chain,
		// not just the step it appears on.
		Name:     "Safe Access Guards The Whole Chain",
		Source:   "struct Inner { v: Int } let nothing = null; \"\\(nothing?.i.v)\";",
		Expected: "null",
	},
	{
		Name:     "Safe Access Then Coalesce",
		Source:   "struct P { name: String } let nothing = null; nothing?.name ?? \"none\";",
		Expected: "none",
	},
	{
		Name:     "IsNull Is A Builtin",
		Source:   "let a = null; \"\\(isNull(a)) \\(isNull(1))\";",
		Expected: "true false",
	},
	{
		Name:     "Coalesce Leaves The Ternary Alone",
		Source:   "let a = null; let n = 5; a ?? n > 3 ? \"big\" : \"small\";",
		Expected: "big",
	},

	// ── MEMBER CHAINS AND UNIONS ──────────────────────────────────────────
	// A chain used to infer as `Any`, so the backend produced the value with
	// no information about it. These confirm the values both engines reach
	// through the newly-checked path.
	{
		Name:     "Member Chain Three Deep",
		Source:   "struct A { v: Int } struct B { a: A } struct C { b: B } let c = C { b: B { a: A { v: 7 } } }; c.b.a.v;",
		Expected: int64(7),
	},
	{
		Name:     "Union Holds A Struct Member",
		Source:   "struct P { a: Int } struct Q { b: Int } let x: P | Q = P { a: 3 }; x.a;",
		Expected: int64(3),
	},
	{
		Name:     "Optional Field Holds Its Base Type",
		Source:   "struct Address { city: String } struct User { name: String, address: ?Address } let u = User { name: \"A\", address: Address { city: \"Dhaka\" } }; u?.address?.city ?? \"?\";",
		Expected: "Dhaka",
	},

	// ── FUNCTION TYPES ───────────────────────────────────────────────────
	// Parameters are contravariant and a return is covariant. Both directions
	// are asserted because an identity check accepts and rejects almost the
	// same programs, and the difference only shows up where a type sits
	// between `Any` and something concrete.
	{
		Name:     "Function Type Passed As A Value",
		Source:   "fn takes(f: fn(Int) -> Int) -> Int { return f(1); } fn one(x: Int) -> Int { return x; } takes(one);",
		Expected: int64(1),
	},
	{
		Name:     "Contravariant Parameter",
		Source:   "fn takes(f: fn(String) -> Int) -> Int { return 1; } fn wide(x: Any) -> Int { return 1; } takes(wide);",
		Expected: int64(1),
	},
	{
		Name:     "Covariant Return",
		Source:   "fn takes(f: fn(Int) -> Any) -> Int { return 1; } fn narrow(x: Int) -> Int { return x; } takes(narrow);",
		Expected: int64(1),
	},
	{
		Name:     "Higher Order Composition",
		Source:   "fn twice(f: fn(Int) -> Int) -> Int { return f(f(1)); } let g = fn(x: Int) -> Int { return x * 2; }; twice(g);",
		Expected: int64(4),
	},
	{
		Name:     "Generic As A Higher Order Argument",
		Source:   "fn apply(f: fn(Int) -> Int, x: Int) -> Int { return f(x); } fn id<T>(v: T) -> T { return v; } apply(id, 5);",
		Expected: int64(5),
	},

	// ── NUMERIC WIDTHS AND NARROWING ─────────────────────────────────────
	// A width is a real type now, so these reach both engines through an
	// ordinary declaration rather than being erased to `Int`.
	{
		Name:     "Unsigned Width Holds Its Value",
		Source:   "let x: u8 = 200; x;",
		Expected: int64(200),
	},
	{
		Name:     "Width Survives Arithmetic",
		Source:   "let x: u8 = 5; let y: u8 = x * 2; y;",
		Expected: int64(10),
	},
	{
		Name:     "Signed Width At Its Minimum",
		Source:   "let x: i8 = -128; x;",
		Expected: int64(-128),
	},
	{
		Name:     "Result Payload Narrows",
		Source:   "unwrap(Ok(5));",
		Expected: int64(5),
	},
	{
		Name:     "Optional Payload Narrows",
		Source:   "let r: ?Int = Some(7); unwrap(r);",
		Expected: int64(7),
	},

	// ── GENERICS ─────────────────────────────────────────────────────────
	// Erased: `id` is compiled once and a call site passes ordinary values, so
	// both engines reach it through the plain call path.
	{
		Name:     "Generic Identity",
		Source:   "fn id<T>(x: T) -> T { return x; } id(5);",
		Expected: int64(5),
	},
	{
		Name:     "Generic Over A Container",
		Source:   "fn size<T>(items: Array<T>) -> Int { return len(items); } size([1, 2, 3]);",
		Expected: int64(3),
	},
	{
		Name:     "Generic Instantiated At Two Types",
		Source:   "fn size<T>(items: Array<T>) -> Int { return len(items); } size([1, 2]) + size([\"a\"]);",
		Expected: int64(3),
	},
	{
		Name:     "Two Type Parameters",
		Source:   "fn pair<A, B>(a: A, b: B) -> String { return \"\\(a)-\\(b)\"; } pair(1, \"x\");",
		Expected: "1-x",
	},
	{
		// Two generics nested: the outer one's `T` must survive the inner
		// one's scope, which is what restoring the parameter set rather than
		// clearing it is for.
		Name:     "One Generic Calling Another",
		Source:   "fn id<T>(x: T) -> T { return x; } fn twice<T>(x: T) -> Array<T> { return [id(x), id(x)]; } twice(\"a\");",
		Expected: "[a, a]",
	},

	// ── RUNTIME ERROR PROPAGATION ─────────────────────────────────────────
	{
		// Unbounded recursion must report the same category from both
		// engines. This is the case that forced the evaluator to gain a call
		// depth bound: it previously recursed in Go until the process died,
		// while the VM reported FrameOverflow cleanly.
		Name:           "Unbounded Recursion Overflows Both Engines",
		Source:         `fn loop(n) { return loop(n + 1); } loop(0);`,
		ExpectError:    "call frame depth exceeded",
		ExpectCategory: object.CategoryFrameOverflow,
	},
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
		Name:           "Call Non Function Error",
		Source:         `let x = 42; x();`,
		ExpectError:    "non-function",
		ExpectCategory: object.CategoryTypeError,
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
		Name:           "Array Index Assignment Out Of Bounds Errors",
		Source:         `let a = [1, 2, 3]; a[5] = 9;`,
		ExpectError:    "index out of bounds",
		ExpectCategory: object.CategoryIndexOutOfBounds,
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
		// Evaluator and VM return 3: the inner `let x = 2` shadows the
		// outer binding (SEMANTICS.md §3.1), so result is 2 and x stays 1.
		Expected: int64(3),
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
		Name: "While Loop Increment With Accumulation",
		Source: `
			let i = 0;
			let total = 0;
			while (i < 5) {
				total = total + i;
				i = i + 1;
			}
			total;
			`,
		Expected: int64(10),
	},

	// ── U2 REACTIVITY: PORTABLE SUBSET (both engines) ─────────────────────
	// These cases use only value-level builtins (no Go-side closure
	// invocation), so the evaluator and the VM must agree exactly.
	// Closure-invoking forms (computed/effect/batch with fn) are covered in
	// compiler/evaluator/reactive_builtins_test.go for the evaluator engine;
	// in the VM they report a diagnostic because a builtin cannot execute
	// VM bytecode without a VM handle (see reactive_builtins.go header).
	{
		Name: "Reactive Signal Get Set",
		Source: `
			let s = signal(10);
			sset(s, 25);
			sget(s);
			`,
		Expected: int64(25),
	},
	{
		Name: "Reactive Signal Version Monotonic",
		Source: `
			let s = signal(0);
			let v0 = sver(s);
			sset(s, 1);
			sset(s, 2);
			sver(s) - v0;
			`,
		Expected: int64(2),
	},
	{
		Name: "Reactive Array Push Len Get",
		Source: `
			let a = reactiveArray(1, 2, 3);
			ra_push(a, 4);
			ra_len(a) * 100 + ra_get(a, 0) * 10 + ra_get(a, 3);
			`,
		Expected: int64(414),
	},
	{
		Name: "Reactive Array Set",
		Source: `
			let a = reactiveArray(1, 2);
			ra_set(a, 1, 9);
			ra_get(a, 0) + ra_get(a, 1);
			`,
		Expected: int64(10),
	},
	{
		Name: "Reactive Store Get Set",
		Source: `
			let st = store({"k": 1});
			store_set(st, "k", 41);
			store_get(st, "k") + 1;
			`,
		Expected: int64(42),
	},
	{
		Name: "Reactive Explicit Batch Groups",
		Source: `
			let s = signal(0);
			batch_begin();
			sset(s, 1);
			sset(s, 2);
			batch_end();
			sget(s) + sver(s);
			`,
		Expected: int64(4),
	},
}

// TestDualEngineConformance runs every case through the interpreter and the virtual
// machine and checks them against the same expectations.
//
// Both engines are checked against each other *and* against the expected value,
// which is not redundant: a pair of engines that agree with each other and not with
// the suite is a pair of engines that are both wrong in the same way, and only the
// expected value catches that.
func TestDualEngineConformance(t *testing.T) {
	for _, tc := range conformanceCases {
		t.Run(tc.Name, func(t *testing.T) {
			// Step 1+2: the frontend gate - parse, type check, and lower.
			//
			// Routing through compiler/frontend rather than hand-rolling
			// lex -> parse -> check is what makes these cases exercise the
			// same path as the CLI. It also means the lowering pass runs
			// here, so a for loop reaches both engines in its lowered form.
			res := frontend.Check(tc.Source, frontend.Options{})
			if len(res.ParseErrors) > 0 {
				t.Fatalf("[%s] Parser errors: %v", tc.Name, res.ParseErrors)
			}
			if !res.OK && tc.ExpectError == "" {
				t.Fatalf("[%s] Gate rejected valid program: %s", tc.Name, res.ErrSummary())
			}
			// The engines execute the lowered program; tooling would use res.Program.
			program := res.Lowered

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
				// Both engines must produce an error.
				//
				// The evaluator returns its failure as an Object and the VM as a
				// Go error, so "did it fail" has to be asked of each in its own
				// shape. ERROR_MODEL.md §4 goes further and requires the same
				// category from both, which is asserted below whenever the test
				// declares an expected category.
				evalErrorMsg := errorMessageOf(evalObj)
				if evalErrorMsg == "" {
					t.Errorf("[%s] Evaluator did not produce expected error %q. Got=%v",
						tc.Name, tc.ExpectError, evalObj)
				}
				if compErr == nil && vmErr == nil {
					t.Errorf("[%s] VM did not produce expected error %q. Got=%v",
						tc.Name, tc.ExpectError, vmObj)
				}

				// Cross-engine category agreement: the point of ERROR_MODEL §4.
				if tc.ExpectCategory != "" {
					evalCategory, evalHasCategory := object.CategoryOf(evalObj)
					if !evalHasCategory {
						t.Errorf("[%s] Evaluator error carries no category, want %q. Got=%v",
							tc.Name, tc.ExpectCategory, evalObj)
					} else if evalCategory != tc.ExpectCategory {
						t.Errorf("[%s] Evaluator category = %q, want %q",
							tc.Name, evalCategory, tc.ExpectCategory)
					}

					if vmErr != nil {
						vmCategory, vmHasCategory := object.CategoryOf(vmErr)
						if !vmHasCategory {
							t.Errorf("[%s] VM error carries no category, want %q. Got=%v",
								tc.Name, tc.ExpectCategory, vmErr)
						} else if vmCategory != tc.ExpectCategory {
							t.Errorf("[%s] VM category = %q, want %q",
								tc.Name, vmCategory, tc.ExpectCategory)
						}
					}
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
			if runtimeErr, ok := evalObj.(*object.RuntimeError); ok {
				t.Fatalf("[%s] Evaluator unexpected runtime error [%s]: %s",
					tc.Name, runtimeErr.Category, runtimeErr.Message)
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

		// Function modifiers. These are checker diagnostics rather than runtime
		// failures: the point of `pure` and `requires` is that the mistake is
		// reported before the program runs, so a conformance case that let it run
		// would be testing the wrong thing.
		{
			name:         "Pure Rejects Effectful Call",
			source:       `fn impure() -> Int { return 1; } pure fn bad() -> Int { return impure(); } bad();`,
			expectedCode: "E0202",
		},
		{
			name:         "Requires Without Grant Fails",
			source:       `fn f(p: String) -> String requires "not_a_capability" { return p; } f("x");`,
			expectedCode: "E0201",
		},
		{
			name:         "Unsafe Needs The Unsafe Capability",
			source:       `unsafe fn raw(p: String) -> String { return p; } raw("x");`,
			expectedCode: "E0201",
		},
		{
			name:         "Pure Cannot Require A Capability",
			source:       `fn f() -> Int requires "filesystem" { return 1; } f();`,
			expectedCode: "E0201",
		},

		// Traits. A method call resolves against the receiver's static type, so
		// every way of getting that wrong is a diagnostic at the call rather than
		// a failure somewhere later.
		{
			name: "Impl Missing A Declared Method",
			source: `trait Greeter { fn greet(self) -> String; fn farewell(self) -> String }
				struct Dog { name: String }
				impl Greeter for Dog { fn greet(self) -> String { return "Woof"; } }`,
			expectedCode: "E0601",
		},
		{
			name: "Impl Adds An Undeclared Method",
			source: `trait Greeter { fn greet(self) -> String }
				struct Dog { name: String }
				impl Greeter for Dog {
					fn greet(self) -> String { return "Woof"; }
					fn fly(self) -> String { return "no"; }
				}`,
			expectedCode: "E0602",
		},
		{
			name: "Call On A Type With No Impl",
			source: `trait Greeter { fn greet(self) -> String }
				struct Dog { name: String }
				let d = Dog { name: "Rex" };
				let result = d.greet();`,
			expectedCode: "E0603",
		},
		{
			name: "Impl Names An Undeclared Trait",
			source: `struct Dog { name: String }
				impl Greeter for Dog { fn greet(self) -> String { return "Woof"; } }`,
			expectedCode: "E0603",
		},

		// Type system completion. Each of these was accepted or reported for the
		// wrong reason before the rule it tests existed.
		{
			name:         "Unknown Member In A Chain",
			source:       `struct Inner { v: Int } struct Outer { i: Inner } let o = Outer { i: Inner { v: 1 } }; let r = o.i.nosuch;`,
			expectedCode: "E0111",
		},
		{
			name:         "Unknown Annotation Is Not A Mismatch",
			source:       `let count: Missing = 0;`,
			expectedCode: "E0502",
		},
		{
			name:         "Union Member That Is Not A Type",
			source:       `let x: Int | Nope = 5;`,
			expectedCode: "E0502",
		},
		{
			name:         "Value In Neither Union Member",
			source:       `let x: Int | String = true;`,
			expectedCode: "E0101",
		},

		// Generics. A structural mismatch is reported against the *instantiated*
		// signature, which is a different code from a plain type mismatch: the two
		// are reached by different reasoning and a developer looking at one wants
		// to know which happened.
		{
			name:         "Generic Structural Mismatch",
			source:       `fn size<T>(items: Array<T>) -> Int { return len(items); } let r = size(5);`,
			expectedCode: "E0204",
		},

		// Narrowing and widths. Each of these was accepted or reported for the
		// wrong reason before the rule it tests existed.
		{
			name:         "Literal Above A Width",
			source:       `let x: u8 = 300;`,
			expectedCode: "E0110",
		},
		{
			name:         "Int Is Not A Width",
			source:       `let a: Int = 5; let x: u8 = a;`,
			expectedCode: "E0101",
		},
		{
			name:         "Unwrap Of A Non Narrowable",
			source:       `let x = unwrap(5);`,
			expectedCode: "E0112",
		},
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
