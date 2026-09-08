package sync

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

// MutationStatus defines state of local mutation
type MutationStatus string

const (
	StatusPending    MutationStatus = "pending"
	StatusSynced     MutationStatus = "synced"
	StatusFailed     MutationStatus = "failed"
	StatusConflict   MutationStatus = "conflict"
	StatusDeadLetter MutationStatus = "dead_letter"
)

// MutationOperation represents an offline atomic operation to sync with central server
type MutationOperation struct {
	OperationID string                 `json:"operation_id"`
	DeviceID    string                 `json:"device_id"`
	EntityID    string                 `json:"entity_id"`
	EntityType  string                 `json:"entity_type"`
	Operation   string                 `json:"operation"` // e.g. "create", "update", "checkout"
	Version     int64                  `json:"version"`
	Timestamp   int64                  `json:"timestamp"`
	Payload     map[string]interface{} `json:"payload"`
	Status      MutationStatus         `json:"status"`
	Attempt     int                    `json:"attempt,omitempty"`
	ErrorMsg    string                 `json:"error_msg,omitempty"`
	RetryAfter  *time.Time             `json:"retry_after,omitempty"`
}

// GenerateOperationID generates a cryptographic unique operation identifier
func GenerateOperationID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// SyncQueue manages local mutations when offline and batches them to upstream
type SyncQueue struct {
	mu         sync.RWMutex
	operations []*MutationOperation
}

// NewSyncQueue creates a new offline queue
func NewSyncQueue() *SyncQueue {
	return &SyncQueue{
		operations: make([]*MutationOperation, 0),
	}
}

// Enqueue adds an offline mutation
func (sq *SyncQueue) Enqueue(op *MutationOperation) {
	sq.mu.Lock()
	defer sq.mu.Unlock()

	if op.OperationID == "" {
		op.OperationID = GenerateOperationID()
	}
	if op.Timestamp == 0 {
		op.Timestamp = time.Now().Unix()
	}
	op.Status = StatusPending

	sq.operations = append(sq.operations, op)
}

// Pending returns all un-synced operations
func (sq *SyncQueue) Pending() []*MutationOperation {
	sq.mu.RLock()
	defer sq.mu.RUnlock()

	pending := make([]*MutationOperation, 0)
	for _, op := range sq.operations {
		if op.Status == StatusPending {
			pending = append(pending, op)
		}
	}
	return pending
}

// MarkSynced marks an operation as successfully synchronized
func (sq *SyncQueue) MarkSynced(operationID string) {
	sq.mu.Lock()
	defer sq.mu.Unlock()

	for _, op := range sq.operations {
		if op.OperationID == operationID {
			op.Status = StatusSynced
			break
		}
	}
}

// MarkFailed marks an operation as failed with reason
func (sq *SyncQueue) MarkFailed(operationID, reason string) {
	sq.mu.Lock()
	defer sq.mu.Unlock()

	for _, op := range sq.operations {
		if op.OperationID == operationID {
			op.Attempt++
			op.ErrorMsg = reason
			if op.Attempt >= 5 {
				op.Status = StatusDeadLetter
			} else {
				op.Status = StatusFailed
			}
			break
		}
	}
}

// Count returns number of total and pending operations
func (sq *SyncQueue) Count() (total int, pending int) {
	sq.mu.RLock()
	defer sq.mu.RUnlock()

	total = len(sq.operations)
	for _, op := range sq.operations {
		if op.Status == StatusPending {
			pending++
		}
	}
	return
}

// ─── DURABLE SQLITE-BACKED SYNC QUEUE ───────────────────────────────────────

// DurableSyncQueue provides a persistent sync queue stored in SQLite.
// Survives system crashes and power loss.
type DurableSyncQueue struct {
	db *data.RealDBPool
	mu sync.Mutex
}

// NewDurableSyncQueue creates a durable sync queue backed by SQLite.
func NewDurableSyncQueue(db *data.RealDBPool) *DurableSyncQueue {
	return &DurableSyncQueue{db: db}
}

