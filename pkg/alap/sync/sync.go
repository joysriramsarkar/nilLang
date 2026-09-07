package sync

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// MutationStatus defines state of local mutation
type MutationStatus string

const (
	StatusPending MutationStatus = "pending"
	StatusSynced  MutationStatus = "synced"
	StatusFailed  MutationStatus = "failed"
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
	ErrorMsg    string                 `json:"error_msg,omitempty"`
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
			op.Status = StatusFailed
			op.ErrorMsg = reason
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

// ─── IDEMPOTENT SERVER RECEIVER ─────────────────────────────────────────────

// SyncEngine handles server-side idempotent reception and application
type SyncEngine struct {
	mu               sync.Mutex
	processedOps     map[string]*MutationOperation
	handlers         map[string]func(op *MutationOperation) error
}

// NewSyncEngine creates server sync engine
func NewSyncEngine() *SyncEngine {
	return &SyncEngine{
		processedOps: make(map[string]*MutationOperation),
		handlers:     make(map[string]func(op *MutationOperation) error),
	}
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
		// 1. Idempotency check: If already processed, acknowledge without re-running
		if _, exists := se.processedOps[op.OperationID]; exists {
			successes = append(successes, op.OperationID)
			continue
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
			successes = append(successes, op.OperationID)
		}
	}

	return successes, failures, nil
}
