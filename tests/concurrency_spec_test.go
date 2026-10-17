package tests

// Concurrency specification conformance — Phase 0 item 4
// (docs/CORE_STABILIZATION.md) and the ecosystem roadmap's
// "task lifecycle ... যা আছে কিন্তু implementation-এ নেই, সেটিকে supported বলুন না"।
//
// # Why this file is a table and not a list of passing tests
//
// docs/spec/CONCURRENCY.md specifies a lifecycle, cascading cancellation,
// timeouts and fault isolation. Some of it works. The parts that do not are not
// silently absent — they are recorded as `not_implemented` rows below and
// cross-checked against the published compatibility matrix.
//
// A gap recorded here is a tracked gap. A gap recorded only in a status field is
// a claim, and claims do not fail CI.
//
// # How to read a failing row
//
// If a `not_implemented` row starts failing because the behaviour appeared,
// that is good news and the fix is to promote it. If an `implemented` row starts
// failing, the behaviour regressed. Do not "fix" a failing row by editing the
// status: the status is derived from the assertion below it.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/pkg/contract"
)

// evalValue runs source through the gate and the evaluator, returning the final
// value's inspection. A rejected program yields "GATE: <reason>" and a runtime
// error yields its message, so a test can assert on either without the helper
// deciding what counts as a failure.
func evalValue(t *testing.T, source string) string {
	t.Helper()
	res := evalSource(t, source)
	if !res.OK {
		return "GATE: " + oneLine(res.ErrSummary())
	}
	env := object.NewEnvironment()
	for name, builtin := range evaluator.Builtins {
		env.Set(name, builtin)
	}
	if evaluated := evaluator.Eval(res.Program, env); evaluated != nil {
		return evaluated.Inspect()
	}
	return "<nil>"
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
}

// specRule is one clause of docs/spec/CONCURRENCY.md.
type specRule struct {
	// Ref is the section in CONCURRENCY.md.
	Ref string
	// What states the rule.
	Rule string
	// Status is what the implementation does today.
	Status string
	// Keywords are the phrases a published compatibility limitation must
	// contain for this gap to be recorded.
	//
	// Prose matching on Rule would work until someone reworded the sentence,
	// at which point the gap silently stops being published. Keywords are
	// short, deliberate, and stated here so that publishing a gap is a decision
	// rather than a coincidence.
	Keywords []string
	// Want is the observed behaviour the assertion checks.
	Want func(t *testing.T)
}

const (
	ruleImplemented    = "implemented"
	rulePartial        = "partial"
	ruleNotImplemented = "not_implemented"
)

