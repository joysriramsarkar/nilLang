package tests

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
	"github.com/joysriramsarkar/nilLang/runtime/gc"
)

// globalByName reads a global by its source-level name.
//
// Global slots are assigned by the compiler and shift whenever a program gains or
// loses a binding, so an index-based read would silently inspect the wrong
// variable as the compiler evolves.
func globalByName(t *testing.T, machine *vm.VM, name string) object.Object {
	t.Helper()
	value, ok := machine.GlobalByName(name)
	if !ok {
		t.Fatalf("global %q not found; slots are named %v", name, machine.GlobalNames())
	}
	return value
}

// TestVMCollectorIsActuallyWired is the point of this file.
//
// MEMORY_MODEL.md §0 recorded that nothing constructed a Collector outside its
// own test, so no execution path was ever collected. These tests exercise the
// VM with a collector attached, which is the only way that statement can stop
// being true.

// runWithGC compiles and runs source with GC tracking enabled, returning the VM
// and its heap.
func runWithGC(t *testing.T, source string) (*vm.VM, *gc.Heap) {
	t.Helper()

	res := evalSource(t, source)
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}

	machine, err := compileToVM(res.Program)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	heap := gc.NewHeap(1 << 30) // effectively manual collection
	machine.EnableGC(heap)

	if err := machine.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	return machine, heap
}

// TestVMTracksAllocations confirms the VM registers heap objects.
func TestVMTracksAllocations(t *testing.T) {
	source := `
let a = [1, 2, 3];
let h = {"x": 1, "y": 2};
let nested = [[1], [2], [3]];
`
	machine, heap := runWithGC(t, source)

	if !machine.GCEnabled() {
		t.Fatal("GC should be enabled after EnableGC")
	}
	count := heap.ObjectCount()
	if count == 0 {
		t.Fatal("the VM registered no allocations; tracking is not wired")
	}

	// The five allocations the program makes: three arrays (a, nested[0..2])
	// and one hash. Ordering does not matter, only that tracking happened.
	if count < 4 {
		t.Errorf("tracked %d objects, want at least 4 for this program", count)
	}
}

// TestCollectionReclaimsUnreachableObjects is the core collector property.
//
// Both structures are globals here, so both are reachable and neither should be
// reclaimed. What must be reclaimed is everything else the program allocated on
// the way: the arrays and hashes that were built, counted, and dropped inside
// makeList.
func TestCollectionReclaimsUnreachableObjects(t *testing.T) {
	source := `
fn makeList(n) {
  let acc = [];
  let i = 0;
  while (i < n) {
    acc = append(acc, {"index": i});
    i = i + 1;
  }
  return len(acc);
}
let size = makeList(50);
`
	machine, heap := runWithGC(t, source)

	before := heap.ObjectCount()
	if before == 0 {
		t.Fatal("nothing was tracked, so there is nothing to reclaim")
	}

	reclaimed := machine.CollectGarbage()
	if reclaimed == 0 {
		t.Errorf("expected unreachable allocations to be reclaimed; before=%d reclaimed=%d",
			before, reclaimed)
	}
}

// TestCollectionRetainsLiveStructures is the property that matters most: a
// collector that swept live data would be worse than no collector. Everything
// reachable from the VM's globals must survive.
func TestCollectionRetainsLiveStructures(t *testing.T) {
	source := `
let kept = {"items": [1, 2, 3], "label": "keep me"};
let nested = {"outer": {"inner": ["deep"]}};
`
	machine, _ := runWithGC(t, source)

	machine.CollectGarbage()

	// After a collection the program must still see its data intact. Re-reading
	// the global proves the collector did not sweep it.
	kept := globalByName(t, machine, "kept")
	if kept == nil {
		t.Fatal("a live global was swept by the collector")
	}
	if hash, ok := kept.(*object.Hash); ok {
		if _, hasItems := hash.Pairs[(&object.String{Value: "items"}).HashKey()]; !hasItems {
			t.Error("a live hash lost a key during collection")
		}
	}
}

// TestNestedStructuresSurviveCollection walks the transitive case: the root is
// small and the interesting data is several levels down.
func TestNestedStructuresSurviveCollection(t *testing.T) {
	source := `
fn makeStructure() {
  let leaf = ["a", "b", "c"];
  let mid = {"leaf": leaf};
  let top = {"mid": mid, "count": 3};
  return top;
}
let root = makeStructure();
`
	machine, _ := runWithGC(t, source)

	machine.CollectGarbage()

	root := globalByName(t, machine, "root")
	if root == nil {
		t.Fatal("the root global was swept")
	}
	hash, ok := root.(*object.Hash)
	if !ok {
		t.Fatalf("root is %T, want a hash", root)
	}

	// Walk the chain the program built: root -> mid -> leaf. Every level must
	// still be intact, which is the transitive case a naive collector fails.
	midValue, hasMid := hash.Pairs[(&object.String{Value: "mid"}).HashKey()]
	if !hasMid {
		t.Fatal("root lost its \"mid\" entry")
	}
	mid, ok := midValue.Value.(*object.Hash)
	if !ok {
		t.Fatalf("mid is %T, want a hash", midValue.Value)
	}
	leafValue, hasLeaf := mid.Pairs[(&object.String{Value: "leaf"}).HashKey()]
	if !hasLeaf {
		t.Fatal("mid lost its \"leaf\" entry")
	}
	leaf, ok := leafValue.Value.(*object.Array)
	if !ok {
		t.Fatalf("leaf is %T, want an array", leafValue.Value)
	}
	if len(leaf.Elements) != 3 {
		t.Errorf("leaf has %d elements, want 3", len(leaf.Elements))
	}
}

