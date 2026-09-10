package vm

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/code"
	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

func runVm(input string) (object.Object, error) {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()

	comp := compiler.New()
	err := comp.Compile(program)
	if err != nil {
		return nil, err
	}

	vm := New(comp.Bytecode())
	err = vm.Run()
	if err != nil {
		return nil, err
	}

	return vm.LastPoppedStackElem(), nil
}

func TestFloatArithmetic(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"1.5 + 2.25", 3.75},
		{"5 - 0.5", 4.5},
		{"2 * 1.25", 2.5},
		{"5.0 / 2", 2.5},
		{"5.5 % 2", 1.5},
		{"-1.25", -1.25},
	}

	for _, test := range tests {
		result, err := runVm(test.input)
		if err != nil {
			t.Fatalf("runVm failed on %q: %s", test.input, err)
		}
		value, ok := result.(*object.Float)
		if !ok {
			t.Fatalf("object is not Float for %q. got=%T", test.input, result)
		}
		if math.Abs(value.Value-test.expected) > 1e-12 {
			t.Errorf("wrong float value on %q. got=%g, want=%g", test.input, value.Value, test.expected)
		}
	}
}

func TestAppStatementExecution(t *testing.T) {
	result, err := runVm(`
let launches = 0;
app Counter {
	launches = launches + 1;
	let ready = true;
}
if (ready) { launches } else { 0 };
`)
	if err != nil {
		t.Fatalf("runVm failed: %s", err)
	}
	value, ok := result.(*object.Integer)
	if !ok || value.Value != 1 {
		t.Fatalf("expected app result 1, got %T (%+v)", result, result)
	}
}

func TestNamedAppStateExecution(t *testing.T) {
	result, err := runVm(`app Hello { state count: i32 = 0 } count;`)
	if err != nil {
		t.Fatalf("runVm failed: %s", err)
	}
	value, ok := result.(*object.Integer)
	if !ok || value.Value != 0 {
		t.Fatalf("expected state value 0, got %T (%+v)", result, result)
	}
}

func TestDeclarativeComponentExecution(t *testing.T) {
	result, err := runVm(`
component Counter {
	state count: i32 = 0;
	render { return {"type": "Text", "value": count}; }
	on click { count = count + 1; }
}
Counter.events.click();
Counter.render()["value"];
`)
	if err != nil {
		t.Fatalf("runVm failed: %s", err)
	}
	value, ok := result.(*object.Integer)
	if !ok || value.Value != 1 {
		t.Fatalf("expected rendered count 1, got %T (%+v)", result, result)
	}
}

func TestDeclarativeComponentEventPayload(t *testing.T) {
	result, err := runVm(`
component Form {
	state name = "";
	on submit(payload) { name = payload["name"]; }
}
Form.events.submit({"name": "Ada"});
name;
`)
	if err != nil {
		t.Fatalf("runVm failed: %s", err)
	}
	value, ok := result.(*object.String)
	if !ok || value.Value != "Ada" {
		t.Fatalf("expected payload name Ada, got %T (%+v)", result, result)
	}
}

func TestTaskAwait(t *testing.T) {
	result, err := runVm(`let base = 40; let work = task { base + 2; }; await work;`)
	if err != nil {
		t.Fatalf("runVm failed: %s", err)
	}
	value, ok := result.(*object.Integer)
	if !ok || value.Value != 42 {
		t.Fatalf("expected task result 42, got %T (%+v)", result, result)
	}
}

func TestTaskChannelCommunication(t *testing.T) {
	result, err := runVm(`
let messages = Channel(1);
let producer = task { send(messages, 42); };
let value = receive(messages);
await producer;
value;
`)
	if err != nil {
		t.Fatalf("runVm failed: %s", err)
	}
	value, ok := result.(*object.Integer)
	if !ok || value.Value != 42 {
		t.Fatalf("expected channel value 42, got %T (%+v)", result, result)
	}
}

