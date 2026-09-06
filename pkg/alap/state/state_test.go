package state

import (
	"testing"
)

func TestStore(t *testing.T) {
	store := NewStore()

	notifiedKey := ""
	var notifiedVal interface{}
	store.Subscribe("count", func(key string, newVal, oldVal interface{}) {
		notifiedKey = key
		notifiedVal = newVal
	})

	store.Set("count", 42)
	if val, ok := store.Get("count"); !ok || val != 42 {
		t.Errorf("expected count 42, got %v", val)
	}

	if notifiedKey != "count" || notifiedVal != 42 {
		t.Errorf("subscriber not notified properly: key=%s val=%v", notifiedKey, notifiedVal)
	}

	if store.GetInt("count", 0) != 42 {
		t.Errorf("GetInt returned wrong value")
	}
}

func TestStoreBatch(t *testing.T) {
	store := NewStore()
	callCount := 0

	store.SubscribeAll(func(key string, newVal, oldVal interface{}) {
		callCount++
	})

	store.Batch(func() {
		store.Set("a", 1)
		store.Set("b", 2)
		store.Set("c", 3)
	})

	// After batch completes, subscribers should be notified for the changed keys
	if callCount != 3 {
		t.Errorf("expected 3 batch subscriber calls, got %d", callCount)
	}
	if store.GetInt("a", 0) != 1 || store.GetInt("b", 0) != 2 || store.GetInt("c", 0) != 3 {
		t.Errorf("store values incorrect after batch")
	}
}

func TestSignalAndComputed(t *testing.T) {
	count := NewSignal(5)
	doubled := NewComputed(func() int {
		return count.Get() * 2
	})

	if doubled.Get() != 10 {
		t.Errorf("expected doubled 10, got %d", doubled.Get())
	}

	// Update signal and invalidate computed
	count.Set(15)
	doubled.Invalidate()
	if doubled.Get() != 30 {
		t.Errorf("expected doubled 30, got %d", doubled.Get())
	}
}

func TestDiffNodes(t *testing.T) {
	patches := DiffNodes("btn-1", "Click Me", "btn-1", "Submitted!")
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	if patches[0].Type != PatchText || patches[0].Payload["text"] != "Submitted!" {
		t.Errorf("unexpected patch: %+v", patches[0])
	}
}
