//go:build race

package parity

// perfBudgetScale widens the frame budget for instrumented runs.
//
// A wall-clock budget is meaningless under `-race`: the detector instruments
// every memory access (40x+ on this rasterizer's pixel loops on a loaded CI
// machine, far beyond this scale factor) and CI shares its cores, so the
// same frames that take ~4ms natively take ~200ms instrumented. Asserting a
// scaled budget there fails a fast renderer for being observed slowly, and
// any fixed scale flakes on a sufficiently loaded machine (observed: p95
// 201ms against a 10x budget on ordinary hardware).
//
// The scale is kept for the report line, but the budget is no longer
// asserted on race builds: the frames still all run, the raw numbers are
// still logged, and a functional regression still fails the test. The strict
// budget is asserted by every non-race run — including CI on Windows, which
// runs without `-race`.
func perfBudgetScale() float64 { return 10 }

// perfBudgetEnforced reports whether the wall-clock budget is asserted.
//
// Always false under `-race`: see above.
func perfBudgetEnforced() bool { return false }
