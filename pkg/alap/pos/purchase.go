package pos

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

var supplierLedgerSeq int64

// SupplierLedgerEntry records a credit/debit transaction in a supplier's account.
type SupplierLedgerEntry struct {
	ID           string    `json:"id"`
	SupplierID   string    `json:"supplier_id"`
	Type         string    `json:"type"` // "PURCHASE", "PAYMENT", "DEBIT_NOTE", "ADJUSTMENT"
	AmountMinor  int64     `json:"amount_minor"`
	BalanceAfter int64     `json:"balance_after"`
	Reference    string    `json:"reference"`
	Notes        string    `json:"notes,omitempty"`
	CreatedBy    string    `json:"created_by,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// PurchaseReturnItem specifies product and quantity returned to supplier.
type PurchaseReturnItem struct {
	ProductID string       `json:"product_id"`
	Quantity  data.Decimal `json:"quantity"`
	CostPrice data.Money   `json:"cost_price"`
	Reason    string       `json:"reason"`
}

// PurchaseReturnResult details the executed return to vendor.
type PurchaseReturnResult struct {
	ID           string               `json:"id"`
	PurchaseID   string               `json:"purchase_id"`
	SupplierID   string               `json:"supplier_id"`
	TotalMinor   int64                `json:"total_minor"`
	Items        []PurchaseReturnItem `json:"items"`
	Timestamp    time.Time            `json:"timestamp"`
	BalanceAfter int64                `json:"balance_after"`
}

// PurchaseStatus tracks the lifecycle of a purchase order.
type PurchaseStatus string

const (
	PurchasePending   PurchaseStatus = "PENDING"
	PurchasePartial   PurchaseStatus = "PARTIAL"
	PurchaseReceived  PurchaseStatus = "RECEIVED"
	PurchaseCancelled PurchaseStatus = "CANCELLED"
)

// Supplier represents an inventory vendor.
type Supplier struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Phone        string `json:"phone"`
	Email        string `json:"email,omitempty"`
	Address      string `json:"address,omitempty"`
	PayableMinor int64  `json:"payable_minor"`
	Active       bool   `json:"active"`
}

// PurchaseItem represents an item within a purchase order.
type PurchaseItem struct {
	ID            string       `json:"id"`
	PurchaseID    string       `json:"purchase_id"`
	ProductID     string       `json:"product_id"`
	ProductName   string       `json:"product_name,omitempty"`
	Quantity      data.Decimal `json:"quantity"`
	ReceivedQty   data.Decimal `json:"received_qty"`
	CostPrice     data.Money   `json:"cost_price"`
	SubtotalMinor int64        `json:"subtotal_minor"`
}

// Purchase represents a purchase order or inventory restock record.
type Purchase struct {
	ID            string         `json:"id"`
	PONumber      string         `json:"po_number"`
	SupplierID    string         `json:"supplier_id"`
	SupplierName  string         `json:"supplier_name,omitempty"`
	Status        PurchaseStatus `json:"status"`
	SubtotalMinor int64          `json:"subtotal_minor"`
	TaxMinor      int64          `json:"tax_minor"`
	TotalMinor    int64          `json:"total_minor"`
	Items         []PurchaseItem `json:"items"`
	ReceivedAt    *time.Time     `json:"received_at,omitempty"`
	Notes         string         `json:"notes,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// PurchaseService manages inventory procurement, goods receipts, and WAC recalculation.
type PurchaseService struct {
	mu        sync.Mutex
	catalog   *CatalogRepository
	inventory *InventoryLedger
	audit     *AuditTrail
	db        *data.RealDBPool
	purchases map[string]*Purchase
	suppliers map[string]*Supplier
	orderSeq  int64
}

// NewPurchaseService creates a PurchaseService.
func NewPurchaseService(
	catalog *CatalogRepository,
	inventory *InventoryLedger,
	audit *AuditTrail,
) *PurchaseService {
	return &PurchaseService{
		catalog:   catalog,
		inventory: inventory,
		audit:     audit,
		purchases: make(map[string]*Purchase),
		suppliers: make(map[string]*Supplier),
	}
}

// SetDB configures the real database pool for purchase persistence.
func (ps *PurchaseService) SetDB(db *data.RealDBPool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.db = db
}

// AddSupplier registers a vendor.
func (ps *PurchaseService) AddSupplier(s *Supplier) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.suppliers[s.ID] = s

	if ps.db != nil {
		nowStr := time.Now().UTC().Format(time.RFC3339)
		_, _ = ps.db.Exec(
			`INSERT INTO suppliers (id, name, phone, email, address, payable_minor, active, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?)`,
			s.ID, s.Name, s.Phone, s.Email, s.Address, s.PayableMinor, nowStr, nowStr,
		)
	}
}

