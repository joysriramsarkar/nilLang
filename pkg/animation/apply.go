package animation

import (
	"math"
	"sync"
	"time"
)

// U9 — animation application layer.
//
// The core fix (U9.1): computed values are APPLIED to bound targets instead
// of discarded (`_ = values`). Every entry may carry a Target; the scheduler
// writes each tick's values into it. ReducedMotion (U9.10) short-circuits
// every animation to its end state.

// Target receives animated property values.
type Target interface {
	SetProperty(name string, value float64)
	GetProperty(name string) float64
}

// MapTarget is an in-memory target (tests, previews, headless UI).
type MapTarget struct {
	mu     sync.RWMutex
	Values map[string]float64
}

// NewMapTarget creates a target.
func NewMapTarget() *MapTarget { return &MapTarget{Values: map[string]float64{}} }

// SetProperty writes.
func (m *MapTarget) SetProperty(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Values[name] = value
}

// GetProperty reads.
func (m *MapTarget) GetProperty(name string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Values[name]
}

// ApplyValues writes every value into t (U9.1 fix point).
func ApplyValues(t Target, values map[string]float64) {
	if t == nil {
		return
	}
	for k, v := range values {
		if p := GetProperty(k); p != nil {
			v = p.Clamp(v)
		}
		t.SetProperty(k, v)
	}
}

// ReducedMotion, when true, snaps every animation to its end state (U9.10,
// a11y gate U11.6).
var ReducedMotion = false

// ─── Tick-based cancellable scheduler (U9.2) ────────────────────────────────

// Scheduled is one running animation.
type Scheduled struct {
	Anim      *Animation
	Target    Target
	Elapsed   float64
	Done      bool
	Cancelled bool
	OnDone    func()
}

// Scheduler steps animations deterministically (Tick) or asynchronously
// (Play). No blocking Sleep on the caller: Play runs on its own goroutine
// and Stop cancels via channel.
type Scheduler struct {
	mu     sync.Mutex
	items  []*Scheduled
	stopCh chan struct{}
	wg     sync.WaitGroup
	fps    int
}

// NewScheduler creates a scheduler.
func NewScheduler(fps int) *Scheduler {
	if fps <= 0 {
		fps = 60
	}
	return &Scheduler{fps: fps, stopCh: make(chan struct{})}
}

// Start schedules an animation on a target.
func (s *Scheduler) Start(a *Animation, t Target, onDone func()) *Scheduled {
	sc := &Scheduled{Anim: a, Target: t, OnDone: onDone}
	s.mu.Lock()
	s.items = append(s.items, sc)
	s.mu.Unlock()
	return sc
}

// Cancel stops a scheduled animation.
func (s *Scheduler) Cancel(sc *Scheduled) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sc.Cancelled = true
	sc.Done = true
}

// Tick advances every animation by dt seconds (deterministic; tests use this).
func (s *Scheduler) Tick(dt float64) {
	s.mu.Lock()
	items := append([]*Scheduled{}, s.items...)
	s.mu.Unlock()
	var live []*Scheduled
	for _, sc := range items {
		if sc.Done || sc.Cancelled {
			continue
		}
		a := sc.Anim
		if ReducedMotion {
			ApplyValues(sc.Target, a.GetValuesAt(1))
			sc.Done = true
			if sc.OnDone != nil {
				sc.OnDone()
			}
			continue
		}
		sc.Elapsed += dt
		local := sc.Elapsed - a.Delay
		if local < 0 {
			live = append(live, sc)
			continue
		}
		t := 1.0
		if a.Duration > 0 {
			t = local / a.Duration
		}
		if t >= 1 {
			if a.Loop && (a.LoopCount == -1 || a.LoopCount > 1) {
				if a.LoopCount > 0 {
					a.LoopCount--
				}
				sc.Elapsed = a.Delay
				t = 0
			} else {
				t = 1
				ApplyValues(sc.Target, a.GetValuesAt(t))
				if a.OnUpdate != nil {
					a.OnUpdate(t)
				}
				sc.Done = true
				if a.OnComplete != nil {
					a.OnComplete()
				}
				if sc.OnDone != nil {
					sc.OnDone()
				}
				continue
			}
		}
		ApplyValues(sc.Target, a.GetValuesAt(t))
		if a.OnUpdate != nil {
			a.OnUpdate(t)
		}
		live = append(live, sc)
	}
	s.mu.Lock()
	s.items = live
	s.mu.Unlock()
}

