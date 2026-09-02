package animation

import "math"

// EasingFunc defines an easing function
type EasingFunc func(t float64) float64

// Standard easing functions
var (
	// Linear
	Linear EasingFunc = func(t float64) float64 {
		return t
	}

	// Quadratic
	EaseInQuad EasingFunc = func(t float64) float64 {
		return t * t
	}
	EaseOutQuad EasingFunc = func(t float64) float64 {
		return t * (2 - t)
	}
	EaseInOutQuad EasingFunc = func(t float64) float64 {
		if t < 0.5 {
			return 2 * t * t
		}
		return -1 + (4-2*t)*t
	}

	// Cubic
	EaseInCubic EasingFunc = func(t float64) float64 {
		return t * t * t
	}
	EaseOutCubic EasingFunc = func(t float64) float64 {
		t--
		return t*t*t + 1
	}
	EaseInOutCubic EasingFunc = func(t float64) float64 {
		if t < 0.5 {
			return 4 * t * t * t
		}
		t--
		return 1 + 4*t*t*t
	}

	// Quartic
	EaseInQuart EasingFunc = func(t float64) float64 {
		return t * t * t * t
	}
	EaseOutQuart EasingFunc = func(t float64) float64 {
		t--
		return 1 - t*t*t*t
	}
	EaseInOutQuart EasingFunc = func(t float64) float64 {
		if t < 0.5 {
			return 8 * t * t * t * t
		}
		t = t*2 - 2
		return (8 - t*t*t*t) / 2
	}

	// Elastic
	EaseInElastic EasingFunc = func(t float64) float64 {
		if t == 0 || t == 1 {
			return t
		}
		return -math.Pow(2, 10*(t-1)) * math.Sin((t-1.1)*5*math.Pi)
	}
	EaseOutElastic EasingFunc = func(t float64) float64 {
		if t == 0 || t == 1 {
			return t
		}
		return math.Pow(2, -10*t)*math.Sin((t-0.1)*5*math.Pi) + 1
	}
	EaseInOutElastic EasingFunc = func(t float64) float64 {
		if t == 0 || t == 1 {
			return t
		}
		t *= 2
		if t < 1 {
			return -0.5 * math.Pow(2, 10*(t-1)) * math.Sin((t-1.1)*5*math.Pi)
		}
		return 0.5*math.Pow(2, -10*(t-1))*math.Sin((t-1.1)*5*math.Pi) + 1
	}

	// Bounce
	EaseInBounce EasingFunc = func(t float64) float64 {
		return 1 - EaseOutBounce(1-t)
	}
	EaseOutBounce EasingFunc = func(t float64) float64 {
		if t < 1/2.75 {
			return 7.5625 * t * t
		} else if t < 2/2.75 {
			t -= 1.5 / 2.75
			return 7.5625*t*t + 0.75
		} else if t < 2.5/2.75 {
			t -= 2.25 / 2.75
			return 7.5625*t*t + 0.9375
		}
		t -= 2.625 / 2.75
		return 7.5625*t*t + 0.984375
	}
	EaseInOutBounce EasingFunc = func(t float64) float64 {
		if t < 0.5 {
			return EaseInBounce(t*2) * 0.5
		}
		return EaseOutBounce(t*2-1)*0.5 + 0.5
	}

	// Back
	EaseInBack EasingFunc = func(t float64) float64 {
		s := 1.70158
		return t * t * ((s+1)*t - s)
	}
	EaseOutBack EasingFunc = func(t float64) float64 {
		s := 1.70158
		t--
		return t*t*((s+1)*t+s) + 1
	}
	EaseInOutBack EasingFunc = func(t float64) float64 {
		s := 1.70158 * 1.525
		t *= 2
		if t < 1 {
			return 0.5 * (t * t * ((s+1)*t - s))
		}
		t -= 2
		return 0.5 * (t*t*((s+1)*t+s) + 2)
	}

	// Sine
	EaseInSine EasingFunc = func(t float64) float64 {
		return 1 - math.Cos(t*math.Pi/2)
	}
	EaseOutSine EasingFunc = func(t float64) float64 {
		return math.Sin(t * math.Pi / 2)
	}
	EaseInOutSine EasingFunc = func(t float64) float64 {
		return -(math.Cos(math.Pi*t) - 1) / 2
	}

	// Exponential
	EaseInExpo EasingFunc = func(t float64) float64 {
		if t == 0 {
			return 0
		}
		return math.Pow(2, 10*(t-1))
	}
	EaseOutExpo EasingFunc = func(t float64) float64 {
		if t == 1 {
			return 1
		}
		return 1 - math.Pow(2, -10*t)
	}
	EaseInOutExpo EasingFunc = func(t float64) float64 {
		if t == 0 {
			return 0
		}
		if t == 1 {
			return 1
		}
		if t < 0.5 {
			return math.Pow(2, 20*t-10) / 2
		}
		return (2 - math.Pow(2, -20*t+10)) / 2
	}

	// Circular
	EaseInCirc EasingFunc = func(t float64) float64 {
		return 1 - math.Sqrt(1-t*t)
	}
	EaseOutCirc EasingFunc = func(t float64) float64 {
		t--
		return math.Sqrt(1 - t*t)
	}
	EaseInOutCirc EasingFunc = func(t float64) float64 {
		if t < 0.5 {
			return (1 - math.Sqrt(1-4*t*t)) / 2
		}
		t = t*2 - 2
		return (math.Sqrt(1-t*t) + 1) / 2
	}
)

// GetEasingByName returns an easing function by name
func GetEasingByName(name string) EasingFunc {
	easings := map[string]EasingFunc{
		"linear":           Linear,
		"easeInQuad":       EaseInQuad,
		"easeOutQuad":      EaseOutQuad,
		"easeInOutQuad":    EaseInOutQuad,
		"easeInCubic":      EaseInCubic,
		"easeOutCubic":     EaseOutCubic,
		"easeInOutCubic":   EaseInOutCubic,
		"easeInQuart":      EaseInQuart,
		"easeOutQuart":     EaseOutQuart,
		"easeInOutQuart":   EaseInOutQuart,
		"easeInElastic":    EaseInElastic,
		"easeOutElastic":   EaseOutElastic,
		"easeInOutElastic": EaseInOutElastic,
		"easeInBounce":     EaseInBounce,
		"easeOutBounce":    EaseOutBounce,
		"easeInOutBounce":  EaseInOutBounce,
		"easeInBack":       EaseInBack,
		"easeOutBack":      EaseOutBack,
		"easeInOutBack":    EaseInOutBack,
		"easeInSine":       EaseInSine,
		"easeOutSine":      EaseOutSine,
		"easeInOutSine":    EaseInOutSine,
		"easeInExpo":       EaseInExpo,
		"easeOutExpo":      EaseOutExpo,
		"easeInOutExpo":    EaseInOutExpo,
		"easeInCirc":       EaseInCirc,
		"easeOutCirc":      EaseOutCirc,
		"easeInOutCirc":    EaseInOutCirc,
	}

	if fn, ok := easings[name]; ok {
		return fn
	}
	return Linear
}
