package pos

import (
	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

// ─── TAX ENGINE ─────────────────────────────────────────────────────────────

type TaxType string

const (
	TaxExclusive TaxType = "exclusive" // Subtotal + Tax
	TaxInclusive TaxType = "inclusive" // Tax embedded in item price
)

// TaxRate represents a VAT / Sales Tax rule
type TaxRate struct {
	Name    string       `json:"name"`
	Rate    data.Decimal `json:"rate"` // e.g. 0.05 for 5%
	Type    TaxType      `json:"type"`
	Enabled bool         `json:"enabled"`
}

// CalculateTax calculates tax amount for a given taxable minor amount
func (tr TaxRate) CalculateTax(taxableMinor int64) int64 {
	if !tr.Enabled || tr.Rate.IsZero() {
		return 0
	}

	if tr.Type == TaxInclusive {
		// inclusive: tax = price - (price / (1 + rate))
		// rate is fixed point (DecimalScale = 10000)
		denom := data.DecimalScale + tr.Rate.Value
		baseMinor := (taxableMinor * data.DecimalScale) / denom
		return taxableMinor - baseMinor
	}

	// exclusive: tax = price * rate
	return (taxableMinor * tr.Rate.Value) / data.DecimalScale
}

// ─── DISCOUNT ENGINE ────────────────────────────────────────────────────────

type DiscountType string

const (
	DiscountPercentage DiscountType = "percentage"
	DiscountFixedMinor DiscountType = "fixed"
)

// Discount represents a price reduction rule
type Discount struct {
	Type   DiscountType `json:"type"`
	Value  data.Decimal `json:"value"` // Percentage (e.g. 10.0 for 10%) or Fixed Minor (e.g. 20000 for ৳200)
	Reason string       `json:"reason"`
}

// CalculateDiscount calculates discount minor reduction on a given amount
func (d Discount) CalculateDiscount(amountMinor int64) int64 {
	if d.Value.IsZero() || amountMinor <= 0 {
		return 0
	}

	if d.Type == DiscountPercentage {
		// (amountMinor * (percentage / 100))
		// percentage Value is scaled by DecimalScale (10000)
		reduction := (amountMinor * d.Value.Value) / (100 * data.DecimalScale)
		if reduction > amountMinor {
			return amountMinor
		}
		return reduction
	}

	// Fixed minor
	fixedReduction := d.Value.Value / data.DecimalScale // or raw if provided in minor
	if fixedReduction > amountMinor {
		return amountMinor
	}
	return fixedReduction
}
