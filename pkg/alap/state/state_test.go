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

func TestSignal(t *testing.T) {
	sig := NewSignal("initial")
	if sig.Get() != "initial" {
		t.Errorf("expected 'initial', got %s", sig.Get())
	}

	observed := ""
	sig.Watch(func(newVal string) {
		observed = newVal
	})

	sig.Set("updated")
	if sig.Get() != "updated" || observed != "updated" {
		t.Errorf("signal update failed: current=%s observed=%s", sig.Get(), observed)
	}
}
