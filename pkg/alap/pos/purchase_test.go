package pos

import (
	"path/filepath"
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

func TestPurchaseServiceAndWAC(t *testing.T) {
	// Create SQLite test DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "purchase_test.db")
	pool, err := data.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer pool.Close()

	runner := data.NewRealMigrationRunner(pool)
	data.RegisterPOSMigrations(runner)
	if err := runner.Up(); err != nil {
		t.Fatalf("migrations up: %v", err)
	}

	engine := NewPOSEngine()
	engine.SetDB(pool)
	engine.SeedDefaultEnterpriseData()

	supplier := &Supplier{
		ID:      "sup-01",
		Name:    "Square Consumer Products Ltd",
		Phone:   "02-9888777",
		Address: "Uttara, Dhaka",
	}
	engine.Purchases.AddSupplier(supplier)

	// Create PO for 10 bags of Miniket rice at ৳3,200/bag (old stock was 45 bags at ৳2,900)
	rice, ok := engine.Catalog.FindByID("p-01")
	if !ok {
		t.Fatal("p-01 not found")
	}
	initialStock := rice.Stock
	initialCost := rice.Cost

	po := &Purchase{
		SupplierID: supplier.ID,
		Items: []PurchaseItem{
			{
				ProductID: rice.ID,
				Quantity:  data.NewDecimalFromInt(10),
				CostPrice: data.NewMoney(320000, "BDT"), // ৳3,200.00
			},
		},
	}

	if err := engine.Purchases.CreatePurchase(po); err != nil {
		t.Fatalf("CreatePurchase: %v", err)
	}
	if po.Status != PurchasePending {
		t.Fatalf("expected PENDING status, got %s", po.Status)
	}
	if po.SubtotalMinor != 3200000 { // 10 * ৳3,200 = ৳32,000
		t.Fatalf("expected subtotal 3200000, got %d", po.SubtotalMinor)
	}

	// Receive 10 bags
	received, err := engine.Purchases.ReceiveGoods(po.ID, nil, "Batch 2026-09A arrival")
	if err != nil {
		t.Fatalf("ReceiveGoods: %v", err)
	}
	if received.Status != PurchaseReceived {
		t.Fatalf("expected RECEIVED status, got %s", received.Status)
	}

	// Verify stock increased by 10
	updatedRice, _ := engine.Catalog.FindByID(rice.ID)
	expectedStock := initialStock.Add(data.NewDecimalFromInt(10))
	if updatedRice.Stock.Cmp(expectedStock) != 0 {
		t.Fatalf("expected stock %s, got %s", expectedStock.String(), updatedRice.Stock.String())
	}

	// Verify WAC:
	// old: 45 bags * 2900 = 130,500
	// new: 10 bags * 3200 = 32,000
	// total = 162,500 / 55 bags = 2,954.5454... BDT (295454 minor)
	expectedWAC := calculateWAC(initialStock, initialCost, data.NewDecimalFromInt(10), data.NewMoney(320000, "BDT"))
	if updatedRice.Cost.Minor != expectedWAC.Minor {
		t.Fatalf("expected WAC %d, got %d", expectedWAC.Minor, updatedRice.Cost.Minor)
	}

	// Verify supplier payable updated
	sup, _ := engine.Purchases.FindSupplier(supplier.ID)
	if sup.PayableMinor != 3200000 {
		t.Fatalf("expected payable 3200000, got %d", sup.PayableMinor)
	}

	// Test Damage Write-off: 2 bags damaged in transit
	mov, err := engine.Purchases.RecordDamage(rice.ID, data.NewDecimalFromInt(2), "Rain damage during unloading", "manager-01")
	if err != nil {
		t.Fatalf("RecordDamage: %v", err)
	}
	if mov.Type != MovementDamage {
		t.Fatalf("expected movement type DAMAGE, got %s", mov.Type)
	}
	damagedRice, _ := engine.Catalog.FindByID(rice.ID)
	expectedAfterDamage := expectedStock.Sub(data.NewDecimalFromInt(2))
	if damagedRice.Stock.Cmp(expectedAfterDamage) != 0 {
		t.Fatalf("expected stock after damage %s, got %s", expectedAfterDamage.String(), damagedRice.Stock.String())
	}
}

func TestSupplierLedgerAccountingLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "supplier_ledger_test.db")
	pool, err := data.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer pool.Close()

	runner := data.NewRealMigrationRunner(pool)
	data.RegisterPOSMigrations(runner)
	if err := runner.Up(); err != nil {
		t.Fatalf("migrations up: %v", err)
	}

	engine := NewPOSEngine()
	engine.SetDB(pool)
	engine.SeedDefaultEnterpriseData()

	supplier := &Supplier{
		ID:      "sup-acct-01",
		Name:    "Pran Agro Ltd",
		Phone:   "01700112233",
		Address: "Ghorashal, Narsingdi",
	}
	engine.Purchases.AddSupplier(supplier)

	// Step 1: Initial supplier payable is 0
	s0, _ := engine.Purchases.FindSupplier(supplier.ID)
	if s0.PayableMinor != 0 {
		t.Fatalf("expected initial payable 0, got %d", s0.PayableMinor)
	}

	// Step 2: Create and receive PO for 20 units of p-01 (Miniket rice) at ৳3,000.00 each = ৳60,000.00 (6,000,000 minor)
	rice, _ := engine.Catalog.FindByID("p-01")
	stockBefore := rice.Stock

	po := &Purchase{
		SupplierID: supplier.ID,
		Items: []PurchaseItem{
			{
				ProductID: rice.ID,
				Quantity:  data.NewDecimalFromInt(20),
				CostPrice: data.NewMoney(300000, "BDT"), // ৳3,000.00
			},
		},
	}
	if err := engine.Purchases.CreatePurchase(po); err != nil {
		t.Fatalf("CreatePurchase: %v", err)
	}

	_, err = engine.Purchases.ReceiveGoods(po.ID, nil, "Batch 2026-09B")
	if err != nil {
		t.Fatalf("ReceiveGoods: %v", err)
	}

	// Verify supplier payable updated to 6,000,000 in memory and in DB
	s1, _ := engine.Purchases.FindSupplier(supplier.ID)
	if s1.PayableMinor != 6000000 {
		t.Fatalf("expected payable 6000000, got %d", s1.PayableMinor)
	}
	var dbPayable int64
	_ = pool.QueryRow("SELECT payable_minor FROM suppliers WHERE id = ?", supplier.ID).Scan(&dbPayable)
	if dbPayable != 6000000 {
		t.Fatalf("expected DB payable 6000000, got %d", dbPayable)
	}

	// Step 3: Record payment of ৳35,000.00 to supplier
	payEntry, err := engine.Purchases.RecordSupplierPayment(supplier.ID, 3500000, "CHQ-88990", "Cheque payment", "cashier-01")
	if err != nil {
		t.Fatalf("RecordSupplierPayment: %v", err)
	}
	if payEntry.BalanceAfter != 2500000 { // 60,000 - 35,000 = 25,000
		t.Fatalf("expected balance after payment 2500000, got %d", payEntry.BalanceAfter)
	}
	_ = pool.QueryRow("SELECT payable_minor FROM suppliers WHERE id = ?", supplier.ID).Scan(&dbPayable)
	if dbPayable != 2500000 {
		t.Fatalf("expected DB payable 2500000, got %d", dbPayable)
	}

	// Step 4: Record Purchase Return of 3 units at ৳3,000.00 = ৳9,000.00 (900,000 minor)
	retItems := []PurchaseReturnItem{
		{
			ProductID: rice.ID,
			Quantity:  data.NewDecimalFromInt(3),
			CostPrice: data.NewMoney(300000, "BDT"),
			Reason:    "Packaging damaged",
		},
	}
	retResult, err := engine.Purchases.RecordPurchaseReturn(po.ID, supplier.ID, retItems, "Packaging torn", "manager-01")
	if err != nil {
		t.Fatalf("RecordPurchaseReturn: %v", err)
	}
	if retResult.BalanceAfter != 1600000 { // 25,000 - 9,000 = 16,000
		t.Fatalf("expected balance after return 1600000, got %d", retResult.BalanceAfter)
	}
	_ = pool.QueryRow("SELECT payable_minor FROM suppliers WHERE id = ?", supplier.ID).Scan(&dbPayable)
	if dbPayable != 1600000 {
		t.Fatalf("expected DB payable 1600000, got %d", dbPayable)
	}

	// Verify stock is stockBefore + 20 - 3 = stockBefore + 17
	riceAfter, _ := engine.Catalog.FindByID(rice.ID)
	expectedStock := stockBefore.Add(data.NewDecimalFromInt(17))
	if riceAfter.Stock.Cmp(expectedStock) != 0 {
		t.Fatalf("expected final stock %s, got %s", expectedStock.String(), riceAfter.Stock.String())
	}

	// Step 5: Query supplier ledger and verify mathematical invariants
	ledger, err := engine.Purchases.GetSupplierLedger(supplier.ID)
	if err != nil {
		t.Fatalf("GetSupplierLedger: %v", err)
	}
	if len(ledger) != 3 {
		t.Fatalf("expected 3 ledger entries (PURCHASE, PAYMENT, DEBIT_NOTE), got %d", len(ledger))
	}

	expectedTypes := []string{"PURCHASE", "PAYMENT", "DEBIT_NOTE"}
	expectedDeltas := []int64{6000000, -3500000, -900000}
	expectedBalances := []int64{6000000, 2500000, 1600000}

	var runningBal int64 = 0
	for i, entry := range ledger {
		if entry.Type != expectedTypes[i] {
			t.Errorf("entry %d: expected type %s, got %s", i, expectedTypes[i], entry.Type)
		}
		runningBal += expectedDeltas[i]
		if entry.BalanceAfter != expectedBalances[i] || entry.BalanceAfter != runningBal {
			t.Errorf("entry %d: expected balance %d, got %d", i, expectedBalances[i], entry.BalanceAfter)
		}
	}
}
