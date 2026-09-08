package pos

import (
	"fmt"
	"strings"
	"sync"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

// Category represents a product category
type Category struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	NameBn string `json:"name_bn"`
	Icon   string `json:"icon"`
}

// Product represents a retail catalog item
type Product struct {
	ID          string       `json:"id"`
	SKU         string       `json:"sku"`
	Barcode     string       `json:"barcode"`
	Name        string       `json:"name"`
	NameBn      string       `json:"name_bn"`
	CategoryID  string       `json:"category_id"`
	Unit        string       `json:"unit"`
	Price       data.Money   `json:"price"`     // Selling price
	Cost        data.Money   `json:"cost"`      // WAC cost
	Stock       data.Decimal `json:"stock"`     // Current physical stock
	LowStockMin data.Decimal `json:"low_stock"` // Threshold for low-stock warnings
	Active      bool         `json:"active"`
}

// CatalogRepository manages products and fast index lookups
type CatalogRepository struct {
	mu         sync.RWMutex
	products   map[string]*Product
	bySKU      map[string]*Product
	byBarcode  map[string]*Product
	categories map[string]*Category
}

// NewCatalogRepository creates a new catalog repository
func NewCatalogRepository() *CatalogRepository {
	return &CatalogRepository{
		products:   make(map[string]*Product),
		bySKU:      make(map[string]*Product),
		byBarcode:  make(map[string]*Product),
		categories: make(map[string]*Category),
	}
}

// AddCategory registers a category
func (c *CatalogRepository) AddCategory(cat *Category) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.categories[cat.ID] = cat
}

// AddProduct registers or updates a product
func (c *CatalogRepository) AddProduct(p *Product) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.products[p.ID] = p
	if p.SKU != "" {
		c.bySKU[strings.ToUpper(p.SKU)] = p
	}
	if p.Barcode != "" {
		c.byBarcode[p.Barcode] = p
	}
}

// FindByID looks up product by ID
func (c *CatalogRepository) FindByID(id string) (*Product, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.products[id]
	return p, ok
}

// FindBySKU looks up product by exact SKU
func (c *CatalogRepository) FindBySKU(sku string) (*Product, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.bySKU[strings.ToUpper(strings.TrimSpace(sku))]
	return p, ok
}

// FindByBarcode looks up product by exact Barcode
func (c *CatalogRepository) FindByBarcode(barcode string) (*Product, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.byBarcode[strings.TrimSpace(barcode)]
	return p, ok
}

// Search finds products matching SKU, Barcode, English name or Bengali name
func (c *CatalogRepository) Search(query string) []*Product {
	c.mu.RLock()
	defer c.mu.RUnlock()

	q := strings.ToLower(strings.TrimSpace(query))
	results := make([]*Product, 0)
	for _, p := range c.products {
		if !p.Active {
			continue
		}
		if q == "" ||
			strings.Contains(strings.ToLower(p.Name), q) ||
			strings.Contains(strings.ToLower(p.NameBn), q) ||
			strings.Contains(strings.ToLower(p.SKU), q) ||
			strings.Contains(p.Barcode, q) {
			results = append(results, p)
		}
	}
	return results
}

// AllProducts returns list of all products
func (c *CatalogRepository) AllProducts() []*Product {
	c.mu.RLock()
	defer c.mu.RUnlock()
	list := make([]*Product, 0, len(c.products))
	for _, p := range c.products {
		list = append(list, p)
	}
	return list
}

// AllCategories returns list of all categories
func (c *CatalogRepository) AllCategories() []*Category {
	c.mu.RLock()
	defer c.mu.RUnlock()
	list := make([]*Category, 0, len(c.categories))
	for _, cat := range c.categories {
		list = append(list, cat)
	}
	return list
}

// UpdateStock updates physical stock for a product
func (c *CatalogRepository) UpdateStock(productID string, delta data.Decimal) (data.Decimal, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	p, exists := c.products[productID]
	if !exists {
		return data.Decimal{}, fmt.Errorf("product not found: %s", productID)
	}

	newStock := p.Stock.Add(delta)
	if newStock.IsNegative() {
		return p.Stock, fmt.Errorf("insufficient stock for %s (current: %s, delta: %s)", p.Name, p.Stock.String(), delta.String())
	}
	p.Stock = newStock
	return p.Stock, nil
}
