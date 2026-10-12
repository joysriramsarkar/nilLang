package animation

import (
	"testing"
)

func TestApplyValuesToTarget(t *testing.T) {
	tg := NewMapTarget()
	a := FadeIn(1.0)
	a.Target = tg
	// Simulate one tick at t=0.5.
	ApplyValues(tg, a.GetValuesAt(0.5))
	if got := tg.GetProperty("opacity"); got != 0.5 {
		t.Fatalf("opacity must apply 0.5, got %v", got)
	}
}

func TestSchedulerTickAndCancel(t *testing.T) {
	s := NewScheduler(60)
	tg := NewMapTarget()
	a := FadeIn(1.0)
	sc := s.Start(a, tg, nil)
	for i := 0; i < 30; i++ {
		s.Tick(1.0 / 60)
	}
	if got := tg.GetProperty("opacity"); got <= 0 || got >= 1 {
		t.Fatalf("mid-flight opacity in (0,1): %v", got)
	}
	s.Cancel(sc)
	before := tg.GetProperty("opacity")
	s.Tick(1.0)
	if tg.GetProperty("opacity") != before {
		t.Fatal("cancelled animation must freeze")
	}
	if s.Pending() != 0 {
		t.Fatal("cancelled item must leave the queue")
	}
}

func TestSchedulerCompletes(t *testing.T) {
	s := NewScheduler(60)
	tg := NewMapTarget()
	done := false
	s.Start(FadeIn(0.5), tg, func() { done = true })
	for i := 0; i < 60; i++ {
		s.Tick(1.0 / 60)
	}
	if tg.GetProperty("opacity") != 1 || !done {
		t.Fatalf("must complete at 1: %v done=%v", tg.GetProperty("opacity"), done)
	}
}

func TestSequenceComposition(t *testing.T) {
	a := FadeIn(1.0)
	b := FadeOut(1.0)
	seq := Sequence(a, b)
	if seq.Duration != 2.0 {
		t.Fatalf("sequence duration: %v", seq.Duration)
	}
	// First half fades in, second half fades out.
	if v := seq.ValuesAt(0.25)["opacity"]; v != 0.5 {
		t.Fatalf("seq t=0.25: %v", v)
	}
	if v := seq.ValuesAt(0.75)["opacity"]; v != 0.5 {
		t.Fatalf("seq t=0.75: %v", v)
	}
	if v := seq.ValuesAt(1)["opacity"]; v != 0 {
		t.Fatalf("seq end: %v", v)
	}
}

func TestParallelComposition(t *testing.T) {
	a := FadeIn(2.0)
	b := SlideIn(1.0, 0, 100)
	par := Parallel(a, b)
	if par.Duration != 2.0 {
		t.Fatalf("parallel duration: %v", par.Duration)
	}
	v := par.ValuesAt(0.5)
	if v["opacity"] != 0.5 || v["x"] != 100 {
		t.Fatalf("parallel values: %v", v)
	}
}

func TestImplicit(t *testing.T) {
	tg := NewMapTarget()
	tg.SetProperty("x", 10)
	a := Implicit("x", 20, 1.0)
	ResolveImplicit(a, tg)
	ApplyValues(tg, a.GetValuesAt(0.5))
	if got := tg.GetProperty("x"); got != 15 {
		t.Fatalf("implicit midpoint: %v", got)
	}
}

func TestTransition(t *testing.T) {
	enter := Transition(true, 1.0)
	if v := enter.GetValuesAt(0)["opacity"]; v != 0 {
		t.Fatalf("enter starts hidden: %v", v)
	}
	exit := Transition(false, 1.0)
	if v := exit.GetValuesAt(1)["opacity"]; v != 0 {
		t.Fatalf("exit ends hidden: %v", v)
	}
}

func TestSpringSettles(t *testing.T) {
	s := NewSpring(0, 100, 60)
	settled := false
	for i := 0; i < 10000; i++ {
		if s.Step(1.0 / 60) {
			settled = true
			break
		}
	}
	if !settled {
		t.Fatal("spring must settle")
	}
	if s.X < 99 || s.X > 101 {
		t.Fatalf("spring must land near target: %v", s.X)
	}
}

func TestKeyframeMultiProperty(t *testing.T) {
	a := NewAnimation("multi", 1.0)
	tx := NewKeyframeTrack("x")
	tx.AddKeyframe(NewKeyframe(0).SetValue("x", 0))
	tx.AddKeyframe(NewKeyframe(1).SetValue("x", 10))
	to := NewKeyframeTrack("opacity")
	to.AddKeyframe(NewKeyframe(0).SetValue("opacity", 1))
	to.AddKeyframe(NewKeyframe(1).SetValue("opacity", 0))
	a.AddTrack(tx).AddTrack(to)
	v := a.GetValuesAt(0.5)
	if v["x"] != 5 || v["opacity"] != 0.5 {
		t.Fatalf("keyframe interp broken: %v", v)
	}
}

func TestStagger(t *testing.T) {
	items := Stagger(FadeIn(1.0), 3, 0.5)
	if items[0].Delay != 0 || items[1].Delay != 0.5 || items[2].Delay != 1.0 {
		t.Fatalf("stagger offsets broken: %v %v %v", items[0].Delay, items[1].Delay, items[2].Delay)
	}
}

func TestHero(t *testing.T) {
	h := Hero(HeroRect{0, 0, 50, 50}, HeroRect{100, 200, 150, 150}, 1.0)
	v := h.GetValuesAt(0.5)
	if v["x"] != 50 || v["y"] != 100 || v["width"] != 100 || v["height"] != 100 {
		t.Fatalf("hero midpoint broken: %v", v)
	}
}

func TestReducedMotion(t *testing.T) {
	old := ReducedMotion
	ReducedMotion = true
	defer func() { ReducedMotion = old }()
	s := NewScheduler(60)
	tg := NewMapTarget()
	s.Start(FadeIn(10.0), tg, nil)
	s.Tick(1.0 / 60)
	if got := tg.GetProperty("opacity"); got != 1 {
		t.Fatalf("reduced motion must snap to end: %v", got)
	}
}
