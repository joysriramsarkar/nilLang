package evaluator

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

func TestEvalEntityDeclaration(t *testing.T) {
	input := `
entity Product {
    id: uuid primary
    sku: string required unique
    name: string required
    price: money
    stock: quantity
}
`
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	env := object.NewEnvironment()
	res := Eval(program, env)

	if res == nil || res.Type() != object.ENTITY_OBJ {
		t.Fatalf("expected ENTITY_OBJ result, got %v", res)
	}

	entObj, ok := res.(*object.Entity)
	if !ok || entObj.Name != "Product" {
		t.Fatalf("expected evaluated entity name 'Product', got %v", entObj)
	}

	// Check environment binding
	val, exists := env.Get("Product")
	if !exists || val != entObj {
		t.Fatalf("expected 'Product' bound in environment")
	}

	if len(entObj.Fields) != 5 {
		t.Fatalf("expected 5 fields, got %d", len(entObj.Fields))
	}
}
