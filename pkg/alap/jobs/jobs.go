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

	stat.Running = true
	stat.LastRun = time.Now()
	err := job.Run(s.ctx)
	stat.Running = false
	stat.RunCount++
	if err != nil {
		stat.LastError = err.Error()
	} else {
		stat.LastError = ""
	}
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
	stat := s.status[name]
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-jobCtx.Done():
				return
			case <-ticker.C:
				stat.Running = true
				stat.LastRun = time.Now()
				err := job.Run(jobCtx)
				stat.Running = false
				stat.RunCount++
				if err != nil {
					stat.LastError = err.Error()
				} else {
					stat.LastError = ""
				}
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
