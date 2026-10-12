package tests

import (
	"strings"
	"testing"
)

// Three bugs that a boolean crossing an engine boundary, or a failing builtin,
// would let through. Each was live in the language until this batch, and each
// had the same shape: the evaluator was right and the VM was not, so only the
// `--vm` path was broken and nothing else in the suite noticed.

// TestBuiltinBooleanIsFalsyInCondition covers truthiness of a value a Go builtin
// produced.
//
// The VM compared booleans by pointer identity against its own `True` and
// `False`. A builtin returning `evaluator.FALSE` is a different pointer, so it
// missed both cases, fell through to "everything else is true", and
// `if (isOk(x)) { ... } else { ... }` always took the first branch.
func TestBuiltinBooleanIsFalsyInCondition(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "false from isOk takes the else",
			source: `let r = Err("x"); let out = if (isOk(r)) { "yes" } else { "no" }; let result = out;`,
			want:   "no",
		},
		{
			name:   "true from isOk takes the then",
			source: `let r = Ok(1); let out = if (isOk(r)) { "yes" } else { "no" }; let result = out;`,
			want:   "yes",
		},
		{
			name:   "false from isErr",
			source: `let r = Ok(1); let out = if (isErr(r)) { "yes" } else { "no" }; let result = out;`,
			want:   "no",
		},
		{
			name:   "false from isNone",
			source: `let o = Some(1); let out = if (isNone(o)) { "yes" } else { "no" }; let result = out;`,
			want:   "no",
		},
		{
			name:   "false from contains",
			source: `let out = if (contains([1, 2], 9)) { "yes" } else { "no" }; let result = out;`,
			want:   "no",
		},
		{
			name:   "false from fileExists",
			source: `let out = if (fileExists("no-such-file-here.txt")) { "yes" } else { "no" }; let result = out;`,
			want:   "no",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := evalSource(t, tc.source)
			if !res.OK {
				t.Fatalf("source was rejected: %s", res.ErrSummary())
			}

			evalOut := mustEvaluatorEval(t, res.Program, "result")
			vmOut := mustVMEval(t, res.Program, "result")

			if evalOut != tc.want {
				t.Errorf("evaluator produced %q, want %q", evalOut, tc.want)
			}
			if vmOut != tc.want {
				t.Errorf("VM produced %q, want %q", vmOut, tc.want)
			}
		})
	}
}

// TestBangOnBuiltinBoolean covers `!` over a value a builtin produced, which had
// the same pointer-identity cause and the opposite symptom: `!isOk(x)` was always
// true.
func TestBangOnBuiltinBoolean(t *testing.T) {
	cases := []struct {
		source string
		want   string
	}{
		{`let r = Err("x"); let result = "\(!isOk(r))";`, "true"},
		{`let r = Ok(1); let result = "\(!isOk(r))";`, "false"},
		{`let r = Ok(1); let result = "\(!isErr(r))";`, "true"},
	}

	for _, tc := range cases {
		res := evalSource(t, tc.source)
		if !res.OK {
			t.Fatalf("source %q was rejected: %s", tc.source, res.ErrSummary())
		}

		for label, out := range map[string]string{
			"evaluator": mustEvaluatorEval(t, res.Program, "result"),
			"VM":        mustVMEval(t, res.Program, "result"),
		} {
			if out != tc.want {
				t.Errorf("%s produced %q for %q, want %q", label, out, tc.source, tc.want)
			}
		}
	}
}

// TestEqualityOfBuiltinBoolean covers `==` and `!=` over the same values, which
// compared the Go interfaces directly and so asked whether two pointers were the
// same pointer rather than whether two booleans held the same value.
func TestEqualityOfBuiltinBoolean(t *testing.T) {
	cases := []struct {
		source string
		want   string
	}{
		{`let r = Err("x"); let result = "\(isOk(r) == false)";`, "true"},
		{`let r = Ok(1); let result = "\(isOk(r) == true)";`, "true"},
		{`let r = Err("x"); let result = "\(isOk(r) != true)";`, "true"},
		{`let r = Ok(1); let result = "\(isOk(r) != false)";`, "true"},
	}

	for _, tc := range cases {
		res := evalSource(t, tc.source)
		if !res.OK {
			t.Fatalf("source %q was rejected: %s", tc.source, res.ErrSummary())
		}

		for label, out := range map[string]string{
			"evaluator": mustEvaluatorEval(t, res.Program, "result"),
			"VM":        mustVMEval(t, res.Program, "result"),
		} {
			if out != tc.want {
				t.Errorf("%s produced %q for %q, want %q", label, out, tc.source, tc.want)
			}
		}
	}
}

// TestFailingBuiltinStopsTheProgram covers the VM halting when a builtin returns
// an error.
//
// The VM pushed the error object and carried on, so `assert(false, "x")` was a
// no-op under `--vm`: the program reached the statement after it and the
// assertion never happened. ERROR_MODEL.md §4 requires both engines to fail on
// the same programs, and this was the largest gap in that contract.
func TestFailingBuiltinStopsTheProgram(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "assert stops",
			source: `assert(false, "custom message");`,
			want:   "custom message",
		},
		{
			name:   "unwrap on Err stops",
			source: `unwrap(Err("nope"));`,
			want:   "unwrap failed on Err: nope",
		},
		{
			name:   "unwrap on None stops",
			source: `unwrap(None());`,
			want:   "unwrap failed on None",
		},
		{
			name:   "unreachable stops",
			source: `unreachable("reached the end");`,
			want:   "reached the end",
		},
		{
			name:   "errValue on Ok stops",
			source: `errValue(Ok(1));`,
			want:   "errValue",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := evalSource(t, tc.source)
			if !res.OK {
				t.Fatalf("source was rejected at compile time: %s", res.ErrSummary())
			}

			for _, engine := range []string{"evaluator", "VM"} {
				err := mustRuntimeError(t, engine, res.Program)
				if err == nil {
					t.Errorf("%s ran the program to completion; it must stop at the failing builtin", engine)
					continue
				}
				if !strings.Contains(err.Error(), tc.want) {
					t.Errorf("%s reported %q, want it to mention %q", engine, err.Error(), tc.want)
				}
			}
		})
	}
}

// TestFailingBuiltinInsideALoopStops confirms the halt is not specific to the
// top level, since a builtin can fail partway through a computation.
func TestFailingBuiltinInsideALoopStops(t *testing.T) {
	res := evalSource(t, `
let total = 0;
for i in 0..10 { total = total + i; }
assert(total == 999, "the sum is not what was expected");
`)
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}

	for _, engine := range []string{"evaluator", "VM"} {
		err := mustRuntimeError(t, engine, res.Program)
		if err == nil {
			t.Errorf("%s ran the program to completion; the assertion should have stopped it", engine)
			continue
		}
		if !strings.Contains(err.Error(), "the sum is not what was expected") {
			t.Errorf("%s reported %q", engine, err.Error())
		}
	}
}