func TestPopClearsStackRoot(t *testing.T) {
	machine := New(&compiler.Bytecode{})
	value := &object.Array{Elements: []object.Object{&object.Integer{Value: 1}}}
	if err := machine.push(value); err != nil {
		t.Fatal(err)
	}
	if popped := machine.pop(); popped != value {
		t.Fatalf("pop returned %T, want original array", popped)
	}
	if machine.stack[0] != nil {
		t.Fatalf("popped stack slot still retains %T", machine.stack[0])
	}
}

func TestTruncateStackClearsAllVacatedRoots(t *testing.T) {
	machine := New(&compiler.Bytecode{})
	for index := 0; index < 3; index++ {
		if err := machine.push(&object.Integer{Value: int64(index)}); err != nil {
			t.Fatal(err)
		}
	}
	machine.truncateStack(1)
	if machine.sp != 1 {
		t.Fatalf("stack pointer = %d, want 1", machine.sp)
	}
	if machine.stack[0] == nil || machine.stack[1] != nil || machine.stack[2] != nil {
		t.Fatalf("unexpected roots after truncation: %v", machine.stack[:3])
	}
}

func TestPopFrameClearsFrameRoot(t *testing.T) {
	machine := New(&compiler.Bytecode{})
	frame := NewFrame(&object.Closure{Fn: &object.CompiledFunction{}}, 0)
	machine.pushFrame(frame)
	if popped := machine.popFrame(); popped != frame {
		t.Fatal("popFrame did not return the pushed frame")
	}
	if machine.frames[1] != nil {
		t.Fatal("popped frame slot still retains its closure")
	}
}

func TestFloatComparisons(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"1.5 < 2", true},
		{"2 >= 2.0", true},
		{"3.0 == 3", true},
		{"3.0 != 3", false},
	}
	for _, test := range tests {
		result, err := runVm(test.input)
		if err != nil {
			t.Fatalf("runVm failed on %q: %s", test.input, err)
		}
		value, ok := result.(*object.Boolean)
		if !ok || value.Value != test.expected {
			t.Errorf("wrong comparison on %q. got=%v, want=%t", test.input, result, test.expected)
		}
	}
}

func TestIntegerArithmetic(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"1", 1},
		{"2", 2},
		{"1 + 2", 3},
		{"1 - 2", -1},
		{"1 * 2", 2},
		{"4 / 2", 2},
		{"50 / 2 * 2 + 10 - 5", 55},
		{"5 + 5 + 5 + 5 - 10", 10},
		{"2 * 2 * 2 * 2 * 2", 32},
		{"5 * 2 + 10", 20},
		{"5 + 2 * 10", 25},
		{"5 * (2 + 10)", 60},
	}

	for _, tt := range tests {
		stackElem, err := runVm(tt.input)
		if err != nil {
			t.Fatalf("runVm failed on %q: %s", tt.input, err)
		}

		result, ok := stackElem.(*object.Integer)
		if !ok {
			t.Fatalf("object is not Integer. got=%T (%+v)", stackElem, stackElem)
		}

		if result.Value != tt.expected {
			t.Errorf("wrong integer value on %q. got=%d, want=%d",
				tt.input, result.Value, tt.expected)
		}
	}
}

func TestBooleanExpressions(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"false", false},
		{"1 < 2", true},
		{"1 > 2", false},
		{"1 < 1", false},
		{"1 > 1", false},
		{"1 == 1", true},
		{"1 != 1", false},
		{"1 == 2", false},
		{"1 != 2", true},
		{"true == true", true},
		{"false == false", true},
		{"true == false", false},
		{"true != false", true},
		{"false != true", true},
		{"!true", false},
		{"!false", true},
		{"!5", false},
		{"!!true", true},
		{"!!false", false},
		{"!!5", true},
	}

	for _, tt := range tests {
		stackElem, err := runVm(tt.input)
		if err != nil {
			t.Fatalf("runVm failed on %q: %s", tt.input, err)
		}

		result, ok := stackElem.(*object.Boolean)
		if !ok {
			t.Fatalf("object is not Boolean. got=%T (%+v)", stackElem, stackElem)
		}

		if result.Value != tt.expected {
			t.Errorf("wrong boolean value on %q. got=%t, want=%t",
				tt.input, result.Value, tt.expected)
		}
	}
}

