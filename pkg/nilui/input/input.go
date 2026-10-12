// Package input implements U6: pointer fan-out + HitTest, gesture arena
// (tap-vs-scroll), Tap/Drag/LongPress/Scale/Pan, scroll physics, 48x48dp
// touch targets, FocusScope + tab order, keyboard, IME/Bangla composing
// (Avro/Gboard phonetic), and haptic stubs.
package input

import (
	"math"
	"sort"
	"time"
)

// ─── Pointer events + HitTest (U6.1) ────────────────────────────────────────

// PointerKind is touch/mouse/stylus.
type PointerKind int

const (
	// PointerTouch is a finger.
	PointerTouch PointerKind = iota
	// PointerMouse is a mouse.
	PointerMouse
	// PointerStylus is a stylus.
	PointerStylus
)

// PointerEvent is one low-level event.
type PointerEvent struct {
	Kind     PointerKind
	X, Y     float64
	Down     bool
	Time     time.Time
	TargetID string
}

// Rect is a hit rect.
type Rect struct{ X, Y, W, H float64 }

// HitNode is a hittable node.
type HitNode struct {
	ID       string
	Rect     Rect
	Depth    int
	Children []*HitNode
}

// HitTest returns the deepest node containing (x,y), or "".
func HitTest(root *HitNode, x, y float64) string {
	best := ""
	bestDepth := -1
	var walk func(n *HitNode)
	walk = func(n *HitNode) {
		if n == nil {
			return
		}
		if x >= n.Rect.X && x < n.Rect.X+n.Rect.W && y >= n.Rect.Y && y < n.Rect.Y+n.Rect.H {
			if n.Depth >= bestDepth && n.ID != "" {
				best, bestDepth = n.ID, n.Depth
			}
			for _, c := range n.Children {
				walk(c)
			}
		}
	}
	walk(root)
	return best
}

// FanOut delivers an event to every subscriber of its target.
func FanOut(subs map[string][]func(PointerEvent), e PointerEvent) int {
	n := 0
	for _, fn := range subs[e.TargetID] {
		fn(e)
		n++
	}
	return n
}

// ─── Gesture arena (U6.2/U6.3) ─────────────────────────────────────────────

// Gesture is a recognized gesture.
type Gesture int

const (
	// GestureNone undecided.
	GestureNone Gesture = iota
	// GestureTap is tap.
	GestureTap
	// GestureDrag is drag.
	GestureDrag
	// GestureLongPress is long-press.
	GestureLongPress
	// GestureScale is pinch scale.
	GestureScale
	// GesturePan is two-finger pan.
	GesturePan
)

// Thresholds (dp/ms).
const (
	TapSlopDp    = 10.0
	TapTimeoutMs = 200
	LongPressMs  = 500
	DragSlopDp   = 12.0
	MinTargetDp  = 48.0 // U6.5: minimum touch target
)

// Arena resolves competing recognizers: tap-vs-scroll etc.
type Arena struct {
	startX, startY float64
	start          time.Time
	moved          float64
	pointers       int
	decided        Gesture
	scaleStart     float64
}

// NewArena starts tracking a press.
func NewArena(x, y float64, now time.Time) *Arena {
	return &Arena{startX: x, startY: y, start: now, pointers: 1}
}

// Move feeds movement; returns decided gesture (or None while ambiguous).
func (a *Arena) Move(x, y float64, now time.Time, pointers int) Gesture {
	if a.decided != GestureNone && a.decided != GestureDrag && a.decided != GestureScale && a.decided != GesturePan {
		return a.decided
	}
	a.moved = math.Hypot(x-a.startX, y-a.startY)
	a.pointers = pointers
	if pointers >= 2 {
		a.decided = GesturePan
		if a.scaleStart > 0 {
			a.decided = GestureScale
		}
		return a.decided
	}
	if a.moved > DragSlopDp {
		a.decided = GestureDrag
		return a.decided
	}
	if now.Sub(a.start).Milliseconds() >= LongPressMs && a.moved <= TapSlopDp {
		a.decided = GestureLongPress
		return a.decided
	}
	return GestureNone
}

// Up ends the press; tap iff quick + inside slop.
func (a *Arena) Up(x, y float64, now time.Time) Gesture {
	if a.decided != GestureNone {
		d := a.decided
		return d
	}
	if now.Sub(a.start).Milliseconds() <= TapTimeoutMs && math.Hypot(x-a.startX, y-a.startY) <= TapSlopDp {
		return GestureTap
	}
	return GestureNone
}

// SetScaleStart records pinch start distance.
func (a *Arena) SetScaleStart(d float64) { a.scaleStart = d }

// ─── Scroll physics (U6.4) ──────────────────────────────────────────────────