// Enqueue inserts an offline mutation into the local database table.
func (dq *DurableSyncQueue) Enqueue(op *MutationOperation) error {
	dq.mu.Lock()
	defer dq.mu.Unlock()

	if op.OperationID == "" {
		op.OperationID = GenerateOperationID()
	}
	now := time.Now()
	nowStr := now.UTC().Format(time.RFC3339)
	payloadJSON, err := json.Marshal(op.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	id := fmt.Sprintf("sync-%s", op.OperationID)
	_, err = dq.db.Exec(
		`INSERT INTO sync_operations (id, operation_id, device_id, entity_type, entity_id, version, payload, status, attempt, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?)`,
		id, op.OperationID, op.DeviceID, op.EntityType, op.EntityID, op.Version, string(payloadJSON), string(StatusPending), nowStr,
	)
	if err != nil {
		return fmt.Errorf("insert sync_operation: %w", err)
	}
	op.Status = StatusPending
	return nil
}

// Pending returns up to `limit` un-synced operations ordered by creation time.
func (dq *DurableSyncQueue) Pending(limit int) ([]*MutationOperation, error) {
	dq.mu.Lock()
	defer dq.mu.Unlock()

	if limit <= 0 {
		limit = 50
	}

	rows, err := dq.db.Query(
		`SELECT operation_id, device_id, entity_type, entity_id, version, payload, status, attempt, last_error
		 FROM sync_operations WHERE status = ? ORDER BY created_at ASC LIMIT ?`,
		string(StatusPending), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query pending sync: %w", err)
	}
	defer rows.Close()

	ops := make([]*MutationOperation, 0)
	for rows.Next() {
		var op MutationOperation
		var payloadStr string
		var lastErr *string
		var status string
		err := rows.Scan(&op.OperationID, &op.DeviceID, &op.EntityType, &op.EntityID, &op.Version, &payloadStr, &status, &op.Attempt, &lastErr)
		if err != nil {
			return nil, err
		}
		op.Status = MutationStatus(status)
		if lastErr != nil {
			op.ErrorMsg = *lastErr
		}
		_ = json.Unmarshal([]byte(payloadStr), &op.Payload)
		ops = append(ops, &op)
	}
	return ops, nil
}

// MarkSynced records successful synchronization to SQLite.
func (dq *DurableSyncQueue) MarkSynced(operationID string) error {
	dq.mu.Lock()
	defer dq.mu.Unlock()

	nowStr := time.Now().UTC().Format(time.RFC3339)
	_, err := dq.db.Exec(
		`UPDATE sync_operations SET status = ?, synced_at = ? WHERE operation_id = ?`,
		string(StatusSynced), nowStr, operationID,
	)
	return err
}

// MarkFailed increments attempt count and records exponential backoff.
func (dq *DurableSyncQueue) MarkFailed(operationID, reason string) error {
	dq.mu.Lock()
	defer dq.mu.Unlock()

	var attempt int
	row := dq.db.QueryRow(`SELECT attempt FROM sync_operations WHERE operation_id = ?`, operationID)
	_ = row.Scan(&attempt)
	attempt++

	newStatus := StatusFailed
	if attempt >= 5 {
		newStatus = StatusDeadLetter // Dead-letter queue after 5 failures
	}

	// Exponential backoff: 1s, 2s, 4s, 8s, 16s
	backoffSecs := 1 << (attempt - 1)
	if backoffSecs > 60 {
		backoffSecs = 60
	}
	retryAfter := time.Now().Add(time.Duration(backoffSecs) * time.Second).UTC().Format(time.RFC3339)

	_, err := dq.db.Exec(
		`UPDATE sync_operations SET status = ?, attempt = ?, last_error = ?, retry_after = ? WHERE operation_id = ?`,
		string(newStatus), attempt, reason, retryAfter, operationID,
	)
	return err
}

// MarkConflict flags an operation as having a version or vector clock conflict.
func (dq *DurableSyncQueue) MarkConflict(operationID, reason string) error {
	dq.mu.Lock()
	defer dq.mu.Unlock()

	_, err := dq.db.Exec(
		`UPDATE sync_operations SET status = ?, last_error = ? WHERE operation_id = ?`,
		string(StatusConflict), reason, operationID,
	)
	return err
}

// Count returns statistics from the database table.
func (dq *DurableSyncQueue) Count() (total, pending, synced, failed int, err error) {
	dq.mu.Lock()
	defer dq.mu.Unlock()

	row := dq.db.QueryRow(`SELECT 
		COUNT(*),
		COUNT(CASE WHEN status = 'pending' THEN 1 END),
		COUNT(CASE WHEN status = 'synced' THEN 1 END),
		COUNT(CASE WHEN status = 'failed' OR status = 'dead_letter' THEN 1 END)
		FROM sync_operations`)
	err = row.Scan(&total, &pending, &synced, &failed)
	return
}

// ─── IDEMPOTENT SERVER RECEIVER ─────────────────────────────────────────────

// SyncEngine handles server-side idempotent reception and application
type SyncEngine struct {
	mu           sync.Mutex
	db           *data.RealDBPool
	processedOps map[string]*MutationOperation
	handlers     map[string]func(op *MutationOperation) error
}

// NewSyncEngine creates server sync engine
func NewSyncEngine() *SyncEngine {
	return &SyncEngine{
		processedOps: make(map[string]*MutationOperation),
		handlers:     make(map[string]func(op *MutationOperation) error),
	}
}

// SetDB configures real database persistence for server idempotency.
func (se *SyncEngine) SetDB(db *data.RealDBPool) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.db = db
}

// RegisterHandler registers an entity processor
func (se *SyncEngine) RegisterHandler(entityType string, handler func(op *MutationOperation) error) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.handlers[entityType] = handler
}

// Process processes incoming batch idempotently
func (se *SyncEngine) Process(ops []*MutationOperation) ([]string, []string, error) {
	se.mu.Lock()
	defer se.mu.Unlock()

	successes := make([]string, 0)
	failures := make([]string, 0)

	for _, op := range ops {
		// 1. Idempotency check: in-memory map
		if _, exists := se.processedOps[op.OperationID]; exists {
			successes = append(successes, op.OperationID)
			continue
		}

		// 2. Idempotency check: persistent processed_operations table
		if se.db != nil {
			var count int
			row := se.db.QueryRow(`SELECT COUNT(*) FROM processed_operations WHERE operation_id = ?`, op.OperationID)
			if err := row.Scan(&count); err == nil && count > 0 {
				se.processedOps[op.OperationID] = op
				successes = append(successes, op.OperationID)
				continue
			}
		}

		handler, hasHandler := se.handlers[op.EntityType]
		if !hasHandler {
			failures = append(failures, op.OperationID)
			continue
		}

		if err := handler(op); err != nil {
			failures = append(failures, op.OperationID)
		} else {
			se.processedOps[op.OperationID] = op
			if se.db != nil {
				nowStr := time.Now().UTC().Format(time.RFC3339)
				_, _ = se.db.Exec(
					`INSERT OR IGNORE INTO processed_operations (operation_id, processed_at) VALUES (?, ?)`,
					op.OperationID, nowStr,
				)
			}
			successes = append(successes, op.OperationID)
		}
	}

	return successes, failures, nil
}

