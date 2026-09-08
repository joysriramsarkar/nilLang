package pos_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
	"github.com/joysriramsarkar/nilLang/pkg/alap/pos"
)

// newFailureTestDB creates a fresh engine + DB for each failure test.
func newFailureTestDB(t *testing.T) (*data.RealDBPool, *pos.POSEngine) {
	t.Helper()
	pool, err := data.OpenSQLite(filepath.Join(t.TempDir(), "failure_test.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	runner := data.NewRealMigrationRunner(pool)
	data.RegisterPOSMigrations(runner)
	if err := runner.Up(); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	engine := pos.NewPOSEngine(pos.CheckoutConfig{StoreName: "Failure Test Store"})
	engine.SetDB(pool)
	return pool, engine
}

// quickProduct seeds a product with the given stock and returns it.
func quickProduct(t *testing.T, pool *data.RealDBPool, engine *pos.POSEngine,
	id, sku string, price, cost int64, stock int64,
) *pos.Product {
	t.Helper()
	p := &pos.Product{
		ID: id, SKU: sku, Name: "Product " + sku,
		Unit:   "pcs",
		Price:  data.NewMoney(price, "BDT"),
		Cost:   data.NewMoney(cost, "BDT"),
		Stock:  data.NewDecimalFromInt(stock),
		Active: true,
	}
	engine.Catalog.AddProduct(p)
	engine.Inventory.RecordOpeningStock(id, data.NewDecimalFromInt(stock), data.NewMoney(cost, "BDT"))
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = pool.Exec(
		`INSERT OR IGNORE INTO products
		 (id,sku,barcode,name,unit,price_minor,cost_minor,stock_raw,low_stock_raw,currency,active,version,created_at,updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, sku, sku+"-BC", "Product "+sku, "pcs",
		price, cost, stock*data.DecimalScale, 0, "BDT", 1, 0, now, now,
	)
	return p
}

// dbStockInt returns the current DB stock_raw / DecimalScale for a product.
func dbStockInt(t *testing.T, pool *data.RealDBPool, productID string) int64 {
	t.Helper()
	var raw int64
	if err := pool.QueryRow("SELECT stock_raw FROM products WHERE id=?", productID).Scan(&raw); err != nil {
		t.Fatalf("read stock for %s: %v", productID, err)
	}
	return raw / data.DecimalScale
}

// ─── FAILURE TEST 1: Insufficient stock ──────────────────────────────────────

func TestCheckoutInsufficientStock(t *testing.T) {
	pool, engine := newFailureTestDB(t)
	quickProduct(t, pool, engine, "p1", "SKU-001", 1000, 600, 2)

	product, _ := engine.Catalog.FindByID("p1")
	cart := pos.NewCart("cart-insuf", "")
	cart.AddProduct(product, data.NewDecimalFromInt(3)) // request 3, only 2 in stock
	cart.Recalculate()

	_, err := engine.Checkout.Execute(cart, []pos.PaymentRecord{
		{Method: pos.MethodCash, AmountMinor: 3000},
	}, "cashier-1", "reg-1", "")

	if err == nil {
		t.Fatal("expected 'insufficient stock' error, got nil")
	}
	t.Logf("✓ Correctly rejected: %v", err)

	// DB stock must be unchanged at 2
	if stock := dbStockInt(t, pool, "p1"); stock != 2 {
		t.Errorf("expected DB stock=2, got %d", stock)
	}
	t.Log("✓ DB stock unchanged after rejected checkout")

	// No sale rows should have been inserted
	var saleCount int
	_ = pool.QueryRow("SELECT COUNT(*) FROM sales").Scan(&saleCount)
	if saleCount != 0 {
		t.Errorf("expected 0 sales in DB, got %d", saleCount)
	}
	t.Log("✓ No sale records committed on insufficient stock")
}

// ─── FAILURE TEST 2: Insufficient payment ────────────────────────────────────

func TestCheckoutInsufficientPayment(t *testing.T) {
	pool, engine := newFailureTestDB(t)
	quickProduct(t, pool, engine, "p2", "SKU-002", 5000, 3000, 10)

	product, _ := engine.Catalog.FindByID("p2")
	cart := pos.NewCart("cart-pay", "")
	cart.AddProduct(product, data.NewDecimalFromInt(1))
	cart.Recalculate()

	_, err := engine.Checkout.Execute(cart, []pos.PaymentRecord{
		{Method: pos.MethodCash, AmountMinor: 2000}, // only ৳20, need ৳50
	}, "cashier-2", "reg-1", "")

	if err == nil {
		t.Fatal("expected 'payment insufficient' error, got nil")
	}
	t.Logf("✓ Correctly rejected: %v", err)

	// DB stock unchanged
	if stock := dbStockInt(t, pool, "p2"); stock != 10 {
		t.Errorf("expected DB stock=10, got %d", stock)
	}
	t.Log("✓ DB stock unchanged after payment rejection")
}

// ─── FAILURE TEST 3: Empty cart ───────────────────────────────────────────────

func TestCheckoutEmptyCart(t *testing.T) {
	_, engine := newFailureTestDB(t)

	cart := pos.NewCart("cart-empty", "")
	// No items added
	cart.Recalculate()

	_, err := engine.Checkout.Execute(cart, []pos.PaymentRecord{
		{Method: pos.MethodCash, AmountMinor: 1000},
	}, "cashier-3", "reg-1", "")

	if err == nil {
		t.Fatal("expected 'cart is empty' error, got nil")
	}
	t.Logf("✓ Correctly rejected empty cart: %v", err)
}

// ─── FAILURE TEST 4: Stock race condition (2 goroutines, stock=1) ─────────────

func TestCheckoutStockRaceCondition(t *testing.T) {
	pool, engine := newFailureTestDB(t)
	quickProduct(t, pool, engine, "p4", "RACE-ONE", 1000, 600, 1)

	type result struct {
		err error
	}
	ch := make(chan result, 2)

	for i := 0; i < 2; i++ {
		cashier := fmt.Sprintf("cashier-%d", i)
		go func(c string) {
			product, _ := engine.Catalog.FindByID("p4")
			cart := pos.NewCart(fmt.Sprintf("cart-%s-%d", c, time.Now().UnixNano()), "")
			cart.AddProduct(product, data.NewDecimalFromInt(1))
			cart.Recalculate()
			_, err := engine.Checkout.Execute(cart, []pos.PaymentRecord{
				{Method: pos.MethodCash, AmountMinor: 1000},
			}, c, "reg-1", "")
			ch <- result{err}
		}(cashier)
	}

	r1 := <-ch
	r2 := <-ch

	successes := 0
	if r1.err == nil {
		successes++
	}
	if r2.err == nil {
		successes++
	}

	if successes != 1 {
		t.Errorf("expected exactly 1 success for stock=1, got %d (err1=%v err2=%v)",
			successes, r1.err, r2.err)
	}
	t.Log("✓ Exactly 1 of 2 concurrent cashiers succeeded for stock=1")

	// DB stock must be 0
	if stock := dbStockInt(t, pool, "p4"); stock != 0 {
		t.Errorf("expected DB stock=0, got %d", stock)
	}
	t.Log("✓ Final DB stock=0")

	// Exactly 1 sale row
	var saleCount int
	_ = pool.QueryRow("SELECT COUNT(*) FROM sales").Scan(&saleCount)
	if saleCount != 1 {
		t.Errorf("expected exactly 1 sale in DB, got %d", saleCount)
	}
	t.Log("✓ Exactly 1 sale committed")
}

// ─── FAILURE TEST 5: Panic recovery inside transaction ───────────────────────

func TestCheckoutPanicRecovery(t *testing.T) {
	pool, engine := newFailureTestDB(t)
	quickProduct(t, pool, engine, "p5", "PANIC-001", 1000, 600, 10)

	// We verify that Transaction()'s built-in panic recovery works by
	// testing it directly on the DB layer.
	var committed bool
	err := pool.Transaction(func(tx *data.RealTx) error {
		// Valid insert
		_, _ = tx.Exec(
			`INSERT INTO sales (id,invoice_number,register_id,cashier_id,subtotal_minor,
			  discount_minor,tax_minor,total_minor,paid_minor,change_minor,status,created_at,updated_at)
			 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			"panic-sale", "INV-PANIC-0001", "reg-1", "cashier-5",
			1000, 0, 0, 1000, 1000, 0, "COMPLETED",
			time.Now().UTC().Format(time.RFC3339),
			time.Now().UTC().Format(time.RFC3339),
		)
		committed = true
		// Panic mid-transaction
		panic("simulated hardware failure")
	})

	if err == nil {
		t.Fatal("expected error from panicking transaction, got nil")
	}
	t.Logf("✓ Panic correctly recovered: %v", err)

	// The sale must NOT be in the DB — transaction was rolled back
	var count int
	_ = pool.QueryRow("SELECT COUNT(*) FROM sales WHERE id='panic-sale'").Scan(&count)
	if count != 0 {
		t.Errorf("panic transaction was NOT rolled back — sale exists in DB (committed=%v)", committed)
	}
	t.Log("✓ Panicking transaction was fully rolled back")
}

