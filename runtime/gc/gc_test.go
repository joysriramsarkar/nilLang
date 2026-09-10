package gc

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/object"
)

func TestGCTrackingAndSweep(t *testing.T) {
	collector := NewCollector(10)

	obj1 := &object.Integer{Value: 42}
	obj2 := &object.String{Value: "hello"}
	unreachableObj := &object.Integer{Value: 999}

	collector.Track(obj1)
	collector.Track(obj2)
	collector.Track(unreachableObj)

	if collector.ObjectCount() != 3 {
		t.Fatalf("expected 3 tracked objects, got %d", collector.ObjectCount())
	}

	// Roots only contain obj1
	roots := []object.Object{obj1}
	reclaimed := collector.Collect(roots)

	if reclaimed != 2 {
		t.Fatalf("expected 2 reclaimed objects, got %d", reclaimed)
	}

	if collector.ObjectCount() != 1 {
		t.Fatalf("expected 1 live object, got %d", collector.ObjectCount())
	}
}

func TestGCCyclicReferenceReclamation(t *testing.T) {
	collector := NewCollector(10)

	// Create a cycle: arr1 contains arr2, arr2 contains arr1
	arr1 := &object.Array{Elements: []object.Object{}}
	arr2 := &object.Array{Elements: []object.Object{arr1}}
	arr1.Elements = append(arr1.Elements, arr2)

	collector.Track(arr1)
	collector.Track(arr2)

	if collector.ObjectCount() != 2 {
		t.Fatalf("expected 2 objects before GC, got %d", collector.ObjectCount())
	}

	// Roots: Empty (cycle is isolated from roots)
	reclaimed := collector.Collect([]object.Object{})

	if reclaimed != 2 {
		t.Fatalf("expected cyclic reference to be fully collected, reclaimed %d", reclaimed)
	}

	if collector.ObjectCount() != 0 {
		t.Fatalf("expected 0 objects remaining, got %d", collector.ObjectCount())
	}
}

func TestGCNativeHandleRetention(t *testing.T) {
	collector := NewCollector(10)

	nativeObj := &object.String{Value: "anchored across FFI"}
	handle := collector.RetainHandle(nativeObj)

	// Run GC with no regular roots
	reclaimed := collector.Collect([]object.Object{})
	if reclaimed != 0 {
		t.Fatalf("native handle was erroneously collected: %d", reclaimed)
	}

	resolved, ok := collector.GetHandle(handle)
	if !ok || resolved != nativeObj {
		t.Fatalf("failed to retrieve anchored handle object")
	}

	// Release handle and collect
	collector.ReleaseHandle(handle)
	reclaimedAfterRelease := collector.Collect([]object.Object{})

	if reclaimedAfterRelease != 1 {
		t.Fatalf("expected 1 object reclaimed after handle release, got %d", reclaimedAfterRelease)
	}
}

func TestGCLargeRingCycleReclamation(t *testing.T) {
	collector := NewCollector(50)

	const ringSize = 10
	nodes := make([]*object.Array, ringSize)
	for i := 0; i < ringSize; i++ {
		nodes[i] = &object.Array{Elements: []object.Object{}}
		collector.Track(nodes[i])
	}

	// Link in a ring: 0 -> 1 -> 2 -> ... -> 9 -> 0
	for i := 0; i < ringSize; i++ {
		nextIndex := (i + 1) % ringSize
		nodes[i].Elements = append(nodes[i].Elements, nodes[nextIndex])
	}

	if collector.ObjectCount() != ringSize {
		t.Fatalf("expected %d tracked ring nodes, got %d", ringSize, collector.ObjectCount())
	}

	// 1. When root contains nodes[0], entire ring is reachable and preserved
	reclaimed := collector.Collect([]object.Object{nodes[0]})
	if reclaimed != 0 {
		t.Fatalf("expected 0 reclaimed when ring root is live, got %d", reclaimed)
	}
	if collector.ObjectCount() != ringSize {
		t.Fatalf("expected %d live objects in ring, got %d", ringSize, collector.ObjectCount())
	}

	// 2. When roots are cleared, entire 10-node cycle is reclaimed
	reclaimedAll := collector.Collect([]object.Object{})
	if reclaimedAll != ringSize {
		t.Fatalf("expected all %d ring nodes to be collected, got %d", ringSize, reclaimedAll)
	}
	if collector.ObjectCount() != 0 {
		t.Fatalf("expected 0 remaining objects, got %d", collector.ObjectCount())
	}
}