// FindSupplier returns a supplier by ID.
func (ps *PurchaseService) FindSupplier(id string) (*Supplier, bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	s, ok := ps.suppliers[id]
	return s, ok
}

// CreatePurchase creates and registers a new Purchase Order.
func (ps *PurchaseService) CreatePurchase(p *Purchase) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if p.ID == "" {
		p.ID = fmt.Sprintf("po-%d", time.Now().UnixNano())
	}
	if p.PONumber == "" {
		ps.orderSeq++
		p.PONumber = fmt.Sprintf("PO-%s-%04d", time.Now().Format("20060102"), ps.orderSeq)
	}
	if p.Status == "" {
		p.Status = PurchasePending
	}
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now

	// Calculate totals
	var subtotal int64
	for i := range p.Items {
		item := &p.Items[i]
		if item.ID == "" {
			item.ID = fmt.Sprintf("poi-%d-%d", now.UnixNano(), i)
		}
		item.PurchaseID = p.ID
		costMoney := item.CostPrice.MulDecimal(item.Quantity)
		item.SubtotalMinor = costMoney.Minor
		subtotal += item.SubtotalMinor
	}
	p.SubtotalMinor = subtotal
	p.TotalMinor = subtotal + p.TaxMinor

	ps.purchases[p.ID] = p

	// DB write
	if ps.db != nil {
		nowStr := now.UTC().Format(time.RFC3339)
		err := ps.db.Transaction(func(tx *data.RealTx) error {
			_, err := tx.Exec(
				`INSERT INTO purchases (id, po_number, supplier_id, status, subtotal_minor, tax_minor, total_minor, notes, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				p.ID, p.PONumber, p.SupplierID, string(p.Status), p.SubtotalMinor, p.TaxMinor, p.TotalMinor, p.Notes, nowStr, nowStr,
			)
			if err != nil {
				return err
			}
			for _, item := range p.Items {
				_, err = tx.Exec(
					`INSERT INTO purchase_items (id, purchase_id, product_id, quantity_raw, received_raw, cost_minor, subtotal_minor)
					 VALUES (?, ?, ?, ?, ?, ?, ?)`,
					item.ID, p.ID, item.ProductID, item.Quantity.Value, item.ReceivedQty.Value, item.CostPrice.Minor, item.SubtotalMinor,
				)
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("persist purchase: %w", err)
		}
	}

	return nil
}

// ReceiveGoods records the physical arrival of ordered items, updates stock, and computes WAC.
func (ps *PurchaseService) ReceiveGoods(
	purchaseID string,
	receivedQuantities map[string]data.Decimal, // productID -> qty received
	notes string,
) (*Purchase, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	p, ok := ps.purchases[purchaseID]
	if !ok {
		return nil, fmt.Errorf("purchase order not found: %s", purchaseID)
	}
	if p.Status == PurchaseReceived || p.Status == PurchaseCancelled {
		return nil, fmt.Errorf("purchase order is already %s", p.Status)
	}

	now := time.Now()
	nowStr := now.UTC().Format(time.RFC3339)

	// Precalculate exact quantities to receive per item
	toReceive := make(map[string]data.Decimal)
	for _, it := range p.Items {
		recQty, specified := receivedQuantities[it.ProductID]
		if !specified {
			recQty = it.Quantity.Sub(it.ReceivedQty)
		}
		if recQty.Value > 0 {
			newReceived := it.ReceivedQty.Add(recQty)
			if newReceived.Cmp(it.Quantity) > 0 {
				return nil, fmt.Errorf("received quantity %s exceeds ordered quantity %s for %s",
					newReceived.String(), it.Quantity.String(), it.ProductID)
			}
			toReceive[it.ProductID] = recQty
		}
	}

	allFullyReceived := true
	var receivedBatchMinor int64
	for _, it := range p.Items {
		recQty, ok := toReceive[it.ProductID]
		if ok && recQty.Value > 0 {
			receivedBatchMinor += it.CostPrice.MulDecimal(recQty).Minor
		}
	}

	// Step 1: Execute in DB if pool is available
	if ps.db != nil {
		err := ps.db.Transaction(func(tx *data.RealTx) error {
			for i := range p.Items {
				it := &p.Items[i]
				recQty, ok := toReceive[it.ProductID]
				if !ok || recQty.Value <= 0 {
					continue
				}

				newReceived := it.ReceivedQty.Add(recQty)
				if newReceived.Cmp(it.Quantity) < 0 {
					allFullyReceived = false
				}

				// Update DB purchase_items
				_, err := tx.Exec(
					`UPDATE purchase_items SET received_raw=? WHERE id=?`,
					newReceived.Value, it.ID,
				)
				if err != nil {
					return err
				}

				// Update DB product stock and WAC cost
				prod, prodFound := ps.catalog.FindByID(it.ProductID)
				if !prodFound {
					return fmt.Errorf("product %s not found in catalog", it.ProductID)
				}

				newWAC := calculateWAC(prod.Stock, prod.Cost, recQty, it.CostPrice)
				newStockRaw := prod.Stock.Value + recQty.Value

				_, err = tx.Exec(
					`UPDATE products SET stock_raw=?, cost_minor=?, version=version+1, updated_at=? WHERE id=?`,
					newStockRaw, newWAC.Minor, nowStr, it.ProductID,
				)
				if err != nil {
					return err
				}

				// Record DB stock movement
				movID := fmt.Sprintf("mov-po-%s-%d", it.ProductID, now.UnixNano())
				_, err = tx.Exec(
					`INSERT INTO stock_movements (id, product_id, product_name, type, delta_raw, balance_raw, cost_minor, reference, timestamp, notes)
					 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					movID, it.ProductID, prod.Name, string(MovementPurchase),
					recQty.Value, newStockRaw, it.CostPrice.Minor,
					p.PONumber, nowStr, fmt.Sprintf("Goods received from %s", p.PONumber),
				)
				if err != nil {
					return err
				}
			}

			// Update supplier payable and insert supplier_ledger entry
			if p.SupplierID != "" && receivedBatchMinor > 0 {
				var curPayable int64
				_ = tx.QueryRow(`SELECT payable_minor FROM suppliers WHERE id = ?`, p.SupplierID).Scan(&curPayable)
				newPayable := curPayable + receivedBatchMinor
				_, err := tx.Exec(`UPDATE suppliers SET payable_minor=?, updated_at=? WHERE id=?`, newPayable, nowStr, p.SupplierID)
				if err != nil {
					return err
				}

				sledID := fmt.Sprintf("sled-po-%d-%d", now.UnixNano(), atomic.AddInt64(&supplierLedgerSeq, 1))
				_, err = tx.Exec(
					`INSERT INTO supplier_ledger (id, supplier_id, type, amount_minor, balance_after, reference, notes, created_by, timestamp)
					 VALUES (?, ?, 'PURCHASE', ?, ?, ?, ?, 'system', ?)`,
					sledID, p.SupplierID, receivedBatchMinor, newPayable, p.PONumber,
					fmt.Sprintf("Goods received on %s", p.PONumber), nowStr,
				)
				if err != nil {
					return err
				}
			}

			// Update purchase status
			newStatus := PurchaseReceived
			if !allFullyReceived {
				newStatus = PurchasePartial
			}
			_, err := tx.Exec(
				`UPDATE purchases SET status=?, received_at=?, updated_at=? WHERE id=?`,
				string(newStatus), nowStr, nowStr, p.ID,
			)
			return err
		})
		if err != nil {
			return nil, fmt.Errorf("receive goods tx failed: %w", err)
		}
	}

	// Step 2: Update in-memory state post-commit
	p.ReceivedAt = &now
	p.UpdatedAt = now
	for i := range p.Items {
		it := &p.Items[i]
		recQty, ok := toReceive[it.ProductID]
		if !ok || recQty.Value <= 0 {
			continue
		}

		it.ReceivedQty = it.ReceivedQty.Add(recQty)
		if it.ReceivedQty.Cmp(it.Quantity) < 0 {
			allFullyReceived = false
		}

		// Update product in memory
		if prod, found := ps.catalog.FindByID(it.ProductID); found {
			newWAC := calculateWAC(prod.Stock, prod.Cost, recQty, it.CostPrice)
			prod.Cost = newWAC
			if ps.inventory != nil {
				_, _ = ps.inventory.RecordMovement(it.ProductID, MovementPurchase, recQty, p.PONumber,
					fmt.Sprintf("Goods received from %s", p.PONumber))
			} else {
				_, _ = ps.catalog.UpdateStock(it.ProductID, recQty)
			}
		}
	}

	if allFullyReceived {
		p.Status = PurchaseReceived
	} else {
		p.Status = PurchasePartial
	}

	// Update supplier payable in memory
	if s, found := ps.suppliers[p.SupplierID]; found {
		s.PayableMinor += receivedBatchMinor
	}

	if ps.audit != nil {
		ps.audit.Record(ActionStockAdjusted, p.ID, "system", nil, nil,
			fmt.Sprintf("Received PO %s (status: %s)", p.PONumber, p.Status))
	}

	return p, nil
}

