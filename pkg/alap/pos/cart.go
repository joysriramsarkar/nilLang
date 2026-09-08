package pos

import (
	"fmt"
	"sync"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

// CartItem represents an item line in the cart
type CartItem struct {
	ProductID     string       `json:"product_id"`
	SKU           string       `json:"sku"`
	Barcode       string       `json:"barcode"`
	Name          string       `json:"name"`
	CategoryID    string       `json:"category_id,omitempty"`
	Unit          string       `json:"unit"`
	UnitPrice     data.Money   `json:"unit_price"`
	CostPrice     data.Money   `json:"cost_price"`
	Quantity      data.Decimal `json:"quantity"`
	LineDiscount  int64        `json:"line_discount_minor"`
	SubtotalMinor int64        `json:"subtotal_minor"`
	TotalMinor    int64        `json:"total_minor"`
}

// Cart represents a shopping cart instance
type Cart struct {
	mu              sync.RWMutex
	ID              string      `json:"id"`
	TabName         string      `json:"tab_name"`
	CustomerID      string      `json:"customer_id"`
	CouponCode      string      `json:"coupon_code,omitempty"`
	Items           []*CartItem `json:"items"`
	OrderDiscount   Discount    `json:"order_discount"`
	TaxRate         TaxRate     `json:"tax_rate"`
	SubtotalMinor   int64       `json:"subtotal_minor"`
	DiscountMinor   int64       `json:"discount_minor"`
	TaxMinor        int64       `json:"tax_minor"`
	GrandTotalMinor int64       `json:"grand_total_minor"`
}

// NewCart creates a new cart for a register tab
func NewCart(id, tabName string) *Cart {
	return &Cart{
		ID:         id,
		TabName:    tabName,
		CustomerID: "",
		Items:      make([]*CartItem, 0),
		TaxRate: TaxRate{
			Name:    "VAT",
			Rate:    data.NewDecimal(0), // Default 0%, can be set
			Type:    TaxExclusive,
			Enabled: false,
		},
	}
}

// AddProduct adds a product or increments its quantity
func (c *Cart) AddProduct(p *Product, qty data.Decimal) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, it := range c.Items {
		if it.ProductID == p.ID {
			it.Quantity = it.Quantity.Add(qty)
			c.recalculateLocked()
			return
		}
	}

	// New line item
	c.Items = append(c.Items, &CartItem{
		ProductID:  p.ID,
		SKU:        p.SKU,
		Barcode:    p.Barcode,
		Name:       p.Name,
		CategoryID: p.CategoryID,
		Unit:       p.Unit,
		UnitPrice:  p.Price,
		CostPrice:  p.Cost,
		Quantity:   qty,
	})
	c.recalculateLocked()
}

// SetQuantity sets explicit quantity for an item
func (c *Cart) SetQuantity(productID string, qty data.Decimal) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if qty.IsZero() || qty.IsNegative() {
		// Remove item
		newItems := make([]*CartItem, 0)
		for _, it := range c.Items {
			if it.ProductID != productID {
				newItems = append(newItems, it)
			}
		}
		c.Items = newItems
		c.recalculateLocked()
		return nil
	}

	for _, it := range c.Items {
		if it.ProductID == productID {
			it.Quantity = qty
			c.recalculateLocked()
			return nil
		}
	}

	return fmt.Errorf("item not in cart: %s", productID)
}

// RemoveItem removes product line completely
func (c *Cart) RemoveItem(productID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	newItems := make([]*CartItem, 0)
	for _, it := range c.Items {
		if it.ProductID != productID {
			newItems = append(newItems, it)
		}
	}
	c.Items = newItems
	c.recalculateLocked()
}

// SetCustomer sets attached customer ID
func (c *Cart) SetCustomer(customerID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CustomerID = customerID
}

// ApplyCoupon sets coupon code for cart
func (c *Cart) ApplyCoupon(coupon string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CouponCode = coupon
	c.recalculateLocked()
}

// ApplyDiscount sets cart-level discount
func (c *Cart) ApplyDiscount(d Discount) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.OrderDiscount = d
	c.recalculateLocked()
}

