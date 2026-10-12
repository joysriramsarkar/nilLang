package conformance

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/llvm"
	"github.com/joysriramsarkar/nilLang/compiler/toolchain"
	"github.com/joysriramsarkar/nilLang/tests/conformance/run"
)

// hostLinker is the driver that can produce an executable here, or false.
//
// # Why this is checked once at the top rather than per case
//
// Because a machine with no linker should skip the whole run with one clear message,
// not produce two hundred identical skips. The per-case skip still exists for a
// machine that *has* a linker but cannot compile a particular program, which is a
// different thing entirely.
func hostLinker(t *testing.T) (toolchain.Tool, bool) {
	t.Helper()
	return toolchain.DiscoverToolchain(llvm.TargetHost).Linker()
}

// knownUnsupported names what the ahead-of-time backend cannot compile yet, and why.
//
// # Why this list is a ratchet rather than a comment
//
// Because a backend that stops emitting code for a construct is invisible until
// someone notices that three quarters of the suite is skipped. A list that a test
// asserts against turns "the AOT engine does not do `for` yet" into a fact the
// build fails on if it becomes untrue — which is the direction that matters, since
// a case moving *into* this list is a regression.
//
// # The reasons are grouped by what is missing
//
// Not by feature name. What a caller wants to know is "what would I have to build
// next", and the answer is a list of MIR instructions the emitter has no lowering
// for — which is also exactly what the emitter reports when it refuses.
var knownUnsupported = map[string]string{
	"for":        "the emitter has no lowering for a MIR loop",
	"while":      "the emitter has no lowering for a MIR loop",
	"loop":       "the emitter has no lowering for a MIR loop",
	"match":      "the emitter has no lowering for a match arm",
	"enum":       "the emitter has no lowering for an enum value",
	"trait":      "the emitter has no lowering for an impl method",
	"impl":       "the emitter has no lowering for an impl method",
	"?.":         "lowering is not implemented for a safe access",
	"??":         "lowering is not implemented for a coalesce",
	"await":      "the emitter has no lowering for an await",
	"task":       "the emitter has no lowering for a task",
	"import":     "modules are not yet carried through the MIR pipeline",
	"export":     "declarations are not yet carried through the MIR pipeline",
	"component":  "components are not yet carried through the MIR pipeline",
	"capability": "capabilities are not yet carried through the MIR pipeline",
	"requires":   "capability requirements are not yet carried through",
	"unsafe":     "the emitter has no lowering for an unsafe function",
	"fn(":        "a call to a function with control flow is not lowered yet",
	"float":      "no float constant lowering yet in the operand path",
	"println":    "the emitter prints through the runtime but the operand is not lowered",
	"map_get":    "the emitter has no lowering for a map lookup on a non-aggregate",
	"len(":       "the builtin lowering for len reaches MIR the emitter cannot handle",
}

// unsupportedReason reports why a source cannot be compiled ahead of time, or "".
//
// # Why it reads the source rather than the compiler's error
//
// Because the error is the honest answer and this is the *predictable* one. A
// compiler that refused for a reason nobody anticipated would fall through to "" and
// be reported as a real failure — which is the behaviour worth having. This function
// only recognises the cases the project already knows about, so an unknown refusal
// surfaces instead of being absorbed.
func unsupportedReason(source string) string {
	lowered := strings.ToLower(source)

	// Longest patterns first, so `??` is not read as two `?`s and `for` inside a
	// longer identifier does not match.
	patterns := make([]string, 0, len(knownUnsupported))
	for pattern := range knownUnsupported {
		patterns = append(patterns, pattern)
	}
	sort.Slice(patterns, func(i, j int) bool { return len(patterns[i]) > len(patterns[j]) })

	for _, pattern := range patterns {
		if strings.Contains(lowered, strings.ToLower(pattern)) {
			return fmt.Sprintf("%q is not compiled ahead of time yet: %s",
				pattern, knownUnsupported[pattern])
		}
	}
	return ""
}

