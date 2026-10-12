package state

import "testing"

// U2.3: effect teardown must be reliable and idempotent; the stop channel
// must actually be observed.
func TestEffectDisposeStops(t *testing.T) {
	runs := 0
	e := NewEffect(func() { runs++ })
	if runs != 1 {
		t.Fatalf("effect must run once, runs=%d", runs)
	}
	if e.Stopped() {
		t.Fatal("fresh effect must not report stopped")
	}
	e.Dispose()
	if !e.Stopped() {
		t.Fatal("disposed effect must report stopped")
	}
	e.Dispose() // idempotent, must not panic
	e.RunIfLive()
	if runs != 1 {
		t.Fatalf("disposed effect must not rerun, runs=%d", runs)
	}
}

func TestEffectRunIfLive(t *testing.T) {
	runs := 0
	e := NewEffect(func() { runs++ })
	e.RunIfLive()
	if runs != 2 {
		t.Fatalf("live effect must rerun, runs=%d", runs)
	}
}
