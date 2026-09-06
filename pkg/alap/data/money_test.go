package data

import (
	"testing"
)

func TestMoneyArithmetic(t *testing.T) {
	// ৳12.50 + ৳2.50 = ৳15.00
	m1 := NewMoney(1250, "BDT")
	m2 := NewMoney(250, "BDT")

	sum := m1.Add(m2)
	if sum.Minor != 1500 {
		t.Fatalf("expected 1500, got %d", sum.Minor)
	}
	if sum.Format() != "৳15.00" {
		t.Fatalf("expected ৳15.00, got %s", sum.Format())
	}

	diff := m1.Sub(m2)
	if diff.Minor != 1000 {
		t.Fatalf("expected 1000, got %d", diff.Minor)
	}

	// Multiplication by scalar: ৳12.50 * 2 = ৳25.00
	mul := m1.Mul(2.0)
	if mul.Minor != 2500 {
		t.Fatalf("expected 2500, got %d", mul.Minor)
	}

	// Multiplication by quantity: ৳20.00/kg * 2.5 kg = ৳50.00
	unitPrice := NewMoney(2000, "BDT")
	total := unitPrice.MulQty(2500, 1000)
	if total.Minor != 5000 {
		t.Fatalf("expected 5000 (৳50.00), got %d", total.Minor)
	}
}

func TestParseMoney(t *testing.T) {
	m, err := ParseMoney("1,250.75", "BDT")
	if err != nil {
		t.Fatalf("ParseMoney error: %v", err)
	}
	if m.Minor != 125075 {
		t.Fatalf("expected 125075, got %d", m.Minor)
	}
	if m.Format() != "৳1,250.75" {
		t.Fatalf("expected ৳1,250.75, got %s", m.Format())
	}
}

func TestDecimalArithmetic(t *testing.T) {
	d1 := NewDecimal(1.1)
	d2 := NewDecimal(2.2)

	sum := d1.Add(d2)
	// 1.1 + 2.2 = 3.3 without IEEE floating point 3.3000000000000003 drift
	if sum.Float64() < 3.299 || sum.Float64() > 3.301 {
		t.Fatalf("expected ~3.3, got %f", sum.Float64())
	}
}
