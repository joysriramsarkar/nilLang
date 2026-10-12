// Package reactive is the Go core of U2 reactivity: glitch-free push-pull
// signals with batching, priorities, computed memoization, effects with
// teardown, reactive arrays, and app-level stores. The Nilang builtins
// (compiler/evaluator/reactive_builtins.go) delegate to this package so both
// engines share semantics.
package reactive

import (
	"sort"
	"sync"
)

// Priority orders effect scheduling at frame boundaries.
type Priority int

const (
	PriorityHigh Priority = iota
	PriorityNormal
	PriorityLow
)

var (
	mu       sync.Mutex
	epoch    uint64
	batching int
	pending  []*Effect
	// deps tracks the currently-collecting effect (single-threaded collect).
	current *Effect
)

// Signal is a reactive value cell.
type Signal[T any] struct {
	mu        sync.Mutex
	value     T
	version   uint64
	watchers  map[*Effect]struct{}
	listeners []func(T)
}

// NewSignal creates a reactive signal.
func NewSignal[T any](v T) *Signal[T] {
	return &Signal[T]{value: v, watchers: make(map[*Effect]struct{})}
}

// Get reads the value and records a dependency when inside an effect.
func (s *Signal[T]) Get() T {
	mu.Lock()
	if current != nil {
		s.mu.Lock()
		if s.watchers == nil {
			s.watchers = make(map[*Effect]struct{})
		}
		s.watchers[current] = struct{}{}
		current.deps = append(current.deps, dep{onInvalidate: func() { s.unwatch(current) }})
		s.mu.Unlock()
	}
	mu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.value
}

// Peek reads without tracking.
func (s *Signal[T]) Peek() T {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.value
}

// Set writes and notifies (batched when inside Batch).
func (s *Signal[T]) Set(v T) {
	s.mu.Lock()
	s.value = v
	mu.Lock()
	epoch++
	watchers := make([]*Effect, 0, len(s.watchers))
	for e := range s.watchers {
		watchers = append(watchers, e)
	}
	listeners := append([]func(T){}, s.listeners...)
	mu.Unlock()
	s.mu.Unlock()
	for _, l := range listeners {
		l(v)
	}
	schedule(watchers)
}

// Watch subscribes a plain listener.
func (s *Signal[T]) Watch(fn func(T)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, fn)
}

func (s *Signal[T]) unwatch(e *Effect) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.watchers, e)
}

type dep struct {
	onInvalidate func()
}

// Effect is a tracked side-effect with teardown.
type Effect struct {
	fn       func(dispose func())
	cleanup  func()
	deps     []dep
	priority Priority
	disposed bool
	mu       sync.Mutex
	stopCh   chan struct{}
}

// NewEffect creates and runs fn immediately, collecting dependencies.
func NewEffect(fn func(), opts ...EffectOption) *Effect {
	e := &Effect{stopCh: make(chan struct{})}
	for _, o := range opts {
		o(e)
	}
	e.run(func() {})
	_ = fn
	// Wrap plain fn into tracked run.
	plain := fn
	e.rerun(func() { plain() })
	return e
}

// EffectOption configures an effect.
type EffectOption func(*Effect)

// WithPriority sets scheduling priority.
func WithPriority(p Priority) EffectOption {
	return func(e *Effect) { e.priority = p }
}

// WithCleanup registers teardown run on rerun/dispose.
func WithCleanup(c func()) EffectOption {
	return func(e *Effect) { e.cleanup = c }
}

func (e *Effect) run(body func()) {
	mu.Lock()
	prev := current
	current = e
	e.deps = nil
	mu.Unlock()
	body()
	mu.Lock()
	current = prev
	mu.Unlock()
}

func (e *Effect) rerun(body func()) {
	e.mu.Lock()
	if e.disposed {
		e.mu.Unlock()
		return
	}
	if e.cleanup != nil {
		c := e.cleanup
		e.mu.Unlock()
		c()
		e.mu.Lock()
	}
	e.mu.Unlock()
	for _, d := range e.deps {
		if d.onInvalidate != nil {
			d.onInvalidate()
		}
	}
	e.run(body)
}

// Dispose tears down deps + cleanup exactly once.
func (e *Effect) Dispose() {
	e.mu.Lock()
	if e.disposed {
		e.mu.Unlock()
		return
	}
	e.disposed = true
	c := e.cleanup
	deps := e.deps
	e.deps = nil
	e.mu.Unlock()
	for _, d := range deps {
		if d.onInvalidate != nil {
			d.onInvalidate()
		}
	}
	if c != nil {
		c()
	}
	select {
	case <-e.stopCh:
	default:
		close(e.stopCh)
	}
}

// Stopped reports disposal.
func (e *Effect) Stopped() bool {
	select {
	case <-e.stopCh:
		return true
	default:
		return false
	}
}

func schedule(es []*Effect) {
	mu.Lock()
	// Deduplicate.
	seen := map[*Effect]bool{}
	for _, e := range pending {
		seen[e] = true
	}
	for _, e := range es {
		if !seen[e] {
			seen[e] = true
			pending = append(pending, e)
		}
	}
	if batching > 0 {
		mu.Unlock()
		return
	}
	queue := pending
	pending = nil
	mu.Unlock()
	sort.SliceStable(queue, func(i, j int) bool { return queue[i].priority < queue[j].priority })
	for _, e := range queue {
		e.mu.Lock()
		disposed := e.disposed
		e.mu.Unlock()
		if !disposed {
			e.rerun(func() {})
		}
	}
}

