package input

import (
	"testing"
	"time"
)

func TestHitTestDeepest(t *testing.T) {
	root := &HitNode{ID: "root", Rect: Rect{0, 0, 200, 200}, Depth: 0}
	child := &HitNode{ID: "btn", Rect: Rect{10, 10, 100, 48}, Depth: 1}
	root.Children = []*HitNode{child}
	if got := HitTest(root, 20, 20); got != "btn" {
		t.Fatalf("want btn got %q", got)
	}
	if got := HitTest(root, 150, 150); got != "root" {
		t.Fatalf("want root got %q", got)
	}
	if got := HitTest(root, 300, 300); got != "" {
		t.Fatalf("want miss got %q", got)
	}
}

func TestFanOut(t *testing.T) {
	var got []string
	subs := map[string][]func(PointerEvent){
		"a": {func(PointerEvent) { got = append(got, "1") }, func(PointerEvent) { got = append(got, "2") }},
	}
	if n := FanOut(subs, PointerEvent{TargetID: "a"}); n != 2 || len(got) != 2 {
		t.Fatalf("fanout broken: %d %v", n, got)
	}
}

func TestArenaTapVsScroll(t *testing.T) {
	now := time.Now()
	a := NewArena(0, 0, now)
	// Small move, quick up → tap.
	if g := a.Move(2, 2, now.Add(50*time.Millisecond), 1); g != GestureNone {
		t.Fatalf("must stay ambiguous, got %v", g)
	}
	if g := a.Up(2, 2, now.Add(100*time.Millisecond)); g != GestureTap {
		t.Fatalf("want tap got %v", g)
	}
	// Big move → drag (scroll wins over tap).
	b := NewArena(0, 0, now)
	if g := b.Move(0, 50, now.Add(50*time.Millisecond), 1); g != GestureDrag {
		t.Fatalf("want drag got %v", g)
	}
	if g := b.Up(0, 50, now.Add(80*time.Millisecond)); g != GestureDrag {
		t.Fatalf("decided drag must stick, got %v", g)
	}
}

func TestArenaLongPressAndPinch(t *testing.T) {
	now := time.Now()
	a := NewArena(5, 5, now)
	if g := a.Move(5, 5, now.Add(600*time.Millisecond), 1); g != GestureLongPress {
		t.Fatalf("want longpress got %v", g)
	}
	p := NewArena(0, 0, now)
	p.SetScaleStart(100)
	if g := p.Move(10, 10, now.Add(50*time.Millisecond), 2); g != GestureScale {
		t.Fatalf("want scale got %v", g)
	}
	q := NewArena(0, 0, now)
	if g := q.Move(10, 10, now.Add(50*time.Millisecond), 2); g != GesturePan {
		t.Fatalf("want pan got %v", g)
	}
}

func TestScrollPhysicsSettles(t *testing.T) {
	s := NewScrollSim(0, 1000)
	s.Fling(2000)
	settled := false
	for i := 0; i < 1000; i++ {
		if s.Step(1.0 / 60) {
			settled = true
			break
		}
	}
	if !settled {
		t.Fatal("fling must settle")
	}
	if s.Pos < 0 || s.Pos > 1000 {
		t.Fatalf("clamped: %v", s.Pos)
	}
}

func TestTarget48(t *testing.T) {
	if !TargetOK(48, 48) || TargetOK(47.9, 48) || TargetOK(48, 24) {
		t.Fatal("48dp gate broken")
	}
}

func TestFocusScopeTabOrder(t *testing.T) {
	f := NewFocusScope("a", "b", "c")
	if f.Current() != "a" || f.Next() != "b" || f.Next() != "c" || f.Next() != "a" {
		t.Fatal("tab order broken")
	}
	if f.Prev() != "c" {
		t.Fatal("shift+tab broken")
	}
}

func TestKeyboard(t *testing.T) {
	if !Activates(KeyEnter) || !Activates(KeySpace) || Activates(KeyEscape) {
		t.Fatal("activation keys broken")
	}
	if !Dismisses(KeyEscape) {
		t.Fatal("esc broken")
	}
}

func TestAvroCompose(t *testing.T) {
	var c Composer
	for _, ch := range []byte("ka") {
		c.Type(ch)
	}
	if out := c.Commit(); out == "" {
		t.Fatal("compose must produce output")
	}
	var d Composer
	for _, ch := range []byte("kha") {
		d.Type(ch)
	}
	if d.Commit() == "" {
		t.Fatal("kha must compose")
	}
}

func TestHapticStub(t *testing.T) {
	h := &StubHaptic{}
	h.Pulse(HapticTick)
	h.Pulse(HapticConfirm)
	if len(h.Pulses) != 2 {
		t.Fatal("haptic stub broken")
	}
}