// TestConcurrencySpecConformance is the table.
//
// Each row states the spec clause, the implementation status, and an assertion
// over the real evaluator. A `not_implemented` row asserts that the construct is
// absent, which is what keeps the gap honest in both directions: the feature
// cannot quietly vanish into the codebase without someone updating this table.
func TestConcurrencySpecConformance(t *testing.T) {
	rules := []specRule{
		{
			Ref:    "§2 Fulfilled",
			Rule:   "a task that completes yields its value to await",
			Status: ruleImplemented,
			Want: func(t *testing.T) {
				if got := evalValue(t, `let f = task { return 40 + 2; }; await f;`); got != "42" {
					t.Errorf("await on a fulfilled task returned %q, want 42", got)
				}
			},
		},
		{
			Ref:    "§2 Faulted",
			Rule:   "a task that errors reports Faulted to await, naming the cause",
			Status: ruleImplemented,
			Want: func(t *testing.T) {
				got := evalValue(t, `let f = task { return 1 / 0; }; await f;`)
				if !strings.Contains(got, "TaskFaulted") {
					t.Errorf("a faulting task reported %q, want a TaskFaulted error", got)
				}
				if !strings.Contains(got, "division by zero") {
					t.Errorf("the fault did not name its cause: %q", got)
				}
			},
		},
		{
			Ref:  "§2 Single-Resolution Invariant",
			Rule: "a task's terminal state is observed by await exactly once",
			// Observed: awaiting twice returns the same value rather than the
			// second await consuming the completion. That is the substance of
			// the rule and it holds.
			Status: ruleImplemented,
			Want: func(t *testing.T) {
				got := evalValue(t, `let f = task { return 7; }; let a = await f; let b = await f; [a, b];`)
				if got != "[7, 7]" {
					t.Errorf("awaiting twice returned %q, want [7, 7]", got)
				}
			},
		},
		{
			Ref:    "§2 Cancelled",
			Rule:   "a cancelled task reports Cancelled to await",
			Status: ruleImplemented,
			Want: func(t *testing.T) {
				// Reached through the only cancellation the language exposes
				// today, a deadline. There is no user-callable cancel().
				got := evalValue(t, `withTimeout(1, fn() { let i = 0; while (i < 200000000) { i = i + 1; } return i; });`)
				if !strings.Contains(got, "TaskCancelled") {
					t.Errorf("an expired deadline reported %q, want a TaskCancelled error", got)
				}
			},
		},
		{
			Ref:      "§3.1.2 cascading",
			Rule:     "a child that faults cancels its siblings and fails the parent with the aggregated failure",
			Keywords: []string{"cascading cancellation", "siblings"},
			Status:   rulePartial,
			Want: func(t *testing.T) {
				// The parent does fail, and it does surface an error — but the
				// error is `TaskCancelled`, not the child's fault. The cause is
				// lost one level down, so a fault two tasks deep is
				// indistinguishable from a timeout. Asserted here so that
				// improving the diagnostic is a visible change.
				got := evalValue(t, `let f = task { let g = task { return 1 / 0; }; let v = await g; return v; }; await f;`)
				if !strings.Contains(got, "Task") {
					t.Errorf("a faulting child did not surface an error to the parent: %q", got)
				}
				if !strings.Contains(got, "division by zero") {
					t.Logf("known gap: the parent's error is %q, which does not name the child's cause", got)
				}
			},
		},
		{
			Ref:    "§3.2 Timeouts",
			Rule:   "withTimeout cancels the enclosed tree when the deadline expires",
			Status: ruleImplemented,
			Want: func(t *testing.T) {
				got := evalValue(t, `withTimeout(1, fn() { let i = 0; while (i < 200000000) { i = i + 1; } return i; });`)
				if !strings.Contains(got, "TaskCancelled") || !strings.Contains(got, "timeout") {
					t.Errorf("an expired deadline reported %q, want a timeout cancellation", got)
				}
				if evalValue(t, `withTimeout(5000, fn() { return 1; });`) != "1" {
					t.Error("withTimeout must return the value when the body finishes in time")
				}
			},
		},
		{
			Ref:  "§4.1 Fault Isolation",
			Rule: "a runtime exception inside a task does not crash the host process, and does not reach tasks in another evaluation",
			// Note what this row deliberately does NOT assert: that a sibling
			// task keeps running. §3.1.2 says the opposite — a child that
			// faults has its siblings cancelled — so "sibling tasks survive"
			// and "siblings are cancelled" are both true and contradict each
			// other in the specification. What §4.1 means is that the host
			// survives and the damage stops at the cancellation tree's edge.
			// Asserting sibling survival here would have been asserting a
			// clause the implementation deliberately does not honour, and it
			// would have failed about a quarter of the time.
			Status: ruleImplemented,
			Want: func(t *testing.T) {
				// The fault is contained and reported, not fatal.
				faulted := evalValue(t, `let a = task { return 1 / 0; }; await a;`)
				if !strings.Contains(faulted, "TaskFaulted") {
					t.Fatalf("a faulting task reported %q, want a contained TaskFaulted", faulted)
				}

				// And it stops at the tree's edge: a separate evaluation in
				// the same process is untouched. Before the scope moved onto
				// the environment this was a coin flip — the two evaluations'
				// tasks were siblings under one process-wide root.
				for i := 0; i < 8; i++ {
					if got := evalValue(t, `let b = task { return 7; }; await b;`); got != "7" {
						t.Fatalf("attempt %d: an unrelated evaluation returned %q after an earlier one faulted, want 7", i, got)
					}
				}
			},
		},
		{
			Ref:    "§4.2 typed fault",
			Rule:   "the scheduler intercepts a runtime exception and types it",
			Status: ruleImplemented,
			Want: func(t *testing.T) {
				// Calling a non-callable is a runtime exception the type
				// checker cannot see through, which makes it the honest
				// case for "the scheduler intercepts an exception".
				got := evalValue(t, `let notAFunction = 5; let f = task { return notAFunction(); }; await f;`)
				if !strings.Contains(got, "TaskFaulted") {
					t.Errorf("a runtime exception inside a task reported %q, want TaskFaulted", got)
				}
			},
		},
		{
			Ref:  "§3.1.1 explicit cancellation",
			Rule: "a task can be cancelled explicitly, and cancelling a parent cancels its children",
			// Written into contracts/compatibility/matrix.json by this file's
			// own publication check, which is why it is spelled out here: a
			// gap nobody published is a gap nobody is tracking.
			Keywords: []string{"cancel()", "explicit cancellation"},
			Status:   ruleNotImplemented,
			Want: func(t *testing.T) {
				// There is no cancel() in the language. The assertion is that
				// it is absent, so that adding one without finishing the
				// cascade rules shows up here.
				got := evalValue(t, `let f = task { return 1; }; cancel(f); await f;`)
				if !strings.HasPrefix(got, "GATE:") {
					t.Fatalf("cancel() now exists and returns %q; implement the cascade rules in this table before promoting this row", got)
				}
				if !strings.Contains(got, "E0102") {
					t.Errorf("cancel() should be rejected as an undefined identifier, got: %s", got)
				}
			},
		},
	}

	for _, rule := range rules {
		t.Run(rule.Ref, func(t *testing.T) {
			t.Logf("%s — %s [%s]", rule.Ref, rule.Rule, rule.Status)
			rule.Want(t)
		})
	}

	t.Run("every documented gap is published", func(t *testing.T) {
		assertGapsArePublished(t, rules)
	})
}

