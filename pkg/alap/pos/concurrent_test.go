package pos_test

import (
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
	"github.com/joysriramsarkar/nilLang/pkg/alap/pos"
)

// openTestDB opens a SQLite test database, runs all POS migrations, and
// returns the pool and a POS engine backed by the real DB.
func openTestDB(t *testing.T) (*data.RealDBPool, *pos.POSEngine) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "concur_test.db")
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
		StoreName:    "Concurrent Test Store",
		StoreSubname: "Race Condition Test",
	})
	engine.SetDB(pool)
	return pool, engine
}

// seedProduct inserts a product into both the catalog and the DB with the
// given stock quantity.
func seedProduct(t *testing.T, pool *data.RealDBPool, engine *pos.POSEngine,
	id, sku, name string, priceMinor, costMinor int64, stock int64,
) *pos.Product {
	t.Helper()
	p := &pos.Product{
		ID:      id,
		SKU:     sku,
		Barcode: sku + "-BC",
		Name:    name,
		Unit:    "pcs",
		Price:   data.NewMoney(priceMinor, "BDT"),
		Cost:    data.NewMoney(costMinor, "BDT"),
		Stock:   data.NewDecimalFromInt(stock),
		Active:  true,
	}
	engine.Catalog.AddProduct(p)
	engine.Inventory.RecordOpeningStock(id, data.NewDecimalFromInt(stock), data.NewMoney(costMinor, "BDT"))

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := pool.Exec(
		`INSERT OR IGNORE INTO products
		 (id,sku,barcode,name,unit,price_minor,cost_minor,stock_raw,low_stock_raw,currency,active,version,created_at,updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, sku, sku+"-BC", name, "pcs",
		priceMinor, costMinor, stock*data.DecimalScale, 0, "BDT", 1, 0,
		now, now,
	)
	if err != nil {
		t.Fatalf("seed product %s: %v", sku, err)
	}
	return p
}

// ─── TEST: 100 concurrent cashiers, 30-unit stock ────────────────────────────

// TestConcurrent100CashierSameStock spawns 100 goroutines that all attempt to
// buy 1 unit of the same product. Only 30 should succeed; the remaining 70 must
// get "insufficient stock" or "concurrent conflict" errors. The final DB stock
// must be exactly 0, and the total quantity in sale_items must equal 30.
func TestConcurrent100CashierSameStock(t *testing.T) {
	const (
		initialStock = 30
		goroutines   = 100
	)

	pool, engine := openTestDB(t)
	seedProduct(t, pool, engine, "p-race", "RACE-001", "Race Stock Item", 1000, 600, initialStock)

	var (
		wg         sync.WaitGroup
		succeeded  int64 // atomic counter of successful checkouts
		failed     int64 // atomic counter of expected failures
		errsMu     sync.Mutex
		sampleErrs []string
	)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		cashierID := fmt.Sprintf("cashier-%03d", i)
		go func(cashier string) {
			defer wg.Done()

			cart := pos.NewCart(fmt.Sprintf("cart-%s-%d", cashier, time.Now().UnixNano()), "")
			product, ok := engine.Catalog.FindByID("p-race")
			if !ok {
				return
			}
			cart.AddProduct(product, data.NewDecimalFromInt(1))
			cart.Recalculate()

			_, err := engine.Checkout.Execute(cart, []pos.PaymentRecord{
				{Method: pos.MethodCash, AmountMinor: 1000},
			}, cashier, "reg-01", "")

			if err == nil {
				atomic.AddInt64(&succeeded, 1)
			} else {
				atomic.AddInt64(&failed, 1)
				errsMu.Lock()
				if len(sampleErrs) < 5 {
					sampleErrs = append(sampleErrs, err.Error())
				}
				errsMu.Unlock()
			}
		}(cashierID)
	}
	wg.Wait()

	t.Logf("✓ Concurrent results: %d succeeded / %d failed (stock was %d), sample errors: %v",
		succeeded, failed, initialStock, sampleErrs)

	// Assertion 1: exactly initialStock checkouts succeeded
	if succeeded != int64(initialStock) {
		t.Errorf("expected exactly %d successes, got %d", initialStock, succeeded)
	}
	// Assertion 2: total goroutines accounted for
	if succeeded+failed != int64(goroutines) {
		t.Errorf("succeeded(%d)+failed(%d) != goroutines(%d)", succeeded, failed, goroutines)
	}

	// Assertion 3: DB stock is exactly 0
	var dbStock int64
	if err := pool.QueryRow("SELECT stock_raw FROM products WHERE id='p-race'").Scan(&dbStock); err != nil {
		t.Fatalf("read db stock: %v", err)
	}
	if dbStock != 0 {
		t.Errorf("expected DB stock_raw=0 after %d sales, got %d", initialStock, dbStock)
	}
	t.Logf("✓ Final DB stock_raw = %d (expected 0)", dbStock)

	// Assertion 4: sum of quantity_raw in sale_items equals initialStock * DecimalScale
	var totalSoldRaw int64
	if err := pool.QueryRow(
		"SELECT COALESCE(SUM(quantity_raw),0) FROM sale_items WHERE product_id='p-race'",
	).Scan(&totalSoldRaw); err != nil {
		t.Fatalf("sum quantity_raw: %v", err)
	}
	expectedRaw := int64(initialStock) * data.DecimalScale
	if totalSoldRaw != expectedRaw {
		t.Errorf("sum(quantity_raw)=%d, expected %d (%d units × scale %d)",
			totalSoldRaw, expectedRaw, initialStock, data.DecimalScale)
	}
	t.Logf("✓ Total sold quantity_raw = %d = %d units", totalSoldRaw, initialStock)

	// Assertion 5: no stock_movements have negative balance_raw
	var negMov int
	if err := pool.QueryRow(
		"SELECT COUNT(*) FROM stock_movements WHERE balance_raw < 0",
	).Scan(&negMov); err != nil {
		t.Fatalf("check negative balances: %v", err)
	}
	if negMov > 0 {
		t.Errorf("found %d stock_movements with negative balance_raw — stock went below zero!", negMov)
	}
	t.Log("✓ No stock_movements with negative balance_raw")
}

// TestConcurrentMultiProductStock spawns 5 × 20 goroutines across 5 different
// SKUs, each with 10 units of stock. Each product should sell exactly 10 units.
func TestConcurrentMultiProductStock(t *testing.T) {
	const (
		products     = 5
		stockPerSKU  = 10
		buyersPerSKU = 20 // 2× the stock → exactly 10 succeed per SKU
	)

	pool, engine := openTestDB(t)

	for i := 0; i < products; i++ {
		id := fmt.Sprintf("p-multi-%d", i)
		seedProduct(t, pool, engine, id, fmt.Sprintf("MULTI-%03d", i),
			fmt.Sprintf("Multi Product %d", i), 500, 300, stockPerSKU)
	}

	var wg sync.WaitGroup
	successes := make([]int64, products)

	for pIdx := 0; pIdx < products; pIdx++ {
		for buyer := 0; buyer < buyersPerSKU; buyer++ {
			wg.Add(1)
			pid := fmt.Sprintf("p-multi-%d", pIdx)
			si := pIdx
			cashier := fmt.Sprintf("c%d-%03d", pIdx, buyer)
			go func() {
				defer wg.Done()
				product, ok := engine.Catalog.FindByID(pid)
				if !ok {
					return
				}
				cart := pos.NewCart(fmt.Sprintf("cart-%s-%d", cashier, time.Now().UnixNano()), "")
				cart.AddProduct(product, data.NewDecimalFromInt(1))
				cart.Recalculate()
				_, err := engine.Checkout.Execute(cart, []pos.PaymentRecord{
					{Method: pos.MethodCash, AmountMinor: 500},
				}, cashier, "reg-01", "")
				if err == nil {
					atomic.AddInt64(&successes[si], 1)
				}
			}()
		}
	}
	wg.Wait()

	for i := 0; i < products; i++ {
		if successes[i] != int64(stockPerSKU) {
			t.Errorf("product p-multi-%d: expected %d successes, got %d",
				i, stockPerSKU, successes[i])
		}
	}
	t.Logf("✓ Multi-product concurrent test: each of %d SKUs sold exactly %d units",
		products, stockPerSKU)

	// Verify all DB stocks are 0
	for i := 0; i < products; i++ {
		var stock int64
		_ = pool.QueryRow("SELECT stock_raw FROM products WHERE id=?",
			fmt.Sprintf("p-multi-%d", i)).Scan(&stock)
		if stock != 0 {
			t.Errorf("p-multi-%d: DB stock_raw=%d, expected 0", i, stock)
		}
	}
	t.Log("✓ All product DB stocks are exactly 0")
}

// TestConcurrentOrderCountUniqueness verifies that concurrent checkouts never
// generate duplicate invoice numbers (atomic.AddInt64 must be truly atomic).
func TestConcurrentOrderCountUniqueness(t *testing.T) {
	const (
		stock      = 200
		goroutines = 50
	)

	pool, engine := openTestDB(t)
	seedProduct(t, pool, engine, "p-inv", "INV-SKU", "Invoice Uniqueness Product", 100, 60, stock)

	var wg sync.WaitGroup
	var mu sync.Mutex
	invoices := make(map[string]struct{})
	var dupCount int

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		cashier := fmt.Sprintf("cashier-%03d", i)
		go func() {
			defer wg.Done()
			product, ok := engine.Catalog.FindByID("p-inv")
			if !ok {
				return
			}
			cart := pos.NewCart(fmt.Sprintf("cart-%s-%d", cashier, time.Now().UnixNano()), "")
			cart.AddProduct(product, data.NewDecimalFromInt(1))
			cart.Recalculate()
			result, err := engine.Checkout.Execute(cart, []pos.PaymentRecord{
				{Method: pos.MethodCash, AmountMinor: 100},
			}, cashier, "reg-01", "")
			if err == nil && result != nil {
				mu.Lock()
				if _, exists := invoices[result.Sale.InvoiceNumber]; exists {
					dupCount++
				}
				invoices[result.Sale.InvoiceNumber] = struct{}{}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if dupCount > 0 {
		t.Errorf("found %d duplicate invoice numbers — atomic orderCount is broken", dupCount)
	}
	t.Logf("✓ All %d concurrent checkouts produced unique invoice numbers", goroutines)
}
