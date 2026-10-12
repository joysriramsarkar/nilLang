package tests

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/frontend"
)

// frontendCheck runs the frontend gate with a source file name so diagnostics
// carry spans, and fails the test if the helper itself misbehaves.
func frontendCheck(t *testing.T, source, filename string) frontend.Result {
	t.Helper()
	res := frontend.Check(source, frontend.Options{Filename: filename})
	if res.Program == nil {
		t.Fatalf("frontend gate returned no program for %s", filename)
	}
	return res
}

// evalSource runs the frontend gate and returns a Result whose Program is the
// *lowered* program, which is what an execution backend compiles.
//
// This mirrors the real pipeline: `nil run` and `nil build` both take
// Result.Lowered, because `for` loops and ranges have no bytecode of their own.
// Swapping it in here means every execution test exercises the same path the CLI
// does, so a lowering bug shows up as a wrong value rather than as a confusing
// "internal error: a `for` loop reached the bytecode compiler".
//
// Tests that care about the source as written use frontendCheck instead.
func evalSource(t *testing.T, source string) frontend.Result {
	t.Helper()
	res := frontendCheck(t, source, "determinism.nil")
	if res.Lowered != nil {
		res.Program = res.Lowered
	}
	return res
}
