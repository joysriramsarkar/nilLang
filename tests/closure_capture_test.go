package tests

import (
	"testing"
)

// TestMutableCaptureAgreesAcrossEngines covers what SEMANTICS.md §3.2 calls
// "mutable by-reference capture".
//
// This was documented as "implemented by the evaluator but planned for the Stack
// VM", which made it a real divergence: the same counter program returned 1 from
// the evaluator and 0 from the VM, because the VM boxed a captured variable into
// a cell the enclosing frame never looked at again.
func TestMutableCaptureAgreesAcrossEngines(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "counter increments are visible to the parent",
			source: `
fn makeCounter() {
  let count = 0;
  let inc = fn() { count = count + 1; return count; };
  inc();
  inc();
  inc();
  return count;
}
let result = makeCounter();
`,
			want: "3",
		},
		{
			name: "returned closure keeps state between calls",
			source: `
fn makeCounter() {
  let count = 0;
  return fn() { count = count + 1; return count; };
}
let counter = makeCounter();
counter();
counter();
let result = counter();
`,
			want: "3",
		},
		{
			// Two closures sharing one variable: the point of a shared cell.
			name: "two closures observe the same variable",
			source: `
fn makePair() {
  let shared = 0;
  let bump = fn() { shared = shared + 1; return shared; };
  let read = fn() { return shared; };
  bump();
  bump();
  return read();
}
let result = makePair();
`,
			want: "2",
		},
		{
			name: "capture from an enclosing function through a middle frame",
			source: `
fn outer() {
  let deep = 10;
  fn middle() {
    fn inner() { deep = deep + 5; return deep; }
    return inner();
  }
  middle();
  return deep;
}
let result = outer();
`,
			want: "15",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := evalSource(t, tc.source)
			if !res.OK {
				t.Fatalf("source was rejected: %s", res.ErrSummary())
			}

			evalOut, err := evaluatorEval(res.Program, "result")
			if err != nil {
				t.Fatalf("evaluator: %v", err)
			}
			if evalOut != tc.want {
				t.Errorf("evaluator produced %q, want %q", evalOut, tc.want)
			}

			vmOut, err := vmEval(res.Program, "result")
			if err != nil {
				t.Fatalf("VM: %v", err)
			}
			if vmOut != tc.want {
				t.Errorf("VM produced %q, want %q", vmOut, tc.want)
			}
		})
	}
}

// TestReadOnlyCaptureStillWorks guards against the boxing change breaking the
// ordinary case. Boxing only matters for mutation; a closure that merely reads
// must keep working.
func TestReadOnlyCaptureStillWorks(t *testing.T) {
	source := `
fn makeAdder(n) {
  return fn(x) { return x + n; };
}
let add5 = makeAdder(5);
let result = add5(10);
`
	res := evalSource(t, source)
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}

	for label, out := range map[string]string{
		"evaluator": mustEvaluatorEval(t, res.Program, "result"),
		"VM":        mustVMEval(t, res.Program, "result"),
	} {
		if out != "15" {
			t.Errorf("%s produced %q, want 15", label, out)
		}
	}
}

// TestCapturedLoopVariableIsPerInvocation checks that each call gets its own
// captured state rather than sharing one cell across invocations.
func TestCapturedLoopVariableIsPerInvocation(t *testing.T) {
	source := `
fn makeStepper() {
  let step = 0;
  return fn() { step = step + 1; return step; };
}
let a = makeStepper();
let b = makeStepper();
a();
a();
let result = b();
`
	res := evalSource(t, source)
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}

	for label, out := range map[string]string{
		"evaluator": mustEvaluatorEval(t, res.Program, "result"),
		"VM":        mustVMEval(t, res.Program, "result"),
	} {
		if out != "1" {
			t.Errorf("%s produced %q, want 1; each call must capture its own state", label, out)
		}
	}
}

// TestBoxingDoesNotLeakIntoUncapturedLocals guards the main risk of boxing in
// place: a local that no closure captures must behave exactly as before.
func TestBoxingDoesNotLeakIntoUncapturedLocals(t *testing.T) {
	source := `
fn plain() {
  let a = 1;
  let b = 2;
  a = a + b;
  return a * b;
}
let result = plain();
`
	res := evalSource(t, source)
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}

	for label, out := range map[string]string{
		"evaluator": mustEvaluatorEval(t, res.Program, "result"),
		"VM":        mustVMEval(t, res.Program, "result"),
	} {
		if out != "6" {
			t.Errorf("%s produced %q, want 6", label, out)
		}
	}
}

// TestNestedFunctionValueCapture checks that a captured function value, not just
// a captured variable, survives.
func TestNestedFunctionValueCapture(t *testing.T) {
	source := `
fn outer() {
  let helper = fn(x) { return x * 2; };
  return fn(x) { return helper(x) + 1; };
}
let doublePlusOne = outer();
let result = doublePlusOne(5);
`
	res := evalSource(t, source)
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}

	for label, out := range map[string]string{
		"evaluator": mustEvaluatorEval(t, res.Program, "result"),
		"VM":        mustVMEval(t, res.Program, "result"),
	} {
		if out != "11" {
			t.Errorf("%s produced %q, want 11", label, out)
		}
	}
}

// TestBoxingIsIdempotent checks that boxing an already boxed local does not
// replace the cell, which would split the two views of a shared variable.
func TestBoxingIsIdempotent(t *testing.T) {
	source := `
fn shared() {
  let value = 1;
  let read1 = fn() { return value; };
  let bump = fn() { value = value + 1; return value; };
  let read2 = fn() { return value; };
  bump();
  let a = read1();
  let b = read2();
  return a + b;
}
let result = shared();
`
	res := evalSource(t, source)
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}

	for label, out := range map[string]string{
		"evaluator": mustEvaluatorEval(t, res.Program, "result"),
		"VM":        mustVMEval(t, res.Program, "result"),
	} {
		if out != "4" {
			t.Errorf("%s produced %q, want 4; both readers must see the same cell", label, out)
		}
	}
}
