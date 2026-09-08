package pos

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
	"github.com/joysriramsarkar/nilLang/pkg/alap/device"
)

var refundSeq int64

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

// RefundService handles retail returns, database persistence, and inventory restoration
type RefundService struct {
	mu        sync.Mutex
	checkout  *CheckoutService
	inventory *InventoryLedger
	audit     *AuditTrail
	db        *data.RealDBPool
	customers *CustomerRepository
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

// SetDB configures the persistent database pool for transactional refunds.
func (rs *RefundService) SetDB(db *data.RealDBPool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.db = db
}

// SetCustomers configures the customer repository for credit account adjustments.
func (rs *RefundService) SetCustomers(customers *CustomerRepository) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.customers = customers
}

// GetRefund returns a refund record by ID.
func (rs *RefundService) GetRefund(id string) (*RefundRecord, bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	r, ok := rs.refunds[id]
	return r, ok
}

// ProcessRefund processes a product return on an existing sale with RBAC authorization and ACID transactions.
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
		if req.Quantity.Value <= 0 {
			return nil, fmt.Errorf("invalid refund quantity for product %s: must be > 0", req.ProductID)
		}
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

	seq := atomic.AddInt64(&refundSeq, 1)
	now := time.Now()
	nowStr := now.UTC().Format(time.RFC3339)
	refundID := fmt.Sprintf("ref-%d-%d", now.UnixNano(), seq)

	allRefunded := true
	for _, req := range items {
		for _, it := range sale.Items {
			if it.ProductID == req.ProductID {
				if it.RefundedQuantity.Add(req.Quantity).Cmp(it.Quantity) < 0 {
					allRefunded = false
				}
			} else {
				if it.RefundedQuantity.Cmp(it.Quantity) < 0 {
					allRefunded = false
				}
			}
		}
	}

	targetSaleStatus := StatusPartiallyRefunded
	if allRefunded {
		targetSaleStatus = StatusRefunded
	}

	// 3. Execute in DB within TransactionWithRetry if DB is available
	if rs.db != nil {
		err := rs.db.TransactionWithRetry(func(tx *data.RealTx) error {
			// A. Insert into refunds table
			refundType := "PARTIAL"
			if allRefunded {
				refundType = "FULL"
			}
			_, err := tx.Exec(
				`INSERT INTO refunds (id, sale_id, cashier_id, manager_id, type, reason, subtotal_minor, tax_minor, total_minor, status, created_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, 'COMPLETED', ?)`,
				refundID, sale.ID, cashierID, cashierID, refundType, reason, totalRefundMinor, totalRefundMinor, nowStr,
			)
			if err != nil {
				return fmt.Errorf("insert refund record: %w", err)
			}

			// B. Insert into refund_items & update physical stock in products
			for i, req := range items {
				refItemID := fmt.Sprintf("refi-%d-%d", seq, i)
				var saleItemID string
				var unitPriceMinor int64
				for _, si := range sale.Items {
					if si.ProductID == req.ProductID {
						saleItemID = si.ID
						unitPriceMinor = si.UnitPriceMinor
						break
					}
				}

				lineMoney := data.NewMoney(unitPriceMinor, "BDT").MulDecimal(req.Quantity)
				_, err = tx.Exec(
					`INSERT INTO refund_items (id, refund_id, sale_item_id, product_id, quantity_raw, unit_price_minor, subtotal_minor)
					 VALUES (?, ?, ?, ?, ?, ?, ?)`,
					refItemID, refundID, saleItemID, req.ProductID, req.Quantity.Value, unitPriceMinor, lineMoney.Minor,
				)
				if err != nil {
					return fmt.Errorf("insert refund_item: %w", err)
				}

				// Restore physical stock
				var currentStock int64
				row := tx.QueryRow(`SELECT stock_raw FROM products WHERE id = ?`, req.ProductID)
				if err := row.Scan(&currentStock); err != nil {
					return fmt.Errorf("query product stock: %w", err)
				}
				newStock := currentStock + req.Quantity.Value

				_, err = tx.Exec(
					`UPDATE products SET stock_raw = stock_raw + ?, version = version + 1, updated_at = ? WHERE id = ?`,
					req.Quantity.Value, nowStr, req.ProductID,
				)
				if err != nil {
					return fmt.Errorf("update product stock for return: %w", err)
				}

				// Insert stock movement
				movID := fmt.Sprintf("mov-ref-%d-%d", seq, i)
				_, err = tx.Exec(
					`INSERT INTO stock_movements (id, product_id, product_name, type, delta_raw, balance_raw, cost_minor, reference, timestamp, notes)
					 VALUES (?, ?, ?, 'RETURN', ?, ?, 0, ?, ?, ?)`,
					movID, req.ProductID, req.ProductID, req.Quantity.Value, newStock,
					sale.InvoiceNumber, nowStr, fmt.Sprintf("Refund return for %s (Reason: %s)", sale.InvoiceNumber, reason),
				)
				if err != nil {
					return fmt.Errorf("insert stock movement for return: %w", err)
				}
			}

			// C. Customer ledger credit due reduction if credit customer
			if sale.CustomerID != "" && rs.customers != nil {
				var dueBalance int64
				row := tx.QueryRow(`SELECT due_balance_minor FROM customers WHERE id = ?`, sale.CustomerID)
				if err := row.Scan(&dueBalance); err == nil && dueBalance > 0 {
					creditRefund := totalRefundMinor
					if creditRefund > dueBalance {
						creditRefund = dueBalance
					}
					if creditRefund > 0 {
						_, err = tx.Exec(
							`UPDATE customers SET due_balance_minor = due_balance_minor - ?, updated_at = ? WHERE id = ?`,
							creditRefund, nowStr, sale.CustomerID,
						)
						if err != nil {
							return fmt.Errorf("update customer due balance: %w", err)
						}

						newBalance := dueBalance - creditRefund
						cledID := fmt.Sprintf("cled-ref-%d", seq)
						_, err = tx.Exec(
							`INSERT INTO customer_ledger (id, customer_id, type, amount_minor, balance_after, reference, notes, created_by, timestamp)
							 VALUES (?, ?, 'REFUND', ?, ?, ?, ?, ?, ?)`,
							cledID, sale.CustomerID, creditRefund, newBalance, sale.InvoiceNumber,
							fmt.Sprintf("Refund credit adjustment on invoice %s", sale.InvoiceNumber), cashierID, nowStr,
						)
						if err != nil {
							return fmt.Errorf("insert customer ledger refund: %w", err)
						}
					}
				}
			}

			// D. Update sale status in DB
			_, err = tx.Exec(
				`UPDATE sales SET status = ?, updated_at = ? WHERE id = ?`,
				string(targetSaleStatus), nowStr, sale.ID,
			)
			if err != nil {
				return fmt.Errorf("update sale status: %w", err)
			}

			// E. Record audit log
			beforeJSON, _ := json.Marshal(map[string]interface{}{"status": sale.Status})
			afterJSON, _ := json.Marshal(map[string]interface{}{"status": targetSaleStatus, "refund_amount_minor": totalRefundMinor})
			auditID := fmt.Sprintf("audit-ref-%d", seq)
			_, err = tx.Exec(
				`INSERT INTO audit_log (id, action, entity_id, entity_type, actor, before_json, after_json, notes, timestamp)
				 VALUES (?, 'SALE_REFUNDED', ?, 'SALE', ?, ?, ?, ?, ?)`,
				auditID, sale.ID, cashierID, string(beforeJSON), string(afterJSON),
				fmt.Sprintf("Refund for invoice %s (Reason: %s)", sale.InvoiceNumber, reason), nowStr,
			)
			return err
		}, 5)
		if err != nil {
			return nil, fmt.Errorf("process refund transaction failed: %w", err)
		}
	}

	// 4. Update in-memory state post-commit
	for _, req := range items {
		for i := range sale.Items {
			if sale.Items[i].ProductID == req.ProductID {
				sale.Items[i].RefundedQuantity = sale.Items[i].RefundedQuantity.Add(req.Quantity)
				break
			}
		}

		_, _ = rs.inventory.RecordMovement(
			req.ProductID,
			MovementReturn,
			req.Quantity, // Positive delta restores stock
			sale.InvoiceNumber,
			fmt.Sprintf("Refund on %s (Reason: %s)", sale.InvoiceNumber, reason),
		)
	}

	sale.Status = targetSaleStatus
	sale.UpdatedAt = now

	// 5. Generate Refund Receipt
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
		DateStr:       now.Format("02/01/2006 03:04 PM"),
		Cashier:       cashierID,
		Items:         receiptLines,
		GrandTotal:    data.NewMoney(totalRefundMinor, "BDT").Format(),
		PaymentMethod: "Refund Payout (Cash)",
		PaidAmount:    data.NewMoney(totalRefundMinor, "BDT").Format(),
		FooterNote:    "Refund processed successfully.",
	}

	formatter := device.NewESCPOSFormatter(device.Width58mm)
	receiptText := formatter.FormatPlainText(receiptData)

	// 6. Record in-memory audit
	rs.audit.Record(
		ActionSaleRefunded,
		sale.ID,
		cashierID,
		map[string]interface{}{"status": StatusCompleted},
		map[string]interface{}{"status": targetSaleStatus, "refundMinor": totalRefundMinor},
		fmt.Sprintf("Refund for invoice %s (Reason: %s)", sale.InvoiceNumber, reason),
	)

	record := &RefundRecord{
		ID:            refundID,
		SaleID:        saleID,
		InvoiceNumber: sale.InvoiceNumber,
		CashierID:     cashierID,
		RefundAmount:  totalRefundMinor,
		Items:         items,
		Timestamp:     now,
		ReceiptText:   receiptText,
	}

	rs.refunds[refundID] = record
	return record, nil
}
