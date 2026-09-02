package animation

import (
	"fmt"
	"sync"
)

// Engine is the main animation engine
type Engine struct {
	mu         sync.RWMutex
	timelines  map[string]*Timeline
	sprites    map[string]*Sprite
	fps        int
	isRunning  bool
	stopChan   chan struct{}
}

// NewEngine creates a new animation engine
func NewEngine(fps int) *Engine {
	if fps <= 0 {
		fps = 60
	}
	return &Engine{
		timelines: make(map[string]*Timeline),
		sprites:   make(map[string]*Sprite),
		fps:       fps,
		stopChan:  make(chan struct{}),
	}
}

// CreateTimeline creates a new timeline
func (e *Engine) CreateTimeline(name string) *Timeline {
	e.mu.Lock()
	defer e.mu.Unlock()

	timeline := NewTimeline(e.fps)
	e.timelines[name] = timeline
	return timeline
}

// GetTimeline returns a timeline by name
func (e *Engine) GetTimeline(name string) *Timeline {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.timelines[name]
}

// CreateSprite creates a new sprite
func (e *Engine) CreateSprite(name string) *Sprite {
	e.mu.Lock()
	defer e.mu.Unlock()

	sprite := NewSprite(name)
	e.sprites[name] = sprite
	return sprite
}

// GetSprite returns a sprite by name
func (e *Engine) GetSprite(name string) *Sprite {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.sprites[name]
}

// Start starts the animation engine
func (e *Engine) Start() {
	e.mu.Lock()
	if e.isRunning {
		e.mu.Unlock()
		return
	}
	e.isRunning = true
	e.mu.Unlock()

	fmt.Println("🎬 Animation Engine started")
}

// Stop stops the animation engine
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.isRunning {
		return
	}

	e.isRunning = false
	close(e.stopChan)
	fmt.Println("🎬 Animation Engine stopped")
}

// --- Animation Builders (Fluent API) ---

// FadeIn creates a fade-in animation
func FadeIn(duration float64) *Animation {
	anim := NewAnimation("fadeIn", duration)
	track := NewKeyframeTrack("opacity")
	track.AddKeyframe(NewKeyframe(0).SetValue("opacity", 0))
	track.AddKeyframe(NewKeyframe(1).SetValue("opacity", 1))
	anim.AddTrack(track)
	return anim
}

// FadeOut creates a fade-out animation
func FadeOut(duration float64) *Animation {
	anim := NewAnimation("fadeOut", duration)
	track := NewKeyframeTrack("opacity")
	track.AddKeyframe(NewKeyframe(0).SetValue("opacity", 1))
	track.AddKeyframe(NewKeyframe(1).SetValue("opacity", 0))
	anim.AddTrack(track)
	return anim
}

// SlideIn creates a slide-in animation
func SlideIn(duration float64, fromX, toX float64) *Animation {
	anim := NewAnimation("slideIn", duration)
	track := NewKeyframeTrack("x")
	track.AddKeyframe(NewKeyframe(0).SetValue("x", fromX).SetEasing(EaseOutCubic))
	track.AddKeyframe(NewKeyframe(1).SetValue("x", toX))
	anim.AddTrack(track)
	return anim
}

// SlideUp creates a slide-up animation
func SlideUp(duration float64, fromY, toY float64) *Animation {
	anim := NewAnimation("slideUp", duration)
	track := NewKeyframeTrack("y")
	track.AddKeyframe(NewKeyframe(0).SetValue("y", fromY).SetEasing(EaseOutCubic))
	track.AddKeyframe(NewKeyframe(1).SetValue("y", toY))
	anim.AddTrack(track)
	return anim
}

// Scale creates a scale animation
func Scale(duration float64, fromScale, toScale float64) *Animation {
	anim := NewAnimation("scale", duration)
	track := NewKeyframeTrack("scale")
	track.AddKeyframe(NewKeyframe(0).SetValue("scale", fromScale).SetEasing(EaseOutBack))
	track.AddKeyframe(NewKeyframe(1).SetValue("scale", toScale))
	anim.AddTrack(track)
	return anim
}

