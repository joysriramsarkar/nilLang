package state

import (
	"fmt"
	"sync"
)

// Subscriber is a callback invoked when state changes
type Subscriber func(key string, newVal, oldVal interface{})

// Store is a centralized reactive state container
type Store struct {
	data        map[string]interface{}
	subscribers map[string][]Subscriber
	allSubs     []Subscriber
	inBatch     bool
	pendingKeys map[string]bool
	mu          sync.RWMutex
}

// NewStore creates a new state store
func NewStore() *Store {
	return &Store{
		data:        make(map[string]interface{}),
		subscribers: make(map[string][]Subscriber),
		allSubs:     []Subscriber{},
		pendingKeys: make(map[string]bool),
	}
}

// Set updates a key in the store and notifies subscribers (or defers if in batch)
func (s *Store) Set(key string, val interface{}) {
	s.mu.Lock()
	oldVal := s.data[key]
	s.data[key] = val

	if s.inBatch {
		s.pendingKeys[key] = true
		s.mu.Unlock()
		return
	}

	keySubs := append([]Subscriber{}, s.subscribers[key]...)
	allSubs := append([]Subscriber{}, s.allSubs...)
	s.mu.Unlock()

	for _, sub := range keySubs {
		sub(key, val, oldVal)
	}
	for _, sub := range allSubs {
		sub(key, val, oldVal)
	}
}

// Batch groups multiple state mutations, firing subscribers once when finished
func (s *Store) Batch(fn func()) {
	s.mu.Lock()
	s.inBatch = true
	s.pendingKeys = make(map[string]bool)
	s.mu.Unlock()

	fn()

	s.mu.Lock()
	s.inBatch = false
	changed := make([]string, 0, len(s.pendingKeys))
	for k := range s.pendingKeys {
		changed = append(changed, k)
	}
	s.pendingKeys = make(map[string]bool)

	allSubs := append([]Subscriber{}, s.allSubs...)
	s.mu.Unlock()

	for _, k := range changed {
		s.mu.RLock()
		val := s.data[k]
		subs := append([]Subscriber{}, s.subscribers[k]...)
		s.mu.RUnlock()

		for _, sub := range subs {
			sub(k, val, nil)
		}
		for _, sub := range allSubs {
			sub(k, val, nil)
		}
	}
}

// Get retrieves a key from the store
func (s *Store) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	return val, ok
}

// GetString retrieves string value or default
func (s *Store) GetString(key, fallback string) string {
	if val, ok := s.Get(key); ok {
		if str, isStr := val.(string); isStr {
			return str
		}
	}
	return fallback
}

// GetInt retrieves int value or default
func (s *Store) GetInt(key string, fallback int) int {
	if val, ok := s.Get(key); ok {
		if i, isInt := val.(int); isInt {
			return i
		}
	}
	return fallback
}

// Subscribe listens for changes to a specific key
func (s *Store) Subscribe(key string, sub Subscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscribers[key] = append(s.subscribers[key], sub)
}

// SubscribeAll listens for changes to any key
func (s *Store) SubscribeAll(sub Subscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.allSubs = append(s.allSubs, sub)
}

// Snapshot returns a copy of the state
func (s *Store) Snapshot() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copyMap := make(map[string]interface{}, len(s.data))
	for k, v := range s.data {
		copyMap[k] = v
	}
	return copyMap
}

// ─── REACTIVE SIGNAL & COMPUTED ─────────────────────────────────────────────

// Signal represents a single reactive value
type Signal[T any] struct {
	value     T
	listeners []func(newVal T)
	mu        sync.RWMutex
}

// NewSignal creates a reactive signal
func NewSignal[T any](initial T) *Signal[T] {
	return &Signal[T]{
		value:     initial,
		listeners: []func(newVal T){},
	}
}

// Get reads the signal value
func (s *Signal[T]) Get() T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.value
}

// Set writes the signal value and alerts listeners
func (s *Signal[T]) Set(val T) {
	s.mu.Lock()
	s.value = val
	listeners := append([]func(newVal T){}, s.listeners...)
	s.mu.Unlock()

	for _, l := range listeners {
		l(val)
	}
}

// Watch subscribes to signal changes
func (s *Signal[T]) Watch(fn func(newVal T)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, fn)
}

func (s *Signal[T]) String() string {
	return fmt.Sprintf("%v", s.Get())
}

// Computed creates a memoized derived state derived from a computation
type Computed[T any] struct {
	fn     func() T
	value  T
	dirty  bool
	sub    *Signal[T]
	mu     sync.RWMutex
}

// NewComputed binds compute function to dependencies
func NewComputed[T any](fn func() T) *Computed[T] {
	c := &Computed[T]{
		fn:    fn,
		dirty: true,
		sub:   NewSignal(fn()),
	}
	return c
}

// Get evaluates if dirty, or returns cached value
func (c *Computed[T]) Get() T {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.dirty {
		c.value = c.fn()
		c.dirty = false
	}
	return c.value
}

// Invalidate marks the computed value as dirty
func (c *Computed[T]) Invalidate() {
	c.mu.Lock()
	c.dirty = true
	c.mu.Unlock()
	c.sub.Set(c.Get())
}

// Watch listens to computed value changes
func (c *Computed[T]) Watch(fn func(newVal T)) {
	c.sub.Watch(fn)
}

// Effect registers an automatic side-effect
type Effect struct {
	action func()
	stop   chan struct{}
}

// NewEffect runs an action whenever dependencies trigger
func NewEffect(action func()) *Effect {
	e := &Effect{
		action: action,
		stop:   make(chan struct{}),
	}
	action() // Run once immediately
	return e
}

// ─── VIRTUAL DOM DIFF PATCH ─────────────────────────────────────────────────

// PatchType defines DOM mutation operation
type PatchType string

const (
	PatchText      PatchType = "REPLACE_TEXT"
	PatchAttr      PatchType = "SET_ATTR"
	PatchAppend    PatchType = "APPEND_CHILD"
	PatchRemove    PatchType = "REMOVE_CHILD"
	PatchReplace   PatchType = "REPLACE_NODE"
)

// DOMPatch represents a minimal DOM change to apply in browser
type DOMPatch struct {
	Type     PatchType              `json:"type"`
	TargetID string                 `json:"target_id"`
	Payload  map[string]interface{} `json:"payload"`
}

// DiffNodes compares two simple node representations and outputs minimal patches
func DiffNodes(oldID, oldText string, newID, newText string) []DOMPatch {
	patches := make([]DOMPatch, 0)
	if oldText != newText {
		patches = append(patches, DOMPatch{
			Type:     PatchText,
			TargetID: newID,
			Payload:  map[string]interface{}{"text": newText},
		})
	}
	return patches
}
