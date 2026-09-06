package evaluator

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

func testEval(input string) object.Object {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	env := object.NewEnvironment()

	for name, builtin := range Builtins {
		env.Set(name, builtin)
	}

	return Eval(program, env)
}

func TestEvalIntegerExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"5", 5},
		{"10", 10},
		{"-5", -5},
		{"-10", -10},
		{"5 + 5 + 5 + 5 - 10", 10},
		{"2 * 2 * 2 * 2 * 2", 32},
		{"-50 + 100 + -50", 0},
		{"5 * 2 + 10", 20},
		{"5 + 2 * 10", 25},
		{"20 + 2 * -10", 0},
		{"50 / 2 * 2 + 10", 60},
		{"2 * (5 + 10)", 30},
		{"3 * 3 * 3 + 10", 37},
		{"3 * (3 * 3) + 10", 37},
		{"(5 + 10 * 2 + 15 / 3) * 2 + -10", 50},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testIntegerObject(t, evaluated, tt.expected)
	}
}

func TestWhileLoop(t *testing.T) {
	input := `
let x = 0;
let sum = 0;
while (x < 5) {
	let sum = sum + x;
	let x = x + 1;
}
sum;
`
	evaluated := testEval(input)
	testIntegerObject(t, evaluated, 10)
}

func TestFunctions(t *testing.T) {
	input := `
let add = fn(a, b) { a + b; };
add(5, 7);
`
	evaluated := testEval(input)
	testIntegerObject(t, evaluated, 12)
}

func TestStringInterpolation(t *testing.T) {
	input := `
let name = "Onuron";
"Hello, \(name)!";
`
	evaluated := testEval(input)
	str, ok := evaluated.(*object.String)
	if !ok {
		t.Fatalf("object is not String. got=%T (%+v)", evaluated, evaluated)
	}
	if str.Value != "Hello, Onuron!" {
		t.Errorf("String has wrong value. got=%q", str.Value)
	}
}

func testIntegerObject(t *testing.T, obj object.Object, expected int64) bool {
	result, ok := obj.(*object.Integer)
	if !ok {
		t.Errorf("object is not Integer. got=%T (%+v)", obj, obj)
		return false
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%d, want=%d",
			result.Value, expected)
		return false
	}
	return true
}

func TestImportNativeModules(t *testing.T) {
	// Test money module import and calculation
	moneyInput := `
import "money";
let price = money.ofMinor(1250, "BDT");
let qty = 2;
let total = money.add(price, price);
total["formatted"];
`
	mEval := testEval(moneyInput)
	str, ok := mEval.(*object.String)
	if !ok {
		t.Fatalf("expected string from money format, got=%T (%+v)", mEval, mEval)
	}
	if str.Value != "৳25.00" {
		t.Errorf("expected ৳25.00, got=%s", str.Value)
	}

	// Test web module import and app setup
	webInput := `
import "web";
let app = web.new("pos-app");
app.get("/products", fn(req) { return "ok"; });
let r = app.routes();
len(r);
`
	wEval := testEval(webInput)
	testIntegerObject(t, wEval, 1)

	// Test data module import and table operations
	dataInput := `
import "data";
let users = data.table("users");
users.insert({"name": "Alice", "role": "admin"});
users.count();
`
	dEval := testEval(dataInput)
	testIntegerObject(t, dEval, 1)
}

func TestComponentEvaluation(t *testing.T) {
	compInput := `
component Counter {
	state count = 0;
}
Counter["name"];
`
	cEval := testEval(compInput)
	str, ok := cEval.(*object.String)
	if !ok || str.Value != "Counter" {
		t.Fatalf("expected component name 'Counter', got=%v", cEval)
	}
}
