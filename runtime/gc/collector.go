package gc

import (
	"github.com/joysriramsarkar/nilLang/compiler/object"
)

// Locked-suffix methods assume the caller holds the mutex.
//
// Heap serialises everything through its own mutex and calls these directly,
// which avoids taking the same lock twice in Track's allocation path. The
// public methods below remain for callers that hold a Collector on its own.

// TrackLocked registers an allocation.
func (c *Collector) TrackLocked(obj object.Object) {
	if obj == nil {
		return
	}
	c.objects[obj] = true
}

// ObjectCountLocked returns the tracked object count.
func (c *Collector) ObjectCountLocked() int {
	return len(c.objects)
}

// RetainHandleLocked anchors an object across the native boundary.
func (c *Collector) RetainHandleLocked(obj object.Object) Handle {
	if obj == nil {
		return 0
	}

	handle := c.nextHandle
	c.nextHandle++
	c.nativeHandles[handle] = obj
	c.objects[obj] = true
	return handle
}

// ReleaseHandleLocked releases a native handle.
func (c *Collector) ReleaseHandleLocked(handle Handle) {
	delete(c.nativeHandles, handle)
}

// GetHandleLocked resolves a native handle.
func (c *Collector) GetHandleLocked(handle Handle) (object.Object, bool) {
	obj, ok := c.nativeHandles[handle]
	return obj, ok
}

// CollectLocked runs a mark-and-sweep cycle.
//
// Objects reachable from the roots, from another tracked object, or from a live
// native handle are marked; everything else is reclaimed.
func (c *Collector) CollectLocked(roots []object.Object) int {
	marked := make(map[object.Object]bool, len(c.objects))

	// Mark from the roots.
	for _, root := range roots {
		c.markLocked(root, marked)
	}

	// A native handle is a root by definition: the native side holds it and
	// cannot be asked what it points at.
	for _, nativeObj := range c.nativeHandles {
		c.markLocked(nativeObj, marked)
	}

	// Sweep.
	reclaimed := 0
	for obj := range c.objects {
		if !marked[obj] {
			delete(c.objects, obj)
			reclaimed++
		}
	}

	c.Collections++
	return reclaimed
}

// markLocked transitively marks an object and everything it references.
//
// An object already in the marked set short-circuits, which is what keeps a
// cyclic structure from causing infinite recursion.
func (c *Collector) markLocked(obj object.Object, marked map[object.Object]bool) {
	if obj == nil || marked[obj] {
		return
	}
	marked[obj] = true

	switch typed := obj.(type) {
	case *object.Array:
		for _, elem := range typed.Elements {
			c.markLocked(elem, marked)
		}

	case *object.Hash:
		for _, pair := range typed.Pairs {
			c.markLocked(pair.Key, marked)
			c.markLocked(pair.Value, marked)
		}

	case *object.Closure:
		for _, free := range typed.Free {
			c.markLocked(free, marked)
		}

	case *object.ReturnValue:
		c.markLocked(typed.Value, marked)

	case *object.Future:
		// A completed Future holds its result; an unresolved one holds a
		// channel that the collector must not traverse, so Resolved() is a
		// non-blocking query rather than Await.
		if value, done := typed.Resolved(); done {
			c.markLocked(value, marked)
		}

	case *object.Result:
		c.markLocked(typed.Value, marked)
		c.markLocked(typed.Error, marked)

	case *object.CaptureCell:
		// A shared mutable cell is the mechanism by which two closures observe
		// the same variable, so its value is a live reference.
		c.markLocked(typed.Value, marked)

	case *object.Entity:
		// Entity schema metadata carries no heap child pointers.

	case *object.Error, *object.RuntimeError, *object.String, *object.Integer,
		*object.Float, *object.Boolean, *object.Null:
		// Leaves.
	}
}