// Batch groups mutations, flushing once.
func Batch(fn func()) {
	mu.Lock()
	batching++
	mu.Unlock()
	defer func() {
		mu.Lock()
		batching--
		flush := batching == 0
		mu.Unlock()
		if flush {
			schedule(nil)
		}
	}()
	fn()
}

// Computed is a memoized derived value.
type Computed[T any] struct {
	mu      sync.Mutex
	fn      func() T
	value   T
	version uint64
	dirty   bool
	inner   *Signal[uint64]
}

// NewComputed creates a memoized computed.
func NewComputed[T any](fn func() T) *Computed[T] {
	return &Computed[T]{fn: fn, dirty: true, inner: NewSignal[uint64](0)}
}

// Get evaluates when dirty.
func (c *Computed[T]) Get() T {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.dirty {
		c.value = c.fn()
		c.dirty = false
	}
	return c.value
}

// Invalidate marks dirty.
func (c *Computed[T]) Invalidate() {
	c.mu.Lock()
	c.dirty = true
	c.mu.Unlock()
	c.inner.Set(c.inner.Peek() + 1)
}

// ReactiveArray is an observable slice.
type ReactiveArray[T any] struct {
	mu        sync.Mutex
	items     []T
	version   *Signal[uint64]
	listeners []func([]T)
}

// NewReactiveArray creates an observable array.
func NewReactiveArray[T any](items ...T) *ReactiveArray[T] {
	return &ReactiveArray[T]{items: append([]T{}, items...), version: NewSignal[uint64](0)}
}

// Items returns a snapshot.
func (a *ReactiveArray[T]) Items() []T {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]T{}, a.items...)
}

// Push appends and notifies.
func (a *ReactiveArray[T]) Push(v T) {
	a.mu.Lock()
	a.items = append(a.items, v)
	snap := append([]T{}, a.items...)
	listeners := append([]func([]T){}, a.listeners...)
	a.mu.Unlock()
	a.version.Set(a.version.Peek() + 1)
	for _, l := range listeners {
		l(snap)
	}
}

// SetAt replaces index and notifies.
func (a *ReactiveArray[T]) SetAt(i int, v T) {
	a.mu.Lock()
	if i < 0 || i >= len(a.items) {
		a.mu.Unlock()
		return
	}
	a.items[i] = v
	snap := append([]T{}, a.items...)
	listeners := append([]func([]T){}, a.listeners...)
	a.mu.Unlock()
	a.version.Set(a.version.Peek() + 1)
	for _, l := range listeners {
		l(snap)
	}
}

// Len returns length.
func (a *ReactiveArray[T]) Len() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.items)
}

// Watch subscribes.
func (a *ReactiveArray[T]) Watch(fn func([]T)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.listeners = append(a.listeners, fn)
}

// Store is an app-level key/value state container with selectors.
type Store struct {
	mu        sync.RWMutex
	data      map[string]interface{}
	subs      map[string][]func(interface{})
	allSubs   []func(string, interface{})
	inBatch   bool
	pendingKs map[string]bool
}

// NewStore creates a store.
func NewStore() *Store {
	return &Store{data: map[string]interface{}{}, subs: map[string][]func(interface{}){}, pendingKs: map[string]bool{}}
}

// Set writes a key.
func (s *Store) Set(key string, v interface{}) {
	s.mu.Lock()
	s.data[key] = v
	if s.inBatch {
		s.pendingKs[key] = true
		s.mu.Unlock()
		return
	}
	subs := append([]func(interface{}){}, s.subs[key]...)
	all := append([]func(string, interface{}){}, s.allSubs...)
	s.mu.Unlock()
	for _, f := range subs {
		f(v)
	}
	for _, f := range all {
		f(key, v)
	}
}

// Get reads a key.
func (s *Store) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	return v, ok
}

// Select derives a value with a selector function.
func (s *Store) Select(fn func(map[string]interface{}) interface{}) interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make(map[string]interface{}, len(s.data))
	for k, v := range s.data {
		cp[k] = v
	}
	return fn(cp)
}

// Subscribe observes one key.
func (s *Store) Subscribe(key string, fn func(interface{})) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subs[key] = append(s.subs[key], fn)
}

// SubscribeAll observes every key.
func (s *Store) SubscribeAll(fn func(string, interface{})) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.allSubs = append(s.allSubs, fn)
}

// Batch groups mutations.
func (s *Store) Batch(fn func()) {
	s.mu.Lock()
	s.inBatch = true
	s.pendingKs = map[string]bool{}
	s.mu.Unlock()
	fn()
	s.mu.Lock()
	s.inBatch = false
	keys := make([]string, 0, len(s.pendingKs))
	for k := range s.pendingKs {
		keys = append(keys, k)
	}
	s.pendingKs = map[string]bool{}
	all := append([]func(string, interface{}){}, s.allSubs...)
	s.mu.Unlock()
	for _, k := range keys {
		s.mu.RLock()
		v := s.data[k]
		subs := append([]func(interface{}){}, s.subs[k]...)
		s.mu.RUnlock()
		for _, f := range subs {
			f(v)
		}
		for _, f := range all {
			f(k, v)
		}
	}
}
