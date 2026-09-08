package pos_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
	"github.com/joysriramsarkar/nilLang/pkg/alap/pos"
)

func setupCustomerTestDB(t *testing.T) (*data.RealDBPool, *pos.POSEngine) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "customer_test.db")
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

	engine := pos.NewPOSEngine(pos.CheckoutConfig{
		StoreName:    "Customer Accounting Test Store",
		StoreSubname: "Ledger Hardening",
	})
	engine.SetDB(pool)
	return pool, engine
}

// TestCustomerLedgerRunningBalanceLifecycle verifies:
// 1. Initial balance 0
// 2. Credit Sale ৳1,500.00 -> balance_after ৳1,500.00
// 3. Credit Sale ৳2,000.00 -> balance_after ৳3,500.00
// 4. Due Payment ৳2,500.00 -> balance_after ৳1,000.00
// 5. Debit Adjustment ৳200.00 -> balance_after ৳1,200.00
// 6. Refund Credit ৳500.00 -> balance_after ৳700.00
// 7. Final Due Payment ৳700.00 -> balance_after ৳0.00
//
// In each step, we verify:
// - DB customer.due_balance_minor matches expected
// - DB customer_ledger entry has exact balance_after (never 0 unless fully cleared!)
// - In-memory Customer.DueBalanceMinor matches expected
// - Math invariant holds: balance_after[k] = balance_after[k-1] + delta[k]
func TestCustomerLedgerRunningBalanceLifecycle(t *testing.T) {
	pool, engine := setupCustomerTestDB(t)

	now := time.Now().UTC().Format(time.RFC3339)
	const custID = "cust-100"
	_, err := pool.Exec(
		`INSERT INTO customers (id,name,phone,total_purchases_minor,due_balance_minor,credit_limit_minor,active,created_at,updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		custID, "Abdur Razzak", "01811223344", 0, 0, 500000, 1, now, now,
	)
	if err != nil {
		t.Fatalf("insert customer: %v", err)
	}

	customer := &pos.Customer{
		ID:    custID,
		Name:  "Abdur Razzak",
		Phone: "01811223344",
	}
	engine.Customers.AddCustomer(customer)

	type step struct {
		name         string
		action       func(tx *data.RealTx) (*pos.CustomerLedgerEntry, error)
		expectedType string
		expectedAmt  int64
		expectedBal  int64
	}

	steps := []step{
		{
			name: "First credit sale ৳1,500",
			action: func(tx *data.RealTx) (*pos.CustomerLedgerEntry, error) {
				return engine.Customers.RecordCreditSaleTx(tx, custID, 150000, "INV-2026-0001", "cashier-1")
			},
			expectedType: "CREDIT_SALE",
			expectedAmt:  150000,
			expectedBal:  150000,
		},
		{
			name: "Second credit sale ৳2,000",
			action: func(tx *data.RealTx) (*pos.CustomerLedgerEntry, error) {
				return engine.Customers.RecordCreditSaleTx(tx, custID, 200000, "INV-2026-0002", "cashier-1")
			},
			expectedType: "CREDIT_SALE",
			expectedAmt:  200000,
			expectedBal:  350000,
		},
		{
			name: "Partial due payment ৳2,500",
			action: func(tx *data.RealTx) (*pos.CustomerLedgerEntry, error) {
				return engine.Customers.RecordDuePaymentTx(tx, custID, 250000, "PAY-REC-001", "Cash payment at counter", "cashier-1")
			},
			expectedType: "DUE_PAYMENT",
			expectedAmt:  250000,
			expectedBal:  100000,
		},
		{
			name: "Debit adjustment ৳200 (fee or missed item)",
			action: func(tx *data.RealTx) (*pos.CustomerLedgerEntry, error) {
				return engine.Customers.RecordAdjustmentTx(tx, custID, 20000, "ADJ-001", "Delivery fee added", "manager-1")
			},
			expectedType: "ADJUSTMENT_DEBIT",
			expectedAmt:  20000,
			expectedBal:  120000,
		},
		{
			name: "Refund credit ৳500 on returned item",
			action: func(tx *data.RealTx) (*pos.CustomerLedgerEntry, error) {
				return engine.Customers.RecordRefundCreditTx(tx, custID, 50000, "INV-2026-0001", "manager-1")
			},
			expectedType: "REFUND_CREDIT",
			expectedAmt:  50000,
			expectedBal:  70000,
		},
		{
			name: "Final full payment ৳700",
			action: func(tx *data.RealTx) (*pos.CustomerLedgerEntry, error) {
				return engine.Customers.RecordDuePaymentTx(tx, custID, 70000, "PAY-REC-002", "Cleared all dues", "cashier-1")
			},
			expectedType: "DUE_PAYMENT",
			expectedAmt:  70000,
			expectedBal:  0,
		},
	}

	for i, s := range steps {
		var entry *pos.CustomerLedgerEntry
		txErr := pool.Transaction(func(tx *data.RealTx) error {
			var err error
			entry, err = s.action(tx)
			return err
		})
		if txErr != nil {
			t.Fatalf("step %d (%s) failed: %v", i+1, s.name, txErr)
		}

		if entry.Type != s.expectedType {
			t.Errorf("step %d: expected entry type %s, got %s", i+1, s.expectedType, entry.Type)
		}
		if entry.BalanceAfter != s.expectedBal {
			t.Errorf("step %d: expected entry balance_after %d, got %d", i+1, s.expectedBal, entry.BalanceAfter)
		}

		// Verify DB customer table
		var dbDue int64
		if err := pool.QueryRow("SELECT due_balance_minor FROM customers WHERE id=?", custID).Scan(&dbDue); err != nil {
			t.Fatalf("step %d: query db due: %v", i+1, err)
		}
		if dbDue != s.expectedBal {
			t.Errorf("step %d: DB due_balance_minor=%d, expected %d", i+1, dbDue, s.expectedBal)
		}

		// Verify in-memory customer
		memCust, found := engine.Customers.FindByID(custID)
		if !found {
			t.Fatalf("step %d: customer not in memory", i+1)
		}
		if memCust.DueBalanceMinor != s.expectedBal {
			t.Errorf("step %d: in-memory due_balance_minor=%d, expected %d", i+1, memCust.DueBalanceMinor, s.expectedBal)
		}

		t.Logf("✓ Step %d [%s]: amount=৳%.2f, balance_after=৳%.2f (DB & Memory synced)",
			i+1, s.expectedType, float64(s.expectedAmt)/100, float64(s.expectedBal)/100)
	}

	// Verify complete chronological ledger sequence in DB
	rows, err := pool.Query(
		"SELECT type, amount_minor, balance_after FROM customer_ledger WHERE customer_id=? ORDER BY rowid ASC",
		custID,
	)
	if err != nil {
		t.Fatalf("query ledger rows: %v", err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var (
			entryType    string
			amountMinor  int64
			balanceAfter int64
		)
		if err := rows.Scan(&entryType, &amountMinor, &balanceAfter); err != nil {
			t.Fatalf("scan ledger row: %v", err)
		}
		if count >= len(steps) {
			t.Fatalf("more ledger rows than steps")
		}
		expected := steps[count]
		if balanceAfter != expected.expectedBal {
			t.Errorf("ledger row %d: balance_after=%d, expected %d", count+1, balanceAfter, expected.expectedBal)
		}
		count++
	}
	if count != len(steps) {
		t.Errorf("expected %d ledger rows in DB, found %d", len(steps), count)
	}
	t.Logf("✓ Verified all %d ledger rows form a mathematically contiguous running balance", count)
}