// Play runs the scheduler asynchronously until Stop.
func (s *Scheduler) Play() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		tick := time.NewTicker(time.Second / time.Duration(s.fps))
		defer tick.Stop()
		for {
			select {
			case <-s.stopCh:
				return
			case <-tick.C:
				s.Tick(float64(time.Second) / float64(time.Second*time.Duration(s.fps)) / float64(s.fps))
			}
		}
	}()
}

// Stop cancels async play.
func (s *Scheduler) Stop() {
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
	s.wg.Wait()
	s.stopCh = make(chan struct{})
}

// Pending counts live animations.
func (s *Scheduler) Pending() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.items)
}

// ─── Sequence / Parallel (U9.3: real composition) ───────────────────────────

// SequenceOf chains animations end-to-end as one composite.
func SequenceOf(anims ...*Animation) *Animation {
	total := 0.0
	for _, a := range anims {
		total += a.Delay + a.Duration
	}
	out := NewAnimation("sequence", total)
	for _, a := range anims {
		for _, tr := range a.Tracks {
			out.AddTrack(tr)
		}
	}
	out.composite = &composite{kind: compositeSeq, parts: anims}
	return out
}

// ParallelOf overlays animations; duration is the max.
func ParallelOf(anims ...*Animation) *Animation {
	max := 0.0
	for _, a := range anims {
		if d := a.Delay + a.Duration; d > max {
			max = d
		}
	}
	out := NewAnimation("parallel", max)
	out.composite = &composite{kind: compositePar, parts: anims}
	return out
}

type compositeKind int

const (
	compositeSeq compositeKind = iota
	compositePar
)

type composite struct {
	kind  compositeKind
	parts []*Animation
}