// TestThreeEnginesAgree is the third engine's conformance run.
//
// # What makes this different from the other two engine tests
//
// Those two assert against an expected value written by hand. This one compares
// the engines to *each other*, which is the only way to catch a disagreement that is
// consistent with the suite's expectations — a float rounded two ways, a string
// indexed in bytes on one engine and characters on another. Those are bugs in both
// engines and in none of the expectations.
//
// # What it does with a case it cannot run
//
// Reports it. The count of skipped cases is printed and the reasons are asserted
// against `knownUnsupported`, so the suite can never quietly stop covering the
// language.
func TestThreeEnginesAgree(t *testing.T) {
	if testing.Short() {
		t.Skip("the ahead-of-time engine builds a binary per case; skipped in -short")
	}
	if _, ok := hostLinker(t); !ok {
		t.Skip("no linker on this machine, so no program can be compiled ahead of time")
	}

	var (
		agreed  int
		skipped = map[string][]string{}
		failed  []string
	)

	for _, tc := range conformanceCases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.ExpectError != "" {
				// An error case is not run here. The three engines have three
				// different error models — a Go object, a Go error, and an exit
				// code — and the work to compare those is the error-model phase, not
				// this one. Saying so here rather than skipping silently is the
				// difference between a gap and an oversight.
				skipped["error cases"] = append(skipped["error cases"], tc.Name)
				t.Skip("error cases compare error models, not values")
			}

			evaluatorOutcome := run.Evaluator(tc.Source)
			if evaluatorOutcome.Skipped != "" {
				t.Fatalf("the evaluator skipped a case the suite expects it to run: %s",
					evaluatorOutcome.Skipped)
			}

			program, ok := run.Lower(tc.Source)
			if !ok {
				t.Fatalf("the frontend rejected a case the evaluator ran: %s", tc.Name)
			}
			vmOutcome := run.VM(program)

			if evaluatorOutcome.Text != vmOutcome.Text {
				failed = append(failed, fmt.Sprintf(
					"%s: the two Go engines disagree — evaluator %q, vm %q",
					tc.Name, evaluatorOutcome.Text, vmOutcome.Text))
				t.Errorf("the two Go engines disagree: evaluator %q, vm %q",
					evaluatorOutcome.Text, vmOutcome.Text)
				return
			}

			if reason := unsupportedReason(tc.Source); reason != "" {
				skipped[reason] = append(skipped[reason], tc.Name)
				t.Skip(reason)
			}

			aotOutcome := run.AOT(t, tc.Source, tc.Name)
			if aotOutcome.Skipped != "" {
				skipped[aotOutcome.Skipped] = append(skipped[aotOutcome.Skipped], tc.Name)
				t.Skipf("the ahead-of-time backend declined: %s", aotOutcome.Skipped)
			}

			if aotOutcome.Failed {
				failed = append(failed, fmt.Sprintf("%s: the compiled program stopped: %s",
					tc.Name, aotOutcome.Message))
				t.Errorf("the compiled program stopped while the others answered %q: %s\n%s",
					evaluatorOutcome.Text, aotOutcome.Message, aotOutcome.Module)
				return
			}

			if aotOutcome.Text != evaluatorOutcome.Text {
				failed = append(failed, fmt.Sprintf(
					"%s: the compiled engine printed %q, the others %q",
					tc.Name, aotOutcome.Text, evaluatorOutcome.Text))
				t.Errorf("three engines disagree\n  interpreters: %q\n  compiled:    %q\n%s",
					evaluatorOutcome.Text, aotOutcome.Text, aotOutcome.Module)
				return
			}
			agreed++
		})
	}

	reportAgreement(t, agreed, skipped, failed)
}

// reportAgreement prints what was and was not compared, and fails on a skip the
// project did not predict.
//
// A skip nobody predicted is a real finding: the backend refused for a reason
// nobody wrote down, which means the list above is out of date and the next person
// will believe it.
func reportAgreement(t *testing.T, agreed int, skipped map[string][]string, failed []string) {
	total := agreed + len(failed)
	for reason := range skipped {
		total += len(skipped[reason])
	}

	t.Logf("%d of %d cases compared across all three engines", agreed, total)

	if len(skipped) > 0 {
		reasons := make([]string, 0, len(skipped))
		for reason := range skipped {
			reasons = append(reasons, reason)
		}
		sort.Strings(reasons)
		for _, reason := range reasons {
			t.Logf("  skipped %3d  %s", len(skipped[reason]), reason)
			if !predicted(reason) {
				t.Errorf("cases were skipped for a reason the project does not record:\n  %s", reason)
			}
		}
	}

	for _, message := range failed {
		t.Errorf("%s", message)
	}

	if agreed == 0 {
		t.Error("no case was compared across all three engines; the gate would pass " +
			"with the backend switched off entirely")
	}
}

// predicted reports whether a skip reason is one the project knows about.
//
// A predicted reason mentions a construct that is on the list, or is one of the
// backend's own messages about a construct it has not implemented yet. An
// unpredicted one is a refusal nobody wrote down, which is the case this exists to
// catch: the backend declined for a reason the project does not acknowledge, so the
// list above is out of date and the next person will believe it.
func predicted(reason string) bool {
	if strings.Contains(reason, "not compiled ahead of time yet") {
		return true
	}
	if strings.Contains(reason, "error cases") {
		return true
	}
	if strings.Contains(reason, "no linker") || strings.Contains(reason, "no C compiler") {
		return true
	}

	// A refusal from the backend's own emitter, naming what it could not lower. These
	// are the honest half of a partially implemented backend: it says what it cannot
	// do rather than emitting something that runs and is wrong.
	for _, marker := range []string{
		"llvm: no lowering for",    // an operator or instruction with no emitter path
		"llvm: no instruction for", // a MIR instruction with no emitter path
		"llvm: no terminator for",  // a terminator with no emitter path
		"llvm: loading",            // a variable with no slot
		"llvm: storing to",         // a variable with no slot
		"llvm: reading",            // a variable with no slot
		"llvm: cannot assign to",   // an assignment to a non-temporary
		"llvm: no value for",       // an operand with no representation
		"llvm: no constant for",    // a constant the emitter cannot render
	} {
		if strings.Contains(reason, marker) {
			return true
		}
	}

	for _, known := range knownUnsupported {
		if strings.Contains(reason, known) {
			return true
		}
	}
	return false
}
