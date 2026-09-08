package data_test

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

var migTestSeq int64

func TestRealMigrationRunnerChecksumAndTamperDetection(t *testing.T) {
	seq := atomic.AddInt64(&migTestSeq, 1)
	pool, err := data.OpenSQLite(fmt.Sprintf("file:test_mig_%d?mode=memory&cache=shared", seq))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer pool.Close()

	runner := data.NewRealMigrationRunner(pool)
	data.RegisterPOSMigrations(runner)

	// Run initial migration
	if err := runner.Up(); err != nil {
		t.Fatalf("runner.Up() failed: %v", err)
	}

	currVer, err := runner.CurrentVersion()
	if err != nil {
		t.Fatalf("CurrentVersion: %v", err)
	}
	if currVer < 12 {
		t.Fatalf("expected CurrentVersion >= 12, got %d", currVer)
	}

	status, err := runner.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(status) < 12 {
		t.Fatalf("expected at least 12 migration records, got %d", len(status))
	}
	for _, rec := range status {
		if rec.Checksum == "" {
			t.Errorf("expected migration %d (%s) to have non-empty checksum", rec.Version, rec.Name)
		}
	}

	// Idempotency: second Up() must succeed
	if err := runner.Up(); err != nil {
		t.Fatalf("second Up() failed: %v", err)
	}

	// Tamper detection test: create a runner with a modified migration 1 UpSQL
	tamperedRunner := data.NewRealMigrationRunner(pool)
	tamperedRunner.Register(data.Migration{
		Version: 1,
		Name:    "create_organizations_and_stores",
		UpSQL:   "CREATE TABLE IF NOT EXISTS modified_orgs (id TEXT PRIMARY KEY);",
		DownSQL: "DROP TABLE IF EXISTS modified_orgs;",
	})

	tamperErr := tamperedRunner.Up()
	if tamperErr == nil {
		t.Fatalf("expected tamper error due to checksum mismatch, got nil")
	}
	if !strings.Contains(tamperErr.Error(), "checksum mismatch") {
		t.Fatalf("expected error containing 'checksum mismatch', got %v", tamperErr)
	}
}

func TestRealMigrationRunnerRollbackN(t *testing.T) {
	seq := atomic.AddInt64(&migTestSeq, 1)
	pool, err := data.OpenSQLite(fmt.Sprintf("file:test_rollback_%d?mode=memory&cache=shared", seq))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer pool.Close()

	runner := data.NewRealMigrationRunner(pool)
	data.RegisterPOSMigrations(runner)

	if err := runner.Up(); err != nil {
		t.Fatalf("Up: %v", err)
	}

	verBefore, _ := runner.CurrentVersion()
	if verBefore < 12 {
		t.Fatalf("verBefore expected >= 12, got %d", verBefore)
	}

	// Rollback 2 migrations
	if err := runner.RollbackN(2); err != nil {
		t.Fatalf("RollbackN(2): %v", err)
	}

	verAfter, _ := runner.CurrentVersion()
	if verAfter != verBefore-2 {
		t.Fatalf("expected version after rollback to be %d, got %d", verBefore-2, verAfter)
	}

	// Re-run Up() to restore
	if err := runner.Up(); err != nil {
		t.Fatalf("re-Up after rollback failed: %v", err)
	}

	verRestored, _ := runner.CurrentVersion()
	if verRestored != verBefore {
		t.Fatalf("expected version after re-Up to be %d, got %d", verBefore, verRestored)
	}
}
