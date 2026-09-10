package evaluator

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

func TestTaskAwait(t *testing.T) {
	evaluated := testEval(`let base = 40; let work = task { base + 2; }; await work;`)
	testIntegerObject(t, evaluated, 42)
}

func TestAwaitRejectsNonFuture(t *testing.T) {
	evaluated := testEval(`await 42;`)
	errorObject, ok := evaluated.(*object.Error)
	if !ok || errorObject.Message != "cannot await non-future value: INTEGER" {
		t.Fatalf("unexpected await result: %T (%+v)", evaluated, evaluated)
	}
}

func TestAssignmentRejectsUndefinedVariable(t *testing.T) {
	l := lexer.New(`missing = 1;`)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	env := object.NewEnvironment()

	evaluated := Eval(program, env)
	errorObject, ok := evaluated.(*object.Error)
	if !ok || errorObject.Message != "identifier not found: missing" {
		t.Fatalf("unexpected assignment result: %T (%+v)", evaluated, evaluated)
	}
	if _, exists := env.Get("missing"); exists {
		t.Fatal("undefined assignment must not create a binding")
	}
}

func TestTaskChannelCommunication(t *testing.T) {
	evaluated := testEval(`
let messages = Channel(1);
let producer = task { messages.send(42); };
let value = messages.receive();
await producer;
value;
`)
	testIntegerObject(t, evaluated, 42)
}

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

func TestTensorBuiltins(t *testing.T) {
	input := `
let features = tensor([1, 2, 3, 4, 5, 6], [2, 3]);
let weights = tensor([7, 8, 9, 10, 11, 12], [3, 2]);
let batch = tensorMul(tensorSlice(features, [1, 0], [2, 3]), tensor([1, 2, 1], [3]));
let output = tensorAdd(tensorMatmul(batch, weights), tensor([0.5, -0.5], [2]));
tensorGet(output, [0, 1]) + tensorDot(tensor([1, 2, 3], [3]), tensor([4, 5, 6], [3]));
`
	result := testEval(input)
	value, ok := result.(*object.Float)
	if !ok {
		t.Fatalf("expected float tensor result, got=%T (%+v)", result, result)
	}
	if value.Value != 235.5 {
		t.Fatalf("tensor result=%g, want 235.5", value.Value)
	}
}

func TestTensorDtypeBuiltins(t *testing.T) {
	input := `
let values = tensor([16777216], [1], "float32");
let incremented = tensorAdd(values, tensor([1], [1], "float32"));
let promoted = tensorAdd(incremented, tensorCast(tensor([2], [1], "int32"), "float64"));
tensorDtype(incremented) + ":" + tensorDtype(promoted) + ":" + str(tensorGet(incremented, [0]));
`
	result := testEval(input)
	value, ok := result.(*object.String)
	if !ok || value.Value != "float32:float64:1.6777216e+07" {
		t.Fatalf("dtype result=%T (%+v)", result, result)
	}
}

func TestAppStatementEvaluation(t *testing.T) {
	result := testEval(`
let launches = 0;
app {
	launches = launches + 1;
	let ready = true;
}
if (ready) { launches } else { 0 };
`)
	testIntegerObject(t, result, 1)
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

func TestDeclarativeComponentRenderAndEvents(t *testing.T) {
	result := testEval(`
component Counter {
	state count: i32 = 0;
	render { return {"type": "Text", "value": count}; }
	on click {
		count = count + 1;
		emit("changed", count);
	}
}
Counter.dispatch("click");
let tree = Counter.render();
tree["value"] + Counter.state["count"] + Counter.lastEvent["payload"];
`)
	testIntegerObject(t, result, 3)
}

func TestDeclarativeComponentEventPayload(t *testing.T) {
	result := testEval(`
component Form {
	state name = "";
	on submit(payload) { name = payload["name"]; }
}
Form.dispatch("submit", {"name": "Ada"});
Form.state["name"];
`)
	name, ok := result.(*object.String)
	if !ok || name.Value != "Ada" {
		t.Fatalf("expected payload name Ada, got %T (%+v)", result, result)
	}

	result = testEval(`
component Optional {
	state missing = false;
	on submit(payload) { missing = payload == null; }
}
Optional.dispatch("submit");
Optional.state["missing"];
`)
	missing, ok := result.(*object.Boolean)
	if !ok || !missing.Value {
		t.Fatalf("expected omitted payload to bind null, got %T (%+v)", result, result)
	}
}
