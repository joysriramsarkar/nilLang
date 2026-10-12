// Package buildcache provides incremental builds (L9.5): compiled bytecode
// cached under ~/.nil/cache keyed by content hash, so an unchanged project
// rebuilds from cache instead of recompiling.
package buildcache

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entry is one cached compilation.
type Entry struct {
	Key       string
	Bytes     []byte
	CreatedAt time.Time
}

// Cache is a content-addressed bytecode store with a disk backing.
type Cache struct {
	mu  sync.Mutex
	dir string
	mem map[string][]byte
	// Hits/Misses count lookups for tests and `nil build` reporting.
	Hits   int
	Misses int
}

// New returns a cache rooted at dir (created on demand).
func New(dir string) *Cache {
	return &Cache{dir: dir, mem: map[string][]byte{}}
}

// DefaultDir returns ~/.nil/cache.
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), ".nil", "cache")
	}
	return filepath.Join(home, ".nil", "cache")
}

// KeyOf hashes the project inputs: entry source plus every extra file's
// content. Same inputs, same key — renames and mtimes are ignored on
// purpose, because a rebuild after `touch` must still hit.
func KeyOf(entrySource string, extra map[string][]byte) string {
	h := sha256.New()
	h.Write([]byte(entrySource))
	h.Write([]byte{0})
	// Map order is random, so hash names sorted: same files, same key.
	names := make([]string, 0, len(extra))
	for name := range extra {
		names = append(names, name)
	}
	sortStrings(names)
	for _, name := range names {
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write(extra[name])
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// Lookup returns cached bytes for key, checking memory first, then disk.
func (c *Cache) Lookup(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if b, ok := c.mem[key]; ok {
		c.Hits++
		return append([]byte(nil), b...), true
	}
	path := filepath.Join(c.dir, key+".nabc")
	if b, err := os.ReadFile(path); err == nil {
		c.mem[key] = b
		c.Hits++
		return append([]byte(nil), b...), true
	}
	c.Misses++
	return nil, false
}

// Store writes bytes under key to memory and disk.
func (c *Cache) Store(key string, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mem[key] = append([]byte(nil), data...)
	if err := os.MkdirAll(c.dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.dir, key+".nabc"), data, 0644)
}

// CompileFunc compiles source to bytecode bytes (wired by the caller to the
// real pipeline so this package stays dependency-free).
type CompileFunc func(source string) ([]byte, error)

// Build returns cached bytecode when inputs are unchanged, compiling and
// storing otherwise. The second build of identical inputs is a Lookup —
// orders of magnitude faster than a compile (L9.5 gate: cache-hit).
func (c *Cache) Build(entrySource string, extra map[string][]byte, compile CompileFunc) ([]byte, bool, error) {
	key := KeyOf(entrySource, extra)
	if b, ok := c.Lookup(key); ok {
		return b, true, nil
	}
	b, err := compile(entrySource)
	if err != nil {
		return nil, false, err
	}
	if err := c.Store(key, b); err != nil {
		return nil, false, err
	}
	return b, false, nil
}

// Stats reports hit/miss counters.
func (c *Cache) Stats() (hits, misses int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Hits, c.Misses
}
