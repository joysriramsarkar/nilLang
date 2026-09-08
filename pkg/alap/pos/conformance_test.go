package pos

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

// TestReferencePOSConformanceSuite tests parity across business fixtures (web-implications.md Section 27)
func TestReferencePOSConformanceSuite(t *testing.T) {
	// Setup engine and SQLite-backed DBPool
	pool := data.NewDBPool(data.DBPoolConfig{Driver: data.DriverSQLite})
	engine := NewPOSEngine()
	engine.SetDBPool(pool)
	engine.SeedDefaultEnterpriseData()

	// ─── FIXTURE 1: Standard Basket with Fixed Discount & Cash Tender ────────
	t.Run("Fixture1_StandardBasketFixedDiscount", func(t *testing.T) {
		cart := NewCart("c-fix-1", "Standard Basket")
		p1, _ := engine.Catalog.FindByID("p-01") // ৳3,400.00 (340000 minor)
		p3, _ := engine.Catalog.FindByID("p-03") // ৳890.00 (89000 minor)

		// 2 units of p-01, 1 unit of p-03
		cart.AddProduct(p1, data.NewDecimalFromInt(2)) // 680000
		cart.AddProduct(p3, data.NewDecimalFromInt(1)) // 89000
		// Subtotal: 769000

		// Apply fixed discount of ৳190.00 (19000 minor)
		cart.ApplyDiscount(Discount{
			Type:  DiscountFixedMinor,
			Value: data.NewDecimalFromInt(19000),
		})
		cart.Recalculate()

		expectedSubtotal := int64(769000)
		expectedDiscount := int64(19000)
		expectedTotal := int64(750000) // ৳7,500.00

		if cart.SubtotalMinor != expectedSubtotal {
			t.Fatalf("Subtotal mismatch: expected %d, got %d", expectedSubtotal, cart.SubtotalMinor)
		}
		if cart.DiscountMinor != expectedDiscount {
			t.Fatalf("Discount mismatch: expected %d, got %d", expectedDiscount, cart.DiscountMinor)
		}
		if cart.GrandTotalMinor != expectedTotal {
			t.Fatalf("GrandTotal mismatch: expected %d, got %d", expectedTotal, cart.GrandTotalMinor)
		}

		// Tender ৳8,000.00 in cash -> Change ৳500.00 (50000 minor)
		payments := []PaymentRecord{
			{Method: MethodCash, AmountMinor: 800000},
		}
		res, err := engine.Checkout.Execute(cart, payments, "cashier-01", "reg-01", "c-01")
		if err != nil {
			t.Fatalf("Checkout failed: %v", err)
		}
		if res.Tender.ChangeDueMinor != 50000 {
			t.Fatalf("ChangeDue mismatch: expected 50000, got %d", res.Tender.ChangeDueMinor)
		}
		if !res.TriggerDrawer {
			t.Fatalf("Cash payment must trigger drawer")
		}
	})

	// ─── FIXTURE 2: Fractional Weight & Unit Compatibility ───────────────────
	t.Run("Fixture2_FractionalWeightPricing", func(t *testing.T) {
		cart := NewCart("c-fix-2", "Fractional Weight")
		pSugar, _ := engine.Catalog.FindByID("p-06") // Deshi Sugar, ৳140.00/kg (14000 minor), Unit: কেজি

		// 2.5 kg sugar
		qty, _ := data.ParseDecimal("2.5")
		cart.AddProduct(pSugar, qty)
		cart.Recalculate()

		// 2.5 * 14000 = 35000 (৳350.00)
		expectedTotal := int64(35000)
		if cart.GrandTotalMinor != expectedTotal {
			t.Fatalf("Fractional price mismatch: expected %d, got %d", expectedTotal, cart.GrandTotalMinor)
		}
	})

	// ─── FIXTURE 3: Split Payment Tender (Cash + UPI/MFS) ────────────────────
	t.Run("Fixture3_SplitPayment", func(t *testing.T) {
		cart := NewCart("c-fix-3", "Split Payment")
		pAtta, _ := engine.Catalog.FindByID("p-05") // ৳125.00 (12500 minor)

		// 4 packets of Atta -> ৳500.00 (50000 minor)
		cart.AddProduct(pAtta, data.NewDecimalFromInt(4))
		cart.Recalculate()

		// Split payment: Cash ৳200.00 + bKash ৳300.00 = ৳500.00
		payments := []PaymentRecord{
			{Method: MethodCash, AmountMinor: 20000},
			{Method: MethodBKash, AmountMinor: 30000, Reference: "TRX-BKASH-9988"},
		}

		res, err := engine.Checkout.Execute(cart, payments, "cashier-01", "reg-01", "c-02")
		if err != nil {
			t.Fatalf("Split checkout failed: %v", err)
		}
		if res.Tender.ChangeDueMinor != 0 {
			t.Fatalf("Expected 0 change, got %d", res.Tender.ChangeDueMinor)
		}
		if res.Sale.TotalMinor != 50000 {
			t.Fatalf("Expected sale total 50000, got %d", res.Sale.TotalMinor)
		}
	})

	// ─── FIXTURE 4: Stock Ledger Reconciliation ──────────────────────────────
	t.Run("Fixture4_StockLedgerReconciliation", func(t *testing.T) {
		p, _ := engine.Catalog.FindByID("p-08") // Molla Salt
		initialStock := p.Stock

		// Sell 5 units
		negDelta := data.Decimal{Value: -5 * data.DecimalScale}
		_, err := engine.Inventory.RecordMovement(p.ID, MovementSale, negDelta, "INV-TEST-01", "Test sale")
		if err != nil {
			t.Fatalf("RecordMovement error: %v", err)
		}

		// Reconcile
		rec, err := engine.Inventory.ReconcileProduct(p.ID)
		if err != nil {
			t.Fatalf("ReconcileProduct error: %v", err)
		}
		if !rec.Balanced {
			t.Fatalf("Inventory must be balanced with ledger: discrepancy=%s", rec.Discrepancy.String())
		}
		expectedAfter := initialStock.Sub(data.NewDecimalFromInt(5))
		if rec.CatalogStock.Cmp(expectedAfter) != 0 {
			t.Fatalf("Expected stock %s, got %s", expectedAfter.String(), rec.CatalogStock.String())
		}
	})

	// ─── FIXTURE 5: Shift Cash Drops and Reconciliation ──────────────────────
	t.Run("Fixture5_ShiftReconciliation", func(t *testing.T) {
		shift, err := engine.Shifts.CurrentShift()
		if err != nil {
			t.Fatalf("CurrentShift error: %v", err)
		}
		expectedBeforeDrop := shift.ExpectedCashMinor

		// Drop ৳2,000.00 cash to safe (Cash Out)
		cmov, err := engine.Shifts.RecordCashMovement(CashMovementOut, 200000, "Drop cash to safe")
		if err != nil {
			t.Fatalf("RecordCashMovement error: %v", err)
		}
		if cmov.AmountMinor != 200000 {
			t.Fatalf("Expected 200000 drop amount, got %d", cmov.AmountMinor)
		}

		updatedShift, _ := engine.Shifts.CurrentShift()
		if updatedShift.ExpectedCashMinor != expectedBeforeDrop-200000 {
			t.Fatalf("Expected cash must decrease after drop: expected %d, got %d",
				expectedBeforeDrop-200000, updatedShift.ExpectedCashMinor)
		}
	})
}
