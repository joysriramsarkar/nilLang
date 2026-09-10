package gc

import (
	"sync"

	"github.com/joysriramsarkar/nilLang/compiler/object"
)

// Handle represents an anchored reference across native FFI boundaries
type Handle uint64

// Collector manages object allocation and tracing garbage collection
type Collector struct {
	mu            sync.Mutex
	objects       map[object.Object]bool
	nativeHandles map[Handle]object.Object
	nextHandle    Handle
	Threshold     int
	Collections   int
}

func NewCollector(threshold int) *Collector {
	if threshold <= 0 {
		threshold = 1000
	}
	return &Collector{
		objects:       make(map[object.Object]bool),
		nativeHandles: make(map[Handle]object.Object),
		nextHandle:    1,
		Threshold:     threshold,
	}
}

// Track registers a newly allocated heap object with the collector
func (c *Collector) Track(obj object.Object) {
	if obj == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.objects[obj] = true
}

// RetainHandle anchors an object across the native boundary so GC won't reclaim it
func (c *Collector) RetainHandle(obj object.Object) Handle {
	if obj == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	h := c.nextHandle
	c.nextHandle++
	c.nativeHandles[h] = obj
	c.objects[obj] = true
	return h
}

// ReleaseHandle unregisters a native handle, allowing the object to be collected
func (c *Collector) ReleaseHandle(h Handle) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.nativeHandles, h)
}

// GetHandle resolves an active native handle
func (c *Collector) GetHandle(h Handle) (object.Object, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	obj, ok := c.nativeHandles[h]
	return obj, ok
}

// ObjectCount returns the number of currently tracked objects
func (c *Collector) ObjectCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.objects)
}

// Collect runs a complete mark-and-sweep cycle given the active root set
func (c *Collector) Collect(roots []object.Object) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	marked := make(map[object.Object]bool)

	// Step 1: Mark all roots
	for _, root := range roots {
		c.mark(root, marked)
	}

	// Step 2: Mark all native handle roots
	for _, nativeObj := range c.nativeHandles {
		c.mark(nativeObj, marked)
	}

	// Step 3: Sweep unreachable objects
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

func (c *Collector) mark(obj object.Object, marked map[object.Object]bool) {
	if obj == nil || marked[obj] {
		return
	}

	marked[obj] = true

	// Recursively traverse child references
	switch o := obj.(type) {
	case *object.Array:
		for _, elem := range o.Elements {
			c.mark(elem, marked)
		}

	case *object.Hash:
		for _, pair := range o.Pairs {
			c.mark(pair.Key, marked)
			c.mark(pair.Value, marked)
		}

	case *object.Closure:
		for _, free := range o.Free {
			c.mark(free, marked)
		}

	case *object.ReturnValue:
		c.mark(o.Value, marked)

	case *object.Entity:
		// Entity schema metadata has no heap child pointers
	}
}
