package pos

import (
	"strings"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

// ─── TAX ENGINE (web-implications.md Section 7) ─────────────────────────────

type TaxType string

const (
	TaxExclusive TaxType = "exclusive" // Subtotal + Tax
	TaxInclusive TaxType = "inclusive" // Tax embedded in item price
)

type TaxRounding string

const (
	RoundHalfUp TaxRounding = "half_up"
	RoundFloor  TaxRounding = "floor"
	RoundCeil   TaxRounding = "ceil"
)

// TaxRate represents a VAT / Sales Tax rule (backward-compatible)
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
		denom := data.DecimalScale + tr.Rate.Value
		baseMinor := (taxableMinor * data.DecimalScale) / denom
		return taxableMinor - baseMinor
	}

	// exclusive: tax = (price * rate) with half-up rounding
	prod := taxableMinor * tr.Rate.Value
	return (prod + data.DecimalScale/2) / data.DecimalScale
}

// TaxRule represents an enterprise multi-tier or compound tax rule
type TaxRule struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Rate             data.Decimal `json:"rate"`
	Type             TaxType      `json:"type"`
	IsCompound       bool         `json:"is_compound"` // Applied on top of prior taxes
	ExemptCategories []string     `json:"exempt_categories,omitempty"`
	ExemptCustomers  []string     `json:"exempt_customers,omitempty"`
	Rounding         TaxRounding  `json:"rounding"`
	Enabled          bool         `json:"enabled"`
}

// TaxBreakdown represents an itemized tax calculation result
type TaxBreakdown struct {
	RuleID    string `json:"rule_id"`
	RuleName  string `json:"rule_name"`
	RateStr   string `json:"rate_str"`
	TaxMinor  int64  `json:"tax_minor"`
	Inclusive bool   `json:"inclusive"`
}

// TaxEngine manages and evaluates multiple tax rules deterministically
type TaxEngine struct {
	rules []TaxRule
}

// NewTaxEngine creates a tax engine instance
func NewTaxEngine() *TaxEngine {
	return &TaxEngine{rules: make([]TaxRule, 0)}
}

// AddRule registers a tax rule
func (te *TaxEngine) AddRule(r TaxRule) {
	te.rules = append(te.rules, r)
}

// Calculate computes all applicable taxes for a taxable subtotal
func (te *TaxEngine) Calculate(
	taxableMinor int64,
	categoryID string,
	customerID string,
) (totalTaxMinor int64, breakdowns []TaxBreakdown) {
	if taxableMinor <= 0 {
		return 0, nil
	}

	currentBase := taxableMinor
	accumulatedPriorTaxes := int64(0)

	for _, rule := range te.rules {
		if !rule.Enabled || rule.Rate.IsZero() {
			continue
		}

		// Check category exemption
		exemptCat := false
		for _, cat := range rule.ExemptCategories {
			if cat == categoryID {
				exemptCat = true
				break
			}
		}
		if exemptCat {
			continue
		}

		// Check customer exemption
		exemptCust := false
		for _, cust := range rule.ExemptCustomers {
			if cust == customerID {
				exemptCust = true
				break
			}
		}
		if exemptCust {
			continue
		}

		var lineBase int64
		if rule.IsCompound {
			lineBase = currentBase + accumulatedPriorTaxes
		} else {
			lineBase = currentBase
		}

		var computedTax int64
		if rule.Type == TaxInclusive {
			denom := data.DecimalScale + rule.Rate.Value
			baseWithoutTax := (lineBase * data.DecimalScale) / denom
			computedTax = lineBase - baseWithoutTax
		} else {
			prod := lineBase * rule.Rate.Value
			switch rule.Rounding {
			case RoundFloor:
				computedTax = prod / data.DecimalScale
			case RoundCeil:
				computedTax = (prod + data.DecimalScale - 1) / data.DecimalScale
			default: // RoundHalfUp
				computedTax = (prod + data.DecimalScale/2) / data.DecimalScale
			}
		}

		totalTaxMinor += computedTax
		accumulatedPriorTaxes += computedTax

		breakdowns = append(breakdowns, TaxBreakdown{
			RuleID:    rule.ID,
			RuleName:  rule.Name,
			RateStr:   rule.Rate.String(),
			TaxMinor:  computedTax,
			Inclusive: rule.Type == TaxInclusive,
		})
	}

	return totalTaxMinor, breakdowns
}

// ─── DISCOUNT ENGINE (web-implications.md Section 8) ────────────────────────

type DiscountType string

const (
	DiscountPercentage DiscountType = "percentage"
	DiscountFixedMinor DiscountType = "fixed"
	DiscountBuyXGetY   DiscountType = "buy_x_get_y"
	DiscountTiered     DiscountType = "tiered"
)

type DiscountScope string

