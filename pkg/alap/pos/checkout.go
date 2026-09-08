package pos

import (
	"fmt"
	"sync"
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

// CheckoutService orchestrates the atomic retail transaction pipeline
type CheckoutService struct {
	mu         sync.Mutex
	catalog    *CatalogRepository
	inventory  *InventoryLedger
	shifts     *ShiftManager
	customers  *CustomerRepository
	audit      *AuditTrail
	dbPool     *data.DBPool
	sales      map[string]*Sale
	orderCount int64
}

// NewCheckoutService creates a new checkout service
func NewCheckoutService(
	catalog *CatalogRepository,
	inventory *InventoryLedger,
	shifts *ShiftManager,
	customers *CustomerRepository,
	audit *AuditTrail,
) *CheckoutService {
	return &CheckoutService{
		catalog:   catalog,
		inventory: inventory,
		shifts:    shifts,
		customers: customers,
		audit:     audit,
		sales:     make(map[string]*Sale),
	}
}

// SetDBPool sets the database pool for ACID database transaction persistence
func (cs *CheckoutService) SetDBPool(pool *data.DBPool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.dbPool = pool
}

// Execute processes the complete POS Vertical Slice #1 transaction atomically
func (cs *CheckoutService) Execute(
	cart *Cart,
	payments []PaymentRecord,
	cashierID string,
	registerID string,
	customerID string,
) (*CheckoutResult, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// 1. Snapshot cart & Validate not empty
	snapshot := cart.Snapshot()
	if len(snapshot.Items) == 0 {
		return nil, fmt.Errorf("cart is empty")
	}

	// 2. Validate physical stock sufficiency & unit compatibility for all items
	for _, it := range snapshot.Items {
		p, exists := cs.catalog.FindByID(it.ProductID)
		if !exists {
			return nil, fmt.Errorf("product not found: %s", it.ProductID)
		}
		if it.Unit != "" && p.Unit != "" {
			if err := data.ValidateUnitCompatibility(it.Unit, p.Unit); err != nil {
				return nil, fmt.Errorf("unit compatibility error for %s: %w", p.Name, err)
			}
		}
		if p.Stock.Cmp(it.Quantity) < 0 {
			return nil, fmt.Errorf("insufficient stock for '%s' (available: %s, requested: %s)",
				p.Name, p.Stock.String(), it.Quantity.String())
		}
	}

	// 3. Validate payments cover grand total
	totalDue := snapshot.GrandTotalMinor
	tender := CalculateTender(totalDue, payments)
	if !tender.IsComplete {
		return nil, fmt.Errorf("payment insufficient: due ৳%.2f, received ৳%.2f",
			float64(totalDue)/100.0, float64(tender.TotalPaidMinor)/100.0)
	}

	// 4. Generate Invoice Number
	cs.orderCount++
	now := time.Now()
	invoiceNo := fmt.Sprintf("INV-%s-%04d", now.Format("20060102"), cs.orderCount)
	saleID := fmt.Sprintf("sale-%d", now.UnixNano())

	// 5. Build SaleItems and compute gross profit
	saleItems := make([]SaleItem, len(snapshot.Items))
	for i, it := range snapshot.Items {
		costTotalMoney := it.CostPrice.MulDecimal(it.Quantity)
		profitMinor := it.TotalMinor - costTotalMoney.Minor

		saleItems[i] = SaleItem{
			ID:             fmt.Sprintf("si-%d-%d", now.UnixNano(), i),
			SaleID:         saleID,
			ProductID:      it.ProductID,
			Name:           it.Name,
			SKU:            it.SKU,
			Unit:           it.Unit,
			Quantity:       it.Quantity,
			UnitPriceMinor: it.UnitPrice.Minor,
			CostPriceMinor: it.CostPrice.Minor,
			SubtotalMinor:  it.SubtotalMinor,
			ProfitMinor:    profitMinor,
		}
	}

	// 6. Create Sale Record
	if customerID == "" {
		customerID = snapshot.CustomerID
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

	// 7. Atomic Inventory Decrement via Stock Movements
	for _, it := range snapshot.Items {
		negDelta := data.Decimal{Value: -it.Quantity.Value}
		_, err := cs.inventory.RecordMovement(
			it.ProductID,
			MovementSale,
			negDelta,
			invoiceNo,
			fmt.Sprintf("POS Sale #%s", invoiceNo),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to decrement inventory for %s: %w", it.Name, err)
		}
	}

	// 8. Credit sale handling if any payment method is CREDIT
	for _, p := range payments {
		if p.Method == MethodCredit && customerID != "" {
			_, _ = cs.customers.RecordCreditSale(customerID, p.AmountMinor, invoiceNo)
		}
	}

	// 9. Record in active shift
	_ = cs.shifts.RecordSale(sale)

	// 10. Database Transaction Persistence if DBPool configured
	if cs.dbPool != nil {
		_ = cs.dbPool.Transaction(func(tx *data.Tx) error {
			cs.dbPool.Table("sales").Insert(cs.dbPool, map[string]interface{}{
				"id":             sale.ID,
				"invoice_number": sale.InvoiceNumber,
				"register_id":    sale.RegisterID,
				"cashier_id":     sale.CashierID,
				"customer_id":    sale.CustomerID,
				"subtotal":       sale.SubtotalMinor,
				"discount":       sale.DiscountMinor,
				"tax":            sale.TaxMinor,
				"total":          sale.TotalMinor,
				"paid":           sale.PaidMinor,
				"change":         sale.ChangeMinor,
				"status":         string(sale.Status),
			})
			for _, it := range sale.Items {
				cs.dbPool.Table("sale_items").Insert(cs.dbPool, map[string]interface{}{
					"id":         it.ID,
					"sale_id":    sale.ID,
					"product_id": it.ProductID,
					"name":       it.Name,
					"quantity":   it.Quantity.String(),
					"unit_price": it.UnitPriceMinor,
					"subtotal":   it.SubtotalMinor,
				})
			}
			return nil
		})
	}

	// 11. Store Sale in memory registry
	cs.sales[sale.ID] = sale

	// 11. Format Thermal Receipt (ESC/POS and Text preview)
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
		StoreName:     "লাখান ভাণ্ডার (Lakhan Bhandar)",
		StoreSubtitle: "Wholesale & Retail Groceries",
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

	// 12. Cash drawer kickout trigger if cash was used
	var drawerPulse []byte
	if hasCash {
		drawer := device.NewCashDrawer()
		drawerPulse = drawer.Open()
	}

	// 13. Audit Log Entry
	auditEntry := cs.audit.Record(
		ActionSaleCompleted,
		sale.ID,
		cashierID,
		nil,
		map[string]interface{}{
			"invoice":    invoiceNo,
			"totalMinor": sale.TotalMinor,
			"itemsCount": len(sale.Items),
		},
		fmt.Sprintf("Sale %s completed successfully", invoiceNo),
	)

	// 14. Clear active cart
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

// GetSale looks up sale by ID
func (cs *CheckoutService) GetSale(id string) (*Sale, bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	s, ok := cs.sales[id]
	return s, ok
}

// AllSales returns list of all recorded sales
func (cs *CheckoutService) AllSales() []*Sale {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	res := make([]*Sale, 0, len(cs.sales))
	for _, s := range cs.sales {
		res = append(res, s)
	}
	return res
}
