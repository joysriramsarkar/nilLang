package pos

import (
	"fmt"
	"sync"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

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

	// Update supplier payable
	if s, found := ps.suppliers[p.SupplierID]; found {
		s.PayableMinor += p.TotalMinor
		if ps.db != nil {
			_, _ = ps.db.Exec(
				`UPDATE suppliers SET payable_minor=payable_minor+?, updated_at=? WHERE id=?`,
				p.TotalMinor, nowStr, s.ID)
		}
	}

	if ps.audit != nil {
		ps.audit.Record(ActionStockAdjusted, p.ID, "system", nil, nil,
			fmt.Sprintf("Received PO %s (status: %s)", p.PONumber, p.Status))
	}

	return p, nil
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
