package pos_test

import (
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
	"github.com/joysriramsarkar/nilLang/pkg/alap/pos"
)

var refundTestSeq int64

func setupRefundTestEnv(t *testing.T) (*data.RealDBPool, *pos.POSEngine) {
	t.Helper()
	seq := atomic.AddInt64(&refundTestSeq, 1)
	dbPath := filepath.Join(t.TempDir(), fmt.Sprintf("refund_test_%d.db", seq))
	pool, err := data.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	migRunner := data.NewRealMigrationRunner(pool)
	data.RegisterPOSMigrations(migRunner)
	if err := migRunner.Up(); err != nil {
		t.Fatalf("migrations Up: %v", err)
	}

	engine := pos.NewPOSEngine(pos.CheckoutConfig{
		StoreName: "Refund Hardening Store",
	})
	engine.SetDB(pool)

	// Add test product with stock = 20
	product := &pos.Product{
		ID:      fmt.Sprintf("prod-rf-%d", seq),
		SKU:     fmt.Sprintf("SKU-RF-%d", seq),
		Barcode: fmt.Sprintf("BAR-RF-%d", seq),
		Name:    "Basmati Rice 5kg",
		Unit:    "pcs",
		Price:   data.NewMoney(60000, "BDT"), // ৳600.00
		Cost:    data.NewMoney(50000, "BDT"), // ৳500.00
		Stock:   data.NewDecimalFromInt(20),  // 20 units
		Active:  true,
	}
	engine.Catalog.AddProduct(product)
	engine.Inventory.RecordOpeningStock(product.ID, product.Stock, product.Cost)

	nowStr := time.Now().UTC().Format(time.RFC3339)
	_, err = pool.Exec(
		`INSERT INTO products (id, sku, barcode, name, unit, price_minor, cost_minor, stock_raw, active, version, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, 0, ?, ?)`,
		product.ID, product.SKU, product.Barcode, product.Name, product.Unit, product.Price.Minor, product.Cost.Minor, product.Stock.Value, nowStr, nowStr,
	)
	if err != nil {
		t.Fatalf("insert product: %v", err)
	}

	return pool, engine
}

func TestRefundHardeningLifecycle(t *testing.T) {
	pool, engine := setupRefundTestEnv(t)

	// 1. Create a customer with credit limit in DB
	const custID = "cust-rf-100"
	nowStr := time.Now().UTC().Format(time.RFC3339)
	_, err := pool.Exec(
		`INSERT INTO customers (id, name, phone, total_purchases_minor, due_balance_minor, credit_limit_minor, active, created_at, updated_at)
		 VALUES (?, ?, '01711122233', 0, 0, 500000, 1, ?, ?)`,
		custID, "Rahim Khan", nowStr, nowStr,
	)
	if err != nil {
		t.Fatalf("create customer: %v", err)
	}
	engine.Customers.AddCustomer(&pos.Customer{
		ID:                  custID,
		Name:                "Rahim Khan",
		Phone:               "01711122233",
		DueBalanceMinor:     0,
		TotalPurchasesMinor: 0,
	})

	// 2. Perform checkout: 2 units = ৳1,200.00 (৳500 cash, ৳700 credit)
	prod, ok := engine.Catalog.FindByID(fmt.Sprintf("prod-rf-%d", refundTestSeq))
	if !ok {
		t.Fatalf("find product failed")
	}

	cart := pos.NewCart("cart-rf-01", "")
	cart.AddProduct(prod, data.NewDecimalFromInt(2))
	cart.Recalculate()

	payments := []pos.PaymentRecord{
		{Method: pos.MethodCash, AmountMinor: 50000},
		{Method: pos.MethodCredit, AmountMinor: 70000},
	}

	res, err := engine.Checkout.Execute(cart, payments, "cashier-1", "reg-1", custID)
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	sale := res.Sale

	// Verify initial stock in catalog and DB: 20 - 2 = 18
	pUpdated, _ := engine.Catalog.FindByID(prod.ID)
	if pUpdated.Stock.Cmp(data.NewDecimalFromInt(18)) != 0 {
		t.Fatalf("expected stock 18, got %s", pUpdated.Stock.String())
	}

	// Verify customer due: ৳700.00 in DB
	var custDue int64
	_ = pool.QueryRow("SELECT due_balance_minor FROM customers WHERE id = ?", custID).Scan(&custDue)
	if custDue != 70000 {
		t.Fatalf("expected customer due 70000, got %d", custDue)
	}

	// 3. Test Partial Refund: 1 unit returned by cashier
	partialItems := []pos.RefundItemRequest{
		{
			ProductID: prod.ID,
			Quantity:  data.NewDecimalFromInt(1), // 1 unit
			Reason:    "Customer changed mind",
		},
	}

	refRec, err := engine.Refund.ProcessRefund(sale.ID, partialItems, "cashier-1", "Returned 1 unit", pos.RoleCashier)
	if err != nil {
		t.Fatalf("partial refund: %v", err)
	}
	if refRec.RefundAmount != 60000 { // ৳600.00
		t.Fatalf("expected refund amount 60000, got %d", refRec.RefundAmount)
	}

	// Verify stock restored to 19 in DB
	var dbStock int64
	_ = pool.QueryRow("SELECT stock_raw FROM products WHERE id = ?", prod.ID).Scan(&dbStock)
	if dbStock != data.NewDecimalFromInt(19).Value {
		t.Fatalf("expected DB stock 19 units, got %d", dbStock)
	}

	// Verify customer due reduced: 70000 - 60000 = 10000 in DB
	_ = pool.QueryRow("SELECT due_balance_minor FROM customers WHERE id = ?", custID).Scan(&custDue)
	if custDue != 10000 {
		t.Fatalf("expected customer due reduced to 10000, got %d", custDue)
	}

	// Verify customer ledger recorded the refund
	var cledCount int
	_ = pool.QueryRow("SELECT COUNT(*) FROM customer_ledger WHERE customer_id = ? AND type = 'REFUND'", custID).Scan(&cledCount)
	if cledCount != 1 {
		t.Fatalf("expected 1 refund ledger entry, got %d", cledCount)
	}

	// Verify DB refunds table
	var dbRefundCount int
	_ = pool.QueryRow("SELECT COUNT(*) FROM refunds WHERE sale_id = ?", sale.ID).Scan(&dbRefundCount)
	if dbRefundCount != 1 {
		t.Fatalf("expected 1 refund in DB, got %d", dbRefundCount)
	}

	// 4. Test Double Refund Prevention: Attempt to refund 2 units when only 1 remains refundable
	exceedItems := []pos.RefundItemRequest{
		{
			ProductID: prod.ID,
			Quantity:  data.NewDecimalFromInt(2), // 2 units
			Reason:    "Exceeding return",
		},
	}
	_, err = engine.Refund.ProcessRefund(sale.ID, exceedItems, "cashier-1", "Too much", pos.RoleCashier)
	if err == nil {
		t.Fatalf("expected double-refund error when exceeding refundable quantity, got nil")
	}

	// 5. Test Remaining Refund: Refund remaining 1 unit
	remainingItems := []pos.RefundItemRequest{
		{
			ProductID: prod.ID,
			Quantity:  data.NewDecimalFromInt(1), // 1 unit
			Reason:    "Return remaining item",
		},
	}
	_, err = engine.Refund.ProcessRefund(sale.ID, remainingItems, "cashier-1", "Remaining unit", pos.RoleCashier)
	if err != nil {
		t.Fatalf("refund remaining unit: %v", err)
	}

	// Verify sale status is now REFUNDED
	sUpdated, _ := engine.Checkout.GetSale(sale.ID)
	if sUpdated.Status != pos.StatusRefunded {
		t.Fatalf("expected sale status REFUNDED, got %s", sUpdated.Status)
	}

	// Verify full stock restored to 20 in DB
	_ = pool.QueryRow("SELECT stock_raw FROM products WHERE id = ?", prod.ID).Scan(&dbStock)
	if dbStock != data.NewDecimalFromInt(20).Value {
		t.Fatalf("expected stock 20 after full refund, got %d", dbStock)
	}
}

