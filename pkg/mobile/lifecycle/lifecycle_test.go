package lifecycle

import "testing"

func TestHappyPath(t *testing.T) {
	m := New()
	for _, ev := range []Event{EvCreate, EvResume, EvPause, EvResume, EvDestroy} {
		if err := m.Fire(ev); err != nil {
			t.Fatalf("Fire failed: %v", err)
		}
	}
	if m.State() != StateDestroyed {
		t.Errorf("expected destroyed, got %s", m.State())
	}
	// Destroyed machines can be recreated (process restart).
	if err := m.Fire(EvCreate); err != nil {
		t.Fatalf("recreate failed: %v", err)
	}
	if m.State() != StateCreated {
		t.Errorf("expected created, got %s", m.State())
	}
}

func TestIllegalTransition(t *testing.T) {
	m := New()
	if err := m.Fire(EvResume); err == nil {
		t.Error("resume before create must fail")
	}
	if err := m.Fire(EvPause); err == nil {
		t.Error("pause before create must fail")
	}
	if m.State() != StateUnknown {
		t.Errorf("failed transitions must not move state, got %s", m.State())
	}
}

func TestLowMemoryKeepsState(t *testing.T) {
	m := New()
	if err := m.Fire(EvCreate); err != nil {
		t.Fatal(err)
	}
	if err := m.Fire(EvResume); err != nil {
		t.Fatal(err)
	}
	if err := m.Fire(EvLowMemory); err != nil {
		t.Fatal(err)
	}
	if m.State() != StateResumed {
		t.Errorf("lowMemory must not change state, got %s", m.State())
	}
	if len(m.Log()) != 3 {
		t.Errorf("expected 3 log entries, got %v", m.Log())
	}
}

func TestSaveRestore(t *testing.T) {
	m := New()
	if got := m.Restore(); got != nil {
		t.Errorf("empty restore should be nil, got %v", got)
	}
	m.Save(map[string]string{"route": "/home", "counter": "41"})
	restored := m.Restore()
	if restored["route"] != "/home" || restored["counter"] != "41" {
		t.Errorf("restore mismatch: %v", restored)
	}
	// Mutating the result must not corrupt the snapshot.
	restored["route"] = "/evil"
	if m.Restore()["route"] != "/home" {
		t.Error("Restore must return a copy")
	}
}
