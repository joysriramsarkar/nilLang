package sync

import (
	"path/filepath"
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

func TestInventoryDeltaConflictResolution(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "conflict_inventory.db")
	pool, err := data.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer pool.Close()

	runner := data.NewRealMigrationRunner(pool)
	data.RegisterPOSMigrations(runner)
	if err := runner.Up(); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	// Insert product with stock = 50
	nowStr := "2026-09-08T12:00:00Z"
	_, err = pool.Exec(`INSERT INTO categories (id, name, created_at, updated_at) VALUES ('cat-01', 'Grains', ?, ?)`, nowStr, nowStr)
	if err != nil {
		t.Fatalf("insert category: %v", err)
	}
	_, err = pool.Exec(`INSERT INTO products (id, sku, barcode, name, unit, category_id, price_minor, cost_minor, stock_raw, active, created_at, updated_at)
		VALUES ('prod-rice-01', 'SKU-RICE-01', 'BC-RICE-01', 'Miniket Rice', 'BAG', 'cat-01', 340000, 290000, ?, 1, ?, ?)`, 50*data.DecimalScale, nowStr, nowStr)
	if err != nil {
		t.Fatalf("insert product: %v", err)
	}

	resolver := NewConflictResolver(pool)

	// Register A sold 5 units offline (delta: -5)
	deltaA := data.NewDecimalFromInt(-5)
	newStockA, err := resolver.ResolveInventoryDelta("prod-rice-01", deltaA)
	if err != nil {
		t.Fatalf("resolve delta A: %v", err)
	}
	if newStockA.Cmp(data.NewDecimalFromInt(45)) != 0 {
		t.Fatalf("expected stock 45 after delta A, got %s", newStockA.String())
	}

	// Register B sold 8 units offline (delta: -8) arriving subsequently
	deltaB := data.NewDecimalFromInt(-8)
	newStockB, err := resolver.ResolveInventoryDelta("prod-rice-01", deltaB)
	if err != nil {
		t.Fatalf("resolve delta B: %v", err)
	}
	if newStockB.Cmp(data.NewDecimalFromInt(37)) != 0 {
		t.Fatalf("expected stock 37 after delta B, got %s", newStockB.String())
	}

	// Verify DB state
	var stockRaw int64
	_ = pool.QueryRow("SELECT stock_raw FROM products WHERE id = 'prod-rice-01'").Scan(&stockRaw)
	if stockRaw != 37*data.DecimalScale {
		t.Fatalf("expected final DB stock %d, got %d", 37*data.DecimalScale, stockRaw)
	}
}

func TestPriceLWWConflictResolution(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "conflict_price.db")
	pool, err := data.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer pool.Close()

	runner := data.NewRealMigrationRunner(pool)
	data.RegisterPOSMigrations(runner)
	if err := runner.Up(); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	nowStr := "2026-09-08T12:00:00Z"
	_, _ = pool.Exec(`INSERT INTO categories (id, name, created_at, updated_at) VALUES ('cat-02', 'Oils', ?, ?)`, nowStr, nowStr)
	_, _ = pool.Exec(`INSERT INTO products (id, sku, barcode, name, unit, category_id, price_minor, cost_minor, stock_raw, version, active, created_at, updated_at)
		VALUES ('prod-oil-01', 'SKU-OIL-01', 'BC-OIL-01', 'Soybean Oil', 'BTL', 'cat-02', 80000, 70000, ?, 5, 1, ?, ?)`, 20*data.DecimalScale, nowStr, nowStr)

	resolver := NewConflictResolver(pool)

	// 1. Stale client mutation with version 4 (Server is already at version 5)
	applied, err := resolver.ResolvePriceProductLWW("prod-oil-01", 4, 75000)
	if err != nil {
		t.Fatalf("resolve LWW: %v", err)
	}
	if applied {
		t.Fatalf("expected stale version 4 to be rejected, but it was applied")
	}

	// 2. Newer client mutation with version 6
	applied2, err := resolver.ResolvePriceProductLWW("prod-oil-01", 6, 89000)
	if err != nil {
		t.Fatalf("resolve newer LWW: %v", err)
	}
	if !applied2 {
		t.Fatalf("expected newer version 6 to be applied, but it was rejected")
	}

	var newPrice, newVersion int64
	_ = pool.QueryRow("SELECT price_minor, version FROM products WHERE id = 'prod-oil-01'").Scan(&newPrice, &newVersion)
	if newPrice != 89000 || newVersion != 6 {
		t.Fatalf("expected price 89000 and version 6, got price=%d, version=%d", newPrice, newVersion)
	}
}

func TestDurableSyncQueuePowerOffCrashRecovery(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "crash_recovery_sync.db")

	// Phase 1: Open DB, enqueue 3 mutations, then abruptly simulate power off (close DB)
	{
		pool1, err := data.OpenSQLite(dbPath)
		if err != nil {
			t.Fatalf("OpenSQLite 1: %v", err)
		}

		runner := data.NewRealMigrationRunner(pool1)
		data.RegisterPOSMigrations(runner)
		_ = runner.Up()

		q1 := NewDurableSyncQueue(pool1)
		for i := 1; i <= 3; i++ {
			err := q1.Enqueue(&MutationOperation{
				OperationID: filepath.Base(dbPath) + string(rune('A'+i)),
				DeviceID:    "reg-crash-01",
				EntityType:  "sale",
				Operation:   "checkout",
				Payload:     map[string]interface{}{"order_no": i},
			})
			if err != nil {
				t.Fatalf("enqueue op %d: %v", i, err)
			}
		}
		// Power cut simulation: close pool abruptly
		pool1.Close()
	}

	// Phase 2: System reboots, reopen DB from disk
	{
		pool2, err := data.OpenSQLite(dbPath)
		if err != nil {
			t.Fatalf("OpenSQLite 2 (after crash): %v", err)
		}
		defer pool2.Close()

		q2 := NewDurableSyncQueue(pool2)
		total, pending, _, _, err := q2.Count()
		if err != nil {
			t.Fatalf("Count after crash: %v", err)
		}
		if total != 3 || pending != 3 {
			t.Fatalf("expected 3 pending operations intact after crash, got total=%d, pending=%d", total, pending)
		}

		pendingOps, err := q2.Pending(10)
		if err != nil || len(pendingOps) != 3 {
			t.Fatalf("expected 3 pending ops retrieved, got %d (err: %v)", len(pendingOps), err)
		}
	}
}