func TestRefundManagerApprovalThreshold(t *testing.T) {
	pool, engine := setupRefundTestEnv(t)

	// Add expensive product: ৳6,000.00
	prod := &pos.Product{
		ID:      "prod-expensive",
		SKU:     "SKU-EXP",
		Barcode: "BAR-EXP",
		Name:    "High-end Tablet",
		Unit:    "pcs",
		Price:   data.NewMoney(600000, "BDT"), // ৳6,000.00 > ৳5,000.00 threshold
		Cost:    data.NewMoney(500000, "BDT"),
		Stock:   data.NewDecimalFromInt(10),
		Active:  true,
	}
	engine.Catalog.AddProduct(prod)
	engine.Inventory.RecordOpeningStock(prod.ID, prod.Stock, prod.Cost)

	nowStr := time.Now().UTC().Format(time.RFC3339)
	_, err := pool.Exec(
		`INSERT INTO products (id, sku, barcode, name, unit, price_minor, cost_minor, stock_raw, active, version, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, 0, ?, ?)`,
		prod.ID, prod.SKU, prod.Barcode, prod.Name, prod.Unit, prod.Price.Minor, prod.Cost.Minor, prod.Stock.Value, nowStr, nowStr,
	)
	if err != nil {
		t.Fatalf("insert expensive prod: %v", err)
	}

	cart := pos.NewCart("cart-exp", "")
	cart.AddProduct(prod, data.NewDecimalFromInt(1))
	cart.Recalculate()

	res, err := engine.Checkout.Execute(cart, []pos.PaymentRecord{{Method: pos.MethodCash, AmountMinor: 600000}}, "cashier-1", "reg-1", "")
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	sale := res.Sale

	// Cashier attempts refund > ৳5,000.00: Must FAIL with authorization error
	refItems := []pos.RefundItemRequest{
		{ProductID: prod.ID, Quantity: data.NewDecimalFromInt(1), Reason: "Defective"},
	}
	_, err = engine.Refund.ProcessRefund(sale.ID, refItems, "cashier-1", "Defective", pos.RoleCashier)
	if err != pos.ErrManagerApprovalRequired {
		t.Fatalf("expected ErrManagerApprovalRequired for refund > ৳5000 by cashier, got %v", err)
	}

	// Manager attempts same refund: Must SUCCEED
	_, err = engine.Refund.ProcessRefund(sale.ID, refItems, "manager-1", "Defective approved", pos.RoleManager)
	if err != nil {
		t.Fatalf("expected manager approval to succeed, got %v", err)
	}
}
