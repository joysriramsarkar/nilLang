package parser

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
)

func TestEventHandlerPayloadParameter(t *testing.T) {
	parser := New(lexer.New(`component Form { on submit(payload) { payload; } on reset() {} on legacy {} }`))
	program := parser.ParseProgram()
	checkParserErrors(t, parser)

	component := program.Statements[0].(*ast.ComponentLiteral)
	if len(component.Handlers) != 3 {
		t.Fatalf("handlers = %d, want 3", len(component.Handlers))
	}
	if got := component.Handlers[0].Parameters; len(got) != 1 || got[0].Value != "payload" {
		t.Fatalf("submit parameters = %+v, want payload", got)
	}
	if len(component.Handlers[1].Parameters) != 0 || len(component.Handlers[2].Parameters) != 0 {
		t.Fatal("zero-parameter event forms did not remain empty")
	}
}

func TestEventHandlerRejectsMultiplePayloadParameters(t *testing.T) {
	parser := New(lexer.New(`component Form { on submit(first, second) {} }`))
	parser.ParseProgram()
	if len(parser.Errors()) == 0 {
		t.Fatal("expected parser error for multiple event payload parameters")
	}
}