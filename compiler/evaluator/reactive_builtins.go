package evaluator

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/joysriramsarkar/nilLang/compiler/object"
)

// U2 — Nilang reactivity builtins: signal/computed/effect/reactiveArray/store.
//
// Engine story (documented honestly): the compiler derives VM builtin names
// from evaluator.Builtins, so every builtin below EXISTS in both engines and
// the value-level operations (signal get/set, reactive arrays, stores,
// explicit batch_begin/batch_end) behave identically in both. The forms that
// must INVOKE a Nilang closure from Go (computed(fn), effect(fn), batch(fn))
// work in the evaluator via applyFunction; in the VM engine a closure value
// is *object.Closure (bytecode) which no builtin can execute without a VM
// handle — that would need compiler/vm changes, which are frozen. There those
// forms return a clean diagnostic instead of crashing. See UI_SPEC.md §4 and
// tests/conformance reactive cases for the portable subset.
//
// Semantics: glitch-free push-pull with batching. Writes inside a batch only
// mark computeds dirty and queue effects; the flush recomputes in dependency
// order, then runs effects by priority (high→low), so no observer ever sees
// a partially-propagated graph.

var (
	reactiveID      atomic.Int64
	reactiveMu      sync.Mutex
	signals         = make(map[int64]*reactiveSignal)
	computeds       = make(map[int64]*reactiveComputed)
	effects         = make(map[int64]*reactiveEffect)
	batchDepth      int
	queuedEffects   []int64
	collectingStack [][]int64 // evaluator-only dep collection frames
)

type reactiveSignal struct {
	handle    *object.Hash
	version   int64
	listeners []func(newVal object.Object)
}

type reactiveComputed struct {
	handle *object.Hash
	fn     object.Object // *object.Function in evaluator; *object.Closure in VM
	cached object.Object
	dirty  bool
	// computing guards re-entrant flushes: a signal written while fn runs
	// must not recompute this same node again before the outer run finishes.
	computing bool
	deps      []int64
}

type reactiveEffect struct {
	handle   *object.Hash
	fn       object.Object
	deps     []int64
	priority int64
	disposed bool
	running  bool
	cleanup  object.Object // optional nil or function
}

func reactiveHandleID(h *object.Hash) (int64, bool) {
	pair, ok := h.Get(&object.String{Value: "__sid"})
	if !ok {
		return 0, false
	}
	iv, ok := pair.Value.(*object.Integer)
	if !ok {
		return 0, false
	}
	return iv.Value, true
}

func reactiveHandleType(h *object.Hash) string {
	pair, ok := h.Get(&object.String{Value: "__type"})
	if !ok {
		return ""
	}
	if s, ok := pair.Value.(*object.String); ok {
		return s.Value
	}
	return ""
}

func newReactiveHandle(typ string) (*object.Hash, int64) {
	id := reactiveID.Add(1)
	h := MakeHashObj(map[string]object.Object{
		"__type": &object.String{Value: typ},
		"__sid":  &object.Integer{Value: id},
	})
	return h, id
}

func hashField(h *object.Hash, key string) object.Object {
	pair, ok := h.Get(&object.String{Value: key})
	if !ok {
		return NULL
	}
	return pair.Value
}

func setHashField(h *object.Hash, key string, v object.Object) {
	h.SetPair(&object.String{Value: key}, v)
}

// invokeNilangFn calls a Nilang function value from a builtin. Only
// *object.Function (evaluator engine) is invocable here; a VM *object.Closure
// carries bytecode and needs a VM handle, so it reports a diagnostic.
//
// Depth and purity come from the function's own environment: reactive
// callbacks run without a dynamic caller in scope, so the lexical chain is
// what there is.
func invokeNilangFn(fn object.Object, args ...object.Object) object.Object {
	switch fn.(type) {
	case *object.Function:
		return applyFunction(fn, args, dotenv(fn))
	case *object.Builtin:
		return applyFunction(fn, args, dotenv(fn))
	default:
		return newError("reactivity: computed/effect/batch callbacks need the evaluator engine; got %s (VM closures cannot be invoked from a builtin)", fn.Type())
	}
}

func isReactiveError(o object.Object) bool {
	_, ok := o.(*object.Error)
	if ok {
		return true
	}
	_, ok = o.(*object.RuntimeError)
	return ok
}

// recordDep notes a signal read inside an open collection frame.
func recordDep(sid int64) {
	reactiveMu.Lock()
	defer reactiveMu.Unlock()
	if len(collectingStack) == 0 {
		return
	}
	top := len(collectingStack) - 1
	for _, d := range collectingStack[top] {
		if d == sid {
			return
		}
	}
	collectingStack[top] = append(collectingStack[top], sid)
}