// RecordSupplierPayment records a payout to vendor, decreasing payable and recording ledger entry
func (ps *PurchaseService) RecordSupplierPayment(
	supplierID string,
	amountMinor int64,
	reference string,
	notes string,
	actor string,
) (*SupplierLedgerEntry, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if amountMinor <= 0 {
		return nil, fmt.Errorf("payment amount must be > 0")
	}
	s, found := ps.suppliers[supplierID]
	if !found {
		return nil, fmt.Errorf("supplier not found: %s", supplierID)
	}

	now := time.Now()
	nowStr := now.UTC().Format(time.RFC3339)
	sledID := fmt.Sprintf("sled-pay-%d-%d", now.UnixNano(), atomic.AddInt64(&supplierLedgerSeq, 1))
	newPayable := s.PayableMinor - amountMinor

	if ps.db != nil {
		err := ps.db.TransactionWithRetry(func(tx *data.RealTx) error {
			var curPayable int64
			row := tx.QueryRow(`SELECT payable_minor FROM suppliers WHERE id = ?`, supplierID)
			if err := row.Scan(&curPayable); err != nil {
				return fmt.Errorf("query supplier: %w", err)
			}
			newPayable = curPayable - amountMinor

			_, err := tx.Exec(`UPDATE suppliers SET payable_minor = ?, updated_at = ? WHERE id = ?`, newPayable, nowStr, supplierID)
			if err != nil {
				return err
			}

			_, err = tx.Exec(
				`INSERT INTO supplier_ledger (id, supplier_id, type, amount_minor, balance_after, reference, notes, created_by, timestamp)
				 VALUES (?, ?, 'PAYMENT', ?, ?, ?, ?, ?, ?)`,
				sledID, supplierID, amountMinor, newPayable, reference, notes, actor, nowStr,
			)
			return err
		}, 5)
		if err != nil {
			return nil, fmt.Errorf("supplier payment tx: %w", err)
		}
	}

	s.PayableMinor = newPayable
	entry := &SupplierLedgerEntry{
		ID:           sledID,
		SupplierID:   supplierID,
		Type:         "PAYMENT",
		AmountMinor:  amountMinor,
		BalanceAfter: newPayable,
		Reference:    reference,
		Notes:        notes,
		CreatedBy:    actor,
		Timestamp:    now,
	}

	if ps.audit != nil {
		ps.audit.Record(ActionSupplierPaid, supplierID, actor, nil,
			map[string]interface{}{"payable_minor": newPayable},
			fmt.Sprintf("Paid ৳%.2f to supplier %s (Ref: %s)", float64(amountMinor)/100.0, s.Name, reference))
	}
	return entry, nil
}

