//go:build race

package parity

// perfBudgetScale widens the frame budget for instrumented runs.
//
// A wall-clock budget is meaningless under `-race`: the detector instruments
// every memory access (typically 5–10x on this rasterizer's pixel loops) and
// CI shares its cores, so the same frames that take ~4ms natively take
// ~48ms instrumented. Asserting 16.7ms there would fail a fast renderer for
// being observed slowly.
//
// The scale is 10, not infinity: the frames still all run, the raw numbers
// are still logged, and a genuine 10x regression still trips the gate. The
// strict budget is asserted by every non-race run — including CI on Windows,
// which runs without `-race`.
func perfBudgetScale() float64 { return 10 }
