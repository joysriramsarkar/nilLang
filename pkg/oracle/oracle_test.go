package oracle

import (
	"testing"
)

func TestLanguageOracle(t *testing.T) {
	lo := NewLanguageOracle()

	// Builtin lookup
	sym, ok := lo.LookupSymbol("puts")
	if !ok {
		t.Fatalf("expected puts symbol to be present")
	}
	if sym.Kind != KindBuiltin {
		t.Errorf("expected puts to be Builtin, got %s", sym.Kind)
	}

	// Verify call to puts (variadic)
	if err := lo.VerifyCall("puts", 3); err != nil {
		t.Errorf("variadic puts call failed: %v", err)
	}

	// Register custom function
	lo.RegisterSymbol(Symbol{
		Name:       "calculateTax",
		Kind:       KindFunction,
		Parameters: []string{"amount: Float", "rate: Float"},
		ReturnType: "Float",
		Effects:    []Effect{EffectPure},
	})

	// Valid call
	if err := lo.VerifyCall("calculateTax", 2); err != nil {
		t.Errorf("valid calculateTax call failed: %v", err)
	}

	// Invalid arity
	if err := lo.VerifyCall("calculateTax", 1); err == nil {
		t.Errorf("expected error for calculateTax with 1 argument, got nil")
	}

	// Explain error
	explanation := lo.ExplainError("identifier not found: foobar")
	if len(explanation) == 0 {
		t.Errorf("expected explanation, got empty string")
	}
}
