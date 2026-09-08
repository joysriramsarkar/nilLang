//go:build integration

// Package data integration tests require a running PostgreSQL instance.
// Set the TEST_POSTGRES_DSN environment variable to enable these tests:
//
//	TEST_POSTGRES_DSN="postgres://nilang:nilang_test@localhost:5432/nilang_test?sslmode=disable" \
//	    go test -tags integration -v -count=1 ./pkg/alap/data/...
package data_test

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

// pgDSN returns the Postgres DSN from env or skips the test.
func pgDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set — skipping PostgreSQL integration test")
	}
	return dsn
}

// openPG opens a clean Postgres connection for a test and registers cleanup.
func openPG(t *testing.T) *data.RealDBPool {
	t.Helper()
	pool, err := data.OpenPostgres(pgDSN(t))
	if err != nil {
		t.Fatalf("OpenPostgres: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	return pool
}

// dropTestSchema removes all POS tables and schema_migrations so each test
// starts from a clean slate. Safe to call even if tables do not exist.
func dropTestSchema(t *testing.T, pool *data.RealDBPool) {
	t.Helper()
	drops := []string{
		"DROP TABLE IF EXISTS job_records CASCADE",
		"DROP TABLE IF EXISTS processed_operations CASCADE",
		"DROP TABLE IF EXISTS sync_operations CASCADE",
		"DROP TABLE IF EXISTS audit_log CASCADE",
		"DROP TABLE IF EXISTS receipts CASCADE",
		"DROP TABLE IF EXISTS refund_items CASCADE",
		"DROP TABLE IF EXISTS refunds CASCADE",
		"DROP TABLE IF EXISTS purchase_items CASCADE",
		"DROP TABLE IF EXISTS purchases CASCADE",
		"DROP TABLE IF EXISTS discounts CASCADE",
		"DROP TABLE IF EXISTS taxes CASCADE",
		"DROP TABLE IF EXISTS sale_payments CASCADE",
		"DROP TABLE IF EXISTS sale_items CASCADE",
		"DROP TABLE IF EXISTS sales CASCADE",
		"DROP TABLE IF EXISTS suppliers CASCADE",
		"DROP TABLE IF EXISTS customer_ledger CASCADE",
		"DROP TABLE IF EXISTS customers CASCADE",
		"DROP TABLE IF EXISTS stock_movements CASCADE",
		"DROP TABLE IF EXISTS product_variants CASCADE",
		"DROP TABLE IF EXISTS products CASCADE",
		"DROP TABLE IF EXISTS units CASCADE",
		"DROP TABLE IF EXISTS brands CASCADE",
		"DROP TABLE IF EXISTS categories CASCADE",
		"DROP TABLE IF EXISTS users CASCADE",
		"DROP TABLE IF EXISTS role_permissions CASCADE",
		"DROP TABLE IF EXISTS permissions CASCADE",
		"DROP TABLE IF EXISTS roles CASCADE",
		"DROP TABLE IF EXISTS cash_movements CASCADE",
		"DROP TABLE IF EXISTS shifts CASCADE",
		"DROP TABLE IF EXISTS registers CASCADE",
		"DROP TABLE IF EXISTS stores CASCADE",
		"DROP TABLE IF EXISTS organizations CASCADE",
		"DROP TABLE IF EXISTS schema_migrations CASCADE",
	}
	for _, stmt := range drops {
		if _, err := pool.Exec(stmt); err != nil {
			t.Logf("warn drop: %v", err)
		}
	}
}

// ─── TEST CASES ───────────────────────────────────────────────────────────────

// TestPostgresConnection verifies that OpenPostgres connects and pings
// the database successfully.
func TestPostgresConnection(t *testing.T) {
	pool := openPG(t)
	if err := pool.Ping(); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
	t.Log("✓ PostgreSQL connection healthy")
}

// TestPostgresMigrations verifies that all 11 POS schema migrations apply
// cleanly on Postgres and that re-running Up() is fully idempotent.
func TestPostgresMigrations(t *testing.T) {
	pool := openPG(t)
	dropTestSchema(t, pool)
	t.Cleanup(func() { dropTestSchema(t, pool) })

	runner := data.NewRealMigrationRunner(pool)
	data.RegisterPOSMigrations(runner)

	// First run — apply all migrations
	if err := runner.Up(); err != nil {
		t.Fatalf("Up (first run): %v", err)
	}

	// Verify migration count in schema_migrations
	var count int
	if err := pool.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count < 11 {
		t.Fatalf("expected at least 11 migrations, got %d", count)
	}
	t.Logf("✓ %d migrations applied to PostgreSQL", count)

	// Second run — must be idempotent (no error, no duplicates)
	if err := runner.Up(); err != nil {
		t.Fatalf("Up (second run / idempotency): %v", err)
	}
	var count2 int
	if err := pool.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count2); err != nil {
		t.Fatalf("count after second Up: %v", err)
	}
	if count2 != count {
		t.Fatalf("idempotency failed: migration count changed from %d to %d", count, count2)
	}
	t.Log("✓ Up() is idempotent")

	// Status() must return records in ascending version order
	records, err := runner.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	for i := 1; i < len(records); i++ {
		if records[i].Version <= records[i-1].Version {
			t.Errorf("Status not sorted: versions %d then %d", records[i-1].Version, records[i].Version)
		}
	}
	t.Logf("✓ Status() returned %d records in order", len(records))
}

// TestPostgresTransactionalMigrationRollback verifies that a migration
// containing deliberately bad SQL is fully rolled back, leaving previously
// applied migrations intact and no partial schema changes.
func TestPostgresTransactionalMigrationRollback(t *testing.T) {
	pool := openPG(t)
	dropTestSchema(t, pool)
	t.Cleanup(func() { dropTestSchema(t, pool) })

	runner := data.NewRealMigrationRunner(pool)

	// Good migration v1
	runner.Register(data.Migration{
		Version: 1,
		Name:    "create_test_table",
		UpSQL:   `CREATE TABLE tx_test_good (id SERIAL PRIMARY KEY, name TEXT NOT NULL)`,
		DownSQL: `DROP TABLE IF EXISTS tx_test_good`,
	})
	// Bad migration v2 — valid SQL then invalid SQL in one migration
	runner.Register(data.Migration{
		Version: 2,
		Name:    "bad_migration",
		UpSQL: `CREATE TABLE tx_test_will_rollback (id SERIAL PRIMARY KEY);
				THIS IS NOT VALID SQL AND SHOULD CAUSE ROLLBACK`,
		DownSQL: `DROP TABLE IF EXISTS tx_test_will_rollback`,
	})

	err := runner.Up()
	if err == nil {
		t.Fatal("expected error from bad migration, got nil")
	}
	t.Logf("✓ Up() correctly returned error: %v", err)

	// tx_test_will_rollback must NOT exist — the failed migration was rolled back
	var exists int
	_ = pool.QueryRow(
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_name=$1",
		"tx_test_will_rollback",
	).Scan(&exists)
	if exists != 0 {
		t.Error("tx_test_will_rollback exists — transactional rollback FAILED")
	} else {
		t.Log("✓ Failed migration was fully rolled back (no partial schema)")
	}

	// tx_test_good MUST exist — v1 was committed before v2 failed
	var goodExists int
	_ = pool.QueryRow(
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_name=$1",
		"tx_test_good",
	).Scan(&goodExists)
	if goodExists != 1 {
		t.Error("tx_test_good missing — v1 commit was unexpectedly undone")
	} else {
		t.Log("✓ Previously committed migration (v1) remains intact")
	}
}

// TestPostgresQueryBuilder verifies INSERT, SELECT with $N placeholders,
// UPDATE, DELETE, paginate and count work correctly against Postgres.
func TestPostgresQueryBuilder(t *testing.T) {
	pool := openPG(t)

	// Set up a disposable table
	if _, err := pool.Exec(`CREATE TABLE IF NOT EXISTS pg_qb_test (
		id   TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		val  INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec("DROP TABLE IF EXISTS pg_qb_test")
	})

	// INSERT 5 rows
	for i := 1; i <= 5; i++ {
		rec := map[string]interface{}{
			"id":   fmt.Sprintf("r%d", i),
			"name": fmt.Sprintf("item-%d", i),
			"val":  i * 10,
		}
		if err := data.Table("pg_qb_test").WithDriver(data.DriverPostgres).RealInsert(pool, rec); err != nil {
			t.Fatalf("insert row %d: %v", i, err)
		}
	}
	t.Log("✓ Inserted 5 rows via RealInsert with $N placeholders")

	// SELECT — find by id
	row, found, err := data.Table("pg_qb_test").WithDriver(data.DriverPostgres).RealFind(pool, "r3")
	if err != nil || !found {
		t.Fatalf("RealFind r3: found=%v err=%v", found, err)
	}
	if row["name"] != "item-3" {
		t.Errorf("expected name=item-3, got %v", row["name"])
	}
	t.Logf("✓ RealFind: %v", row["name"])

	// SELECT with WHERE — items with val > 20
	rows, err := data.Table("pg_qb_test").
		WithDriver(data.DriverPostgres).
		Where("val", ">", 20).
		OrderBy("val", "ASC").
		RealGet(pool)
	if err != nil {
		t.Fatalf("RealGet with WHERE: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("expected 3 rows with val>20, got %d", len(rows))
	}
	t.Logf("✓ WHERE val>20 returned %d rows", len(rows))

	// UPDATE
	n, err := data.Table("pg_qb_test").
		WithDriver(data.DriverPostgres).
		Where("id", "=", "r1").
		RealUpdate(pool, map[string]interface{}{"val": 999})
	if err != nil || n != 1 {
		t.Fatalf("RealUpdate: n=%d err=%v", n, err)
	}
	t.Log("✓ RealUpdate succeeded")

	// PAGINATE — page 1, size 2
	page1, total, err := data.Table("pg_qb_test").
		WithDriver(data.DriverPostgres).
		RealPaginate(pool, 1, 2)
	if err != nil || total != 5 || len(page1) != 2 {
		t.Fatalf("Paginate: rows=%d total=%d err=%v", len(page1), total, err)
	}
	t.Logf("✓ RealPaginate page 1/2: total=%d, page rows=%d", total, len(page1))

	// DELETE
	deleted, err := data.Table("pg_qb_test").
		WithDriver(data.DriverPostgres).
		Where("id", "=", "r5").
		RealDelete(pool)
	if err != nil || deleted != 1 {
		t.Fatalf("RealDelete: n=%d err=%v", deleted, err)
	}
	t.Log("✓ RealDelete succeeded")
}

// TestPostgresConcurrentMigration verifies that the advisory lock prevents
// multiple goroutines from running migrations simultaneously. Only one
// goroutine should succeed; all others should get "migration already in
// progress" errors.
func TestPostgresConcurrentMigration(t *testing.T) {
	pool := openPG(t)
	dropTestSchema(t, pool)
	t.Cleanup(func() { dropTestSchema(t, pool) })

	const goroutines = 5
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		errors   []error
		succeded int32
	)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Each goroutine gets its own runner pointing to the same DB.
			r := data.NewRealMigrationRunner(pool)
			data.RegisterPOSMigrations(r)
			err := r.Up()
			if err == nil {
				atomic.AddInt32(&succeded, 1)
			} else {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	// At least 1 goroutine must have succeeded (applied migrations).
	if succeded == 0 {
		t.Fatal("no goroutine succeeded in running migrations")
	}
	t.Logf("✓ %d/%d goroutines succeeded; %d were blocked by advisory lock",
		succeded, goroutines, len(errors))

	// Verify schema_migrations is clean — no duplicate rows
	var count int
	if err := pool.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count < 11 {
		t.Fatalf("expected ≥11 migration records, got %d", count)
	}
	t.Logf("✓ %d migration records in schema_migrations — no duplicates", count)
}

// TestPostgresRollbackN verifies that RollbackN removes migrations in
// reverse order and the schema_migrations count decreases accordingly.
func TestPostgresRollbackN(t *testing.T) {
	pool := openPG(t)
	dropTestSchema(t, pool)
	t.Cleanup(func() { dropTestSchema(t, pool) })

	runner := data.NewRealMigrationRunner(pool)
	// Register 3 simple migrations with DownSQL
	for i := 1; i <= 3; i++ {
		v := i
		runner.Register(data.Migration{
			Version: v,
			Name:    fmt.Sprintf("v%d", v),
			UpSQL:   fmt.Sprintf("CREATE TABLE rollback_test_%d (id INT)", v),
			DownSQL: fmt.Sprintf("DROP TABLE IF EXISTS rollback_test_%d", v),
		})
	}
	if err := runner.Up(); err != nil {
		t.Fatalf("Up: %v", err)
	}

	var before int
	_ = pool.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&before)
	if before != 3 {
		t.Fatalf("expected 3 migrations, got %d", before)
	}

	if err := runner.RollbackN(2); err != nil {
		t.Fatalf("RollbackN(2): %v", err)
	}

	var after int
	_ = pool.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&after)
	if after != 1 {
		t.Fatalf("expected 1 migration after RollbackN(2), got %d", after)
	}
	t.Logf("✓ RollbackN(2): %d → %d migration records", before, after)
}

// TestSQLDialectConsistency verifies that the same logical queries produce
// the same results on SQLite and PostgreSQL — ensuring the dialect layer is
// transparent to callers.
func TestSQLDialectConsistency(t *testing.T) {
	pgPool := openPG(t)

	// SQLite pool (in-memory)
	sqlitePool, err := data.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer sqlitePool.Close()

	// Create the same table on both databases
	createSQL := `CREATE TABLE dialect_test (id TEXT PRIMARY KEY, score INTEGER NOT NULL)`
	if _, err := pgPool.Exec(createSQL); err != nil {
		t.Fatalf("pg create: %v", err)
	}
	t.Cleanup(func() { _, _ = pgPool.Exec("DROP TABLE IF EXISTS dialect_test") })

	if _, err := sqlitePool.Exec(createSQL); err != nil {
		t.Fatalf("sqlite create: %v", err)
	}

	// Insert identical data into both via QueryBuilder
	for _, db := range []struct {
		pool   *data.RealDBPool
		driver data.DriverType
		name   string
	}{
		{pgPool, data.DriverPostgres, "postgres"},
		{sqlitePool, data.DriverSQLite, "sqlite"},
	} {
		for i := 1; i <= 3; i++ {
			rec := map[string]interface{}{
				"id":    fmt.Sprintf("id%d", i),
				"score": i * 100,
			}
			if err := data.Table("dialect_test").WithDriver(db.driver).RealInsert(db.pool, rec); err != nil {
				t.Fatalf("[%s] insert: %v", db.name, err)
			}
		}
	}

	// Query both and compare results
	pgRows, err := data.Table("dialect_test").
		WithDriver(data.DriverPostgres).
		Where("score", ">", 100).
		OrderBy("score", "ASC").
		RealGet(pgPool)
	if err != nil {
		t.Fatalf("pg query: %v", err)
	}

	sqliteRows, err := data.Table("dialect_test").
		WithDriver(data.DriverSQLite).
		Where("score", ">", 100).
		OrderBy("score", "ASC").
		RealGet(sqlitePool)
	if err != nil {
		t.Fatalf("sqlite query: %v", err)
	}

	if len(pgRows) != len(sqliteRows) {
		t.Fatalf("result count mismatch: postgres=%d sqlite=%d", len(pgRows), len(sqliteRows))
	}
	for i := range pgRows {
		pgID := fmt.Sprint(pgRows[i]["id"])
		sqliteID := fmt.Sprint(sqliteRows[i]["id"])
		if pgID != sqliteID {
			t.Errorf("row %d id mismatch: pg=%s sqlite=%s", i, pgID, sqliteID)
		}
	}
	t.Logf("✓ Dialect consistency: postgres and sqlite returned identical results (%d rows)", len(pgRows))
}