// RecordPurchaseReturn returns received goods back to the supplier, decrements stock, and decrements supplier payable
func (ps *PurchaseService) RecordPurchaseReturn(
	purchaseID string,
	supplierID string,
	items []PurchaseReturnItem,
	reason string,
	actor string,
) (*PurchaseReturnResult, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if len(items) == 0 {
		return nil, fmt.Errorf("no items specified for return")
	}

	s, found := ps.suppliers[supplierID]
	if !found {
		return nil, fmt.Errorf("supplier not found: %s", supplierID)
	}

	now := time.Now()
	nowStr := now.UTC().Format(time.RFC3339)
	var totalReturnMinor int64

	for _, it := range items {
		if it.Quantity.Value <= 0 {
			return nil, fmt.Errorf("return quantity for %s must be > 0", it.ProductID)
		}
		p, found := ps.catalog.FindByID(it.ProductID)
		if !found {
			return nil, fmt.Errorf("product not found: %s", it.ProductID)
		}
		if p.Stock.Cmp(it.Quantity) < 0 {
			return nil, fmt.Errorf("cannot return %s units of %s: current stock is %s",
				it.Quantity.String(), p.Name, p.Stock.String())
		}
		lineCost := it.CostPrice.MulDecimal(it.Quantity)
		totalReturnMinor += lineCost.Minor
	}

	sledID := fmt.Sprintf("sled-ret-%d-%d", now.UnixNano(), atomic.AddInt64(&supplierLedgerSeq, 1))
	newPayable := s.PayableMinor - totalReturnMinor

	if ps.db != nil {
		err := ps.db.TransactionWithRetry(func(tx *data.RealTx) error {
			for i, it := range items {
				var currentStock int64
				row := tx.QueryRow(`SELECT stock_raw FROM products WHERE id = ?`, it.ProductID)
				if err := row.Scan(&currentStock); err != nil {
					return err
				}
				if currentStock < it.Quantity.Value {
					return fmt.Errorf("insufficient DB stock for return of %s", it.ProductID)
				}
				newStock := currentStock - it.Quantity.Value

				res, err := tx.Exec(
					`UPDATE products SET stock_raw = ?, version = version + 1, updated_at = ? WHERE id = ? AND stock_raw >= ?`,
					newStock, nowStr, it.ProductID, it.Quantity.Value,
				)
				if err != nil {
					return err
				}
				rows, _ := res.RowsAffected()
				if rows == 0 {
					return fmt.Errorf("concurrency conflict during return of %s", it.ProductID)
				}

				movID := fmt.Sprintf("mov-pret-%d-%d", now.UnixNano(), i)
				_, err = tx.Exec(
					`INSERT INTO stock_movements (id, product_id, product_name, type, delta_raw, balance_raw, cost_minor, reference, timestamp, notes)
					 VALUES (?, ?, ?, 'RETURN', ?, ?, ?, ?, ?, ?)`,
					movID, it.ProductID, it.ProductID, -it.Quantity.Value, newStock,
					it.CostPrice.Minor, purchaseID, nowStr, fmt.Sprintf("Purchase return to supplier %s (Reason: %s)", supplierID, reason),
				)
				if err != nil {
					return err
				}
			}

			var curPayable int64
			_ = tx.QueryRow(`SELECT payable_minor FROM suppliers WHERE id = ?`, supplierID).Scan(&curPayable)
			newPayable = curPayable - totalReturnMinor

			_, err := tx.Exec(`UPDATE suppliers SET payable_minor = ?, updated_at = ? WHERE id = ?`, newPayable, nowStr, supplierID)
			if err != nil {
				return err
			}

			_, err = tx.Exec(
				`INSERT INTO supplier_ledger (id, supplier_id, type, amount_minor, balance_after, reference, notes, created_by, timestamp)
				 VALUES (?, ?, 'DEBIT_NOTE', ?, ?, ?, ?, ?, ?)`,
				sledID, supplierID, totalReturnMinor, newPayable, purchaseID, reason, actor, nowStr,
			)
			return err
		}, 5)
		if err != nil {
			return nil, fmt.Errorf("purchase return tx: %w", err)
		}
	}

	// Update in-memory stock and payable
	for _, it := range items {
		negQty := data.Decimal{Value: -it.Quantity.Value}
		if ps.inventory != nil {
			_, _ = ps.inventory.RecordMovement(it.ProductID, MovementDamage, negQty, purchaseID, "Purchase Return: "+reason)
		} else {
			_, _ = ps.catalog.UpdateStock(it.ProductID, negQty)
		}
	}
	s.PayableMinor = newPayable

	result := &PurchaseReturnResult{
		ID:           fmt.Sprintf("pret-%d", now.UnixNano()),
		PurchaseID:   purchaseID,
		SupplierID:   supplierID,
		TotalMinor:   totalReturnMinor,
		Items:        items,
		Timestamp:    now,
		BalanceAfter: newPayable,
	}
	return result, nil
}

