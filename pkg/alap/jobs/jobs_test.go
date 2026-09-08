package jobs

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestJobScheduler(t *testing.T) {
	scheduler := NewScheduler()
	defer scheduler.StopAll()

	var counter atomic.Int64
	testJob := JobFunc{
		JobName: "sync-job",
		Fn: func(ctx context.Context) error {
			counter.Add(1)
			return nil
		},
	}

	scheduler.Register(testJob)

	// RunOnce
	err := scheduler.RunOnce("sync-job")
	if err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}
	if counter.Load() != 1 {
		t.Fatalf("Expected counter to be 1, got %d", counter.Load())
	}

	// Schedule every 15ms
	err = scheduler.ScheduleEvery("sync-job", 15*time.Millisecond)
	if err != nil {
		t.Fatalf("ScheduleEvery failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	scheduler.StopJob("sync-job")

	if counter.Load() < 2 {
		t.Fatalf("Expected periodic runs to increase counter, got %d", counter.Load())
	}
}
