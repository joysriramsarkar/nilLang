package pos

import (
	"fmt"
	"sync"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
	"github.com/joysriramsarkar/nilLang/pkg/alap/device"
)

// RefundItemRequest specifies item and quantity being returned
type RefundItemRequest struct {
	ProductID string       `json:"product_id"`
	Quantity  data.Decimal `json:"quantity"`
	Reason    string       `json:"reason"`
}

// RefundRecord represents an executed refund
type RefundRecord struct {
	ID            string              `json:"id"`
	SaleID        string              `json:"sale_id"`
	InvoiceNumber string              `json:"invoice_number"`
	CashierID     string              `json:"cashier_id"`
	RefundAmount  int64               `json:"refund_amount_minor"`
	Items         []RefundItemRequest `json:"items"`
	Timestamp     time.Time           `json:"timestamp"`
	ReceiptText   string              `json:"receipt_text"`
}

// RefundService handles retail returns and inventory restoration
type RefundService struct {
	mu        sync.Mutex
	checkout  *CheckoutService
	inventory *InventoryLedger
	audit     *AuditTrail
	refunds   map[string]*RefundRecord
}

// NewRefundService creates a RefundService
func NewRefundService(checkout *CheckoutService, inventory *InventoryLedger, audit *AuditTrail) *RefundService {
	return &RefundService{
		checkout:  checkout,
		inventory: inventory,
		audit:     audit,
		refunds:   make(map[string]*RefundRecord),
	}
}

// ProcessRefund processes a product return on an existing sale with RBAC authorization.
func (rs *RefundService) ProcessRefund(
	saleID string,
	items []RefundItemRequest,
	cashierID string,
	reason string,
	cashierRole ...string,
) (*RefundRecord, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	sale, ok := rs.checkout.GetSale(saleID)
	if !ok {
		return nil, fmt.Errorf("sale not found: %s", saleID)
	}

	if sale.Status != StatusCompleted && sale.Status != StatusPartiallyRefunded {
		return nil, fmt.Errorf("cannot refund sale with status %s", sale.Status)
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no items specified for refund")
	}

	role := RoleCashier
	if len(cashierRole) > 0 && cashierRole[0] != "" {
		role = cashierRole[0]
	}

	var totalRefundMinor int64 = 0

	// 1. Verify items belong to sale, quantities are valid, and compute total
	for _, req := range items {
		var foundItem *SaleItem
		for i := range sale.Items {
			if sale.Items[i].ProductID == req.ProductID {
				foundItem = &sale.Items[i]
				break
			}
		}
		if foundItem == nil {
			return nil, fmt.Errorf("product %s was not in sale %s", req.ProductID, sale.InvoiceNumber)
		}
		remainingQty := foundItem.Quantity.Sub(foundItem.RefundedQuantity)
		if req.Quantity.Cmp(remainingQty) > 0 {
			return nil, fmt.Errorf("refund quantity %s exceeds refundable quantity %s", req.Quantity.String(), remainingQty.String())
		}

		lineRefundMoney := data.NewMoney(foundItem.UnitPriceMinor, "BDT").MulDecimal(req.Quantity)
		totalRefundMinor += lineRefundMoney.Minor
	}

	// 2. Enforce RBAC permission (refunds > ৳5,000 require manager)
	if err := CheckRefundAuthorization(role, totalRefundMinor); err != nil {
		return nil, err
	}

	// 3. Restore physical stock via MovementReturn and update RefundedQuantity
	for _, req := range items {
		for i := range sale.Items {
			if sale.Items[i].ProductID == req.ProductID {
				sale.Items[i].RefundedQuantity = sale.Items[i].RefundedQuantity.Add(req.Quantity)
				break
			}
		}

		_, err := rs.inventory.RecordMovement(
			req.ProductID,
			MovementReturn,
			req.Quantity, // Positive delta restores stock
			sale.InvoiceNumber,
			fmt.Sprintf("Refund on %s (Reason: %s)", sale.InvoiceNumber, reason),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to restore stock: %w", err)
		}
	}

	// 4. Update sale state (PartiallyRefunded vs Refunded)
	allRefunded := true
	for _, it := range sale.Items {
		if it.RefundedQuantity.Cmp(it.Quantity) < 0 {
			allRefunded = false
			break
		}
	}
	if allRefunded {
		sale.Status = StatusRefunded
	} else {
		sale.Status = StatusPartiallyRefunded
	}
	sale.UpdatedAt = time.Now()


	// 4. Generate Refund Receipt
	refundID := fmt.Sprintf("ref-%d", time.Now().UnixNano())
	receiptLines := make([]device.ReceiptLineItem, len(items))
	for i, req := range items {
		receiptLines[i] = device.ReceiptLineItem{
			Name:     req.ProductID,
			Quantity: req.Quantity.String(),
			Price:    "-",
			Total:    "-",
		}
	}

	receiptData := device.ReceiptPayload{
		StoreName:     "লাখান ভাণ্ডার (Lakhan Bhandar)",
		StoreSubtitle: "*** REFUND / RETURN RECEIPT ***",
		InvoiceNo:     sale.InvoiceNumber,
		DateStr:       time.Now().Format("02/01/2006 03:04 PM"),
		Cashier:       cashierID,
		Items:         receiptLines,
		GrandTotal:    data.NewMoney(totalRefundMinor, "BDT").Format(),
		PaymentMethod: "Refund Payout (Cash)",
		PaidAmount:    data.NewMoney(totalRefundMinor, "BDT").Format(),
		FooterNote:    "Refund processed successfully.",
	}

	formatter := device.NewESCPOSFormatter(device.Width58mm)
	receiptText := formatter.FormatPlainText(receiptData)

	// 5. Record Audit
	rs.audit.Record(
		ActionSaleRefunded,
		sale.ID,
		cashierID,
		map[string]interface{}{"status": StatusCompleted},
		map[string]interface{}{"status": StatusRefunded, "refundMinor": totalRefundMinor},
		fmt.Sprintf("Refund for invoice %s (Reason: %s)", sale.InvoiceNumber, reason),
	)

	record := &RefundRecord{
		ID:            refundID,
		SaleID:        saleID,
		InvoiceNumber: sale.InvoiceNumber,
		CashierID:     cashierID,
		RefundAmount:  totalRefundMinor,
		Items:         items,
		Timestamp:     time.Now(),
		ReceiptText:   receiptText,
	}

	rs.refunds[refundID] = record
	return record, nil
}