// SetTaxRate sets cart tax rate
func (c *Cart) SetTaxRate(tr TaxRate) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.TaxRate = tr
	c.recalculateLocked()
}

// Clear clears all cart items
func (c *Cart) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Items = make([]*CartItem, 0)
	c.OrderDiscount = Discount{}
	c.recalculateLocked()
}

// recalculateLocked recalculates subtotals, discounts, taxes, and grand totals
func (c *Cart) recalculateLocked() {
	var subtotal int64 = 0

	for _, it := range c.Items {
		// unit price * quantity
		lineTotalMoney := it.UnitPrice.MulDecimal(it.Quantity)
		it.SubtotalMinor = lineTotalMoney.Minor
		it.TotalMinor = it.SubtotalMinor - it.LineDiscount
		if it.TotalMinor < 0 {
			it.TotalMinor = 0
		}
		subtotal += it.TotalMinor
	}

	c.SubtotalMinor = subtotal

	// Apply cart discount
	discount := c.OrderDiscount.CalculateDiscount(subtotal)
	c.DiscountMinor = discount

	taxable := subtotal - discount
	if taxable < 0 {
		taxable = 0
	}

	// Apply tax
	tax := c.TaxRate.CalculateTax(taxable)
	c.TaxMinor = tax

	if c.TaxRate.Type == TaxInclusive {
		c.GrandTotalMinor = taxable
	} else {
		c.GrandTotalMinor = taxable + tax
	}
}

// Recalculate public thread-safe trigger
func (c *Cart) Recalculate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recalculateLocked()
}

// Clone returns a snapshot copy
func (c *Cart) Snapshot() Cart {
	c.mu.RLock()
	defer c.mu.RUnlock()

	itemsCopy := make([]*CartItem, len(c.Items))
	for i, it := range c.Items {
		itCopy := *it
		itemsCopy[i] = &itCopy
	}

	return Cart{
		ID:              c.ID,
		TabName:         c.TabName,
		CustomerID:      c.CustomerID,
		CouponCode:      c.CouponCode,
		Items:           itemsCopy,
		OrderDiscount:   c.OrderDiscount,
		TaxRate:         c.TaxRate,
		SubtotalMinor:   c.SubtotalMinor,
		DiscountMinor:   c.DiscountMinor,
		TaxMinor:        c.TaxMinor,
		GrandTotalMinor: c.GrandTotalMinor,
	}
}

// ─── MULTI-TAB CART MANAGER ─────────────────────────────────────────────────

// CartManager manages multiple active tabs / hold carts for a cashier terminal
type CartManager struct {
	mu        sync.RWMutex
	carts     map[string]*Cart
	activeTab string
}

// NewCartManager creates a cart manager with default tabs (Tab 1, Tab 2, Tab 3)
func NewCartManager() *CartManager {
	cm := &CartManager{
		carts:     make(map[string]*Cart),
		activeTab: "tab-1",
	}
	cm.carts["tab-1"] = NewCart("cart-1", "Tab 1")
	cm.carts["tab-2"] = NewCart("cart-2", "Tab 2")
	cm.carts["tab-3"] = NewCart("cart-3", "Tab 3")
	return cm
}

// SwitchTab switches active cart tab
func (cm *CartManager) SwitchTab(tabID string) (*Cart, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	c, ok := cm.carts[tabID]
	if !ok {
		return nil, fmt.Errorf("tab not found: %s", tabID)
	}
	cm.activeTab = tabID
	return c, nil
}

// GetActiveCart returns the current cart
func (cm *CartManager) GetActiveCart() *Cart {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.carts[cm.activeTab]
}

// GetCart returns cart for a specific tab
func (cm *CartManager) GetCart(tabID string) (*Cart, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	c, ok := cm.carts[tabID]
	return c, ok
}

// AllTabs returns all carts
func (cm *CartManager) AllTabs() map[string]Cart {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	res := make(map[string]Cart)
	for k, v := range cm.carts {
		res[k] = v.Snapshot()
	}
	return res
}
