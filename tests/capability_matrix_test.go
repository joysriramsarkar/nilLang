package tests

// Capability allow/deny matrix — Phase 0 item 3 (docs/CORE_STABILIZATION.md)
// and roadmap deliverable 4.
//
// # Why this file exists
//
// The individual enforcement tests in capability_enforcement_test.go cover the
// operations someone thought to write down. This file covers every operation in
// the requirement table, generated from that table, so an operation added to
// compiler/capability/manifest.go cannot ship without a row here.
//
// # Why the table has two levels
//
// The exhaustive check runs against the gate itself: for every operation in the
// table, with its requirement granted the gate allows it and without it the gate
// refuses. That is cheap and total.
//
// The end-to-end check runs a representative subset through the real VM, because
// that is what proves the gate is actually *reached* at a call site. A gate that
// is perfect and never consulted would pass the first test and fail the second.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	compilercap "github.com/joysriramsarkar/nilLang/compiler/capability"
	"github.com/joysriramsarkar/nilLang/pkg/contract"
)

// TestEveryGatedOperationIsAllowedWithItsGrant walks the requirement table and
// requires each operation to be permitted when its declared requirement holds.
//
// The failure mode this catches is the dangerous one: an operation added to the
// table with a requirement nothing can satisfy would be refused forever, and an
// operation whose requirement is wrong would be refused or allowed wrongly.
func TestEveryGatedOperationIsAllowedWithItsGrant(t *testing.T) {
	for _, op := range compilercap.KnownOperations() {
		t.Run(op, func(t *testing.T) {
			required, known := compilercap.RequirementsFor(op)
			if !known {
				t.Fatalf("%s is listed as known but has no requirement", op)
			}

			check := compilercap.GrantChecker(required)
			if err := check(op); err != nil {
				t.Errorf("%s is refused even with its own requirement %v: %v", op, required, err)
			}
		})
	}
}

// TestEveryGatedOperationIsRefusedWithoutItsGrant is the other direction.
//
// It runs only for operations that actually require something. `clear` needs no
// capability by design, so "no grant" is the correct state for it and requiring
// a refusal there would be asserting a bug.
func TestEveryGatedOperationIsRefusedWithoutItsGrant(t *testing.T) {
	refused := 0

	for _, op := range compilercap.KnownOperations() {
		required, known := compilercap.RequirementsFor(op)
		if !known {
			t.Fatalf("%s is listed as known but has no requirement", op)
		}
		if len(required) == 0 {
			continue
		}
		refused++

		t.Run(op, func(t *testing.T) {
			err := compilercap.GrantChecker(nil)(op)
			if err == nil {
				t.Fatalf("%s requires %v but ran with no grant", op, required)
			}
			denied, ok := compilercap.AsDenied(err)
			if !ok {
				t.Fatalf("expected a *DeniedError, got %T: %v", err, err)
			}
			if denied.Operation != op {
				t.Errorf("denial names %q, want %q", denied.Operation, op)
			}
			if denied.Required.String() != required.String() {
				t.Errorf("denial requires %v, want %v", denied.Required, required)
			}
			if denied.DiagnosticCode() != compilercap.Code {
				t.Errorf("denial code is %q, want %q", denied.DiagnosticCode(), compilercap.Code)
			}
		})
	}

	if refused == 0 {
		t.Fatal("no operation in the table requires a capability, which cannot be right")
	}
	t.Logf("%d gated operations require a capability", refused)
}

