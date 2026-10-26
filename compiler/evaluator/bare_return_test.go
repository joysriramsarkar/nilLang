package evaluator

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/object"
)

// TestBareReturnEvaluatesToNull pins the end-to-end meaning of `return` with
// no value: the function yields Null instead of an error or a crash.
func TestBareReturnEvaluatesToNull(t *testing.T) {
	cases := []string{
		`fn f() { return; } f();`,
		`fn f() { if true { return; } return 1; } f();`,
		`fn f() { while false { return 1; } return; } f();`,
	}
	for _, src := range cases {
		evaluated := testEval(src)
		if errorObject, ok := evaluated.(*object.Error); ok {
			t.Fatalf("%q evaluated to an error: %s", src, errorObject.Message)
		}
		if evaluated == nil || evaluated.Type() != object.NULL_OBJ {
			t.Fatalf("%q evaluated to %#v, want Null", src, evaluated)
		}
	}
}