// ─── FAILURE TEST 6: Credit customer balance exactness ───────────────────────

func TestCheckoutCreditCustomerBalance(t *testing.T) {
	pool, engine := newFailureTestDB(t)
	quickProduct(t, pool, engine, "p6", "CREDIT-001", 2000, 1000, 20)

	// Register a customer
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := pool.Exec(
		`INSERT INTO customers (id,name,phone,total_purchases_minor,due_balance_minor,credit_limit_minor,active,created_at,updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		"cust-001", "Rahim Mia", "01700000000", 0, 0, 50000, 1, now, now,
	)
	if err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	engine.Customers.AddCustomer(&pos.Customer{
		ID:    "cust-001",
		Name:  "Rahim Mia",
		Phone: "01700000000",
	})

	// Checkout: ৳15 credit + ৳5 cash (product price ৳20)
	product, _ := engine.Catalog.FindByID("p6")
	cart := pos.NewCart("cart-credit", "cust-001")
	cart.AddProduct(product, data.NewDecimalFromInt(1))
	cart.Recalculate()

	result, err := engine.Checkout.Execute(cart, []pos.PaymentRecord{
		{Method: pos.MethodCash, AmountMinor: 500},
		{Method: pos.MethodCredit, AmountMinor: 1500},
	}, "cashier-6", "reg-1", "cust-001")
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	t.Logf("✓ Credit checkout succeeded: invoice=%s", result.Sale.InvoiceNumber)

	// customer.due_balance_minor must equal 1500 (exactly the credit amount)
	var dueBalance int64
	if err := pool.QueryRow(
		"SELECT due_balance_minor FROM customers WHERE id='cust-001'",
	).Scan(&dueBalance); err != nil {
		t.Fatalf("read due_balance: %v", err)
	}
	if dueBalance != 1500 {
		t.Errorf("expected due_balance_minor=1500, got %d", dueBalance)
	}
	t.Logf("✓ Customer due_balance_minor exact: %d", dueBalance)

	// customer_ledger must have exactly 1 CREDIT_SALE entry
	var ledgerCount int
	_ = pool.QueryRow(
		"SELECT COUNT(*) FROM customer_ledger WHERE customer_id='cust-001' AND type='CREDIT_SALE'",
	).Scan(&ledgerCount)
	if ledgerCount != 1 {
		t.Errorf("expected 1 CREDIT_SALE ledger entry, got %d", ledgerCount)
	}
	t.Log("✓ Customer ledger CREDIT_SALE entry created")
}

// ─── FAILURE TEST 7: Transaction atomicity — all-or-nothing ──────────────────

func TestCheckoutTransactionAtomicity(t *testing.T) {
	pool, engine := newFailureTestDB(t)
	quickProduct(t, pool, engine, "p7a", "ATOM-001", 1000, 600, 5)
	quickProduct(t, pool, engine, "p7b", "ATOM-002", 500, 300, 0) // out of stock

	// Cart with two items: first valid, second out-of-stock.
	// The whole transaction must fail atomically — no partial writes.
	p1, _ := engine.Catalog.FindByID("p7a")
	p2, _ := engine.Catalog.FindByID("p7b")
	cart := pos.NewCart("cart-atomic", "")
	cart.AddProduct(p1, data.NewDecimalFromInt(1))
	cart.AddProduct(p2, data.NewDecimalFromInt(1)) // will fail stock check
	cart.Recalculate()

	_, err := engine.Checkout.Execute(cart, []pos.PaymentRecord{
		{Method: pos.MethodCash, AmountMinor: 1500},
	}, "cashier-7", "reg-1", "")

	if err == nil {
		t.Fatal("expected error due to out-of-stock item, got nil")
	}
	t.Logf("✓ Mixed-stock cart correctly rejected: %v", err)

	// p7a stock must be unchanged at 5
	if stock := dbStockInt(t, pool, "p7a"); stock != 5 {
		t.Errorf("p7a stock should be 5 (unchanged), got %d", stock)
	}
	t.Log("✓ Valid product stock unchanged — no partial write")

	// No sales at all
	var saleCount int
	_ = pool.QueryRow("SELECT COUNT(*) FROM sales").Scan(&saleCount)
	if saleCount != 0 {
		t.Errorf("expected 0 sales (atomic rollback), got %d", saleCount)
	}
	t.Log("✓ Zero sales committed — transaction is all-or-nothing")
}
