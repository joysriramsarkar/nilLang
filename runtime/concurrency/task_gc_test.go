package concurrency

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/runtime/gc"
)

// TestCollectFuturesCoversTree pins the traversal the VM's root set relies
// on: every scope's future in the tree, so an in-flight task's result stays
// reachable until awaited (MEMORY_MODEL.md §2.1 item 4).
func TestCollectFuturesCoversTree(t *testing.T) {
	root := NewTaskScope(nil)
	child := NewTaskScope(root)
	grand := NewTaskScope(child)

	arr := &object.Array{Elements: []object.Object{&object.Integer{Value: 7}}}
	grand.future.Complete(arr, nil)

	futs := root.CollectFutures()
	if len(futs) != 3 {
		t.Fatalf("CollectFutures = %d futures, want 3 (root, child, grandchild)", len(futs))
	}
	seen := map[*object.Future]bool{}
	for _, f := range futs {
		if f == nil {
			t.Fatal("CollectFutures returned a nil future")
		}
		seen[f] = true
	}
	if !seen[grand.future] {
		t.Fatal("grandchild future missing from collection")
	}

	// The completed array must survive a collection rooted at these futures
	// and fall once nothing roots it: that is the contract GCRoots relies on.
	heap := gc.NewHeap(1 << 20)
	heap.Track(arr)
	roots := make([]object.Object, 0, len(futs))
	for _, f := range futs {
		roots = append(roots, f)
	}
	heap.SetRoots(roots)
	if got := heap.Collect(); got != 0 {
		t.Fatalf("live task result was swept: reclaimed %d", got)
	}
	heap.SetRoots(nil)
	if got := heap.Collect(); got == 0 {
		t.Fatal("unrooted array was not reclaimed")
	}
}
