package data

import (
	"testing"
)

func TestExtendedDecimal(t *testing.T) {
	d1, err := ParseDecimal("12.50")
	if err != nil {
		t.Fatalf("ParseDecimal error: %v", err)
	}
	d2, err := ParseDecimal("3.75")
	if err != nil {
		t.Fatalf("ParseDecimal error: %v", err)
	}

	sum := d1.Add(d2)
	if sum.String() != "16.25" {
		t.Errorf("Expected 16.25, got %s", sum.String())
	}

	qty := NewQuantity(d1, "kg")
	if qty.String() != "12.50 kg" {
		t.Errorf("Expected '12.50 kg', got '%s'", qty.String())
	}

	// Money * Decimal
	m := NewMoney(10000, "BDT")       // ৳100.00
	factor, _ := ParseDecimal("1.15") // +15% tax
	result := m.MulDecimal(factor)
	if result.Minor != 11500 {
		t.Errorf("Expected 11500, got %d", result.Minor)
	}
}
