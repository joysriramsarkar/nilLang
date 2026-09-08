package sync

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
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

func TestDurableSyncQueueSQLite(t *testing.T) {
	pool, err := data.OpenSQLite(t.TempDir() + "/durable_sync.db")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer pool.Close()

	runner := data.NewRealMigrationRunner(pool)
	data.RegisterPOSMigrations(runner)
	if err := runner.Up(); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	queue := NewDurableSyncQueue(pool)
	serverEngine := NewSyncEngine()
	serverEngine.SetDB(pool)

	syncedOps := 0
	serverEngine.RegisterHandler("sale", func(op *MutationOperation) error {
		syncedOps++
		return nil
	})

	op := &MutationOperation{
		OperationID: "op-durable-101",
		DeviceID:    "pos-reg-01",
		EntityType:  "sale",
		Operation:   "checkout",
		Version:     1,
		Payload: map[string]interface{}{
			"invoice": "INV-DUR-01",
			"total":   968000,
		},
	}

	// 1. Enqueue into SQLite
	if err := queue.Enqueue(op); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	total, pending, _, _, err := queue.Count()
	if err != nil || total != 1 || pending != 1 {
		t.Fatalf("Expected 1 total, 1 pending; got total=%d, pending=%d, err=%v", total, pending, err)
	}

	// 2. Fetch pending from SQLite
	pendingList, err := queue.Pending(10)
	if err != nil || len(pendingList) != 1 {
		t.Fatalf("Expected 1 pending op, got %d (err: %v)", len(pendingList), err)
	}
	if pendingList[0].OperationID != "op-durable-101" {
		t.Fatalf("Expected op-durable-101, got %s", pendingList[0].OperationID)
	}

	// 3. Process with serverEngine backed by SQLite
	successes, failures, err := serverEngine.Process(pendingList)
	if err != nil || len(failures) > 0 || len(successes) != 1 {
		t.Fatalf("Process failed: err=%v, successes=%v, failures=%v", err, successes, failures)
	}
	_ = queue.MarkSynced(op.OperationID)

	_, pendingAfter, synced, _, _ := queue.Count()
	if pendingAfter != 0 || synced != 1 {
		t.Fatalf("Expected 0 pending, 1 synced; got pending=%d, synced=%d", pendingAfter, synced)
	}

	// 4. Test DB-level server idempotency (crash/restart simulation)
	newServerEngine := NewSyncEngine()
	newServerEngine.SetDB(pool)
	newServerEngine.RegisterHandler("sale", func(op *MutationOperation) error {
		syncedOps++
		return nil
	})

	successes2, _, _ := newServerEngine.Process(pendingList)
	if len(successes2) != 1 {
		t.Fatalf("Expected duplicate operation to be recognized as already processed")
	}
	if syncedOps != 1 {
		t.Fatalf("syncedOps must remain 1 due to DB idempotency, got %d", syncedOps)
	}
}

