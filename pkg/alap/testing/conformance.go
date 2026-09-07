package testing

import (
	"fmt"
)

// Scenario represents a business conformance scenario to verify parity between TypeScript POS and NilLang POS
type Scenario struct {
	Name           string
	Description    string
	Input          ScenarioInput
	ExpectedOutput ScenarioOutput
}

// ScenarioInput defines input for a test case
type ScenarioInput struct {
	Items    []ScenarioItem
	Discount float64
	TaxRate  float64
	Tendered int64
}

type ScenarioItem struct {
	SKU        string
	Name       string
	PriceMinor int64
	Quantity   int64
	CostMinor  int64
}

// ScenarioOutput defines expected assertion values
type ScenarioOutput struct {
	SubtotalMinor   int64
	DiscountMinor   int64
	TaxMinor        int64
	GrandTotalMinor int64
	ChangeMinor     int64
}

// ConformanceResult holds results of a scenario run
type ConformanceResult struct {
	ScenarioName string
	Passed       bool
	Difference   string
}

// Evaluate checks if actual matches expected
func Evaluate(sc Scenario, actual ScenarioOutput) ConformanceResult {
	exp := sc.ExpectedOutput
	if actual.SubtotalMinor != exp.SubtotalMinor {
		return ConformanceResult{
			ScenarioName: sc.Name,
			Passed:       false,
			Difference:   fmt.Sprintf("Subtotal mismatch: expected %d, got %d", exp.SubtotalMinor, actual.SubtotalMinor),
		}
	}
	if actual.DiscountMinor != exp.DiscountMinor {
		return ConformanceResult{
			ScenarioName: sc.Name,
			Passed:       false,
			Difference:   fmt.Sprintf("Discount mismatch: expected %d, got %d", exp.DiscountMinor, actual.DiscountMinor),
		}
	}
	if actual.TaxMinor != exp.TaxMinor {
		return ConformanceResult{
			ScenarioName: sc.Name,
			Passed:       false,
			Difference:   fmt.Sprintf("Tax mismatch: expected %d, got %d", exp.TaxMinor, actual.TaxMinor),
		}
	}
	if actual.GrandTotalMinor != exp.GrandTotalMinor {
		return ConformanceResult{
			ScenarioName: sc.Name,
			Passed:       false,
			Difference:   fmt.Sprintf("Grand total mismatch: expected %d, got %d", exp.GrandTotalMinor, actual.GrandTotalMinor),
		}
	}
	if actual.ChangeMinor != exp.ChangeMinor {
		return ConformanceResult{
			ScenarioName: sc.Name,
			Passed:       false,
			Difference:   fmt.Sprintf("Change mismatch: expected %d, got %d", exp.ChangeMinor, actual.ChangeMinor),
		}
	}
	return ConformanceResult{
		ScenarioName: sc.Name,
		Passed:       true,
	}
}
