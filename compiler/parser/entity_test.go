package parser

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
)

func TestParseEntityDeclaration(t *testing.T) {
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
	p := New(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	if len(program.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.EntityStatement)
	if !ok {
		t.Fatalf("expected *ast.EntityStatement, got %T", program.Statements[0])
	}

	if stmt.Name.Value != "Product" {
		t.Fatalf("expected entity name 'Product', got %s", stmt.Name.Value)
	}

	if len(stmt.Fields) != 5 {
		t.Fatalf("expected 5 fields, got %d", len(stmt.Fields))
	}

	// Verify field 0: id uuid primary
	if stmt.Fields[0].Name != "id" || stmt.Fields[0].Type != "uuid" || !stmt.Fields[0].IsPrimary {
		t.Errorf("field 0 mismatch: %+v", stmt.Fields[0])
	}

	// Verify field 1: sku string required unique
	if stmt.Fields[1].Name != "sku" || stmt.Fields[1].Type != "string" || !stmt.Fields[1].IsRequired || !stmt.Fields[1].IsUnique {
		t.Errorf("field 1 mismatch: %+v", stmt.Fields[1])
	}

	// Verify field 3: price money
	if stmt.Fields[3].Name != "price" || stmt.Fields[3].Type != "money" {
		t.Errorf("field 3 mismatch: %+v", stmt.Fields[3])
	}

	// Verify field 4: stock quantity
	if stmt.Fields[4].Name != "stock" || stmt.Fields[4].Type != "quantity" {
		t.Errorf("field 4 mismatch: %+v", stmt.Fields[4])
	}
}
