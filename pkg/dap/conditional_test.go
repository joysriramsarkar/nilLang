package dap

import (
	"testing"
)

// TestDAPConditionalBreakpointTrue verifies a true condition still stops.
func TestDAPConditionalBreakpointTrue(t *testing.T) {
	src := "let x = 5;\nlet y = x + 1;\ny;"
	c := dialDAP(t, "cond.nil", src)

	c.send(t, 1, "initialize", nil)
	c.waitForResponse(t, 1)
	c.waitForEvent(t, "initialized")

	c.send(t, 2, "setBreakpoints", map[string]any{
		"source":      map[string]any{"path": "cond.nil"},
		"breakpoints": []any{map[string]any{"line": 2, "condition": "x == 5"}},
	})
	resp, _ := c.waitForResponse(t, 2)
	bps, _ := body(t, resp)["breakpoints"].([]any)
	if len(bps) != 1 || bps[0].(map[string]any)["verified"] != true {
		t.Fatalf("conditional breakpoint not verified: %+v", bps)
	}

	c.send(t, 3, "configurationDone", nil)
	c.waitForResponse(t, 3)
	c.waitForEvent(t, "stopped") // entry

	c.send(t, 4, "continue", nil)
	c.waitForResponse(t, 4)
	stop := c.waitForEvent(t, "stopped")
	stopBody, _ := stop["body"].(map[string]any)
	if stopBody["reason"] != "breakpoint" {
		t.Fatalf("expected breakpoint stop, got %+v", stopBody)
	}

	c.send(t, 5, "disconnect", nil)
	c.waitForResponse(t, 5)
}

// TestDAPConditionalBreakpointFalse verifies a false condition skips the stop
// and the program runs to exit.
func TestDAPConditionalBreakpointFalse(t *testing.T) {
	src := "let x = 5;\nlet y = x + 1;\ny;"
	c := dialDAP(t, "condfalse.nil", src)

	c.send(t, 1, "initialize", nil)
	c.waitForResponse(t, 1)
	c.waitForEvent(t, "initialized")

	c.send(t, 2, "setBreakpoints", map[string]any{
		"source":      map[string]any{"path": "condfalse.nil"},
		"breakpoints": []any{map[string]any{"line": 2, "condition": "x == 999"}},
	})
	c.waitForResponse(t, 2)

	c.send(t, 3, "configurationDone", nil)
	c.waitForResponse(t, 3)
	c.waitForEvent(t, "stopped") // entry

	c.send(t, 4, "continue", nil)
	c.waitForResponse(t, 4)
	// No breakpoint stop must arrive: the run exits instead.
	ev := c.waitForEvent(t, "exited")
	evBody, _ := ev["body"].(map[string]any)
	if int(evBody["exitCode"].(float64)) != 0 {
		t.Fatalf("expected clean exit, got %+v", evBody)
	}

	c.send(t, 5, "disconnect", nil)
	c.waitForResponse(t, 5)
}

// TestDAPUnsupportedConditionRejected checks set-time refusal (L9.3).
func TestDAPUnsupportedConditionRejected(t *testing.T) {
	c := dialDAP(t, "badcond.nil", testProgram)

	c.send(t, 1, "initialize", nil)
	c.waitForResponse(t, 1)
	c.waitForEvent(t, "initialized")

	c.send(t, 2, "setBreakpoints", map[string]any{
		"source":      map[string]any{"path": "badcond.nil"},
		"breakpoints": []any{map[string]any{"line": 1, "condition": "a + b > 2"}},
	})
	resp, _ := c.waitForResponse(t, 2)
	bps, _ := body(t, resp)["breakpoints"].([]any)
	if len(bps) != 1 || bps[0].(map[string]any)["verified"] != false {
		t.Fatalf("unsupported condition must be unverified: %+v", bps)
	}

	c.send(t, 3, "disconnect", nil)
	c.waitForResponse(t, 3)
}

// TestDAPExceptionBreakpoint verifies filter storage and the exception stop.
func TestDAPExceptionBreakpoint(t *testing.T) {
	// A past-the-end index *write* aborts the run with an error (ADR-0049).
	c := dialDAP(t, "exc.nil", "let xs = [1, 2];\nxs[10] = 9;\n")

	c.send(t, 1, "initialize", nil)
	resp, _ := c.waitForResponse(t, 1)
	caps, _ := body(t, resp)["capabilities"].(map[string]any)
	if caps["supportsConditionalBreakpoints"] != true {
		t.Fatalf("capabilities must advertise conditional breakpoints: %+v", caps)
	}
	if _, ok := caps["exceptionBreakpointFilters"]; !ok {
		t.Fatalf("capabilities must advertise exception filters: %+v", caps)
	}
	c.waitForEvent(t, "initialized")

	c.send(t, 2, "launch", map[string]any{"program": "exc.nil"})
	resp, _ = c.waitForResponse(t, 2)
	if resp["success"] != true {
		t.Fatalf("launch failed: %+v", resp)
	}

	c.send(t, 3, "setExceptionBreakpoints", map[string]any{"filters": []any{"all"}})
	resp, _ = c.waitForResponse(t, 3)
	bps, _ := body(t, resp)["breakpoints"].([]any)
	if len(bps) != 1 {
		t.Fatalf("expected 1 exception breakpoint, got %+v", bps)
	}

	c.send(t, 4, "configurationDone", nil)
	c.waitForResponse(t, 4)
	c.waitForEvent(t, "stopped") // entry

	c.send(t, 5, "continue", nil)
	c.waitForResponse(t, 5)
	stop := c.waitForEvent(t, "stopped")
	stopBody, _ := stop["body"].(map[string]any)
	if stopBody["reason"] != "exception" {
		t.Fatalf("expected exception stop, got %+v", stopBody)
	}

	c.send(t, 6, "disconnect", nil)
	c.waitForResponse(t, 6)
}
