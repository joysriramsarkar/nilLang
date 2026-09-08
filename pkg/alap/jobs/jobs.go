package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
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

// ─── DURABLE SQLITE-BACKED JOB STORE ────────────────────────────────────────

// JobRecordStatus tracks persistent job lifecycle in SQLite
type JobRecordStatus string

const (
	JobStatusPending    JobRecordStatus = "PENDING"
	JobStatusProcessing JobRecordStatus = "PROCESSING"
	JobStatusCompleted  JobRecordStatus = "COMPLETED"
	JobStatusFailed     JobRecordStatus = "FAILED"
)

// JobRecord represents a persistent job row in the job_records table
type JobRecord struct {
	ID          string          `json:"id"`
	JobType     string          `json:"job_type"`
	Status      JobRecordStatus `json:"status"`
	Payload     string          `json:"payload,omitempty"`
	Attempt     int             `json:"attempt"`
	MaxAttempts int             `json:"max_attempts"`
	LastError   string          `json:"last_error,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	FinishedAt  *time.Time      `json:"finished_at,omitempty"`
}

// DurableJobStore manages SQLite-persisted background job queues
type DurableJobStore struct {
	db *data.RealDBPool
	mu sync.Mutex
}

// NewDurableJobStore creates a durable job store backed by SQLite
func NewDurableJobStore(db *data.RealDBPool) *DurableJobStore {
	return &DurableJobStore{db: db}
}

// Enqueue inserts a new job record into SQLite
func (js *DurableJobStore) Enqueue(jobType, payload string, maxAttempts int) (*JobRecord, error) {
	js.mu.Lock()
	defer js.mu.Unlock()

	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	now := time.Now()
	nowStr := now.UTC().Format(time.RFC3339)
	jobID := fmt.Sprintf("job-%d", now.UnixNano())

	rec := &JobRecord{
		ID:          jobID,
		JobType:     jobType,
		Status:      JobStatusPending,
		Payload:     payload,
		MaxAttempts: maxAttempts,
		CreatedAt:   now,
	}

	_, err := js.db.Exec(
		`INSERT INTO job_records (id, job_type, status, payload, attempt, max_attempts, created_at)
		 VALUES (?, ?, ?, ?, 0, ?, ?)`,
		rec.ID, rec.JobType, string(rec.Status), rec.Payload, rec.MaxAttempts, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("enqueue job: %w", err)
	}
	return rec, nil
}

// FetchNextPending retrieves the oldest pending job and marks it PROCESSING
func (js *DurableJobStore) FetchNextPending() (*JobRecord, error) {
	js.mu.Lock()
	defer js.mu.Unlock()

	row := js.db.QueryRow(
		`SELECT id, job_type, status, payload, attempt, max_attempts, created_at
		 FROM job_records WHERE status = ? ORDER BY created_at ASC LIMIT 1`,
		string(JobStatusPending),
	)

	var rec JobRecord
	var status string
	var createdStr string
	err := row.Scan(&rec.ID, &rec.JobType, &status, &rec.Payload, &rec.Attempt, &rec.MaxAttempts, &createdStr)
	if err != nil {
		return nil, err // sql.ErrNoRows if empty
	}
	rec.Status = JobRecordStatus(status)
	rec.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)

	now := time.Now()
	nowStr := now.UTC().Format(time.RFC3339)
	_, _ = js.db.Exec(
		`UPDATE job_records SET status = ?, started_at = ?, attempt = attempt + 1 WHERE id = ?`,
		string(JobStatusProcessing), nowStr, rec.ID,
	)
	rec.Attempt++
	rec.Status = JobStatusProcessing
	rec.StartedAt = &now
	return &rec, nil
}

// MarkCompleted marks a job as successfully finished
func (js *DurableJobStore) MarkCompleted(id string) error {
	js.mu.Lock()
	defer js.mu.Unlock()

	nowStr := time.Now().UTC().Format(time.RFC3339)
	_, err := js.db.Exec(
		`UPDATE job_records SET status = ?, finished_at = ? WHERE id = ?`,
		string(JobStatusCompleted), nowStr, id,
	)
	return err
}

// MarkFailed marks a job as failed, recording the error
func (js *DurableJobStore) MarkFailed(id string, errMsg string) error {
	js.mu.Lock()
	defer js.mu.Unlock()

	nowStr := time.Now().UTC().Format(time.RFC3339)
	_, err := js.db.Exec(
		`UPDATE job_records SET status = ?, last_error = ?, finished_at = ? WHERE id = ?`,
		string(JobStatusFailed), errMsg, nowStr, id,
	)
	return err
}

