package pos

import (
	"strings"
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

func TestPOSEngineEndToEndVerticalSlice(t *testing.T) {
	engine := NewPOSEngine()
	engine.SeedDefaultEnterpriseData()

	// 1. Catalog & Barcode Search
	prod, ok := engine.Catalog.FindByBarcode("8901030012345")
	if !ok || prod.SKU != "RICE-MIN-50" {
		t.Fatalf("Expected Miniket rice by barcode, got ok=%v, prod=%v", ok, prod)
	}

	searchBn := engine.Catalog.Search("মিনিকেট")
	if len(searchBn) == 0 {
		t.Fatalf("Expected Bengali search to find Miniket rice")
	}

	initialStock := prod.Stock

	// 2. Cart Preparation (Tab 1)
	cart := engine.Carts.GetActiveCart()
	cart.AddProduct(prod, data.NewDecimalFromInt(2)) // 2 bags

	prod2, _ := engine.Catalog.FindBySKU("OIL-TEER-5L")
	cart.AddProduct(prod2, data.NewDecimalFromInt(1)) // 1 bottle

	// Apply ৳80 Discount
	cart.ApplyDiscount(Discount{
		Type:  DiscountFixedMinor,
		Value: data.NewDecimalFromInt(8000),
	})

	cart.Recalculate()

	// Subtotal: 2 * 340000 (680000) + 89000 = 769000 (৳7,690.00)
	// Discount: 8000 (৳80.00)
	// Grand Total: 761000 (৳7,610.00)
	if cart.SubtotalMinor != 769000 {
		t.Fatalf("Expected subtotal 769000, got %d", cart.SubtotalMinor)
	}
	if cart.DiscountMinor != 8000 {
		t.Fatalf("Expected discount 8000, got %d", cart.DiscountMinor)
	}
	if cart.GrandTotalMinor != 761000 {
		t.Fatalf("Expected grand total 761000, got %d", cart.GrandTotalMinor)
	}

	// 3. Multi-Tab Hold Cart (Switch to Tab 2 and verify independence)
	_, err := engine.Carts.SwitchTab("tab-2")
	if err != nil {
		t.Fatalf("SwitchTab failed: %v", err)
	}
	tab2Cart := engine.Carts.GetActiveCart()
	if len(tab2Cart.Items) != 0 {
		t.Fatalf("Expected Tab 2 to be empty")
	}

	// Switch back to Tab 1
	_, _ = engine.Carts.SwitchTab("tab-1")

	// 4. Atomic Checkout (Cash Tender: ৳8,000 -> Change: ৳390)
	payments := []PaymentRecord{
		{
			Method:      MethodCash,
			AmountMinor: 800000, // ৳8,000.00
		},
	}

	res, err := engine.Checkout.Execute(cart, payments, "cashier-01", "reg-01", "c-01")
	if err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	if res.Sale.TotalMinor != 761000 {
		t.Fatalf("Expected sale total 761000, got %d", res.Sale.TotalMinor)
	}
	if res.Tender.ChangeDueMinor != 39000 { // ৳390.00 change
		t.Fatalf("Expected change 39000, got %d", res.Tender.ChangeDueMinor)
	}
	if !res.TriggerDrawer {
		t.Fatalf("Expected cash drawer to trigger on cash payment")
	}
	if !strings.Contains(res.ReceiptText, "LAKHAN BHANDAR") && !strings.Contains(res.ReceiptText, "লাখান ভাণ্ডার") {
		t.Fatalf("Expected receipt text to contain store name")
	}

	// 5. Verify Inventory Decrement
	updatedProd, _ := engine.Catalog.FindByID(prod.ID)
	expectedStock := initialStock.Sub(data.NewDecimalFromInt(2))
	if updatedProd.Stock.Cmp(expectedStock) != 0 {
		t.Fatalf("Expected stock to decrement to %s, got %s", expectedStock.String(), updatedProd.Stock.String())
	}

	// 6. Verify Shift Updated
	shift, err := engine.Shifts.CurrentShift()
	if err != nil || shift.TotalOrders != 1 {
		t.Fatalf("Expected 1 order in shift, got %v", shift)
	}
	// Net cash received = 800000 - 39000 = 761000
	if shift.CashSalesMinor != 761000 {
		t.Fatalf("Expected cash sales 761000, got %d", shift.CashSalesMinor)
	}

	// 7. Verify Refund Process
	refundReq := []RefundItemRequest{
		{
			ProductID: prod.ID,
			Quantity:  data.NewDecimalFromInt(1), // Return 1 bag
			Reason:    "Customer changed mind",
		},
	}
	refRecord, err := engine.Refund.ProcessRefund(res.Sale.ID, refundReq, "cashier-01", "Return 1 bag")
	if err != nil {
		t.Fatalf("Refund failed: %v", err)
	}
	if refRecord.RefundAmount != 340000 {
		t.Fatalf("Expected refund amount 340000, got %d", refRecord.RefundAmount)
	}

	// Verify Stock Restored after refund (+1 bag)
	restoredProd, _ := engine.Catalog.FindByID(prod.ID)
	expectedAfterRefund := expectedStock.Add(data.NewDecimalFromInt(1))
	if restoredProd.Stock.Cmp(expectedAfterRefund) != 0 {
		t.Fatalf("Expected stock to restore to %s, got %s", expectedAfterRefund.String(), restoredProd.Stock.String())
	}

	// 8. Customer Due Payment
	cust, _ := engine.Customers.FindByID("c-01")
	initialDue := cust.DueBalanceMinor
	entry, err := engine.Customers.RecordDuePayment("c-01", 500000, "REC-101", "Bank Transfer")
	if err != nil {
		t.Fatalf("RecordDuePayment failed: %v", err)
	}
	if entry.BalanceAfter != initialDue-500000 {
		t.Fatalf("Expected due to decrease to %d, got %d", initialDue-500000, entry.BalanceAfter)
	}

	// 9. Reporting & CSV Export
	rep := engine.Reports.GenerateDailyReport(time.Now())
	if rep.TotalOrders != 1 {
		t.Fatalf("Expected 1 order in daily report, got %d", rep.TotalOrders)
	}

	csvContent, err := engine.Reports.ExportSalesCSV()
	if err != nil || !strings.Contains(csvContent, "INV-") {
		t.Fatalf("ExportSalesCSV failed: %v", err)
	}
}
