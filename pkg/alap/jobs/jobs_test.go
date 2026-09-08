package jobs

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
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

func TestDurableJobStore(t *testing.T) {
	pool, err := data.OpenSQLite(t.TempDir() + "/jobs_test.db")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer pool.Close()

	runner := data.NewRealMigrationRunner(pool)
	data.RegisterPOSMigrations(runner)
	if err := runner.Up(); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	store := NewDurableJobStore(pool)

	// 1. Enqueue job
	rec, err := store.Enqueue("SYNC_SALES", `{"batch_size": 50}`, 3)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if rec.Status != JobStatusPending {
		t.Fatalf("Expected status PENDING, got %s", rec.Status)
	}

	// 2. FetchNextPending
	fetched, err := store.FetchNextPending()
	if err != nil {
		t.Fatalf("FetchNextPending: %v", err)
	}
	if fetched.ID != rec.ID {
		t.Fatalf("Expected job ID %s, got %s", rec.ID, fetched.ID)
	}
	if fetched.Status != JobStatusProcessing {
		t.Fatalf("Expected status PROCESSING, got %s", fetched.Status)
	}

	// 3. MarkCompleted
	if err := store.MarkCompleted(fetched.ID); err != nil {
		t.Fatalf("MarkCompleted: %v", err)
	}

	// 4. Verify no more pending
	_, err = store.FetchNextPending()
	if err == nil {
		t.Fatalf("Expected no pending jobs after completion")
	}
}
