package forms

import (
	"testing"
)

func TestFormValidation(t *testing.T) {
	schema := NewForm("ProductForm")
	schema.Field("sku", "SKU").Required().MinLength(3)
	schema.Field("name", "Product Name").Required()
	schema.Field("price", "Price").Min(1.0)
	schema.Field("email", "Supplier Email").Email()

	// 1. Invalid payload
	badData := map[string]interface{}{
		"sku":   "A",
		"name":  "",
		"price": 0.5,
		"email": "not-an-email",
	}
	res := schema.Validate(badData)
	if res.IsValid {
		t.Fatalf("Expected validation failure")
	}
	if len(res.Errors) != 4 {
		t.Fatalf("Expected 4 errors, got %d: %v", len(res.Errors), res.Errors)
	}

	// 2. Valid payload
	goodData := map[string]interface{}{
		"sku":   "RICE-MIN-50",
		"name":  "Miniket Rice 50kg",
		"price": 3200.0,
		"email": "supplier@grain.com",
	}
	res2 := schema.Validate(goodData)
	if !res2.IsValid {
		t.Fatalf("Expected validation pass, got errors: %v", res2.Errors)
	}
}
