package mir

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
)

// TestAFunctionBodyIsItsLastStatement is the rule that a function's value is
// whatever its body's last statement produced.
//
// The evaluator says so — `evalBlockStatement` returns the last statement's value
// whatever kind it is — and so does the VM, which leaves it on the stack. MIR had
// two copies of the rule, one per path that creates a function, and both returned
// nothing. `fn pick(c: Bool) -> Int { if c { 1 } else { 2 } }` therefore compiled to
// a function that always answered nil, which is a plausible-looking module and a
// wrong program.
func TestAFunctionBodyIsItsLastStatement(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "a function ending in an expression returns it",
			source: `fn pick(c: Bool) -> Int { if c { 1 } else { 2 } } pick(true);`,
			want:   "return _t",
		},
		{
			name:   "an explicit return is still the answer",
			source: `fn twice(n: Int) -> Int { return n + n; } twice(3);`,
			want:   "return _t",
		},
		{
			// A trailing `let` produces nothing, and the evaluator answers null for
			// it. Keeping the previous statement's value would make the two engines
			// disagree about the simplest program containing a declaration.
			name:   "a function ending in a let returns nothing",
			source: `fn quiet() { let x = 1; } quiet();`,
			want:   "return\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text := lowerToMIR(t, tc.source)
			if !strings.Contains(text, tc.want) {
				t.Errorf("the function does not return its body's value:\n%s", text)
			}
		})
	}
}

// TestAnIndexAssignmentReachesMIR is about a write that was silently dropped.
//
// `a[1] = 9` parses to a node the HIR lowerer had no case for, so `lowerStatement`
// fell through its switch and returned nothing — and returning nothing is what that
// switch means elsewhere, so the write disappeared between the parse tree and the
// IR. Nothing failed. The program compiled, ran, and left `a` alone; the next
// read answered the old value.
func TestAnIndexAssignmentReachesMIR(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			// The container is loaded into a temporary first, so the expression on
			// the left is evaluated once and the write reaches what it named. A
			// shape that inlined the name would evaluate it again for the read.
			name:   "a subscript write",
			source: `let a = [1, 2, 3]; a[1] = 9; a[1];`,
			want:   "[1], 9",
		},
		{
			// The parser normalises a member write into a subscript whose key is
			// the string `"name"`, so by MIR both spellings are one instruction.
			// The check is that the write survives at all — it did not, for a long
			// time, and the program that lost it still ran.
			name:   "a member write",
			source: `let m = {"name": "nil"}; m.name = "nilang"; m.name;`,
			want:   `["name"], "nilang"`,
		},
		{
			name:   "a variable write is still a store",
			source: `let a = 1; a = 2; a;`,
			want:   "store a,",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text := lowerToMIR(t, tc.source)
			if !strings.Contains(text, tc.want) {
				t.Errorf("MIR does not contain %q:\n%s", tc.want, text)
			}
		})
	}
}

// TestASubscriptWriteIsNotAVariableWrite guards the shape of the fix.
//
// A container write has to reach the container. Storing into a slot would replace
// the binding — `a[1] = 9` would make `a` the number 9 — which is right for a map
// by accident and wrong for a list always.
func TestASubscriptWriteIsNotAVariableWrite(t *testing.T) {
	text := lowerToMIR(t, `let a = [1, 2, 3]; a[1] = 9; a[1];`)

	if strings.Contains(text, "store a[") {
		t.Errorf("a subscript write was lowered into a store to a variable called \"a[\":\n%s", text)
	}
	if !strings.Contains(text, " = set ") {
		t.Errorf("a subscript write is not a SetIndexInst:\n%s", text)
	}

	// And the binding itself is untouched: `a` is still the list, not the 9.
	if !strings.Contains(text, "store a, _t0") {
		t.Errorf("the list is no longer stored under the name it was bound to:\n%s", text)
	}
}

// TestAnIfIsWorthUsing is the other thing that disappeared with the same fix.
//
// Both arms of an `if` assign the same temporary, because the `if` is *for* a
// value. Without it the arms ran, their results were dropped, and
// `if (c) { 100 } else { 200 }` — whose entire purpose is to produce one of them
// — evaluated to nothing.
func TestAnIfIsWorthUsing(t *testing.T) {
	text := lowerToMIR(t, `let c = true; if (c) { 100 } else { 200 };`)

	writes := strings.Count(text, "= 100") + strings.Count(text, "= 200")
	if writes < 2 {
		t.Errorf("the arms do not both write the merge value:\n%s", text)
	}
	if !strings.Contains(text, "bb_merge") {
		t.Errorf("there is no block the two arms meet at:\n%s", text)
	}
}

// lowerToMIR runs a source through the same path the compiler uses.
func lowerToMIR(t *testing.T, source string) string {
	t.Helper()

	checked := frontend.Check(source, frontend.Options{})
	if !checked.OK {
		t.Fatalf("the program was rejected: %s", checked.ErrSummary())
	}

	optimized := hir.NewOptimizer().Optimize(hir.NewLowerer().LowerProgram(checked.Lowered))
	return NewLowerer().LowerHIR(optimized).String()
}
