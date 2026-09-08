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

// ProcessRefund processes a product return on an existing sale
func (rs *RefundService) ProcessRefund(
	saleID string,
	items []RefundItemRequest,
	cashierID string,
	reason string,
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

	var totalRefundMinor int64 = 0

	// 1. Verify items belong to sale and quantities are valid
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
		if req.Quantity.Cmp(foundItem.Quantity) > 0 {
			return nil, fmt.Errorf("refund quantity %s exceeds sold quantity %s", req.Quantity.String(), foundItem.Quantity.String())
		}

		lineRefundMoney := data.NewMoney(foundItem.UnitPriceMinor, "BDT").MulDecimal(req.Quantity)
		totalRefundMinor += lineRefundMoney.Minor

		// 2. Restore physical stock via MovementReturn
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

	// 3. Update sale state
	sale.Status = StatusRefunded
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
