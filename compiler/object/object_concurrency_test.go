package object

import (
	"fmt"
	"sync"
	"testing"
)

// TestHashConcurrentWriters hammers one shared hash from many goroutines,
// the shape of two tasks writing the same global (L6.2). Before Hash
// gained its mutex this was a concurrent map write — a fatal error, not
// a test failure — so the test's first assertion is that it finishes at
// all. Run with -race it also proves the accessors synchronise.
func TestHashConcurrentWriters(t *testing.T) {
	const writers = 8
	const perWriter = 500

	hash := NewHash(writers * perWriter)
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				key := &Integer{Value: int64(w*perWriter + i)}
				hash.SetPair(key, &Integer{Value: int64(i)})
				if _, ok := hash.Get(key); !ok {
					t.Errorf("key %d missing immediately after SetPair", w*perWriter+i)
					return
				}
				_ = hash.OrderedPairs()
				_ = hash.Len()
			}
		}(w)
	}
	wg.Wait()

	if got, want := hash.Len(), writers*perWriter; got != want {
		t.Fatalf("hash holds %d pairs, want %d", got, want)
	}
	_ = hash.Inspect()
}

// TestArrayConcurrentAccess exercises appends, indexed writes, reads and
// snapshots racing each other, as two tasks sharing an array would.
func TestArrayConcurrentAccess(t *testing.T) {
	const writers = 4
	const perWriter = 250

	arr := &Array{Elements: []Object{}}
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				arr.Append(&Integer{Value: int64(i)})
				_ = arr.Snapshot()
				_ = arr.Len()
				if element, ok := arr.Get(0); ok && element == nil {
					t.Error("Get(0) returned a nil element")
					return
				}
			}
		}(w)
	}
	wg.Wait()

	if got, want := arr.Len(), writers*perWriter; got != want {
		t.Fatalf("array holds %d elements, want %d", got, want)
	}

	// Indexed writes to disjoint slots are the other shape tasks produce.
	for i := 0; i < arr.Len(); i++ {
		if !arr.Set(int64(i), &Integer{Value: int64(i)}) {
			t.Fatalf("Set(%d) refused on an in-range index", i)
		}
	}
	_ = arr.Inspect()
}

// TestCaptureCellConcurrentAccess races readers against writers on one
// shared cell, the closure-capture shape the VM produces under tasks.
func TestCaptureCellConcurrentAccess(t *testing.T) {
	cell := NewCaptureCell(&Integer{Value: 0})
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				cell.Set(&Integer{Value: int64(w*500 + i)})
				if cell.Get() == nil {
					t.Error("cell read back nil after writes began")
					return
				}
				_ = cell.Inspect()
			}
		}(w)
	}
	wg.Wait()

	final, ok := cell.Get().(*Integer)
	if !ok {
		t.Fatalf("cell holds %T, want *Integer", cell.Get())
	}
	if final.Value < 0 || final.Value >= 2000 {
		t.Fatalf("cell holds %d, outside the written range", final.Value)
	}
	_ = fmt.Sprint(final)
}
