package testing

import (
	"testing"
)

func TestConformanceEvaluator(t *testing.T) {
	sc := Scenario{
		Name: "Cash Checkout with 5% VAT",
		Input: ScenarioInput{
			Items: []ScenarioItem{
				{SKU: "RICE", Name: "Rice", PriceMinor: 300000, Quantity: 1},
			},
			Discount: 0,
			TaxRate:  0.05,
			Tendered: 350000,
		},
		ExpectedOutput: ScenarioOutput{
			SubtotalMinor:   300000,
			DiscountMinor:   0,
			TaxMinor:        15000,
			GrandTotalMinor: 315000,
			ChangeMinor:     35000,
		},
	}

	actual := ScenarioOutput{
		SubtotalMinor:   300000,
		DiscountMinor:   0,
		TaxMinor:        15000,
		GrandTotalMinor: 315000,
		ChangeMinor:     35000,
	}

	res := Evaluate(sc, actual)
	if !res.Passed {
		t.Fatalf("Conformance failed: %s", res.Difference)
	}
}