// ValuesAt evaluates a composite at normalized time t.
func (a *Animation) ValuesAt(t float64) map[string]float64 {
	if a.composite == nil {
		return a.GetValuesAt(t)
	}
	out := map[string]float64{}
	switch a.composite.kind {
	case compositeSeq:
		total := a.Duration
		elapsed := t * total
		acc := 0.0
		for _, p := range a.composite.parts {
			d := p.Delay + p.Duration
			if elapsed <= acc+d || p == a.composite.parts[len(a.composite.parts)-1] {
				lt := 0.0
				if d > 0 {
					lt = (elapsed - acc - p.Delay) / p.Duration
				}
				lt = clamp01(lt)
				for k, v := range p.GetValuesAt(lt) {
					out[k] = v
				}
				break
			}
			for k, v := range p.GetValuesAt(1) {
				out[k] = v
			}
			acc += d
		}
	case compositePar:
		for _, p := range a.composite.parts {
			lt := 0.0
			elapsed := t * a.Duration
			if p.Duration > 0 {
				lt = (elapsed - p.Delay) / p.Duration
			}
			for k, v := range p.GetValuesAt(clamp01(lt)) {
				out[k] = v
			}
		}
	}
	return out
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// ─── Implicit animation (U9.4) ──────────────────────────────────────────────

// Implicit tweens one property from its current value to `to`.
func Implicit(prop string, to, duration float64) *Animation {
	a := NewAnimation("implicit:"+prop, duration)
	tr := NewKeyframeTrack(prop)
	tr.AddKeyframe(NewKeyframe(0).SetValue(prop, math.NaN()))
	tr.AddKeyframe(NewKeyframe(1).SetValue(prop, to))
	a.AddTrack(tr)
	a.implicitProp = prop
	a.implicitTo = to
	return a
}

// ResolveImplicit fills the NaN start with the target's current value.
func ResolveImplicit(a *Animation, t Target) {
	if a.implicitProp == "" || t == nil {
		return
	}
	for _, tr := range a.Tracks {
		if tr.Property == a.implicitProp && len(tr.Keyframes) > 0 {
			if math.IsNaN(tr.Keyframes[0].Values[tr.Property]) {
				tr.Keyframes[0].Values[tr.Property] = t.GetProperty(tr.Property)
			}
		}
	}
}

// ─── Transition (U9.5) ─────────────────────────────────────────────────────

// Transition is an enter/exit opacity+offset tween.
func Transition(enter bool, duration float64) *Animation {
	a := NewAnimation("transition", duration)
	op := NewKeyframeTrack("opacity")
	off := NewKeyframeTrack("y")
	if enter {
		op.AddKeyframe(NewKeyframe(0).SetValue("opacity", 0))
		op.AddKeyframe(NewKeyframe(1).SetValue("opacity", 1))
		off.AddKeyframe(NewKeyframe(0).SetValue("y", 12))
		off.AddKeyframe(NewKeyframe(1).SetValue("y", 0))
	} else {
		op.AddKeyframe(NewKeyframe(0).SetValue("opacity", 1))
		op.AddKeyframe(NewKeyframe(1).SetValue("opacity", 0))
		off.AddKeyframe(NewKeyframe(0).SetValue("y", 0))
		off.AddKeyframe(NewKeyframe(1).SetValue("y", -12))
	}
	return a.AddTrack(op).AddTrack(off)
}

// ─── Spring physics (U9.6, critically damped) ───────────────────────────────

// Spring integrates x toward target with stiffness/damping (semi-implicit
// Euler). Critical damping: damping = 2*sqrt(stiffness).
type Spring struct {
	X, V      float64
	Target    float64
	Stiffness float64
	Damping   float64
}

// NewSpring creates a spring (critically damped by default).
func NewSpring(x, target, stiffness float64) *Spring {
	return &Spring{X: x, Target: target, Stiffness: stiffness, Damping: 2 * math.Sqrt(stiffness)}
}

// Step advances dt; returns settled.
func (s *Spring) Step(dt float64) bool {
	f := -s.Stiffness*(s.X-s.Target) - s.Damping*s.V
	s.V += f * dt
	s.X += s.V * dt
	return math.Abs(s.X-s.Target) < 0.01 && math.Abs(s.V) < 0.01
}

// ─── Stagger (U9.8) ─────────────────────────────────────────────────────────

// Stagger offsets copies of a template across n items.
func Stagger(template *Animation, n int, gap float64) []*Animation {
	out := make([]*Animation, n)
	for i := 0; i < n; i++ {
		cp := NewAnimation(template.Name, template.Duration)
		cp.Delay = template.Delay + float64(i)*gap
		for _, tr := range template.Tracks {
			cp.AddTrack(tr)
		}
		out[i] = cp
	}
	return out
}

// ─── Hero / shared element (U9.9) ───────────────────────────────────────────

// HeroRect is a matched rectangle.
type HeroRect struct{ X, Y, W, H float64 }

// Hero interpolates start→end rects and writes x/y/width/height.
func Hero(start, end HeroRect, duration float64) *Animation {
	a := NewAnimation("hero", duration)
	mk := func(prop string, from, to float64) {
		tr := NewKeyframeTrack(prop)
		tr.AddKeyframe(NewKeyframe(0).SetValue(prop, from))
		tr.AddKeyframe(NewKeyframe(1).SetValue(prop, to))
		a.AddTrack(tr)
	}
	mk("x", start.X, end.X)
	mk("y", start.Y, end.Y)
	mk("width", start.W, end.W)
	mk("height", start.H, end.H)
	return a
}