// TestUnknownOperationIsRefusedNotAssumedPure is the default-deny rule, stated
// on its own because it is the rule most likely to be "fixed" by someone who
// wants a passing test.
//
// A false refusal is a visible bug. A false grant is a vulnerability that
// nothing reports.
func TestUnknownOperationIsRefusedNotAssumedPure(t *testing.T) {
	check := compilercap.GrantChecker([]string{"core", "filesystem", "network", "database", "process"})

	for _, op := range []string{
		"",
		"WriteFile",
		"write_file",
		"std.http.postv2",
		"someNewBuiltinNobodyDeclared",
	} {
		t.Run(op, func(t *testing.T) {
			err := check(op)
			if err == nil {
				t.Fatalf("%q was allowed; an unknown operation must be refused", op)
			}
			denied, ok := compilercap.AsDenied(err)
			if !ok {
				t.Fatalf("expected a *DeniedError, got %T: %v", err, err)
			}
			if !strings.Contains(denied.Reason, "unknown") {
				t.Errorf("refusal reason is %q; an unknown operation should say so", denied.Reason)
			}
		})
	}
}

// TestClearIsKnownAndUngated pins the one operation that exists in the table
// without requiring anything.
//
// `clear` is in the table for a reason that has nothing to do with authority: it
// emits ANSI escapes, so it needs no capability, but the compiler must know the
// name to emit OpNativeCall instead of a closure call the VM cannot resolve. That
// makes it the clearest single test of "known and ungated" versus "unknown", and
// the two are indistinguishable if you never test them apart.
func TestClearIsKnownAndUngated(t *testing.T) {
	if !compilercap.IsKnown("clear") {
		t.Fatal("clear must be a known operation")
	}
	req, known := compilercap.RequirementsFor("clear")
	if !known {
		t.Fatal("clear must resolve to a requirement, even an empty one")
	}
	if len(req) != 0 {
		t.Fatalf("clear requires %v; it should require nothing", req)
	}
	if err := compilercap.GrantChecker(nil)("clear"); err != nil {
		t.Errorf("clear was refused with no grant: %v", err)
	}

	// And the contrast that makes the test meaningful.
	if compilercap.IsKnown("clr") || compilercap.IsKnown("clearScreen") {
		t.Error("a misspelling of clear is treated as known")
	}
}

// TestGatedOperationsAreRefusedEndToEnd proves the gate is reached at a call
// site in a real program, on both engines.
//
// This is a sample, not the table: running `exec` and `writeFile` for every
// permutation would make the suite slow and would depend on host state. What it
// proves is the wiring — that OpNativeCall is emitted and the VM consults the
// checker — for a representative member of each category.
func TestGatedOperationsAreRefusedEndToEnd(t *testing.T) {
	// Every case here runs on every platform. `exec` is deliberately absent:
	// its failure mode when the gate is bypassed depends on a POSIX shell, so
	// including it would make the only test that proves the gate is reached skip
	// itself on Windows, which is one of the three platforms the gate has to
	// hold on.
	cases := []struct {
		name    string
		source  string
		granted []string
		wantErr bool
	}{
		{
			name:    "filesystem write without grant",
			source:  `writeFile("gate-tmp.txt", "x");`,
			granted: nil,
			wantErr: true,
		},
		{
			name:    "filesystem write with grant",
			source:  `writeFile("` + filepath.ToSlash(filepath.Join(t.TempDir(), "ok.txt")) + `", "x");`,
			granted: []string{"filesystem"},
			wantErr: false,
		},
		{
			name:    "filesystem read without grant",
			source:  `readFile("` + filepath.ToSlash(filepath.Join(t.TempDir(), "ok.txt")) + `");`,
			granted: nil,
			wantErr: true,
		},
		{
			name:    "clock without grant",
			source:  `time();`,
			granted: nil,
			wantErr: true,
		},
		{
			name:    "clock with grant",
			source:  `time();`,
			granted: []string{"clock"},
			wantErr: false,
		},
		{
			name:    "environment without grant",
			source:  `cwd();`,
			granted: nil,
			wantErr: true,
		},
		{
			name:    "environment with grant",
			source:  `cwd();`,
			granted: []string{"environment"},
			wantErr: false,
		},
		{
			// `input`, `random` and `exit` are gated but have no declaration
			// the type checker recognises, so no source snippet can reach them
			// at a call site. They are covered exhaustively at the gate level
			// above; noted here so their absence reads as a decision rather
			// than an oversight.
			name:    "a different operation in the same category is gated separately",
			source:  `listDir("` + filepath.ToSlash(t.TempDir()) + `");`,
			granted: []string{"filesystem"},
			wantErr: false,
		},
		{
			name:    "and refused when that grant is absent",
			source:  `listDir("` + filepath.ToSlash(t.TempDir()) + `");`,
			granted: []string{"clock"},
			wantErr: true,
		},
		{
			name:    "clear needs nothing and runs ungranted",
			source:  `clear();`,
			granted: nil,
			wantErr: false,
		},
		{
			name:    "a pure operation needs no grant",
			source:  `len([1, 2, 3]);`,
			granted: nil,
			wantErr: false,
		},
		{
			name:    "a pure operation is not broken by an unrelated grant",
			source:  `len([1, 2, 3]);`,
			granted: []string{"filesystem", "network"},
			wantErr: false,
		},
		{
			name:    "holding one capability does not unlock another",
			source:  `time();`,
			granted: []string{"filesystem"},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := runWithCapabilities(t, tc.source, tc.granted)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("the call ran; expected a capability refusal")
				}
				if _, ok := compilercap.AsDenied(err); !ok {
					t.Fatalf("expected a capability denial, got %T: %v", err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("the call was refused with %v: %v", tc.granted, err)
			}
		})
	}
}