func TestTensorBuiltins(t *testing.T) {
	input := `
let positions = tensor([1, 2, 3, 4, 5, 6], [2, 3]);
let translation = tensor([10, 20, 30], [3]);
let active = tensorSlice(positions, [1, 0], [2, 3]);
let moved = tensorMul(tensorAdd(active, translation), tensor([2, 3, 4], [3]));
tensorGet(moved, [0, 2]) + tensorSum(tensorAdd(tensor([1, 2], [2]), tensor([3, 4], [2])));
`
	result, err := runVm(input)
	if err != nil {
		t.Fatal(err)
	}
	value, ok := result.(*object.Float)
	if !ok {
		t.Fatalf("expected float tensor result, got=%T (%+v)", result, result)
	}
	if value.Value != 154 {
		t.Fatalf("tensor result=%g, want 154", value.Value)
	}
}

func TestTensorDtypeBuiltins(t *testing.T) {
	input := `
let left = tensor([1.25, 2.5], [2], "float32");
let right = tensor([2, 3], [2], "int32");
let output = tensorMul(left, right);
tensorDtype(output) + ":" + str(tensorSum(output));
`
	result, err := runVm(input)
	if err != nil {
		t.Fatal(err)
	}
	value, ok := result.(*object.String)
	if !ok || value.Value != "float32:10" {
		t.Fatalf("dtype result=%T (%+v)", result, result)
	}
}

func TestConditionals(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"if (true) { 10 }", 10},
		{"if (true) { 10 } else { 20 }", 10},
		{"if (false) { 10 } else { 20 }", 20},
		{"if (1 < 2) { 10 }", 10},
		{"if (1 < 2) { 10 } else { 20 }", 10},
		{"if (1 > 2) { 10 } else { 20 }", 20},
	}

	for _, tt := range tests {
		stackElem, err := runVm(tt.input)
		if err != nil {
			t.Fatalf("runVm failed on %q: %s", tt.input, err)
		}

		result, ok := stackElem.(*object.Integer)
		if !ok {
			t.Fatalf("object is not Integer. got=%T (%+v)", stackElem, stackElem)
		}

		if result.Value != tt.expected {
			t.Errorf("wrong integer value on %q. got=%d, want=%d",
				tt.input, result.Value, tt.expected)
		}
	}
}

func TestGlobalLetStatements(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"let one = 1; one", 1},
		{"let one = 1; let two = 2; one + two", 3},
		{"let one = 1; let two = one + one; one + two", 3},
	}

	for _, tt := range tests {
		stackElem, err := runVm(tt.input)
		if err != nil {
			t.Fatalf("runVm failed on %q: %s", tt.input, err)
		}

		result, ok := stackElem.(*object.Integer)
		if !ok {
			t.Fatalf("object is not Integer. got=%T (%+v)", stackElem, stackElem)
		}

		if result.Value != tt.expected {
			t.Errorf("wrong integer value on %q. got=%d, want=%d",
				tt.input, result.Value, tt.expected)
		}
	}
}

func TestCapabilitySecurityRejection(t *testing.T) {
	// Emit OpNativeCall for "camera.capture"
	ins := code.Make(code.OpNativeCall, 0, 0)
	constants := []object.Object{&object.String{Value: "camera.capture"}}

	bytecode := &compiler.Bytecode{
		Instructions: ins,
		Constants:    constants,
	}

	machine := New(bytecode)
	machine.CapabilityChecker = func(apiName string) error {
		return fmt.Errorf("camera capability not granted in manifest")
	}

	err := machine.Run()
	if err == nil || !strings.Contains(err.Error(), "capability denied") {
		t.Fatalf("expected capability denied error, got: %v", err)
	}
}

