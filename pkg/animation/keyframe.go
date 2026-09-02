package animation

// Keyframe represents a single keyframe in an animation
type Keyframe struct {
	Time      float64               // Time in seconds (0.0 - 1.0 normalized)
	Values    map[string]float64    // Property values at this keyframe
	Easing    EasingFunc            // Easing to use until next keyframe
}

// KeyframeTrack represents a sequence of keyframes for a property
type KeyframeTrack struct {
	Property   string
	Keyframes  []*Keyframe
}

// NewKeyframe creates a new keyframe
func NewKeyframe(time float64) *Keyframe {
	return &Keyframe{
		Time:   time,
		Values: make(map[string]float64),
		Easing: Linear,
	}
}

// SetValue sets a property value at this keyframe
func (kf *Keyframe) SetValue(property string, value float64) *Keyframe {
	kf.Values[property] = value
	return kf
}

// SetEasing sets the easing function for this keyframe
func (kf *Keyframe) SetEasing(easing EasingFunc) *Keyframe {
	kf.Easing = easing
	return kf
}

// NewKeyframeTrack creates a new keyframe track
func NewKeyframeTrack(property string) *KeyframeTrack {
	return &KeyframeTrack{
		Property:  property,
		Keyframes: []*Keyframe{},
	}
}

// AddKeyframe adds a keyframe to the track
func (kt *KeyframeTrack) AddKeyframe(kf *Keyframe) *KeyframeTrack {
	kt.Keyframes = append(kt.Keyframes, kf)
	return kt
}

// GetValueAt returns the interpolated value at a given time
func (kt *KeyframeTrack) GetValueAt(time float64) float64 {
	if len(kt.Keyframes) == 0 {
		return 0
	}

	if len(kt.Keyframes) == 1 {
		return kt.Keyframes[0].Values[kt.Property]
	}

	// Find surrounding keyframes
	var prev, next *Keyframe
	for i := 0; i < len(kt.Keyframes); i++ {
		if kt.Keyframes[i].Time <= time {
			prev = kt.Keyframes[i]
		}
		if kt.Keyframes[i].Time >= time && next == nil {
			next = kt.Keyframes[i]
		}
	}

	if prev == nil {
		return kt.Keyframes[0].Values[kt.Property]
	}
	if next == nil {
		return kt.Keyframes[len(kt.Keyframes)-1].Values[kt.Property]
	}
	if prev == next {
		return prev.Values[kt.Property]
	}

	// Interpolate
	duration := next.Time - prev.Time
	if duration == 0 {
		return prev.Values[kt.Property]
	}

	progress := (time - prev.Time) / duration

	// Apply easing
	if prev.Easing != nil {
		progress = prev.Easing(progress)
	}

	startVal := prev.Values[kt.Property]
	endVal := next.Values[kt.Property]

	return startVal + (endVal-startVal)*progress
}

// Animation represents a complete animation
type Animation struct {
	Name       string
	Duration   float64 // Duration in seconds
	Delay      float64 // Delay before start
	Loop       bool
	LoopCount  int     // -1 for infinite
	Tracks     []*KeyframeTrack
	OnComplete func()
	OnUpdate   func(progress float64)
}

// NewAnimation creates a new animation
func NewAnimation(name string, duration float64) *Animation {
	return &Animation{
		Name:      name,
		Duration:  duration,
		Delay:     0,
		Loop:      false,
		LoopCount: 1,
		Tracks:    []*KeyframeTrack{},
	}
}

// AddTrack adds a keyframe track to the animation
func (a *Animation) AddTrack(track *KeyframeTrack) *Animation {
	a.Tracks = append(a.Tracks, track)
	return a
}

// SetLoop sets the animation to loop
func (a *Animation) SetLoop(count int) *Animation {
	a.Loop = true
	a.LoopCount = count
	return a
}

// SetDelay sets the delay before animation starts
func (a *Animation) SetDelay(delay float64) *Animation {
	a.Delay = delay
	return a
}

// GetValuesAt returns all property values at a given time
func (a *Animation) GetValuesAt(time float64) map[string]float64 {
	values := make(map[string]float64)
	for _, track := range a.Tracks {
		values[track.Property] = track.GetValueAt(time)
	}
	return values
}