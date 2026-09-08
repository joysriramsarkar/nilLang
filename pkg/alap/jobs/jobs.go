package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Job defines an executable background task
type Job interface {
	Name() string
	Run(ctx context.Context) error
}

// JobFunc adapts a function to the Job interface
type JobFunc struct {
	JobName string
	Fn      func(ctx context.Context) error
}

func (jf JobFunc) Name() string {
	return jf.JobName
}

func (jf JobFunc) Run(ctx context.Context) error {
	return jf.Fn(ctx)
}

// JobStatus tracks execution metrics for a background job
type JobStatus struct {
	Name      string    `json:"name"`
	Running   bool      `json:"running"`
	RunCount  int64     `json:"run_count"`
	LastRun   time.Time `json:"last_run"`
	LastError string    `json:"last_error,omitempty"`
}

// Scheduler manages scheduled background routines (e.g. database sync, backup)
type Scheduler struct {
	mu      sync.RWMutex
	jobs    map[string]Job
	status  map[string]*JobStatus
	cancels map[string]context.CancelFunc
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewScheduler creates a background job scheduler
func NewScheduler() *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		jobs:    make(map[string]Job),
		status:  make(map[string]*JobStatus),
		cancels: make(map[string]context.CancelFunc),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Register registers a job
func (s *Scheduler) Register(job Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.Name()] = job
	s.status[job.Name()] = &JobStatus{
		Name: job.Name(),
	}
}

// RunOnce executes a job synchronously or asynchronously once
func (s *Scheduler) RunOnce(name string) error {
	s.mu.RLock()
	job, exists := s.jobs[name]
	stat := s.status[name]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("job %s not found", name)
	}

	s.mu.Lock()
	stat.Running = true
	stat.LastRun = time.Now()
	s.mu.Unlock()

	err := job.Run(s.ctx)

	s.mu.Lock()
	stat.Running = false
	stat.RunCount++
	if err != nil {
		stat.LastError = err.Error()
	} else {
		stat.LastError = ""
	}
	s.mu.Unlock()

	return err
}

// ScheduleEvery schedules a job to run periodically at the given interval
func (s *Scheduler) ScheduleEvery(name string, interval time.Duration) error {
	s.mu.Lock()
	job, exists := s.jobs[name]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("job %s not found", name)
	}

	// Cancel existing if any
	if c, ok := s.cancels[name]; ok {
		c()
	}

	jobCtx, jobCancel := context.WithCancel(s.ctx)
	s.cancels[name] = jobCancel
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-jobCtx.Done():
				return
			case <-ticker.C:
				s.mu.Lock()
				if st, ok := s.status[name]; ok {
					st.Running = true
					st.LastRun = time.Now()
				}
				s.mu.Unlock()

				err := job.Run(jobCtx)

				s.mu.Lock()
				if st, ok := s.status[name]; ok {
					st.Running = false
					st.RunCount++
					if err != nil {
						st.LastError = err.Error()
					} else {
						st.LastError = ""
					}
				}
				s.mu.Unlock()
			}
		}
	}()

	return nil
}

// StopJob stops a scheduled job
func (s *Scheduler) StopJob(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.cancels[name]; ok {
		c()
		delete(s.cancels, name)
	}
}

// StopAll halts all background jobs
func (s *Scheduler) StopAll() {
	s.cancel()
}

// Statuses returns status of all jobs
func (s *Scheduler) Statuses() map[string]JobStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make(map[string]JobStatus)
	for k, v := range s.status {
		res[k] = *v
	}
	return res
}

// ─── STANDARD PRODUCTION BACKGROUND JOBS (web-implications.md Section 22) ───

// NewSyncJob creates a job that flushes offline queues to central servers
func NewSyncJob(syncHandler func(ctx context.Context) error) Job {
	return JobFunc{
		JobName: "SyncJob",
		Fn: func(ctx context.Context) error {
			if syncHandler != nil {
				return syncHandler(ctx)
			}
			return nil
		},
	}
}

// NewBackupJob creates a periodic database snapshot/backup job
func NewBackupJob(backupHandler func(ctx context.Context) error) Job {
	return JobFunc{
		JobName: "BackupJob",
		Fn: func(ctx context.Context) error {
			if backupHandler != nil {
				return backupHandler(ctx)
			}
			return nil
		},
	}
}

// NewReceiptRetryJob creates a job to retry pending thermal print requests
func NewReceiptRetryJob(retryHandler func(ctx context.Context) error) Job {
	return JobFunc{
		JobName: "ReceiptRetryJob",
		Fn: func(ctx context.Context) error {
			if retryHandler != nil {
				return retryHandler(ctx)
			}
			return nil
		},
	}
}

// NewLowStockJob creates a job that evaluates inventory against minimum thresholds
func NewLowStockJob(checkHandler func(ctx context.Context) error) Job {
	return JobFunc{
		JobName: "LowStockJob",
		Fn: func(ctx context.Context) error {
			if checkHandler != nil {
				return checkHandler(ctx)
			}
			return nil
		},
	}
}

// NewReportJob creates a job that aggregates daily financial reconciliations
func NewReportJob(reportHandler func(ctx context.Context) error) Job {
	return JobFunc{
		JobName: "ReportJob",
		Fn: func(ctx context.Context) error {
			if reportHandler != nil {
				return reportHandler(ctx)
			}
			return nil
		},
	}
}

// NewCleanupJob creates a job that purges stale sync ops, sessions and logs
func NewCleanupJob(cleanupHandler func(ctx context.Context) error) Job {
	return JobFunc{
		JobName: "CleanupJob",
		Fn: func(ctx context.Context) error {
			if cleanupHandler != nil {
				return cleanupHandler(ctx)
			}
			return nil
		},
	}
}
