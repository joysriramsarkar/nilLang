package main

import "testing"

func TestCheckExitCode(t *testing.T) {
	if got := checkExitCode(nil, nil, false); got != 0 {
		t.Errorf("clean run must exit 0, got %d", got)
	}
	if got := checkExitCode(nil, nil, true); got != 0 {
		t.Errorf("clean CI run must exit 0, got %d", got)
	}
	if got := checkExitCode([]string{"e"}, nil, false); got != 1 {
		t.Errorf("errors must fail, got %d", got)
	}
	// L9.4 gate: warnings pass locally, fail in CI.
	if got := checkExitCode(nil, []string{"w"}, false); got != 0 {
		t.Errorf("warnings must pass locally, got %d", got)
	}
	if got := checkExitCode(nil, []string{"w"}, true); got != 1 {
		t.Errorf("warnings must fail in --ci, got %d", got)
	}
}
