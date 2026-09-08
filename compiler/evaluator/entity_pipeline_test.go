package evaluator_test

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/mir"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/compiler/typecheck"
	"github.com/joysriramsarkar/nilLang/compiler/types"
	"github.com/joysriramsarkar/nilLang/pkg/alap/entity"
)

// TestEntityFullCompilerPipeline tests the complete vertical compiler pipeline:
// AST -> Typecheck -> HIR -> MIR -> Canonical Entity -> SQL/REST/Validation/ClientModel
func TestEntityFullCompilerPipeline(t *testing.T) {
	input := `
entity Customer {
    id: UUID primary
    name: String required
    email: Email unique
    phone: String required
    due_balance: Money
    total_orders: Int
}
`
	// 1. Lexer & Parser
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}

	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}

	stmt, ok := prog.Statements[0].(*ast.EntityStatement)
	if !ok {
		t.Fatalf("expected *ast.EntityStatement, got %T", prog.Statements[0])
	}
	if stmt.Name.Value != "Customer" {
		t.Errorf("expected entity name Customer, got %s", stmt.Name.Value)
	}
	t.Log("✓ 1. Parser generated *ast.EntityStatement")

	// 2. Typechecker
	checker := typecheck.NewChecker()
	ok = checker.CheckProgram(prog)
	if !ok || len(checker.Diagnostics) > 0 {
		t.Fatalf("typecheck failed with diagnostics: %+v", checker.Diagnostics)
	}
	t.Log("✓ 2. Typechecker validated EntityStatement and field types without errors")

	// 3. Lower to HIR
	hirLowerer := hir.NewLowerer()
	hirProg := hirLowerer.LowerProgram(prog)
	if len(hirProg.Statements) != 1 {
		t.Fatalf("HIR expected 1 statement, got %d", len(hirProg.Statements))
	}
	hirDecl, ok := hirProg.Statements[0].(*hir.EntityDeclStmt)
	if !ok {
		t.Fatalf("expected *hir.EntityDeclStmt in HIR, got %T", hirProg.Statements[0])
	}
	if hirDecl.Name != "Customer" {
		t.Errorf("HIR entity name expected Customer, got %s", hirDecl.Name)
	}
	if hirDecl.EntityType.Kind() != types.KindEntity {
		t.Errorf("HIR entity kind expected KindEntity, got %s", hirDecl.EntityType.Kind())
	}
	t.Log("✓ 3. HIR lowerer produced typed *hir.EntityDeclStmt")

	// 4. Lower to MIR
	mirLowerer := mir.NewLowerer()
	mirProg := mirLowerer.LowerHIR(hirProg)
	mirEnt, found := mirProg.Entities["Customer"]
	if !found || mirEnt == nil {
		t.Fatalf("MIR Entities map missing Customer")
	}
	if len(mirEnt.Fields) != 6 {
		t.Errorf("MIR entity expected 6 fields, got %d", len(mirEnt.Fields))
	}
	t.Log("✓ 4. MIR lowerer populated Entities map with *mir.EntityDef")

	// 5. Single Source of Truth Bridge: FromAST -> canonical entity.Entity
	canonicalEnt, err := entity.FromAST(stmt)
	if err != nil {
		t.Fatalf("FromAST failed: %v", err)
	}
	if canonicalEnt.Name != "Customer" {
		t.Errorf("canonical entity name expected Customer, got %s", canonicalEnt.Name)
	}

	// 5a. Generate SQL DDL for SQLite & PostgreSQL
	sqliteDDL := canonicalEnt.GenerateSQL("sqlite")
	if !strings.Contains(sqliteDDL, "CREATE TABLE IF NOT EXISTS customers") ||
		!strings.Contains(sqliteDDL, "due_balance BIGINT") {
		t.Errorf("unexpected SQLite DDL: %s", sqliteDDL)
	}

	postgresDDL := canonicalEnt.GenerateSQL("postgres")
	if !strings.Contains(postgresDDL, "id UUID PRIMARY KEY") ||
		!strings.Contains(postgresDDL, "due_balance BIGINT") {
		t.Errorf("unexpected Postgres DDL: %s", postgresDDL)
	}
	t.Log("✓ 5a. Canonical Entity generated multi-dialect SQL DDL (SQLite & Postgres)")

	// 5b. Generate REST API Endpoints
	endpoints := canonicalEnt.GenerateRESTEndpoints()
	if len(endpoints) != 5 {
		t.Errorf("expected 5 REST endpoints, got %d", len(endpoints))
	}
	t.Logf("✓ 5b. Generated %d REST endpoint specifications", len(endpoints))

	// 5c. Generate Client Models (TypeScript & NilLang)
	tsModel := canonicalEnt.GenerateClientModel("typescript")
	if !strings.Contains(tsModel, "export interface Customer") ||
		!strings.Contains(tsModel, "email?: string;") ||
		!strings.Contains(tsModel, "name: string;") {
		t.Errorf("unexpected TS model: %s", tsModel)
	}

	nilModel := canonicalEnt.GenerateClientModel("nillang")
	if !strings.Contains(nilModel, "struct Customer") ||
		!strings.Contains(nilModel, "due_balance: money") {
		t.Errorf("unexpected NilLang struct: %s", nilModel)
	}
	t.Log("✓ 5c. Generated TypeScript interface and NilLang client struct")

	// 5d. Validation Engine
	validRecord := map[string]interface{}{
		"id":           "cust-uuid-1234",
		"name":         "Hasan Mahmud",
		"email":        "hasan@example.com",
		"phone":        "01711223344",
		"due_balance":  int64(25000),
		"total_orders": 3,
	}
	valErrors := canonicalEnt.Validate(validRecord)
	if len(valErrors) > 0 {
		t.Errorf("expected 0 validation errors for valid record, got: %v", valErrors)
	}

	invalidRecord := map[string]interface{}{
		// missing required name and phone
		"email": "invalid-email-format",
	}
	valErrors = canonicalEnt.Validate(invalidRecord)
	if len(valErrors) < 3 {
		t.Errorf("expected at least 3 validation errors for invalid record, got: %v", valErrors)
	}
	t.Logf("✓ 5d. Schema validation successfully validated valid data and caught %d invalid fields", len(valErrors))
}