func beginCollect() {
	reactiveMu.Lock()
	collectingStack = append(collectingStack, []int64{})
	reactiveMu.Unlock()
}

func endCollect() []int64 {
	reactiveMu.Lock()
	defer reactiveMu.Unlock()
	if len(collectingStack) == 0 {
		return nil
	}
	top := collectingStack[len(collectingStack)-1]
	collectingStack = collectingStack[:len(collectingStack)-1]
	return top
}

// propagate marks computeds dirty transitively and queues effects.
func propagate(changed []int64) {
	reactiveMu.Lock()
	dirtySet := map[int64]bool{}
	for _, s := range changed {
		dirtySet[s] = true
	}
	// Transitive computed dirty-marking (creation order ≈ topo order for
	// chains built top-down; iterate to fixpoint for safety).
	for {
		progress := false
		ids := make([]int64, 0, len(computeds))
		for id := range computeds {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, id := range ids {
			c := computeds[id]
			if c.dirty {
				continue
			}
			for _, d := range c.deps {
				if dirtySet[d] || dirtySet[-id] {
					c.dirty = true
					dirtySet[-id] = true
					setHashField(c.handle, "dirty", &object.Boolean{Value: true})
					progress = true
					break
				}
			}
		}
		if !progress {
			break
		}
	}
	// Queue dependent effects.
	for id, e := range effects {
		if e.disposed {
			continue
		}
		for _, d := range e.deps {
			if dirtySet[d] || dirtySet[-d] {
				queuedEffects = append(queuedEffects, id)
				break
			}
		}
	}
	shouldFlush := batchDepth == 0
	reactiveMu.Unlock()
	if shouldFlush {
		flushReactive()
	}
}

// flushReactive recomputes dirty computeds then runs queued effects by priority.
func flushReactive() {
	attempted := map[int64]bool{}
	for {
		reactiveMu.Lock()
		if batchDepth > 0 {
			reactiveMu.Unlock()
			return
		}
		// Recompute dirty computeds in creation order.
		ids := make([]int64, 0, len(computeds))
		for id, c := range computeds {
			if c.dirty && !c.computing && !attempted[id] {
				ids = append(ids, id)
			}
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		reactiveMu.Unlock()
		if len(ids) == 0 {
			break
		}
		for _, id := range ids {
			attempted[id] = true
			recomputeComputed(id)
		}
	}
	reactiveMu.Lock()
	queue := queuedEffects
	queuedEffects = nil
	reactiveMu.Unlock()
	sort.SliceStable(queue, func(i, j int) bool {
		reactiveMu.Lock()
		defer reactiveMu.Unlock()
		return effects[queue[i]].priority < effects[queue[j]].priority
	})
	seen := map[int64]bool{}
	for _, id := range queue {
		if seen[id] {
			continue
		}
		seen[id] = true
		rerunEffect(id)
	}
}

func recomputeComputed(id int64) {
	reactiveMu.Lock()
	c, ok := computeds[id]
	if !ok || !c.dirty || c.computing {
		reactiveMu.Unlock()
		return
	}
	c.computing = true
	c.dirty = false
	fn := c.fn
	reactiveMu.Unlock()
	beginCollect()
	out := invokeNilangFn(fn)
	deps := endCollect()
	reactiveMu.Lock()
	defer reactiveMu.Unlock()
	c.computing = false
	if isReactiveError(out) {
		// Keep old cached value on error; stay dirty so next read retries.
		c.dirty = true
		setHashField(c.handle, "dirty", &object.Boolean{Value: true})
		return
	}
	c.cached = out
	c.deps = deps
	setHashField(c.handle, "value", out)
	setHashField(c.handle, "dirty", &object.Boolean{Value: false})
}

func rerunEffect(id int64) {
	reactiveMu.Lock()
	e, ok := effects[id]
	if !ok || e.disposed || e.running {
		reactiveMu.Unlock()
		return
	}
	e.running = true
	fn := e.fn
	reactiveMu.Unlock()
	beginCollect()
	out := invokeNilangFn(fn)
	deps := endCollect()
	reactiveMu.Lock()
	defer reactiveMu.Unlock()
	e.running = false
	if isReactiveError(out) {
		return
	}
	e.deps = deps
}

func registerReactiveBuiltins() {
	Builtins["signal"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return newError("wrong number of arguments to `signal`. got=%d, want=1", len(args))
		}
		h, id := newReactiveHandle("Signal")
		setHashField(h, "value", args[0])
		setHashField(h, "version", &object.Integer{Value: 0})
		reactiveMu.Lock()
		signals[id] = &reactiveSignal{handle: h}
		reactiveMu.Unlock()
		return h
	}}
	Builtins["sget"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return newError("wrong number of arguments to `sget`. got=%d, want=1", len(args))
		}
		h, ok := args[0].(*object.Hash)
		if !ok {
			return newError("argument to `sget` must be a Signal, got %s", args[0].Type())
		}
		id, ok := reactiveHandleID(h)
		if !ok {
			return newError("argument to `sget` must be a Signal handle")
		}
		reactiveMu.Lock()
		s, ok := signals[id]
		reactiveMu.Unlock()
		if !ok {
			// Maybe a computed handle.
			reactiveMu.Lock()
			c, ok := computeds[id]
			reactiveMu.Unlock()
			if !ok {
				return newError("`sget` handle is stale (unknown id %d)", id)
			}
			_ = c
			return cgetByID(id)
		}
		recordDep(id)
		_ = s
		return hashField(h, "value")
	}}
	Builtins["sset"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return newError("wrong number of arguments to `sset`. got=%d, want=2", len(args))
		}
		h, ok := args[0].(*object.Hash)
		if !ok || reactiveHandleType(h) != "Signal" {
			return newError("first argument to `sset` must be a Signal, got %s", args[0].Type())
		}
		id, _ := reactiveHandleID(h)
		reactiveMu.Lock()
		s, ok := signals[id]
		if !ok {
			reactiveMu.Unlock()
			return newError("`sset` handle is stale (unknown id %d)", id)
		}
		ver, _ := hashField(h, "version").(*object.Integer)
		var vn int64
		if ver != nil {
			vn = ver.Value
		}
		vn++
		s.version = vn
		listeners := append([]func(object.Object){}, s.listeners...)
		reactiveMu.Unlock()
		setHashField(h, "value", args[1])
		setHashField(h, "version", &object.Integer{Value: vn})
		for _, l := range listeners {
			l(args[1])
		}
		propagate([]int64{id})
		return args[1]
	}}
	Builtins["sver"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return newError("wrong number of arguments to `sver`. got=%d, want=1", len(args))
		}
		h, ok := args[0].(*object.Hash)
		if !ok {
			return newError("argument to `sver` must be a Signal, got %s", args[0].Type())
		}
		return hashField(h, "version")
	}}
	Builtins["computed"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return newError("wrong number of arguments to `computed`. got=%d, want=1", len(args))
		}
		if _, ok := args[0].(*object.Function); !ok {
			if _, isBuiltin := args[0].(*object.Builtin); !isBuiltin {
				return newError("argument to `computed` must be a function, got %s (VM closures need the evaluator engine)", args[0].Type())
			}
		}
		h, id := newReactiveHandle("Computed")
		setHashField(h, "dirty", &object.Boolean{Value: true})
		reactiveMu.Lock()
		computeds[id] = &reactiveComputed{handle: h, fn: args[0], cached: NULL, dirty: true}
		reactiveMu.Unlock()
		recomputeComputed(id)
		reactiveMu.Lock()
		c := computeds[id]
		reactiveMu.Unlock()
		if c.dirty {
			return newError("`computed` initial evaluation failed (see earlier error)")
		}
		return h
	}}
	Builtins["cget"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return newError("wrong number of arguments to `cget`. got=%d, want=1", len(args))
		}
		h, ok := args[0].(*object.Hash)
		if !ok || reactiveHandleType(h) != "Computed" {
			return newError("argument to `cget` must be a Computed, got %s", args[0].Type())
		}
		id, _ := reactiveHandleID(h)
		return cgetByID(id)
	}}
	Builtins["effect"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
		if len(args) < 1 || len(args) > 2 {
			return newError("wrong number of arguments to `effect`. got=%d, want=1 or 2", len(args))
		}
		if _, ok := args[0].(*object.Function); !ok {
			if _, isBuiltin := args[0].(*object.Builtin); !isBuiltin {
				return newError("argument to `effect` must be a function, got %s (VM closures need the evaluator engine)", args[0].Type())
			}
		}
		var prio int64 = 1
		if len(args) == 2 {
			pi, ok := args[1].(*object.Integer)
			if !ok {
				return newError("effect priority must be INTEGER, got %s", args[1].Type())
			}
			prio = pi.Value
		}
		h, id := newReactiveHandle("Effect")
		reactiveMu.Lock()
		effects[id] = &reactiveEffect{handle: h, fn: args[0], priority: prio}
		reactiveMu.Unlock()
		// Run once immediately (collect deps).
		beginCollect()
		out := invokeNilangFn(args[0])
		deps := endCollect()
		if isReactiveError(out) {
			reactiveMu.Lock()
			delete(effects, id)
			reactiveMu.Unlock()
			return out
		}
		reactiveMu.Lock()
		effects[id].deps = deps
		reactiveMu.Unlock()
		return h
	}}
	Builtins["dispose"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return newError("wrong number of arguments to `dispose`. got=%d, want=1", len(args))
		}
		h, ok := args[0].(*object.Hash)
		if !ok || reactiveHandleType(h) != "Effect" {
			return newError("argument to `dispose` must be an Effect, got %s", args[0].Type())
		}
		id, _ := reactiveHandleID(h)
		reactiveMu.Lock()
		if e, ok := effects[id]; ok {
			e.disposed = true
			e.deps = nil
		}
		reactiveMu.Unlock()
		return NULL
	}}
	Builtins["batch_begin"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
		if len(args) != 0 {
			return newError("wrong number of arguments to `batch_begin`. got=%d, want=0", len(args))
		}
		reactiveMu.Lock()
		batchDepth++
		reactiveMu.Unlock()
		return NULL
	}}
	Builtins["batch_end"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
		if len(args) != 0 {
			return newError("wrong number of arguments to `batch_end`. got=%d, want=0", len(args))
		}
		reactiveMu.Lock()
		if batchDepth > 0 {
			batchDepth--
		}
		empty := batchDepth == 0
		reactiveMu.Unlock()
		if empty {
			flushReactive()
		}
		return NULL
	}}
	Builtins["batch"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return newError("wrong number of arguments to `batch`. got=%d, want=1", len(args))
		}
		reactiveMu.Lock()
		batchDepth++
		reactiveMu.Unlock()
		out := invokeNilangFn(args[0])
		reactiveMu.Lock()
		if batchDepth > 0 {
			batchDepth--
		}
		empty := batchDepth == 0
		reactiveMu.Unlock()
		if empty {
			flushReactive()
		}
		if isReactiveError(out) {
			return out
		}
		return NULL
	}}
	Builtins["reactiveArray"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
		h, id := newReactiveHandle("ReactiveArray")
		items := append([]object.Object{}, args...)
		arr := &object.Array{Elements: items}
		setHashField(h, "items", arr)
		setHashField(h, "version", &object.Integer{Value: 0})
		reactiveMu.Lock()
		signals[id] = &reactiveSignal{handle: h}
		reactiveMu.Unlock()
		return h
	}}
	Builtins["ra_push"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return newError("wrong number of arguments to `ra_push`. got=%d, want=2", len(args))
		}
		h, ok := args[0].(*object.Hash)
		if !ok || reactiveHandleType(h) != "ReactiveArray" {
			return newError("first argument to `ra_push` must be a ReactiveArray, got %s", args[0].Type())
		}
		arr, _ := hashField(h, "items").(*object.Array)
		if arr == nil {
			return newError("`ra_push` handle is corrupt (no items)")
		}
		arr.Append(args[1])
		bumpReactiveVersion(h)
		id, _ := reactiveHandleID(h)
		propagate([]int64{id})
		return &object.Integer{Value: int64(arr.Len())}
	}}
	Builtins["ra_len"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return newError("wrong number of arguments to `ra_len`. got=%d, want=1", len(args))
		}
		h, ok := args[0].(*object.Hash)
		if !ok || reactiveHandleType(h) != "ReactiveArray" {
			return newError("argument to `ra_len` must be a ReactiveArray, got %s", args[0].Type())
		}
		arr, _ := hashField(h, "items").(*object.Array)
		if arr == nil {
			return newError("`ra_len` handle is corrupt (no items)")
		}
		return &object.Integer{Value: int64(arr.Len())}
	}}
	Builtins["ra_get"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return newError("wrong number of arguments to `ra_get`. got=%d, want=2", len(args))
		}
		h, ok := args[0].(*object.Hash)
		if !ok || reactiveHandleType(h) != "ReactiveArray" {
			return newError("first argument to `ra_get` must be a ReactiveArray, got %s", args[0].Type())
		}
		idx, ok := args[1].(*object.Integer)
		if !ok {
			return newError("index for `ra_get` must be INTEGER, got %s", args[1].Type())
		}
		arr, _ := hashField(h, "items").(*object.Array)
		if arr == nil {
			return NULL
		}
		element, ok := arr.Get(idx.Value)
		if !ok {
			return NULL
		}
		return element
	}}
	Builtins["ra_set"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
		if len(args) != 3 {
			return newError("wrong number of arguments to `ra_set`. got=%d, want=3", len(args))
		}
		h, ok := args[0].(*object.Hash)
		if !ok || reactiveHandleType(h) != "ReactiveArray" {
			return newError("first argument to `ra_set` must be a ReactiveArray, got %s", args[0].Type())
		}
		idx, ok := args[1].(*object.Integer)
		if !ok {
			return newError("index for `ra_set` must be INTEGER, got %s", args[1].Type())
		}
		arr, _ := hashField(h, "items").(*object.Array)
		if arr == nil || !arr.Set(idx.Value, args[2]) {
			return newError("`ra_set` index out of bounds")
		}
		bumpReactiveVersion(h)
		id, _ := reactiveHandleID(h)
		propagate([]int64{id})
		return args[2]
	}}
	Builtins["store"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
		if len(args) != 1 {
			return newError("wrong number of arguments to `store`. got=%d, want=1", len(args))
		}
		src, ok := args[0].(*object.Hash)
		if !ok {
			return newError("argument to `store` must be HASH, got %s", args[0].Type())
		}
		h, id := newReactiveHandle("Store")
		setHashField(h, "version", &object.Integer{Value: 0})
		// Copy pairs into the handle's data namespace.
		for _, pair := range src.OrderedPairs() {
			h.SetPair(pair.Key, pair.Value)
		}
		reactiveMu.Lock()
		signals[id] = &reactiveSignal{handle: h}
		reactiveMu.Unlock()
		return h
	}}
	Builtins["store_get"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
		if len(args) != 2 {
			return newError("wrong number of arguments to `store_get`. got=%d, want=2", len(args))
		}
		h, ok := args[0].(*object.Hash)
		if !ok || reactiveHandleType(h) != "Store" {
			return newError("first argument to `store_get` must be a Store, got %s", args[0].Type())
		}
		key, ok := args[1].(*object.String)
		if !ok {
			return newError("key for `store_get` must be STRING, got %s", args[1].Type())
		}
		id, _ := reactiveHandleID(h)
		recordDep(id)
		pair, ok := h.Get(key)
		if !ok {
			return NULL
		}
		return pair.Value
	}}
	Builtins["store_set"] = &object.Builtin{Fn: func(args ...object.Object) object.Object {
		if len(args) != 3 {
			return newError("wrong number of arguments to `store_set`. got=%d, want=3", len(args))
		}
		h, ok := args[0].(*object.Hash)
		if !ok || reactiveHandleType(h) != "Store" {
			return newError("first argument to `store_set` must be a Store, got %s", args[0].Type())
		}
		key, ok := args[1].(*object.String)
		if !ok {
			return newError("key for `store_set` must be STRING, got %s", args[1].Type())
		}
		h.SetPair(key, args[2])
		bumpReactiveVersion(h)
		id, _ := reactiveHandleID(h)
		reactiveMu.Lock()
		s, ok := signals[id]
		var listeners []func(object.Object)
		if ok {
			listeners = append([]func(object.Object){}, s.listeners...)
		}
		reactiveMu.Unlock()
		for _, l := range listeners {
			l(args[2])
		}
		propagate([]int64{id})
		return args[2]
	}}
}

