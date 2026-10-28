package evaluator

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/object"
)

// TestSetBuiltinReturnsOrderedCopyWithoutMutating pins the fixed `set`
// semantics: the result is a new hash whose iteration order includes the
// added key, and the argument hash is left exactly as it was. The old
// body rebuilt only the lookup table in place, so a key added with
// `set` could be looked up but never appeared in OrderedPairs, prints,
// or iteration — while silently mutating the caller's hash.
func TestSetBuiltinReturnsOrderedCopyWithoutMutating(t *testing.T) {
	evaluated := testEval(`
let h = {"a": 1, "b": 2};
let h2 = set(h, "c", 3);
let seen = "";
for k, v in h2 { seen = seen + k; };
let originalSeen = "";
for k, v in h { originalSeen = originalSeen + k; };
[h2["c"], seen, originalSeen, h["c"]];
`)
	// The program's last value is an array literal built for inspection;
	// walk it directly instead of through Inspect text so the order
	// assertions stay exact.
	arr, ok := evaluated.(*object.Array)
	if !ok {
		t.Fatalf("program evaluated to %T (%v), want *object.Array", evaluated, evaluated)
	}
	elements := arr.Snapshot()
	if len(elements) != 4 {
		t.Fatalf("result array has %d elements, want 4", len(elements))
	}
	if got := elements[0].(*object.Integer).Value; got != 3 {
		t.Errorf("h2[\"c\"] = %d, want 3", got)
	}
	if got := elements[1].(*object.String).Value; got != "abc" {
		t.Errorf("iteration over set result visits %q, want \"abc\" (insertion order incl. new key)", got)
	}
	if got := elements[2].(*object.String).Value; got != "ab" {
		t.Errorf("iteration over the original visits %q, want \"ab\" (original unmutated)", got)
	}
	if elements[3].Type() != object.NULL_OBJ {
		t.Errorf("h[\"c\"] = %v, want null (original must not gain the key)", elements[3])
	}
}

// TestSetBuiltinReplacingKeyKeepsPosition covers the other half of
// order correctness: setting an existing key replaces its value in
// place rather than moving it to the end.
func TestSetBuiltinReplacingKeyKeepsPosition(t *testing.T) {
	evaluated := testEval(`
let h = {"a": 1, "b": 2};
let h2 = set(h, "a", 9);
let seen = "";
for k, v in h2 { seen = seen + k; };
[h2["a"], seen, h["a"]];
`)
	arr, ok := evaluated.(*object.Array)
	if !ok {
		t.Fatalf("program evaluated to %T (%v), want *object.Array", evaluated, evaluated)
	}
	elements := arr.Snapshot()
	if got := elements[0].(*object.Integer).Value; got != 9 {
		t.Errorf("h2[\"a\"] = %d, want 9", got)
	}
	if got := elements[1].(*object.String).Value; got != "ab" {
		t.Errorf("iteration order after replace = %q, want \"ab\"", got)
	}
	if got := elements[2].(*object.Integer).Value; got != 1 {
		t.Errorf("h[\"a\"] = %d, want 1 (original unmutated)", got)
	}
}
