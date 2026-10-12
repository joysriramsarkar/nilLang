package gc

import (
	"sync"

	"github.com/joysriramsarkar/nilLang/compiler/object"
)

// Heap couples a Collector with the root set it is collected against.
//
// A Collector on its own cannot know what is reachable: marking needs roots, and
// roots are the live VM stacks, globals, frames, tasks, and component trees
// (MEMORY_MODEL.md §2). This type is the piece that was missing: nothing
// constructed a Collector outside its own test, so no execution path was ever
// collected.
//
// The collector remains conservative. It only reclaims bookkeeping entries; it
// does not free memory that Go's own allocator owns. Its purpose is to make the
// reachability model executable and observable, not to replace the Go runtime's
// GC.
type Heap struct {
	mu sync.Mutex

	collector *Collector

	// roots holds the live references for the current collection cycle. Callers
	// register and deregister as execution moves between containers, which is
	// what lets a collection happen at an arbitrary allocation point.
	roots []object.Object

	// collectionDisabled suppresses collection while a caller is mid-update.
	// Sweeping a root set that is being mutated concurrently would mark an
	// object unreachable at the moment it is being linked into a structure.
	collectionDisabled bool

	// lastReclaimed is the number of objects reclaimed by the most recent
	// collection, for diagnostics.
	lastReclaimed int
}

// NewHeap creates a heap whose collector triggers after threshold tracked
// objects. A threshold of zero or less uses the collector default.
func NewHeap(threshold int) *Heap {
	return &Heap{collector: NewCollector(threshold)}
}

// Collector exposes the underlying collector for statistics.
func (h *Heap) Collector() *Collector {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.collector
}

// Track registers an allocation with the heap.
//
// Allocation is the natural point to collect: the program has just created an
// object and has not yet had a chance to drop its reference, so a collection
// here is safe and the trigger is self-regulating rather than needing a timer.
func (h *Heap) Track(obj object.Object) {
	if obj == nil {
		return
	}
	h.mu.Lock()
	h.collector.TrackLocked(obj)
	shouldCollect := !h.collectionDisabled && h.collector.ObjectCountLocked() > h.collector.Threshold
	roots := append([]object.Object(nil), h.roots...)
	h.mu.Unlock()

	if shouldCollect {
		h.collectWithRoots(roots)
	}
}

// AddRoot registers a container's contents as GC roots.
//
// The VM calls this for its operand stack, globals, and frames; a host calls it
// for component trees and task state. Passing the individual objects rather
// than a container keeps this package free of a dependency on the VM, which
// tests/architecture_test.go forbids.
func (h *Heap) AddRoot(objs ...object.Object) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, obj := range objs {
		if obj == nil {
			continue
		}
		h.collector.TrackLocked(obj)
		h.roots = append(h.roots, obj)
	}
}

// RemoveRoot unregisters a root object, typically when a container is popped.
//
// Leaving a popped frame registered as a root would pin everything it referenced
// for the process lifetime, which is a leak the collector could not fix.
func (h *Heap) RemoveRoot(target object.Object) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, obj := range h.roots {
		if obj == target {
			h.roots = append(h.roots[:i], h.roots[i+1:]...)
			return true
		}
	}
	return false
}

// SetRoots replaces the entire root set, which is what a VM does before
// collecting: it rebuilds the set from live stacks and frames each cycle.
func (h *Heap) SetRoots(objs []object.Object) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.roots = h.roots[:0]
	for _, obj := range objs {
		if obj == nil {
			continue
		}
		h.collector.TrackLocked(obj)
		h.roots = append(h.roots, obj)
	}
}

// Collect runs a mark-and-sweep cycle against the current root set and returns
// the number of objects reclaimed.
func (h *Heap) Collect() int {
	h.mu.Lock()
	roots := append([]object.Object(nil), h.roots...)
	h.mu.Unlock()
	return h.collectWithRoots(roots)
}

func (h *Heap) collectWithRoots(roots []object.Object) int {
	h.mu.Lock()
	defer h.mu.Unlock()

	reclaimed := h.collector.CollectLocked(roots)
	h.lastReclaimed = reclaimed
	return reclaimed
}

// DisableCollection suspends collection until the returned function is called.
//
// A caller that is in the middle of linking a new object into a structure must
// not have a collection run: the object would be unreachable from the root set
// at that instant and would be swept while still referenced.
func (h *Heap) DisableCollection() func() {
	h.mu.Lock()
	h.collectionDisabled = true
	h.mu.Unlock()

	return func() {
		h.mu.Lock()
		h.collectionDisabled = false
		h.mu.Unlock()
	}
}

// ObjectCount returns the number of tracked objects.
func (h *Heap) ObjectCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.collector.ObjectCountLocked()
}

// RootCount returns the number of registered roots.
func (h *Heap) RootCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.roots)
}

// LastReclaimed returns the count from the most recent collection.
func (h *Heap) LastReclaimed() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastReclaimed
}

// RetainHandle anchors an object across the native boundary.
func (h *Heap) RetainHandle(obj object.Object) Handle {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.collector.RetainHandleLocked(obj)
}

// ReleaseHandle releases a native handle.
func (h *Heap) ReleaseHandle(handle Handle) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.collector.ReleaseHandleLocked(handle)
}

// GetHandle resolves a native handle.
func (h *Heap) GetHandle(handle Handle) (object.Object, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.collector.GetHandleLocked(handle)
}
