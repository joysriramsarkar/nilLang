package typecheck

import (
	"testing"
)

// TestMatchExhaustivenessIgnoresGuardedArms pins the rule that a guard is
// not coverage. A guarded arm fires only when its condition holds, so
// counting it — as the checker once did, in both coveredCases and
// hasWildcardArm — let matches through that leave a value with no arm at
// all: `Ok(0)` against `Ok(x) if x > 0 => ..., Err(e) => ...`.
func TestMatchExhaustivenessIgnoresGuardedArms(t *testing.T) {
	cases := []struct {
		name     string
		source   string
		wantCode string // "" means the program must type-check cleanly
	}{
		{
			name:     "guarded variant arm does not cover its case",
			source:   `let r0 = Ok(1); let r = match r0 { Ok(v) if v > 0 => 1, Err(e) => 2 };`,
			wantCode: CodeNonExhaustive,
		},
		{
			name:     "guarded wildcard is not a catch-all",
			source:   `let r0 = Ok(1); let r = match r0 { Ok(v) => 1, _ if true => 2 };`,
			wantCode: CodeNonExhaustive,
		},
		{
			name:     "guarded literal arm does not cover a bool case",
			source:   `let b = true; let r = match b { true if b => 1, false => 0 };`,
			wantCode: CodeNonExhaustive,
		},
		{
			name:     "guarded enum arm does not cover its variant",
			source:   `enum E { A, B } let v = A; let r = match v { A if true => 1, B => 2 };`,
			wantCode: CodeNonExhaustive,
		},
		{
			name:   "unguarded full match still passes",
			source: `let r0 = Ok(1); let r = match r0 { Ok(v) => 1, Err(e) => 2 };`,
		},
		{
			name:   "guarded arm beside an unguarded wildcard still passes",
			source: `let r0 = Ok(1); let r = match r0 { Ok(v) if v > 0 => 1, _ => 0 };`,
		},
		{
			name:   "guarded arm beside full unguarded coverage still passes",
			source: `let r0 = Ok(1); let r = match r0 { Ok(v) if v > 0 => 1, Ok(v) => 2, Err(e) => 3 };`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := parseProgram(t, tc.source)
			prog := p.ParseProgram()
			if len(p.Errors()) > 0 {
				t.Fatalf("parse errors: %v", p.Errors())
			}

			checker := NewChecker()
			valid := checker.CheckProgram(prog)

			if tc.wantCode == "" {
				if !valid {
					t.Fatalf("expected a clean check, got diagnostics: %v", checker.Diagnostics)
				}
				return
			}
			for _, d := range checker.Diagnostics {
				if d.Code == tc.wantCode {
					return
				}
			}
			t.Fatalf("expected %s, got valid=%v diagnostics=%v", tc.wantCode, valid, checker.Diagnostics)
		})
	}
}
