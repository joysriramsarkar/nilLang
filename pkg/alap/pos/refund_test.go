package pos

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

func TestRefundRBACAndStockRestoration(t *testing.T) {
	engine := NewPOSEngine()
	engine.SeedDefaultEnterpriseData()

	prod, _ := engine.Catalog.FindByID("p-01") // Miniket Rice ৳3,400.00
	initialStock := prod.Stock

	// Execute sale of 2 bags = ৳6,800.00
	cart := engine.Carts.GetActiveCart()
	cart.AddProduct(prod, data.NewDecimalFromInt(2))
	checkoutRes, err := engine.Checkout.Execute(cart, []PaymentRecord{
		{Method: MethodCash, AmountMinor: 680000},
	}, "cashier-01", "reg-01", "c-01")
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}

	saleID := checkoutRes.Sale.ID

	// 1. Attempt refund of 2 bags (৳6,800 > ৳5,000) as CASHIER -> must fail
	_, err = engine.Refund.ProcessRefund(saleID, []RefundItemRequest{
		{ProductID: prod.ID, Quantity: data.NewDecimalFromInt(2), Reason: "Customer changed mind"},
	}, "cashier-01", "Customer changed mind", RoleCashier)

	if err != ErrManagerApprovalRequired {
		t.Fatalf("Expected ErrManagerApprovalRequired for >৳5,000 refund by cashier, got %v", err)
	}

	// 2. Process refund of 1 bag (৳3,400 < ৳5,000) as CASHIER -> must succeed
	refundRec, err := engine.Refund.ProcessRefund(saleID, []RefundItemRequest{
		{ProductID: prod.ID, Quantity: data.NewDecimalFromInt(1), Reason: "1 bag excess"},
	}, "cashier-01", "1 bag excess", RoleCashier)
	if err != nil {
		t.Fatalf("Expected cashier refund for ৳3,400 to succeed, got %v", err)
	}
	if refundRec.RefundAmount != 340000 {
		t.Fatalf("Expected refund amount 340000, got %d", refundRec.RefundAmount)
	}

	// 3. Process remaining 1 bag as MANAGER -> must succeed even though role is manager
	refundRec2, err := engine.Refund.ProcessRefund(saleID, []RefundItemRequest{
		{ProductID: prod.ID, Quantity: data.NewDecimalFromInt(1), Reason: "Manager approved return"},
	}, "manager-01", "Manager approved return", RoleManager)
	if err != nil {
		t.Fatalf("Expected manager refund to succeed, got %v", err)
	}
	if refundRec2.RefundAmount != 340000 {
		t.Fatalf("Expected refund amount 340000, got %d", refundRec2.RefundAmount)
	}

	// Verify inventory was restored back to initialStock (2 sold, 2 refunded)
	updatedProd, _ := engine.Catalog.FindByID(prod.ID)
	if updatedProd.Stock.Cmp(initialStock) != 0 {
		t.Fatalf("Expected stock restored to %s, got %s", initialStock.String(), updatedProd.Stock.String())
	}
}
