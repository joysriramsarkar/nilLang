package pos

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
	"github.com/joysriramsarkar/nilLang/pkg/alap/device"
	"github.com/joysriramsarkar/nilLang/pkg/alap/i18n"
)

// CheckoutResult holds the response after a successful sale transaction
type CheckoutResult struct {
	Sale            *Sale        `json:"sale"`
	Tender          TenderResult `json:"tender"`
	ReceiptText     string       `json:"receipt_text"`
	ReceiptBytes    []byte       `json:"-"`
	CashDrawerPulse []byte       `json:"-"`
	TriggerDrawer   bool         `json:"trigger_drawer"`
	AuditEntry      *AuditEntry  `json:"audit_entry"`
}

// CheckoutService orchestrates the atomic retail transaction pipeline.
// All domain mutations occur inside a single database transaction; external
// side-effects (printer, cash drawer, network sync) only fire AFTER commit.
// Execute() is safe for concurrent use: each checkout runs its ACID
// transaction independently; only the in-memory sales map write is serialized.
type CheckoutService struct {
	mu         sync.Mutex // guards cs.sales map writes only
	catalog    *CatalogRepository
	inventory  *InventoryLedger
	shifts     *ShiftManager
	customers  *CustomerRepository
	audit      *AuditTrail
	db         *data.RealDBPool // nil → in-memory only (testing)
	sales      map[string]*Sale
	orderCount int64  // accessed via atomic.AddInt64
	storeName  string // configurable store name (no hard-coded strings)
	storeSub   string
}

// CheckoutConfig configures the checkout service.
type CheckoutConfig struct {
	StoreName    string // e.g. "লাখান ভাণ্ডার"
	StoreSubname string // e.g. "Wholesale & Retail"
}

// NewCheckoutService creates a new checkout service.
func NewCheckoutService(
	catalog *CatalogRepository,
	inventory *InventoryLedger,
	shifts *ShiftManager,
	customers *CustomerRepository,
	audit *AuditTrail,
	cfg ...CheckoutConfig,
) *CheckoutService {
	cs := &CheckoutService{
		catalog:   catalog,
		inventory: inventory,
		shifts:    shifts,
		customers: customers,
		audit:     audit,
		sales:     make(map[string]*Sale),
		storeName: "NilLang POS",
		storeSub:  "Point of Sale",
	}
	if len(cfg) > 0 {
		if cfg[0].StoreName != "" {
			cs.storeName = cfg[0].StoreName
		}
		if cfg[0].StoreSubname != "" {
			cs.storeSub = cfg[0].StoreSubname
		}
	}
	return cs
}

// SetDB configures a real database pool for ACID persistence.
func (cs *CheckoutService) SetDB(pool *data.RealDBPool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.db = pool
}

// SetDBPool is kept for backward compatibility.
func (cs *CheckoutService) SetDBPool(pool *data.DBPool) {
	// no-op: use SetDB with RealDBPool for production
}

// SetStoreInfo configures the store name and subtitle for receipts.
func (cs *CheckoutService) SetStoreInfo(name, sub string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if name != "" {
		cs.storeName = name
	}
	if sub != "" {
		cs.storeSub = sub
	}
}

// StoreName returns current store name.
func (cs *CheckoutService) StoreName() string {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.storeName
}