// Rotate creates a rotation animation
func Rotate(duration float64, fromDeg, toDeg float64) *Animation {
	anim := NewAnimation("rotate", duration)
	track := NewKeyframeTrack("rotation")
	track.AddKeyframe(NewKeyframe(0).SetValue("rotation", fromDeg))
	track.AddKeyframe(NewKeyframe(1).SetValue("rotation", toDeg))
	anim.AddTrack(track)
	return anim
}

// Bounce creates a bounce animation
func Bounce(duration float64, height float64) *Animation {
	anim := NewAnimation("bounce", duration)
	track := NewKeyframeTrack("y")
	track.AddKeyframe(NewKeyframe(0).SetValue("y", 0))
	track.AddKeyframe(NewKeyframe(0.5).SetValue("y", -height).SetEasing(EaseOutQuad))
	track.AddKeyframe(NewKeyframe(1).SetValue("y", 0).SetEasing(EaseInBounce))
	anim.AddTrack(track)
	return anim
}

// Pulse creates a pulse animation
func Pulse(duration float64, scale float64) *Animation {
	anim := NewAnimation("pulse", duration)
	track := NewKeyframeTrack("scale")
	track.AddKeyframe(NewKeyframe(0).SetValue("scale", 1))
	track.AddKeyframe(NewKeyframe(0.5).SetValue("scale", scale).SetEasing(EaseInOutSine))
	track.AddKeyframe(NewKeyframe(1).SetValue("scale", 1))
	anim.AddTrack(track)
	anim.SetLoop(-1) // Infinite loop
	return anim
}

// Shake creates a shake animation
func Shake(duration float64, intensity float64) *Animation {
	anim := NewAnimation("shake", duration)
	track := NewKeyframeTrack("x")
	track.AddKeyframe(NewKeyframe(0).SetValue("x", 0))
	track.AddKeyframe(NewKeyframe(0.1).SetValue("x", -intensity))
	track.AddKeyframe(NewKeyframe(0.2).SetValue("x", intensity))
	track.AddKeyframe(NewKeyframe(0.3).SetValue("x", -intensity))
	track.AddKeyframe(NewKeyframe(0.4).SetValue("x", intensity))
	track.AddKeyframe(NewKeyframe(0.5).SetValue("x", -intensity*0.5))
	track.AddKeyframe(NewKeyframe(0.6).SetValue("x", intensity*0.5))
	track.AddKeyframe(NewKeyframe(0.7).SetValue("x", -intensity*0.25))
	track.AddKeyframe(NewKeyframe(0.8).SetValue("x", intensity*0.25))
	track.AddKeyframe(NewKeyframe(1).SetValue("x", 0))
	anim.AddTrack(track)
	return anim
}

// Typewriter creates a typewriter effect (for text)
func Typewriter(duration float64, totalChars int) *Animation {
	anim := NewAnimation("typewriter", duration)
	track := NewKeyframeTrack("visibleChars")
	track.AddKeyframe(NewKeyframe(0).SetValue("visibleChars", 0))
	track.AddKeyframe(NewKeyframe(1).SetValue("visibleChars", float64(totalChars)))
	anim.AddTrack(track)
	return anim
}

// Sequence creates a sequence of animations
func Sequence(animations ...*Animation) *Animation {
	if len(animations) == 0 {
		return NewAnimation("empty", 0)
	}

	totalDuration := 0.0
	for _, anim := range animations {
		totalDuration += anim.Delay + anim.Duration
	}

	sequence := NewAnimation("sequence", totalDuration)
	// In a full implementation, this would chain animations
	return sequence
}

// Parallel creates parallel animations
func Parallel(animations ...*Animation) *Animation {
	if len(animations) == 0 {
		return NewAnimation("empty", 0)
	}

	maxDuration := 0.0
	for _, anim := range animations {
		d := anim.Delay + anim.Duration
		if d > maxDuration {
			maxDuration = d
		}
	}

	parallel := NewAnimation("parallel", maxDuration)
	// In a full implementation, this would run animations in parallel
	return parallel
}