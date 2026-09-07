package jobs

import (
	"context"
	"testing"
	"time"
)

func TestJobScheduler(t *testing.T) {
	scheduler := NewScheduler()
	defer scheduler.StopAll()

	counter := 0
	testJob := JobFunc{
		JobName: "sync-job",
		Fn: func(ctx context.Context) error {
			counter++
			return nil
		},
	}

	scheduler.Register(testJob)

	// RunOnce
	err := scheduler.RunOnce("sync-job")
	if err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}
	if counter != 1 {
		t.Fatalf("Expected counter to be 1, got %d", counter)
	}

	// Schedule every 10ms
	err = scheduler.ScheduleEvery("sync-job", 15*time.Millisecond)
	if err != nil {
		t.Fatalf("ScheduleEvery failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	scheduler.StopJob("sync-job")

	if counter < 2 {
		t.Fatalf("Expected periodic runs to increase counter, got %d", counter)
	}
}