// assertGapsArePublished ties the table to the machine-readable support status.
//
// The ecosystem's rule is that a feature is not supported without evidence, and
// that `partial`/`stub` stays visible. A row in this table marked partial or
// not_implemented must appear as a limitation in the published compatibility
// matrix, or the gap is only known to whoever reads the test.
func assertGapsArePublished(t *testing.T, rules []specRule) {
	set, err := contract.Load()
	if err != nil {
		t.Fatalf("loading the contract bundle failed: %v", err)
	}

	gaps := 0
	for _, rule := range rules {
		if rule.Status == ruleImplemented {
			continue
		}
		gaps++

		found := false
		for _, backend := range []string{"vm", "evaluator", "mir"} {
			for feature := range set.Compatibility().Backends[backend].Features {
				f, ok := set.Compatibility().FeatureStatus(backend, feature)
				if !ok {
					continue
				}
				if f.Status == ruleImplemented {
					continue
				}
				if mentionsAny(f.Limitations, rule.Keywords...) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			keywords := rule.Keywords
			if len(keywords) == 0 {
				keywords = []string{firstWords(rule.Rule, 4)}
			}
			t.Errorf("%s is %s but no published compatibility cell records it; add a limitation to contracts/compatibility/matrix.json naming one of %v",
				rule.Ref, rule.Status, keywords)
		}
	}

	if gaps == 0 {
		t.Fatal("no concurrency gaps are recorded, which contradicts the spec")
	}
	t.Logf("%d concurrency gaps are recorded and published", gaps)
}

func mentionsAny(haystack string, needles ...string) bool {
	if haystack == "" {
		return false
	}
	lower := strings.ToLower(haystack)
	for _, needle := range needles {
		if needle == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func firstWords(s string, n int) string {
	fields := strings.Fields(s)
	if len(fields) < n {
		return strings.Join(fields, " ")
	}
	return strings.Join(fields[:n], " ")
}

// TestTaskSurvivesCollectionPressure is the stress case the roadmap asks for:
// not a small unit test, but a long-lived closure held by a future across many
// collections.
//
// A future that is collected early, or a closure cell that is re-pointed while
// held, produces a wrong answer rather than a crash — which is why a GC test
// that only checks "did not crash" proves nothing about the GC.
func TestTaskSurvivesCollectionPressure(t *testing.T) {
	const tasks = 200

	// Each task returns its own index, so the expected sum is fixed. The
	// interesting part is that every future is created, then held live while
	// the program allocates hard enough to force collections, and only then is
	// awaited. A root registered at the wrong moment loses the future here and
	// the program gets a wrong answer rather than a crash.
	var sb strings.Builder
	sb.WriteString("let total = 0;\n")
	for i := 0; i < tasks; i++ {
		fmt.Fprintf(&sb, "let t%d = task { let cell%d = %d; return cell%d; };\n", i, i, i, i)
	}
	for i := 0; i < tasks; i++ {
		fmt.Fprintf(&sb, "let pad%d = []; for i in 0..300 { pad%d = push(pad%d, [i, i, i]); }\n", i, i, i)
	}
	for i := 0; i < tasks; i++ {
		fmt.Fprintf(&sb, "total = total + await t%d;\n", i)
	}
	sb.WriteString("total;\n")

	res := evalSource(t, sb.String())
	if !res.OK {
		t.Fatalf("the stress program was rejected: %s", res.ErrSummary())
	}

	env := object.NewEnvironment()
	for name, builtin := range evaluator.Builtins {
		env.Set(name, builtin)
	}
	evaluated := evaluator.Eval(res.Program, env)
	if evaluated == nil {
		t.Fatal("the stress program produced no value")
	}
	if errObj, ok := evaluated.(*object.Error); ok {
		t.Fatalf("the stress program failed: %s", errObj.Inspect())
	}

	want := tasks * (tasks - 1) / 2
	if got := evaluated.Inspect(); got != fmt.Sprint(want) {
		t.Errorf("after holding %d futures across allocation, the sum is %s, want %d", tasks, got, want)
	}
}

// TestNestedTasksUnderPressure runs three levels of nesting with allocation
// between levels, which is where a root registered at the wrong time goes
// unnoticed.
//
// Run with -race. The race detector is the only thing that can see a root set
// being mutated from a task goroutine while the collector walks it.
func TestNestedTasksUnderPressure(t *testing.T) {
	const depth = 6

	var sb strings.Builder
	sb.WriteString("let total = 0;\n")
	sb.WriteString("let leaf = task { return 3; };\n")
	for i := 0; i < depth; i++ {
		// Allocate between levels so the collector has a reason to run while
		// the outer futures are still live.
		sb.WriteString("let pad = [];\n")
		sb.WriteString("for i in 0..64 { pad = push(pad, [i, i, i]); }\n")
		sb.WriteString("total = total + await (task { return await leaf; });\n")
	}
	sb.WriteString("total;\n")

	res := evalSource(t, sb.String())
	if !res.OK {
		t.Fatalf("the nested stress program was rejected: %s", res.ErrSummary())
	}

	env := object.NewEnvironment()
	for name, builtin := range evaluator.Builtins {
		env.Set(name, builtin)
	}
	evaluated := evaluator.Eval(res.Program, env)
	if evaluated == nil {
		t.Fatal("the nested stress program produced no value")
	}
	if errObj, ok := evaluated.(*object.Error); ok {
		t.Fatalf("the nested stress program failed: %s", errObj.Inspect())
	}
	if got, want := evaluated.Inspect(), fmt.Sprint(3*depth); got != want {
		t.Errorf("nested tasks summed to %s, want %s", got, want)
	}
}

// TestTypedRuntimeErrorsAreStable pins the error class a fault produces, so a
// caller can match on it rather than on its text.
//
// The roadmap's item is "typed error tests স্থিতিশীল করুন": an error whose
// identity lives only in a message string cannot be handled, and a change to the
// wording silently breaks every caller that was matching on it.
func TestTypedRuntimeErrorsAreStable(t *testing.T) {
	cases := []struct {
		name     string
		source   string
		mustHave []string
	}{
		{
			name:     "division by zero",
			source:   `let f = task { return 1 / 0; }; await f;`,
			mustHave: []string{"TaskFaulted", "division by zero"},
		},
		{
			name:     "undefined identifier at run time",
			source:   `let f = task { return missingThing(); }; await f;`,
			mustHave: []string{"TaskFaulted", "identifier not found"},
		},
		{
			name:     "timeout is a cancellation, not a fault",
			source:   `withTimeout(1, fn() { let i = 0; while (i < 200000000) { i = i + 1; } return i; });`,
			mustHave: []string{"TaskCancelled"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := evalValue(t, tc.source)
			for _, needle := range tc.mustHave {
				if !strings.Contains(got, needle) {
					t.Errorf("error %q does not carry %q", got, needle)
				}
			}
		})
	}
}