// TestExecIsGatedOnPOSIX covers the one gated operation whose ungated behaviour
// needs a shell, so it is kept out of the table above rather than skipping the
// whole table with it.
func TestExecIsGatedOnPOSIX(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("exec needs a POSIX shell to succeed on")
	}

	if err := runWithCapabilities(t, `exec("true");`, []string{"process"}); err != nil {
		t.Errorf("exec with a process grant was refused: %v", err)
	}
	err := runWithCapabilities(t, `exec("true");`, nil)
	if err == nil {
		t.Fatal("exec ran without a process grant")
	}
	if _, ok := compilercap.AsDenied(err); !ok {
		t.Fatalf("expected a capability denial, got %T: %v", err, err)
	}
}

// TestGrantSpellingIsNormalisedOnBothSides pins the alias rule where it matters.
//
// The manifest says "Filesystem" and the runtime says "filesystem"; a gate that
// compared the strings literally would refuse everything on a device whose
// manifest was written by an older tool.
func TestGrantSpellingIsNormalisedOnBothSides(t *testing.T) {
	for _, grant := range []string{"filesystem", "Filesystem", "FILESYSTEM", "file-system", "file_system", " File System "} {
		if err := compilercap.GrantChecker([]string{grant})("writeFile"); err != nil {
			t.Errorf("grant %q did not satisfy filesystem: %v", grant, err)
		}
	}

	// pkg/capability calls location "GPS"; the canonical name is "location".
	// Both must reach the same operation, and neither may satisfy a different
	// resource — an alias that over-matched would be its own bug.
	for _, grant := range []string{"location", "GPS", "gps"} {
		if err := compilercap.GrantChecker([]string{grant})("std.location.current"); err != nil {
			t.Errorf("grant %q did not satisfy location: %v", grant, err)
		}
	}
	if err := compilercap.GrantChecker([]string{"filesystem"})("std.location.current"); err == nil {
		t.Error("a filesystem grant satisfied a location requirement")
	}
}

