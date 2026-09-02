package animation

import (
	"math"
	"testing"
)

func TestEasingFunctions(t *testing.T) {
	tests := []struct {
		name   string
		easing EasingFunc
	}{
		{"Linear", Linear},
		{"EaseInQuad", EaseInQuad},
		{"EaseOutQuad", EaseOutQuad},
		{"EaseInOutCubic", EaseInOutCubic},
		{"EaseOutBounce", EaseOutBounce},
		{"EaseOutElastic", EaseOutElastic},
	}

	for _, tt := range tests {
		startVal := tt.easing(0.0)
		endVal := tt.easing(1.0)

		if math.Abs(startVal) > 0.001 {
			t.Errorf("%s: expected f(0) ~ 0, got %f", tt.name, startVal)
		}
		if math.Abs(endVal-1.0) > 0.001 {
			t.Errorf("%s: expected f(1) ~ 1, got %f", tt.name, endVal)
		}
	}
}

func TestKeyframeInterpolation(t *testing.T) {
	track := NewKeyframeTrack("opacity")
	track.AddKeyframe(NewKeyframe(0.0).SetValue("opacity", 0.0).SetEasing(Linear))
	track.AddKeyframe(NewKeyframe(1.0).SetValue("opacity", 100.0))

	vMid := track.GetValueAt(0.5)
	if math.Abs(vMid-50.0) > 0.01 {
		t.Errorf("expected 50.0 at midpoint, got %f", vMid)
	}

	vEnd := track.GetValueAt(1.0)
	if math.Abs(vEnd-100.0) > 0.01 {
		t.Errorf("expected 100.0 at end, got %f", vEnd)
	}
}

func TestAnimationTracks(t *testing.T) {
	anim := NewAnimation("fade_in", 2.0)
	track := NewKeyframeTrack("alpha")
	track.AddKeyframe(NewKeyframe(0.0).SetValue("alpha", 0.0))
	track.AddKeyframe(NewKeyframe(2.0).SetValue("alpha", 1.0))
	anim.AddTrack(track)

	vals := anim.GetValuesAt(1.0)
	alpha, ok := vals["alpha"]
	if !ok {
		t.Fatalf("missing alpha in animated values")
	}

	if math.Abs(alpha-0.5) > 0.01 {
		t.Errorf("expected alpha 0.5 at midpoint, got %f", alpha)
	}
}