func cgetByID(id int64) object.Object {
	reactiveMu.Lock()
	c, ok := computeds[id]
	reactiveMu.Unlock()
	if !ok {
		return newError("`cget` handle is stale (unknown id %d)", id)
	}
	reactiveMu.Lock()
	dirty := c.dirty
	reactiveMu.Unlock()
	if dirty {
		recomputeComputed(id)
	}
	reactiveMu.Lock()
	stillDirty := c.dirty
	cached := c.cached
	reactiveMu.Unlock()
	if stillDirty {
		return newError("`cget` recompute failed (see earlier error)")
	}
	recordDep(-id)
	return cached
}

func bumpReactiveVersion(h *object.Hash) {
	ver, _ := hashField(h, "version").(*object.Integer)
	var vn int64
	if ver != nil {
		vn = ver.Value
	}
	setHashField(h, "version", &object.Integer{Value: vn + 1})
}

func init() { registerReactiveBuiltins() }

// ResetReactiveGraph clears global reactive state. Tests only.
func ResetReactiveGraph() {
	reactiveMu.Lock()
	defer reactiveMu.Unlock()
	signals = make(map[int64]*reactiveSignal)
	computeds = make(map[int64]*reactiveComputed)
	effects = make(map[int64]*reactiveEffect)
	batchDepth = 0
	queuedEffects = nil
	collectingStack = nil
}

var _ = fmt.Sprintf