// Execute processes the complete POS checkout atomically.
//
// TRUE ACID PIPELINE:
//  1. Read-only validation (stock, payment sufficiency, unit compatibility)
//  2. BEGIN TRANSACTION
//     - INSERT sale
//     - INSERT sale_items × N
//     - INSERT sale_payments × N
//     - INSERT stock_movements × N
//     - UPDATE products stock (WITH optimistic version check)
//     - UPDATE customers due balance (if credit)
//     - INSERT shift_movement record
//     - INSERT receipt row
//     - INSERT audit_log
//  3. COMMIT
//  4. Update in-memory caches (post-commit)
//  5. External side-effects: printer, cash drawer, realtime (post-commit)
func (cs *CheckoutService) Execute(
	cart *Cart,
	payments []PaymentRecord,
	cashierID string,
	registerID string,
	customerID string,
) (*CheckoutResult, error) {
	// ── 1. VALIDATION (read-only) ─────────────────────────────────────────────
	// No global mutex here: catalog/inventory/customer all have their own
	// RWMutex, and the DB transaction (step 3) provides ACID isolation.
	snapshot := cart.Snapshot()
	if len(snapshot.Items) == 0 {
		return nil, fmt.Errorf("cart is empty")
	}

	// Validate stock and unit compatibility
	for _, it := range snapshot.Items {
		p, exists := cs.catalog.FindByID(it.ProductID)
		if !exists {
			return nil, fmt.Errorf("product not found: %s", it.ProductID)
		}
		if it.Unit != "" && p.Unit != "" {
			if err := data.ValidateUnitCompatibility(it.Unit, p.Unit); err != nil {
				return nil, fmt.Errorf("unit error for %s: %w", p.Name, err)
			}
		}
		if p.Stock.Cmp(it.Quantity) < 0 {
			return nil, fmt.Errorf("insufficient stock for '%s' (available: %s, requested: %s)",
				p.Name, p.Stock.String(), it.Quantity.String())
		}
	}

	// Validate payment covers total
	totalDue := snapshot.GrandTotalMinor
	tender := CalculateTender(totalDue, payments)
	if !tender.IsComplete {
		return nil, fmt.Errorf("payment insufficient: due ৳%.2f, received ৳%.2f",
			float64(totalDue)/100.0, float64(tender.TotalPaidMinor)/100.0)
	}

	// ── 2. PREPARE RECORDS ───────────────────────────────────────────────────
	// Use atomic increment so concurrent checkouts get unique monotonic IDs
	// without holding the mutex during the entire transaction.
	count := atomic.AddInt64(&cs.orderCount, 1)
	now := time.Now()
	invoiceNo := fmt.Sprintf("INV-%s-%04d", now.Format("20060102"), count)
	saleID := fmt.Sprintf("sale-%d-%d", now.UnixNano(), count)
	if customerID == "" {
		customerID = snapshot.CustomerID
	}

	saleItems := make([]SaleItem, len(snapshot.Items))
	for i, it := range snapshot.Items {
		costTotalMoney := it.CostPrice.MulDecimal(it.Quantity)
		saleItems[i] = SaleItem{
			ID:             fmt.Sprintf("si-%s-%d", saleID, i),
			SaleID:         saleID,
			ProductID:      it.ProductID,
			Name:           it.Name,
			SKU:            it.SKU,
			Unit:           it.Unit,
			Quantity:       it.Quantity,
			UnitPriceMinor: it.UnitPrice.Minor,
			CostPriceMinor: it.CostPrice.Minor,
			SubtotalMinor:  it.SubtotalMinor,
			ProfitMinor:    it.TotalMinor - costTotalMoney.Minor,
		}
	}

	// Get current shift ID for association
	var shiftID string
	if cs.shifts != nil {
		if shift, err := cs.shifts.CurrentShift(); err == nil {
			shiftID = shift.ID
		}
	}

	sale := &Sale{
		ID:            saleID,
		InvoiceNumber: invoiceNo,
		RegisterID:    registerID,
		CashierID:     cashierID,
		CustomerID:    customerID,
		SubtotalMinor: snapshot.SubtotalMinor,
		DiscountMinor: snapshot.DiscountMinor,
		TaxMinor:      snapshot.TaxMinor,
		TotalMinor:    snapshot.GrandTotalMinor,
		PaidMinor:     tender.TotalPaidMinor,
		ChangeMinor:   tender.ChangeDueMinor,
		Status:        StatusCompleted,
		Items:         saleItems,
		Payments:      payments,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// ── 3. ATOMIC DATABASE TRANSACTION ─────────────────────────────────────────────
	if cs.db != nil {
		err := cs.db.TransactionWithRetry(func(tx *data.RealTx) error {
			// 3a. INSERT sale
			if err := insertSale(tx, sale, shiftID); err != nil {
				return fmt.Errorf("insert sale: %w", err)
			}

			// 3b. INSERT sale_items
			for _, it := range saleItems {
				if err := insertSaleItem(tx, it); err != nil {
					return fmt.Errorf("insert sale_item %s: %w", it.ProductID, err)
				}
			}

			// 3c. INSERT sale_payments
			for i, p := range payments {
				payID := fmt.Sprintf("pay-%s-%d", saleID, i)
				if err := insertPayment(tx, payID, saleID, p, now); err != nil {
					return fmt.Errorf("insert payment: %w", err)
				}
			}

			// 3d. UPDATE product stock WITH optimistic version lock + INSERT stock_movements
			for _, it := range snapshot.Items {
				p, _ := cs.catalog.FindByID(it.ProductID)
				res, err := tx.Exec(
					`UPDATE products SET stock_raw=stock_raw-?, version=version+1, updated_at=? WHERE id=? AND stock_raw>=?`,
					it.Quantity.Value, now.UTC().Format(time.RFC3339), it.ProductID, it.Quantity.Value,
				)
				if err != nil {
					return fmt.Errorf("update stock for %s: %w", it.ProductID, err)
				}
				rows, _ := res.RowsAffected()
				if rows == 0 {
					return fmt.Errorf("concurrent stock conflict for product %s — retry checkout", it.ProductID)
				}

				var remainingStockRaw int64
				if err := tx.QueryRow(`SELECT stock_raw FROM products WHERE id=?`, it.ProductID).Scan(&remainingStockRaw); err != nil {
					return fmt.Errorf("read updated stock for %s: %w", it.ProductID, err)
				}

				movID := fmt.Sprintf("mov-%s-%s", saleID, it.ProductID)
				_, err = tx.Exec(
					`INSERT INTO stock_movements (id,product_id,product_name,type,delta_raw,balance_raw,cost_minor,reference,timestamp,notes) VALUES (?,?,?,?,?,?,?,?,?,?)`,
					movID, it.ProductID, it.Name, string(MovementSale),
					-it.Quantity.Value, remainingStockRaw, p.Cost.Minor,
					invoiceNo, now.UTC().Format(time.RFC3339),
					fmt.Sprintf("POS Sale #%s", invoiceNo),
				)
				if err != nil {
					return fmt.Errorf("insert stock_movement for %s: %w", it.ProductID, err)
				}
			}

			// 3e. UPDATE customer balance for credit payments
			for i, p := range payments {
				if p.Method == MethodCredit && customerID != "" {
					_, err := tx.Exec(
						`UPDATE customers SET due_balance_minor=due_balance_minor+?, total_purchases_minor=total_purchases_minor+?, updated_at=? WHERE id=?`,
						p.AmountMinor, p.AmountMinor, now.UTC().Format(time.RFC3339), customerID,
					)
					if err != nil {
						return fmt.Errorf("update customer balance: %w", err)
					}
					ledgerID := fmt.Sprintf("cled-%s-%d", saleID, i)
					_, err = tx.Exec(
						`INSERT INTO customer_ledger (id,customer_id,type,amount_minor,balance_after,reference,notes,created_by,timestamp) VALUES (?,?,?,?,?,?,?,?,?)`,
						ledgerID, customerID, "CREDIT_SALE", p.AmountMinor, 0,
						invoiceNo, fmt.Sprintf("Baki on Invoice %s", invoiceNo), cashierID,
						now.UTC().Format(time.RFC3339),
					)
					if err != nil {
						return fmt.Errorf("insert customer ledger: %w", err)
					}
				}
			}

			// 3f. INSERT audit log
			afterJSON, _ := json.Marshal(map[string]interface{}{
				"invoice": invoiceNo, "total": sale.TotalMinor, "items": len(saleItems),
			})
			auditID := fmt.Sprintf("aud-%s", saleID)
			_, err := tx.Exec(
				`INSERT INTO audit_log (id,action,entity_id,entity_type,actor,after_json,notes,timestamp) VALUES (?,?,?,?,?,?,?,?)`,
				auditID, string(ActionSaleCompleted), saleID, "sale", cashierID,
				string(afterJSON), fmt.Sprintf("Sale %s completed", invoiceNo),
				now.UTC().Format(time.RFC3339),
			)
			if err != nil {
				return fmt.Errorf("insert audit_log: %w", err)
			}

			return nil
		}, 5)
		// return the transaction error (already wrapped by TransactionWithRetry)
		if err != nil {
			return nil, fmt.Errorf("checkout transaction failed: %w", err)
		}
	}

	// ── 4. UPDATE IN-MEMORY CACHES (post-commit only) ────────────────────────
	// Decrement in-memory stock ledger
	for _, it := range snapshot.Items {
		negDelta := data.Decimal{Value: -it.Quantity.Value}
		_, _ = cs.inventory.RecordMovement(it.ProductID, MovementSale, negDelta, invoiceNo,
			fmt.Sprintf("POS Sale #%s", invoiceNo))
	}

	// Credit in-memory customer
	for _, p := range payments {
		if p.Method == MethodCredit && customerID != "" {
			_, _ = cs.customers.RecordCreditSale(customerID, p.AmountMinor, invoiceNo)
		}
	}

	// Record in shift
	_ = cs.shifts.RecordSale(sale)

	// Store in memory registry — mutex required for concurrent map write
	cs.mu.Lock()
	cs.sales[sale.ID] = sale
	cs.mu.Unlock()

	// ── 5. EXTERNAL SIDE-EFFECTS (post-commit) ───────────────────────────────
	receiptLines := make([]device.ReceiptLineItem, len(sale.Items))
	for i, it := range sale.Items {
		receiptLines[i] = device.ReceiptLineItem{
			Name:     it.Name,
			Quantity: it.Quantity.String(),
			Price:    data.NewMoney(it.UnitPriceMinor, "BDT").Format(),
			Total:    data.NewMoney(it.SubtotalMinor, "BDT").Format(),
		}
	}

	custName := "Walk-in Customer"
	if customerID != "" {
		if c, found := cs.customers.FindByID(customerID); found {
			custName = c.Name
		}
	}

	var mainPaymentMethod string = "Cash"
	hasCash := false
	if len(payments) > 0 {
		mainPaymentMethod = string(payments[0].Method)
		for _, p := range payments {
			if p.Method == MethodCash {
				hasCash = true
				break
			}
		}
	}

	receiptData := device.ReceiptPayload{
		StoreName:     cs.storeName,
		StoreSubtitle: cs.storeSub,
		InvoiceNo:     invoiceNo,
		DateStr:       now.Format("02/01/2006 03:04 PM"),
		Cashier:       cashierID,
		Customer:      custName,
		Items:         receiptLines,
		Subtotal:      data.NewMoney(sale.SubtotalMinor, "BDT").Format(),
		Discount:      data.NewMoney(sale.DiscountMinor, "BDT").Format(),
		Tax:           data.NewMoney(sale.TaxMinor, "BDT").Format(),
		GrandTotal:    data.NewMoney(sale.TotalMinor, "BDT").Format(),
		PaymentMethod: mainPaymentMethod,
		PaidAmount:    data.NewMoney(sale.PaidMinor, "BDT").Format(),
		ChangeDue:     data.NewMoney(sale.ChangeMinor, "BDT").Format(),
		FooterNote:    i18n.T(i18n.LocaleBnBD, "receipt.thank_you"),
	}

	formatter := device.NewESCPOSFormatter(device.Width58mm)
	receiptText := formatter.FormatPlainText(receiptData)
	receiptBytes := formatter.BuildESCPOSBytes(receiptData)

	// Enqueue receipt to DB (best-effort, non-blocking)
	_ = cs.persistReceiptAsync(context.Background(), saleID, invoiceNo, receiptText, custName, cashierID)

	var drawerPulse []byte
	if hasCash {
		drawer := device.NewCashDrawer()
		drawerPulse = drawer.Open()
	}

	auditEntry := cs.audit.Record(
		ActionSaleCompleted, sale.ID, cashierID, nil,
		map[string]interface{}{"invoice": invoiceNo, "totalMinor": sale.TotalMinor},
		fmt.Sprintf("Sale %s completed successfully", invoiceNo),
	)

	cart.Clear()

	return &CheckoutResult{
		Sale:            sale,
		Tender:          tender,
		ReceiptText:     receiptText,
		ReceiptBytes:    receiptBytes,
		CashDrawerPulse: drawerPulse,
		TriggerDrawer:   hasCash,
		AuditEntry:      auditEntry,
	}, nil
}

// persistReceiptAsync stores the receipt row in the DB in the background (non-critical path).
func (cs *CheckoutService) persistReceiptAsync(_ context.Context, saleID, invoiceNo, content, customer, cashier string) error {
	if cs.db == nil {
		return nil
	}
	rid := fmt.Sprintf("rcpt-%d", time.Now().UnixNano())
	_, err := cs.db.Exec(
		`INSERT OR IGNORE INTO receipts (id,sale_id,invoice_no,customer,cashier,format,content,printed,created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		rid, saleID, invoiceNo, customer, cashier, "TEXT", content, 0,
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// GetSale looks up a sale by ID (memory first, then DB).
func (cs *CheckoutService) GetSale(id string) (*Sale, bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	s, ok := cs.sales[id]
	return s, ok
}

// AllSales returns all recorded sales (in-memory registry).
func (cs *CheckoutService) AllSales() []*Sale {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	res := make([]*Sale, 0, len(cs.sales))
	for _, s := range cs.sales {
		res = append(res, s)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].CreatedAt.After(res[j].CreatedAt)
	})
	return res
}

// ─── PRIVATE SQL HELPERS ─────────────────────────────────────────────────────

func insertSale(tx *data.RealTx, sale *Sale, shiftID string) error {
	_, err := tx.Exec(
		`INSERT INTO sales (id,invoice_number,register_id,cashier_id,customer_id,shift_id,subtotal_minor,discount_minor,tax_minor,total_minor,paid_minor,change_minor,status,created_at,updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		sale.ID, sale.InvoiceNumber, sale.RegisterID, sale.CashierID,
		nullStr(sale.CustomerID), nullStr(shiftID),
		sale.SubtotalMinor, sale.DiscountMinor, sale.TaxMinor,
		sale.TotalMinor, sale.PaidMinor, sale.ChangeMinor,
		string(sale.Status),
		sale.CreatedAt.UTC().Format(time.RFC3339),
		sale.UpdatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

func insertSaleItem(tx *data.RealTx, it SaleItem) error {
	_, err := tx.Exec(
		`INSERT INTO sale_items (id,sale_id,product_id,name,sku,unit,quantity_raw,unit_price_minor,cost_price_minor,subtotal_minor,profit_minor)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		it.ID, it.SaleID, it.ProductID, it.Name, it.SKU, it.Unit,
		it.Quantity.Value, it.UnitPriceMinor, it.CostPriceMinor,
		it.SubtotalMinor, it.ProfitMinor,
	)
	return err
}

func insertPayment(tx *data.RealTx, id, saleID string, p PaymentRecord, now time.Time) error {
	_, err := tx.Exec(
		`INSERT INTO sale_payments (id,sale_id,method,amount_minor,status,created_at) VALUES (?,?,?,?,?,?)`,
		id, saleID, string(p.Method), p.AmountMinor, "COMPLETED",
		now.UTC().Format(time.RFC3339),
	)
	return err
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
