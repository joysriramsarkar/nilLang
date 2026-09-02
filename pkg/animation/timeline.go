package animation

import (
	"sync"
	"time"
)

// Timeline manages multiple animations
type Timeline struct {
	mu          sync.Mutex
	animations  []*TimelineEntry
	currentTime float64
	isPlaying   bool
	fps         int
	onFrame     func(time float64)
}

// TimelineEntry represents an animation in the timeline
type TimelineEntry struct {
	Animation *Animation
	StartTime float64
	EndTime   float64
	Completed bool
}

// NewTimeline creates a new animation timeline
func NewTimeline(fps int) *Timeline {
	if fps <= 0 {
		fps = 60
	}
	return &Timeline{
		animations: []*TimelineEntry{},
		fps:        fps,
	}
}

// Add adds an animation to the timeline
func (tl *Timeline) Add(anim *Animation, startTime float64) *Timeline {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	entry := &TimelineEntry{
		Animation: anim,
		StartTime: startTime,
		EndTime:   startTime + anim.Delay + anim.Duration,
		Completed: false,
	}

	tl.animations = append(tl.animations, entry)
	return tl
}

// Play starts playing the timeline
func (tl *Timeline) Play() {
	tl.mu.Lock()
	tl.isPlaying = true
	tl.mu.Unlock()

	frameDuration := time.Second / time.Duration(tl.fps)

	for {
		tl.mu.Lock()
		if !tl.isPlaying {
			tl.mu.Unlock()
			break
		}

		// Check if all animations are complete
		allComplete := true
		for _, entry := range tl.animations {
			if !entry.Completed {
				allComplete = false
				break
			}
		}

		if allComplete && len(tl.animations) > 0 {
			tl.isPlaying = false
			tl.mu.Unlock()
			break
		}

		// Update current time
		tl.currentTime += frameDuration.Seconds()

		// Process animations
		for _, entry := range tl.animations {
			if entry.Completed {
				continue
			}

			anim := entry.Animation
			localTime := tl.currentTime - entry.StartTime - anim.Delay

			if localTime < 0 {
				continue // Not started yet
			}

			if localTime >= anim.Duration {
				if anim.Loop {
					if anim.LoopCount == -1 || anim.LoopCount > 1 {
						// Loop
						localTime = float64(int(localTime/anim.Duration)) * anim.Duration
						if anim.LoopCount > 0 {
							anim.LoopCount--
						}
					} else {
						entry.Completed = true
						if anim.OnComplete != nil {
							anim.OnComplete()
						}
						continue
					}
				} else {
					entry.Completed = true
					if anim.OnComplete != nil {
						anim.OnComplete()
					}
					continue
				}
			}

			// Normalize time to 0-1
			normalizedTime := localTime / anim.Duration

			// Get values at this time
			values := anim.GetValuesAt(normalizedTime)

			// Call update callback
			if anim.OnUpdate != nil {
				anim.OnUpdate(normalizedTime)
			}

			// Apply values (in a real implementation, this would update UI elements)
			_ = values
		}

		// Call frame callback
		if tl.onFrame != nil {
			tl.onFrame(tl.currentTime)
		}

		tl.mu.Unlock()

		time.Sleep(frameDuration)
	}
}

// Stop stops the timeline
func (tl *Timeline) Stop() {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.isPlaying = false
}

// Pause pauses the timeline
func (tl *Timeline) Pause() {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.isPlaying = false
}

// Resume resumes the timeline
func (tl *Timeline) Resume() {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.isPlaying = true
}

// Reset resets the timeline to the beginning
func (tl *Timeline) Reset() {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.currentTime = 0
	for _, entry := range tl.animations {
		entry.Completed = false
	}
}

// SetOnFrame sets the frame callback
func (tl *Timeline) SetOnFrame(fn func(time float64)) *Timeline {
	tl.onFrame = fn
	return tl
}

// GetTotalDuration returns the total duration of all animations
func (tl *Timeline) GetTotalDuration() float64 {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	var maxEnd float64
	for _, entry := range tl.animations {
		if entry.EndTime > maxEnd {
			maxEnd = entry.EndTime
		}
	}
	return maxEnd
}