// GetSupplierLedger returns all ledger entries for a supplier
func (ps *PurchaseService) GetSupplierLedger(supplierID string) ([]*SupplierLedgerEntry, error) {
	if ps.db == nil {
		return nil, nil
	}
	rows, err := ps.db.Query(
		`SELECT id, supplier_id, type, amount_minor, balance_after, reference, notes, created_by, timestamp
		 FROM supplier_ledger WHERE supplier_id = ? ORDER BY timestamp ASC`,
		supplierID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*SupplierLedgerEntry
	for rows.Next() {
		var e SupplierLedgerEntry
		var notes *string
		var createdBy *string
		var timeStr string
		err := rows.Scan(&e.ID, &e.SupplierID, &e.Type, &e.AmountMinor, &e.BalanceAfter, &e.Reference, &notes, &createdBy, &timeStr)
		if err != nil {
			return nil, err
		}
		if notes != nil {
			e.Notes = *notes
		}
		if createdBy != nil {
			e.CreatedBy = *createdBy
		}
		e.Timestamp, _ = time.Parse(time.RFC3339, timeStr)
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}

// RecordDamage logs damaged stock, decreases inventory, and records the write-off.
func (ps *PurchaseService) RecordDamage(
	productID string,
	qty data.Decimal,
	reason string,
	actor string,
) (*StockMovement, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	p, found := ps.catalog.FindByID(productID)
	if !found {
		return nil, fmt.Errorf("product not found: %s", productID)
	}

	if p.Stock.Cmp(qty) < 0 {
		return nil, fmt.Errorf("cannot write off %s of %s: current stock is %s",
			qty.String(), p.Name, p.Stock.String())
	}

	negQty := data.Decimal{Value: -qty.Value}
	now := time.Now()
	nowStr := now.UTC().Format(time.RFC3339)

	if ps.db != nil {
		err := ps.db.Transaction(func(tx *data.RealTx) error {
			newStockRaw := p.Stock.Value - qty.Value
			res, err := tx.Exec(
				`UPDATE products SET stock_raw=?, version=version+1, updated_at=? WHERE id=? AND stock_raw>=?`,
				newStockRaw, nowStr, productID, qty.Value,
			)
			if err != nil {
				return err
			}
			rows, _ := res.RowsAffected()
			if rows == 0 {
				return fmt.Errorf("concurrent stock conflict on product %s", productID)
			}

			movID := fmt.Sprintf("mov-dmg-%s-%d", productID, now.UnixNano())
			_, err = tx.Exec(
				`INSERT INTO stock_movements (id, product_id, product_name, type, delta_raw, balance_raw, cost_minor, reference, timestamp, notes)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				movID, productID, p.Name, string(MovementDamage),
				-qty.Value, newStockRaw, p.Cost.Minor,
				"DAMAGE-WRITEOFF", nowStr, reason,
			)
			return err
		})
		if err != nil {
			return nil, fmt.Errorf("damage transaction failed: %w", err)
		}
	}

	// Update in-memory
	var mov *StockMovement
	if ps.inventory != nil {
		mov, _ = ps.inventory.RecordMovement(productID, MovementDamage, negQty, "DAMAGE-WRITEOFF", reason)
	} else {
		_, _ = ps.catalog.UpdateStock(productID, negQty)
	}

	if ps.audit != nil {
		ps.audit.Record(ActionStockAdjusted, productID, actor, nil, nil,
			fmt.Sprintf("Wrote off %s damaged units of %s: %s", qty.String(), p.Name, reason))
	}

	return mov, nil
}

// calculateWAC calculates the new Weighted Average Cost:
// WAC = (old_stock * old_cost + rec_qty * new_cost) / (old_stock + rec_qty)
func calculateWAC(oldStock data.Decimal, oldCost data.Money, recQty data.Decimal, newCost data.Money) data.Money {
	if oldStock.Value <= 0 {
		return newCost
	}
	totalUnits := oldStock.Add(recQty)
	if totalUnits.Value <= 0 {
		return newCost
	}

	oldTotalValueMinor := oldCost.MulDecimal(oldStock).Minor
	newTotalValueMinor := newCost.MulDecimal(recQty).Minor
	combinedTotalMinor := oldTotalValueMinor + newTotalValueMinor

	// Weighted unit cost = combinedTotalMinor / totalUnits
	// Note: totalUnits.Value has scale DecimalScale (10000)
	unitCostMinor := (combinedTotalMinor * data.DecimalScale) / totalUnits.Value
	return data.NewMoney(unitCostMinor, oldCost.Currency)
}