// TestPublishedRequirementTableMatchesTheCode keeps the cross-repo contract
// honest.
//
// contracts/capabilities/requirements.json is what a verifier in Rust or
// JavaScript reads. If it falls behind compiler/capability/manifest.go, a
// consumer answers "this operation needs nothing" for an operation the language
// actually gates — a silent widening of what an embedder may do.
func TestPublishedRequirementTableMatchesTheCode(t *testing.T) {
	set, err := contract.Load()
	if err != nil {
		t.Fatalf("loading the contract bundle failed: %v", err)
	}

	for _, op := range compilercap.KnownOperations() {
		fromCode, _ := compilercap.RequirementsFor(op)

		fromContract, known := set.Requirements().RequirementsFor(op)
		if !known {
			t.Errorf("operation %q is gated in the compiler but absent from the published requirement table", op)
			continue
		}
		if len(fromCode) != len(fromContract) {
			t.Errorf("operation %q requires %v in code but %v in the contract", op, fromCode, fromContract)
			continue
		}
		for i := range fromCode {
			if fromCode[i] != fromContract[i] {
				t.Errorf("operation %q requires %v in code but %v in the contract", op, fromCode, fromContract)
				break
			}
		}
	}

	// And the other direction: a requirement the code does not have is a
	// capability a verifier will enforce that the language never asks for.
	ops := set.Requirements().Operations()
	for _, op := range ops {
		if !compilercap.IsKnown(op) {
			t.Errorf("the contract gates %q but the compiler has no requirement for it", op)
		}
	}
}

// TestEveryCapabilityRequiredIsInTheVocabulary closes the loop between the
// requirement table and the published vocabulary.
//
// A requirement naming a capability no verifier can validate is a grant that
// cannot be checked at a boundary, which is worse than no gate.
func TestEveryCapabilityRequiredIsInTheVocabulary(t *testing.T) {
	set, err := contract.Load()
	if err != nil {
		t.Fatalf("loading the contract bundle failed: %v", err)
	}

	for _, op := range compilercap.KnownOperations() {
		required, _ := compilercap.RequirementsFor(op)
		for _, name := range required {
			if _, err := set.Vocabulary().Canonicalize(name); err != nil {
				t.Errorf("operation %q requires %q: %v", op, name, err)
			}
		}
	}
}

// TestCapabilityVocabularyStatusIsAccurate keeps the documented divergence
// between the two engines from going stale.
//
// The VM gates at the call site; the evaluator pre-scans and sees direct calls
// only. That asymmetry is recorded in the specification and in the published
// contract. This test asserts the two places still say the same thing, because a
// documented limitation that the documentation has quietly dropped is
// indistinguishable from a fixed one.
func TestCapabilityVocabularyStatusIsAccurate(t *testing.T) {
	set, err := contract.Load()
	if err != nil {
		t.Fatalf("loading the contract bundle failed: %v", err)
	}

	joined := strings.Join(set.Vocabulary().Enforcement.KnownLimitations, " ")
	for _, needle := range []string{"Indirect", "evaluator", "requires"} {
		if !strings.Contains(joined, needle) {
			t.Errorf("the published vocabulary no longer records the %q limitation; the VM and evaluator gates differ and the contract must say so", needle)
		}
	}
}

// TestRequirementTableIsSerializable confirms the table the contract publishes
// can round-trip, since the store and other consumers read it as JSON.
func TestRequirementTableIsSerializable(t *testing.T) {
	data, err := contract.BuildRequirements()
	if err != nil {
		t.Fatalf("building the requirement table failed: %v", err)
	}

	var doc struct {
		Requirements map[string][]string `json:"requirements"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("the generated requirement table is not valid JSON: %v", err)
	}
	if len(doc.Requirements) != len(compilercap.KnownOperations()) {
		t.Errorf("the published table has %d operations, the compiler has %d",
			len(doc.Requirements), len(compilercap.KnownOperations()))
	}

	// An operation with no requirement must serialise as [] and not as null, so
	// a reader in another language does not have to tell "needs nothing" from
	// "field missing".
	raw := string(data)
	if !strings.Contains(raw, `"clear": []`) {
		t.Error(`the generated table must render clear as [] so "needs nothing" is explicit`)
	}

	// And the file on disk must be the generated one.
	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	onDisk, err := os.ReadFile(filepath.Join(repoRoot, "..", "contracts", "capabilities", "requirements.json"))
	if err != nil {
		t.Fatalf("reading the published requirement table failed: %v", err)
	}
	if string(onDisk) != string(data) {
		t.Error("contracts/capabilities/requirements.json is out of date; run: go run ./cmd/nil-contract generate")
	}
}