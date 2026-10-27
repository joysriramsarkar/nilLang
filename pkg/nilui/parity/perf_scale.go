//go:build !race

package parity

// perfBudgetScale widens the frame budget for instrumented runs.
//
// Without the race detector the scale is 1: the 60Hz budget is asserted
// exactly. See perf_scale_race.go for why the race build differs.
func perfBudgetScale() float64 { return 1 }

// perfBudgetEnforced reports whether the wall-clock budget is asserted.
//
// It is true outside `-race`. The race build measures and logs only; see
// perf_scale_race.go.
func perfBudgetEnforced() bool { return true }