// ScrollSim is a 1-D fling simulator with friction.
type ScrollSim struct {
	Pos, Vel float64
	Min, Max float64
	Friction float64 // per-second decay, e.g. 4.0
}

// NewScrollSim creates a simulator.
func NewScrollSim(min, max float64) *ScrollSim { return &ScrollSim{Min: min, Max: max, Friction: 4.0} }

// Fling sets velocity.
func (s *ScrollSim) Fling(v float64) { s.Vel = v }

// Step advances dt seconds; returns settled.
func (s *ScrollSim) Step(dt float64) bool {
	s.Pos += s.Vel * dt
	s.Vel *= math.Exp(-s.Friction * dt)
	if s.Pos < s.Min {
		s.Pos, s.Vel = s.Min, 0
	}
	if s.Pos > s.Max {
		s.Pos, s.Vel = s.Max, 0
	}
	if math.Abs(s.Vel) < 0.5 {
		s.Vel = 0
		return true
	}
	return false
}

// ─── Touch targets (U6.5) ───────────────────────────────────────────────────

// TargetOK reports whether a rect meets the 48×48dp minimum.
func TargetOK(wDp, hDp float64) bool { return wDp >= MinTargetDp && hDp >= MinTargetDp }

// ─── Focus (U6.6) ───────────────────────────────────────────────────────────

// FocusScope orders keyboard focus (tab order).
type FocusScope struct {
	Order []string
	Index int
}

// NewFocusScope creates a scope.
func NewFocusScope(ids ...string) *FocusScope { return &FocusScope{Order: append([]string{}, ids...)} }

// Current returns the focused id.
func (f *FocusScope) Current() string {
	if len(f.Order) == 0 {
		return ""
	}
	return f.Order[f.Index%len(f.Order)]
}

// Next advances (Tab); Prev goes back (Shift+Tab).
func (f *FocusScope) Next() string {
	if len(f.Order) == 0 {
		return ""
	}
	f.Index = (f.Index + 1) % len(f.Order)
	return f.Current()
}

// Prev retreats.
func (f *FocusScope) Prev() string {
	if len(f.Order) == 0 {
		return ""
	}
	f.Index = (f.Index - 1 + len(f.Order)) % len(f.Order)
	return f.Current()
}

// ─── Keyboard (U6.7) ────────────────────────────────────────────────────────

// Key names.
const (
	KeyEnter  = "Enter"
	KeySpace  = "Space"
	KeyEscape = "Escape"
	KeyTab    = "Tab"
)

// Activates reports whether a key activates a focused control.
func Activates(key string) bool { return key == KeyEnter || key == KeySpace }

// Dismisses reports whether a key dismisses (Esc).
func Dismisses(key string) bool { return key == KeyEscape }

// ─── IME / Bangla composing (U6.8) ──────────────────────────────────────────

// AvroMap is a minimal phonetic map (Avro-style) for composing tests.
var AvroMap = map[string]string{
	"a": "আ", "i": "ই", "u": "উ", "e": "এ", "o": "ও",
	"k": "ক", "kh": "খ", "g": "গ", "c": "চ", "j": "জ",
	"T": "ট", "D": "ড", "t": "ত", "d": "দ", "n": "ন",
	"p": "প", "b": "ব", "m": "ম", "r": "র", "l": "ল",
	"s": "স", "h": "হ", "y": "য়",
	"ka": "কা", "ki": "কি", "kI": "কী", "ku": "কু", "ke": "কে",
	"kha": "খা", "ga": "গা", "O": "ও",
}

// Composer holds IME composing state.
type Composer struct {
	Latin    string
	Composed string
	Done     bool
}

// Type feeds a latin char; longest-match wins.
func (c *Composer) Type(ch byte) {
	c.Latin += string(ch)
	best := ""
	bestLen := 0
	for k, v := range AvroMap {
		if len(k) > bestLen && len(c.Latin) >= len(k) && c.Latin[len(c.Latin)-len(k):] == k {
			best, bestLen = v, len(k)
		}
	}
	if best != "" {
		c.Composed += best
	}
}

// Commit finishes composing.
func (c *Composer) Commit() string {
	c.Done = true
	return c.Composed
}

// ─── Haptics (U6.9) ─────────────────────────────────────────────────────────

// HapticKind names feedback.
type HapticKind int

const (
	// HapticTick is a light tick.
	HapticTick HapticKind = iota
	// HapticConfirm is a confirm pulse.
	HapticConfirm
)

// Haptic is the device interface (stub in tests).
type Haptic interface{ Pulse(HapticKind) }

// StubHaptic records pulses.
type StubHaptic struct{ Pulses []HapticKind }

// Pulse records.
func (s *StubHaptic) Pulse(k HapticKind) { s.Pulses = append(s.Pulses, k) }

// SortedStrings sorts for deterministic asserts.
func SortedStrings(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}
