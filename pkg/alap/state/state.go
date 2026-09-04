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
	mu          sync.RWMutex
}

// NewStore creates a new state store
func NewStore() *Store {
	return &Store{
		data:        make(map[string]interface{}),
		subscribers: make(map[string][]Subscriber),
		allSubs:     []Subscriber{},
	}
}

// Set updates a key in the store and notifies subscribers
func (s *Store) Set(key string, val interface{}) {
	s.mu.Lock()
	oldVal := s.data[key]
	s.data[key] = val
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

// ─── REACTIVE SIGNAL ────────────────────────────────────────────────────────

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
