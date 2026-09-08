package pos_test

import (
	"path/filepath"
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
	"github.com/joysriramsarkar/nilLang/pkg/alap/pos"
)

func setupShiftTestDB(t *testing.T) (*data.RealDBPool, *pos.POSEngine) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "shift_test.db")
	pool, err := data.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	runner := data.NewRealMigrationRunner(pool)
	data.RegisterPOSMigrations(runner)
	if err := runner.Up(); err != nil {
		t.Fatalf("migrations Up: %v", err)
	}

	engine := pos.NewPOSEngine()
	engine.SetDB(pool)
	return pool, engine
}

func TestShiftOpenCloseReconciliation(t *testing.T) {
	pool, engine := setupShiftTestDB(t)

	// 1. Open shift with ৳2,000.00 (200,000 minor)
	shift, err := engine.Shifts.OpenShift("reg-01", "cashier-01", "Kamal Hossain", 200000)
	if err != nil {
		t.Fatalf("OpenShift: %v", err)
	}
	if shift.ExpectedCashMinor != 200000 {
		t.Fatalf("expected cash minor %d, got %d", 200000, shift.ExpectedCashMinor)
	}

	// Verify DB record
	var status string
	var startCash int64
	err = pool.QueryRow("SELECT status, starting_cash_minor FROM shifts WHERE id = ?", shift.ID).Scan(&status, &startCash)
	if err != nil || status != "OPEN" || startCash != 200000 {
		t.Fatalf("expected DB shift OPEN with 200000, got status=%s, startCash=%d, err=%v", status, startCash, err)
	}

	// 2. Cash In: +৳500.00 (float top-up)
	_, err = engine.Shifts.RecordCashMovement(pos.CashMovementIn, 50000, "Float top-up")
	if err != nil {
		t.Fatalf("CashMovementIn: %v", err)
	}

	// 3. Cash Out: -৳300.00 (petty cash payout)
	_, err = engine.Shifts.RecordCashMovement(pos.CashMovementOut, 30000, "Tea & snacks")
	if err != nil {
		t.Fatalf("CashMovementOut: %v", err)
	}

	// 4. Record sale: ৳1,500.00 cash
	sale := &pos.Sale{
		ID:          "sale-shift-01",
		TotalMinor:  150000,
		Payments:    []pos.PaymentRecord{{Method: pos.MethodCash, AmountMinor: 150000}},
		ChangeMinor: 0,
	}
	if err := engine.Shifts.RecordSale(sale); err != nil {
		t.Fatalf("RecordSale: %v", err)
	}

	// Expected cash: 200000 + 50000 - 30000 + 150000 = 370000 (৳3,700.00)
	cur, _ := engine.Shifts.CurrentShift()
	if cur.ExpectedCashMinor != 370000 {
		t.Fatalf("expected cash 370000, got %d", cur.ExpectedCashMinor)
	}

	// 5. Count Denominations for exactly ৳3,700.00
	// 3x 1000 = 3000, 1x 500 = 500, 2x 100 = 200 => Total 3700
	denoms := pos.DenominationCount{
		Note1000: 3,
		Note500:  1,
		Note100:  2,
	}
	if denoms.TotalMinor() != 370000 {
		t.Fatalf("expected denoms total 370000, got %d", denoms.TotalMinor())
	}

	// 6. Close Shift with Denominations
	closedShift, err := engine.Shifts.CloseShiftWithDenominations(denoms, "Balanced evening shift", pos.RoleCashier)
	if err != nil {
		t.Fatalf("CloseShiftWithDenominations: %v", err)
	}
	if closedShift.Status != pos.ShiftClosed {
		t.Fatalf("expected CLOSED status, got %s", closedShift.Status)
	}
	if closedShift.CashDifferenceMinor != 0 {
		t.Fatalf("expected difference 0, got %d", closedShift.CashDifferenceMinor)
	}

	// Verify DB record is CLOSED
	var dbStatus string
	var dbActual int64
	var dbDiff int64
	_ = pool.QueryRow("SELECT status, actual_cash_minor, cash_difference_minor FROM shifts WHERE id = ?", shift.ID).Scan(&dbStatus, &dbActual, &dbDiff)
	if dbStatus != "CLOSED" || dbActual != 370000 || dbDiff != 0 {
		t.Fatalf("expected DB CLOSED with actual 370000, diff 0, got status=%s, actual=%d, diff=%d", dbStatus, dbActual, dbDiff)
	}
}

func TestShiftVarianceApprovalRequired(t *testing.T) {
	_, engine := setupShiftTestDB(t)

	// Open shift with ৳1,000.00
	shift, err := engine.Shifts.OpenShift("reg-02", "cashier-02", "Salma Begum", 100000)
	if err != nil {
		t.Fatalf("OpenShift: %v", err)
	}

	// Count physical cash: ৳850.00 (Shortage of ৳150.00 > ৳100.00 allowed threshold)
	countedCash := int64(85000)

	// Cashier attempts to close: Must FAIL
	_, err = engine.Shifts.CloseShift(countedCash, "Missing 150 taka", pos.RoleCashier)
	if err != pos.ErrShiftVarianceManagerApprovalRequired {
		t.Fatalf("expected ErrShiftVarianceManagerApprovalRequired, got %v", err)
	}

	// Manager approves closing with variance: Must SUCCEED
	closedShift, err := engine.Shifts.CloseShift(countedCash, "Shortage approved after drawer audit", pos.RoleManager)
	if err != nil {
		t.Fatalf("manager shift close failed: %v", err)
	}
	if closedShift.CashDifferenceMinor != -15000 {
		t.Fatalf("expected variance -15000, got %d", closedShift.CashDifferenceMinor)
	}
	if closedShift.ID != shift.ID {
		t.Fatalf("expected closed shift ID %s, got %s", shift.ID, closedShift.ID)
	}
}
