package sync

import (
	"testing"
)

func TestOfflineSyncQueueAndIdempotency(t *testing.T) {
	queue := NewSyncQueue()
	engine := NewSyncEngine()

	appliedSales := 0
	engine.RegisterHandler("sale", func(op *MutationOperation) error {
		appliedSales++
		return nil
	})

	op1 := &MutationOperation{
		OperationID: "op-101",
		DeviceID:    "terminal-1",
		EntityType:  "sale",
		Operation:   "checkout",
		Payload: map[string]interface{}{
			"invoice": "INV-001",
			"total":   340000,
		},
	}
	queue.Enqueue(op1)

	total, pending := queue.Count()
	if total != 1 || pending != 1 {
		t.Fatalf("Expected 1 total, 1 pending; got total=%d, pending=%d", total, pending)
	}

	// 1. Process sync
	successes, failures, err := engine.Process(queue.Pending())
	if err != nil || len(failures) > 0 || len(successes) != 1 {
		t.Fatalf("Sync failed: err=%v, successes=%d, failures=%d", err, len(successes), len(failures))
	}
	if appliedSales != 1 {
		t.Fatalf("Expected 1 sale applied, got %d", appliedSales)
	}

	// 2. Process duplicate identical operation_id (Simulating network retry)
	successes2, _, _ := engine.Process([]*MutationOperation{op1})
	if len(successes2) != 1 {
		t.Fatalf("Expected duplicate operation to be acknowledged")
	}
	// Applied count should STILL be 1 (Idempotent!)
	if appliedSales != 1 {
		t.Fatalf("Expected appliedSales to remain 1 after duplicate, got %d", appliedSales)
	}
}