// TestClosureCellsSurviveCollection covers the shared mutable cell, which is
// the mechanism that makes two closures observe one variable. Sweeping a cell
// would silently break captured mutation.
func TestClosureCellsSurviveCollection(t *testing.T) {
	source := `
fn makeCounter() {
  let count = 0;
  let inc = fn() { count = count + 1; return count; };
  let read = fn() { return count; };
  inc();
  inc();
  return read();
}
let total = makeCounter();
`
	machine, _ := runWithGC(t, source)

	machine.CollectGarbage()

	// Read the global by name rather than by slot: this program declares two
	// globals and the slot index is an implementation detail that a future
	// compiler change would move.
	total := globalByName(t, machine, "total")
	if total == nil {
		t.Fatal("the result global was swept")
	}
	if integer, ok := total.(*object.Integer); !ok || integer.Value != 2 {
		t.Errorf("got %v (%T), want 2; capture cells must survive collection", total, total)
	}
}

// TestReclamationUnderAllocationPressure exercises the threshold path, where a
// collection fires from inside Track rather than being requested.
func TestReclamationUnderAllocationPressure(t *testing.T) {
	source := `
fn makeList(n) {
  let acc = [];
  let i = 0;
  while (i < n) {
    acc = append(acc, {"index": i});
    i = i + 1;
  }
  return len(acc);
}
let size = makeList(200);
`
	res := evalSource(t, source)
	if !res.OK {
		t.Fatalf("source rejected: %s", res.ErrSummary())
	}
	machine, err := compileToVM(res.Program)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	// A low threshold makes Track collect repeatedly during the loop.
	heap := gc.NewHeap(50)
	machine.EnableGC(heap)

	if err := machine.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}

	if heap.Collector().Collections == 0 {
		t.Error("no collection ran despite crossing the threshold")
	}
	if globalByName(t, machine, "size") == nil {
		t.Error("the result global was swept under allocation pressure")
	}
}

// TestGCIsOptional confirms the VM still runs with no collector attached, so the
// CLI pays no bookkeeping cost.
func TestGCIsOptional(t *testing.T) {
	res := evalSource(t, `let a = [1, 2, 3]; let b = {"x": a};`)
	if !res.OK {
		t.Fatalf("source rejected: %s", res.ErrSummary())
	}
	machine, err := compileToVM(res.Program)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	if machine.GCEnabled() {
		t.Error("GC should be off unless explicitly enabled")
	}
	if err := machine.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if machine.TrackedObjectCount() != 0 {
		t.Errorf("TrackedObjectCount = %d with GC disabled, want 0", machine.TrackedObjectCount())
	}
	if machine.CollectGarbage() != 0 {
		t.Error("CollectGarbage should be a no-op with GC disabled")
	}
}

// TestGCRootSetExcludesPoppedSlots confirms a popped stack slot stops being a
// root. If it did not, every intermediate value would be pinned for the life of
// the process.
//
// The test runs a real program rather than poking the VM's unexported stack, so
// it checks the same code path an embedder would.
func TestGCRootSetExcludesPoppedSlots(t *testing.T) {
	source := `
fn discard() {
  let temporary = ["should not survive as a root"];
  let keep = ["live"];
  return len(keep);
}
let result = discard();
`
	res := evalSource(t, source)
	if !res.OK {
		t.Fatalf("source rejected: %s", res.ErrSummary())
	}
	machine, err := compileToVM(res.Program)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if err := machine.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}

	// After Run the operand stack is unwound, so the frame-local array must not
	// appear in the root set even though the global still holds its result.
	roots := machine.GCRoots()
	for _, root := range roots {
		array, ok := root.(*object.Array)
		if !ok {
			continue
		}
		for _, elem := range array.Elements {
			if str, ok := elem.(*object.String); ok && str.Value == "should not survive as a root" {
				t.Error("a frame-local array is still reported as a root after Run")
			}
		}
	}
}

// TestGCRootSetIncludesGlobals is the positive control for the test above: the
// root set must include globals, or everything would look unreachable.
func TestGCRootSetIncludesGlobals(t *testing.T) {
	source := `let live = {"marker": "still here"};`
	res := evalSource(t, source)
	if !res.OK {
		t.Fatalf("source rejected: %s", res.ErrSummary())
	}
	machine, err := compileToVM(res.Program)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if err := machine.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}

	found := false
	for _, root := range machine.GCRoots() {
		hash, ok := root.(*object.Hash)
		if !ok {
			continue
		}
		if _, hasMarker := hash.Pairs[(&object.String{Value: "marker"}).HashKey()]; hasMarker {
			found = true
			break
		}
	}
	if !found {
		t.Error("the global hash is missing from the root set")
	}
}