const (
	ScopeCart     DiscountScope = "cart"
	ScopeItem     DiscountScope = "item"
	ScopeCategory DiscountScope = "category"
	ScopeCustomer DiscountScope = "customer"
	ScopeCoupon   DiscountScope = "coupon"
)

// TierRule defines threshold-based tiered discounts
type TierRule struct {
	MinAmountMinor int64        `json:"min_amount_minor"`
	DiscountValue  data.Decimal `json:"discount_value"` // Percentage or fixed
}

// BuyXGetYRule defines promotions like "Buy 2 Get 1 Free"
type BuyXGetYRule struct {
	BuyQty data.Decimal `json:"buy_qty"`
	GetQty data.Decimal `json:"get_qty"`
}

// Discount represents a price reduction rule (backward-compatible)
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
		// (amountMinor * (percentage / 100)) with half-up rounding
		prod := amountMinor * d.Value.Value
		reduction := (prod + (50 * data.DecimalScale)) / (100 * data.DecimalScale)
		if reduction > amountMinor {
			return amountMinor
		}
		return reduction
	}

	// Fixed minor
	fixedReduction := d.Value.Value / data.DecimalScale
	if fixedReduction > amountMinor {
		return amountMinor
	}
	return fixedReduction
}

// DiscountRule defines an enterprise discount or promotion policy
type DiscountRule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Type        DiscountType  `json:"type"`
	Scope       DiscountScope `json:"scope"`
	TargetID    string        `json:"target_id,omitempty"` // Product ID, Category ID, or Customer ID
	CouponCode  string        `json:"coupon_code,omitempty"`
	Value       data.Decimal  `json:"value"`
	Reason      string        `json:"reason"`
	Tiers       []TierRule    `json:"tiers,omitempty"`
	BuyXGetY    *BuyXGetYRule `json:"buy_x_get_y,omitempty"`
	ActiveFrom  *time.Time    `json:"active_from,omitempty"`
	ActiveUntil *time.Time    `json:"active_until,omitempty"`
	Enabled     bool          `json:"enabled"`
}

// DiscountEngine provides deterministic discount calculations
type DiscountEngine struct {
	rules []DiscountRule
}

// NewDiscountEngine creates a new discount engine
func NewDiscountEngine() *DiscountEngine {
	return &DiscountEngine{rules: make([]DiscountRule, 0)}
}

// AddRule registers a discount rule
func (de *DiscountEngine) AddRule(rule DiscountRule) {
	de.rules = append(de.rules, rule)
}

// CalculateCartDiscount computes cart-wide discount deterministically
func (de *DiscountEngine) CalculateCartDiscount(
	subtotalMinor int64,
	customerID string,
	couponCode string,
	now time.Time,
) (totalDiscountMinor int64, appliedReasons []string) {
	if subtotalMinor <= 0 {
		return 0, nil
	}

	couponCode = strings.TrimSpace(strings.ToUpper(couponCode))

	for _, r := range de.rules {
		if !r.Enabled {
			continue
		}

		// Time-based validation
		if r.ActiveFrom != nil && now.Before(*r.ActiveFrom) {
			continue
		}
		if r.ActiveUntil != nil && now.After(*r.ActiveUntil) {
			continue
		}

		// Coupon scope check
		if r.Scope == ScopeCoupon {
			if couponCode == "" || !strings.EqualFold(r.CouponCode, couponCode) {
				continue
			}
		}

		// Customer scope check
		if r.Scope == ScopeCustomer && r.TargetID != "" && r.TargetID != customerID {
			continue
		}

		var reduction int64

		switch r.Type {
		case DiscountPercentage:
			prod := subtotalMinor * r.Value.Value
			reduction = (prod + 50*data.DecimalScale) / (100 * data.DecimalScale)

		case DiscountFixedMinor:
			reduction = r.Value.Value / data.DecimalScale

		case DiscountTiered:
			// Match highest eligible tier
			var matchedTier *TierRule
			for _, tier := range r.Tiers {
				if subtotalMinor >= tier.MinAmountMinor {
					if matchedTier == nil || tier.MinAmountMinor > matchedTier.MinAmountMinor {
						t := tier
						matchedTier = &t
					}
				}
			}
			if matchedTier != nil {
				prod := subtotalMinor * matchedTier.DiscountValue.Value
				reduction = (prod + 50*data.DecimalScale) / (100 * data.DecimalScale)
			}
		}

		if reduction > 0 {
			if totalDiscountMinor+reduction > subtotalMinor {
				reduction = subtotalMinor - totalDiscountMinor
			}
			totalDiscountMinor += reduction
			reason := r.Reason
			if reason == "" {
				reason = r.Name
			}
			appliedReasons = append(appliedReasons, reason)
		}

		if totalDiscountMinor >= subtotalMinor {
			break
		}
	}

	return totalDiscountMinor, appliedReasons
}
