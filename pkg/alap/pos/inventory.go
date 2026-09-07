package pos

import (
	"fmt"
	"sync"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

// StockMovementType defines the reason for stock adjustment
type StockMovementType string

const (
	MovementSale       StockMovementType = "SALE"
	MovementPurchase   StockMovementType = "PURCHASE"
	MovementReturn     StockMovementType = "RETURN"
	MovementAdjustment StockMovementType = "ADJUSTMENT"
	MovementDamage     StockMovementType = "DAMAGE"
)

// StockMovement represents an immutable ledger record of an inventory change
type StockMovement struct {
	ID           string            `json:"id"`
	ProductID    string            `json:"product_id"`
	ProductName  string            `json:"product_name"`
	Type         StockMovementType `json:"type"`
	Delta        data.Decimal      `json:"delta"`
	BalanceAfter data.Decimal      `json:"balance_after"`
	UnitCost     data.Money        `json:"unit_cost"`
	Reference    string            `json:"reference"` // Invoice No, PO No, etc.
	Timestamp    time.Time         `json:"timestamp"`
	Notes        string            `json:"notes,omitempty"`
}

// InventoryLedger maintains immutable movement logs and inventory valuation
type InventoryLedger struct {
	mu        sync.RWMutex
	movements []*StockMovement
	catalog   *CatalogRepository
}

// NewInventoryLedger creates a new inventory ledger
func NewInventoryLedger(catalog *CatalogRepository) *InventoryLedger {
	return &InventoryLedger{
		movements: make([]*StockMovement, 0),
		catalog:   catalog,
	}
}

// RecordMovement records a movement and atomically updates product stock
func (il *InventoryLedger) RecordMovement(
	productID string,
	movementType StockMovementType,
	delta data.Decimal,
	reference string,
	notes string,
) (*StockMovement, error) {
	il.mu.Lock()
	defer il.mu.Unlock()

	p, exists := il.catalog.FindByID(productID)
	if !exists {
		return nil, fmt.Errorf("product not found: %s", productID)
	}

	newStock, err := il.catalog.UpdateStock(productID, delta)
	if err != nil {
		return nil, err
	}

	mov := &StockMovement{
		ID:           fmt.Sprintf("mov-%d", time.Now().UnixNano()),
		ProductID:    productID,
		ProductName:  p.Name,
		Type:         movementType,
		Delta:        delta,
		BalanceAfter: newStock,
		UnitCost:     p.Cost,
		Reference:    reference,
		Timestamp:    time.Now(),
		Notes:        notes,
	}

	il.movements = append(il.movements, mov)
	return mov, nil
}

// MovementsForProduct returns all movements for a single product
func (il *InventoryLedger) MovementsForProduct(productID string) []*StockMovement {
	il.mu.RLock()
	defer il.mu.RUnlock()

	res := make([]*StockMovement, 0)
	for _, m := range il.movements {
		if m.ProductID == productID {
			res = append(res, m)
		}
	}
	return res
}

// RecentMovements returns the latest N stock movements
func (il *InventoryLedger) RecentMovements(limit int) []*StockMovement {
	il.mu.RLock()
	defer il.mu.RUnlock()

	n := len(il.movements)
	if limit > n {
		limit = n
	}

	res := make([]*StockMovement, limit)
	for i := 0; i < limit; i++ {
		res[i] = il.movements[n-1-i]
	}
	return res
}

// TotalInventoryValuationMinor calculates total cost valuation of all items in stock
func (il *InventoryLedger) TotalInventoryValuationMinor() int64 {
	products := il.catalog.AllProducts()
	var totalValuation int64 = 0
	for _, p := range products {
		valMoney := p.Cost.MulDecimal(p.Stock)
		totalValuation += valMoney.Minor
	}
	return totalValuation
}
