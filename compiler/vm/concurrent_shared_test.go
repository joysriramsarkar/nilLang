package vm

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/object"
)

// TestTasksWritingSharedHashDoNotCrash is the regression test for the
// shared-hash fatal error: two tasks writing one global hash ran the
// host into Go's unrecoverable "concurrent map read and map write"
// because Hash.Pairs was a plain map mutated with no lock. The program
// below is the shape that crashed 5 times out of 5 before the fix; its
// first assertion is that Run returns at all, and its second is that
// every write from both tasks is present afterwards.
func TestTasksWritingSharedHashDoNotCrash(t *testing.T) {
	source := `
let h = {};
let t1 = task { let i = 0; while (i < 3000) { h[i] = i; i = i + 1; } return 1; };
let t2 = task { let i = 0; while (i < 3000) { h[100000 + i] = i; i = i + 1; } return 2; };
await t1; await t2;
`
	machine := New(compileSource(t, source))
	if err := machine.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	bound, ok := machine.GlobalByName("h")
	if !ok {
		t.Fatal("global h is not bound after Run")
	}
	hash, ok := bound.(*object.Hash)
	if !ok {
		t.Fatalf("global h is %T, want *object.Hash", bound)
	}
	if got, want := hash.Len(), 6000; got != want {
		t.Fatalf("shared hash holds %d pairs, want %d (writes from both tasks)", got, want)
	}
}

// TestTasksSharingCapturedCell races two tasks on one closure-captured
// variable. The increments themselves are not atomic at the language
// level — read-modify-write never is — so the final count is not
// asserted; what is asserted is that the run completes, which under
// -race also proves the cell's Get/Set synchronisation.
func TestTasksSharingCapturedCell(t *testing.T) {
	source := `
let make = fn() { let count = 0; return fn() { count = count + 1; return count; }; };
let inc = make();
let t1 = task { let i = 0; while (i < 2000) { inc(); i = i + 1; } return 1; };
let t2 = task { let i = 0; while (i < 2000) { inc(); i = i + 1; } return 2; };
await t1; await t2;
`
	machine := New(compileSource(t, source))
	if err := machine.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// TestTasksWritingSharedArrayIndices runs two tasks writing disjoint
// index ranges of one shared global array, the array half of the shared
// mutation fix.
func TestTasksWritingSharedArrayIndices(t *testing.T) {
	source := `
let a = [0, 0, 0, 0, 0, 0, 0, 0];
let t1 = task { let i = 0; while (i < 4) { a[i] = 1; i = i + 1; } return 1; };
let t2 = task { let i = 4; while (i < 8) { a[i] = 2; i = i + 1; } return 2; };
await t1; await t2;
`
	machine := New(compileSource(t, source))
	if err := machine.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	bound, ok := machine.GlobalByName("a")
	if !ok {
		t.Fatal("global a is not bound after Run")
	}
	arr, ok := bound.(*object.Array)
	if !ok {
		t.Fatalf("global a is %T, want *object.Array", bound)
	}
	want := []int64{1, 1, 1, 1, 2, 2, 2, 2}
	for i, w := range want {
		element, ok := arr.Get(int64(i))
		if !ok {
			t.Fatalf("a[%d] missing", i)
		}
		if got := element.(*object.Integer).Value; got != w {
			t.Errorf("a[%d] = %d, want %d", i, got, w)
		}
	}
}
