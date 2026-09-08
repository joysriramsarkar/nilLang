package pos

import (
	"fmt"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

// SaleStatus defines state of the sale order
type SaleStatus string

const (
	StatusDraft             SaleStatus = "DRAFT"
	StatusPendingPayment    SaleStatus = "PENDING_PAYMENT"
	StatusPaid              SaleStatus = "PAID"
	StatusCompleted         SaleStatus = "COMPLETED"
	StatusCancelled         SaleStatus = "CANCELLED"
	StatusRefunded          SaleStatus = "REFUNDED"
	StatusPartiallyRefunded SaleStatus = "PARTIALLY_REFUNDED"
)

// SaleItem represents an immutable item line in a completed or in-flight sale
type SaleItem struct {
	ID               string       `json:"id"`
	SaleID           string       `json:"sale_id"`
	ProductID        string       `json:"product_id"`
	Name             string       `json:"name"`
	SKU              string       `json:"sku"`
	Unit             string       `json:"unit"`
	Quantity         data.Decimal `json:"quantity"`
	RefundedQuantity data.Decimal `json:"refunded_quantity,omitempty"`
	UnitPriceMinor   int64        `json:"unit_price_minor"`
	CostPriceMinor   int64        `json:"cost_price_minor"`
	SubtotalMinor    int64        `json:"subtotal_minor"`
	ProfitMinor      int64        `json:"profit_minor"` // (UnitPrice - CostPrice) * Quantity
}


// Sale represents a POS transaction record
type Sale struct {
	ID            string          `json:"id"`
	InvoiceNumber string          `json:"invoice_number"`
	RegisterID    string          `json:"register_id"`
	CashierID     string          `json:"cashier_id"`
	CustomerID    string          `json:"customer_id,omitempty"`
	SubtotalMinor int64           `json:"subtotal_minor"`
	DiscountMinor int64           `json:"discount_minor"`
	TaxMinor      int64           `json:"tax_minor"`
	TotalMinor    int64           `json:"total_minor"`
	PaidMinor     int64           `json:"paid_minor"`
	ChangeMinor   int64           `json:"change_minor"`
	Status        SaleStatus      `json:"status"`
	Items         []SaleItem      `json:"items"`
	Payments      []PaymentRecord `json:"payments"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// ValidateTransition enforces legal sale state machine transitions
func ValidateTransition(current, next SaleStatus) error {
	valid := false
	switch current {
	case StatusDraft:
		valid = (next == StatusPendingPayment || next == StatusCancelled)
	case StatusPendingPayment:
		valid = (next == StatusPaid || next == StatusCancelled)
	case StatusPaid:
		valid = (next == StatusCompleted || next == StatusRefunded)
	case StatusCompleted:
		valid = (next == StatusRefunded || next == StatusPartiallyRefunded)
	case StatusCancelled, StatusRefunded:
		valid = false // Terminal states
	case StatusPartiallyRefunded:
		valid = (next == StatusRefunded)
	}

	if !valid {
		return fmt.Errorf("illegal sale transition from %s to %s", current, next)
	}
	return nil
}

// GrossProfitMinor returns total gross profit on the sale
func (s *Sale) GrossProfitMinor() int64 {
	var profit int64 = 0
	for _, it := range s.Items {
		profit += it.ProfitMinor
	}
	// Subtract order-level discount
	profit -= s.DiscountMinor
	return profit
}
