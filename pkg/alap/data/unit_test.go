package data

import (
	"testing"
)

func TestUnitSystemAndConversion(t *testing.T) {
	// 1. Convert 1.5 kg to grams -> 1500 g
	d1, _ := ParseDecimal("1.5")
	qKg := NewQuantity(d1, "kg")

	qGram, err := ConvertQuantity(qKg, UnitGram)
	if err != nil {
		t.Fatalf("ConvertQuantity failed: %v", err)
	}
	if qGram.Amount.String() != "1500.00" || qGram.Unit != "g" {
		t.Fatalf("Expected 1500.00 g, got %s", qGram.String())
	}

	// 2. Compatible addition: 1.5 kg + 250 g = 1.75 kg
	d2, _ := ParseDecimal("250")
	q250g := NewQuantity(d2, "g")

	sumKg, err := AddQuantity(qKg, q250g)
	if err != nil {
		t.Fatalf("AddQuantity failed: %v", err)
	}
	if sumKg.Amount.String() != "1.75" || sumKg.Unit != "kg" {
		t.Fatalf("Expected 1.75 kg, got %s", sumKg.String())
	}

	// 3. Incompatible dimension error: kg vs litre
	err = ValidateUnitCompatibility("kg", "l")
	if err == nil {
		t.Fatalf("Expected error when comparing mass with volume")
	}

	// 4. Comparison with automatic conversion
	d3, _ := ParseDecimal("1000")
	q1000g := NewQuantity(d3, "g") // 1000 g == 1 kg
	d4, _ := ParseDecimal("1.0")
	q1kg := NewQuantity(d4, "kg")

	cmp, err := CmpQuantity(q1kg, q1000g)
	if err != nil {
		t.Fatalf("CmpQuantity error: %v", err)
	}
	if cmp != 0 {
		t.Fatalf("Expected 1 kg == 1000 g, got cmp=%d", cmp)
	}
}

func TestFinancialExactnessAndAllocation(t *testing.T) {
	// Test Decimal Rounding
	d, _ := ParseDecimal("12.3456")
	r2 := d.Round(2)
	if r2.String() != "12.35" {
		t.Fatalf("Expected 12.35, got %s", r2.String())
	}
	r0 := d.Round(0)
	if r0.String() != "12.00" {
		t.Fatalf("Expected 12.00, got %s", r0.String())
	}

	// Test DecimalFromMinor
	dFromMin := DecimalFromMinor(1250, 100)
	if dFromMin.String() != "12.50" {
		t.Fatalf("Expected 12.50, got %s", dFromMin.String())
	}

	// Test Money MulRatio
	m := NewMoney(10000, "BDT") // ৳100.00
	mThird, err := m.MulRatio(1, 3)
	if err != nil {
		t.Fatalf("MulRatio error: %v", err)
	}
	if mThird.Minor != 3333 { // 10000 / 3 = 3333 paisa
		t.Fatalf("Expected 3333 paisa, got %d", mThird.Minor)
	}

	// Test Fowler Money Allocation: ৳100.00 split 1:1:1
	// 10000 paisa divided 3 ways: 3334, 3333, 3333 = sum exactly 10000
	shares, err := m.Allocate(1, 1, 1)
	if err != nil {
		t.Fatalf("Allocate error: %v", err)
	}
	var totalAllocated int64
	for _, s := range shares {
		totalAllocated += s.Minor
	}
	if totalAllocated != 10000 {
		t.Fatalf("Expected sum 10000 paisa, got %d", totalAllocated)
	}
	if shares[0].Minor != 3334 || shares[1].Minor != 3333 || shares[2].Minor != 3333 {
		t.Fatalf("Unexpected shares: %v", shares)
	}
}
