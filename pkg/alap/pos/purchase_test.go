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
