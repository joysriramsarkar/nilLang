package tests

// P0 exit gate — Phase 0 item 1 (docs/CORE_STABILIZATION.md) and the roadmap's
// "CORE_FREEZE.md ... DoD-কে release blocker করুন".
//
// # What a release blocker is
//
// A blocker is not a checklist someone reads before shipping. It is a check that
// fails the build. This file is that check: for every P0 item in
// CORE_STABILIZATION.md it names the evidence the item claims, and it fails if
// that evidence is missing, empty, or no longer run by CI.
//
// # Why the evidence is named here rather than in the document
//
// docs/CORE_STABILIZATION.md describes the work; it cannot verify itself. A
// document that says "conformance harness" stays true after the harness is
// deleted, because nothing reads the sentence. This file reads it.
//
// # What a failure means
//
// Either the evidence moved — in which case point this file at the new place —
// or the item's claim is no longer backed. Those are different problems and the
// fix differs, which is why the message says which one it is.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// p0Item is one item of docs/CORE_STABILIZATION.md and the evidence it claims.
type p0Item struct {
	// Number is the item number in CORE_STABILIZATION.md.
	Number string
	// Title is the item's name, used in failure messages.
	Title string
	// Evidence is a path, relative to the repository root, that must exist and
	// be non-empty.
	Evidence []string
	// InCI, when set, is a path that must reference the evidence. It is what
	// turns "a file exists" into "a file is run on every push".
	CIFile string
	// CIPattern must appear in CIFile.
	CIPattern string
}

// p0Evidence is the P0 backlog's evidence table.
//
// Every row is a claim the freeze's exit criterion rests on. Adding an item to
// CORE_STABILIZATION.md without adding it here is fine — the backlog is a plan
// and a plan may grow. The reverse is not: evidence here that no longer exists
// is a release blocker, which is the whole point.
var p0Evidence = []p0Item{
	{
		Number: "P0.1", Title: "Freeze grammar and binding rules",
		Evidence: []string{
			"compiler/parser/grammar_table_test.go",
			"docs/spec/LANGUAGE_SPEC_CANONICAL.md",
		},
		CIFile: ".github/workflows/ci.yml", CIPattern: "compiler/parser",
	},
	{
		Number: "P0.2", Title: "One verification gate for every entry point",
		Evidence: []string{
			"pkg/compiler/entry_rejection_test.go",
			"tests/frontend_gate_test.go",
			"compiler/frontend/frontend.go",
		},
		CIFile: ".github/workflows/ci.yml", CIPattern: "./cmd/nilc",
	},
	{
		Number: "P0.3", Title: "No implicit Any",
		Evidence: []string{
			"tests/typesystem_test.go",
			"compiler/typecheck/any_origins_test.go",
		},
		CIFile: ".github/workflows/ci.yml", CIPattern: "compiler/typecheck",
	},
	{
		Number: "P0.4", Title: "Frozen scope and closure semantics",
		Evidence: []string{
			"tests/scope_corpus_test.go",
			"tests/closure_capture_test.go",
		},
		CIFile: ".github/workflows/ci.yml", CIPattern: "./tests/...",
	},
	{
		Number: "P0.5", Title: "Conformance harness",
		Evidence: []string{
			"tests/conformance/conformance_test.go",
			"tests/conformance/threeengines_test.go",
		},
		CIFile: ".github/workflows/ci.yml", CIPattern: "conformance-matrix",
	},
	{
		Number: "P0.6", Title: "Compiler never panics",
		Evidence: []string{
			"pkg/compiler/pipeline_fuzz_test.go",
			"compiler/typecheck/typecheck_fuzz_test.go",
			"compiler/parser/parser_fuzz_test.go",
		},
		CIFile: ".github/workflows/ci.yml", CIPattern: "fuzz",
	},
	{
		Number: "P0.7", Title: "Standardised diagnostics",
		Evidence: []string{
			"compiler/diagnostics/diagnostics.go",
			"compiler/frontend/diagnostics_audit_test.go",
			"docs/spec/SYNTACTIC_DIAGNOSTICS.md",
		},
	},
	{
		Number: "P0.8", Title: "CI gate on every platform",
		Evidence: []string{".github/workflows/ci.yml"},
		CIFile:   ".github/workflows/ci.yml", CIPattern: "windows-latest",
	},

	// Phase 0 of the ecosystem roadmap adds three items that are not in
	// CORE_STABILIZATION.md but are the same class of claim, so they are gated
	// the same way.
	{
		Number: "E0.3", Title: "Capability enforcement for every gated operation",
		Evidence: []string{
			"tests/capability_enforcement_test.go",
			"tests/capability_matrix_test.go",
			"contracts/capabilities/requirements.json",
		},
		CIFile: ".github/workflows/ci.yml", CIPattern: "./pkg/contract/...",
	},
	{
		Number: "E0.4", Title: "GC, task and cancellation behaviour is stable",
		Evidence: []string{
			"tests/gc_test.go",
			"tests/concurrency_spec_test.go",
			"runtime/concurrency/structured_concurrency_test.go",
		},
	},
	{
		Number: "E0.1", Title: "The freeze contract itself exists and is frozen",
		Evidence: []string{"CORE_FREEZE.md"},
	},
}

// TestP0ExitEvidenceExists is the blocker.
//
// It does not run the evidence — the ordinary test suite does that on every
// push. It checks that the evidence still exists and is still wired into CI,
// which is the part nothing else checks: a file can keep passing in a developer's
// checkout long after CI stopped running it.
func TestP0ExitEvidenceExists(t *testing.T) {
	root := repoRootForP0(t)

	for _, item := range p0Evidence {
		t.Run(item.Number, func(t *testing.T) {
			t.Logf("%s — %s", item.Number, item.Title)

			if len(item.Evidence) == 0 {
				t.Fatalf("%s claims no evidence, so its exit condition cannot be checked", item.Number)
			}

			for _, rel := range item.Evidence {
				path := filepath.Join(root, rel)
				info, err := os.Stat(path)
				if err != nil {
					t.Errorf("%s: evidence %s is missing — either point this at its new location, or the item's claim is no longer backed",
						item.Number, rel)
					continue
				}
				if info.Size() == 0 {
					t.Errorf("%s: evidence %s is empty, which is not evidence", item.Number, rel)
				}
			}

			if item.CIFile == "" {
				return
			}
			workflow, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(item.CIFile)))
			if err != nil {
				t.Errorf("%s: cannot read the workflow that should run this evidence: %v", item.Number, err)
				return
			}
			if !strings.Contains(string(workflow), item.CIPattern) {
				t.Errorf("%s: %s does not reference %q, so this evidence may not run on every push",
					item.Number, item.CIFile, item.CIPattern)
			}
		})
	}
}

// TestEveryP0ItemInTheBacklogIsGated keeps this file and the backlog in step.
//
// The backlog is the plan of record. An item added there and not here is an item
// whose exit condition nothing checks — the same silence this whole file exists
// to end.
func TestEveryP0ItemInTheBacklogIsGated(t *testing.T) {
	root := repoRootForP0(t)
	backlog, err := os.ReadFile(filepath.Join(root, "docs", "CORE_STABILIZATION.md"))
	if err != nil {
		t.Fatalf("reading the backlog failed: %v", err)
	}

	// The P0 section runs from its heading to the P1 heading.
	start := strings.Index(string(backlog), "## P0: Core semantics")
	end := strings.Index(string(backlog), "## P1:")
	if start < 0 || end < 0 || end < start {
		t.Fatal("could not locate the P0 section in docs/CORE_STABILIZATION.md")
	}
	section := string(backlog)[start:end]

	gated := map[string]bool{}
	for _, item := range p0Evidence {
		gated[item.Number] = true
	}

	for _, line := range strings.Split(section, "\n") {
		// Only top-level items: the backlog indents `Owner:` and `Test:` lines
		// under each one, and a numbered-looking line inside a fenced block or a
		// table would otherwise be read as an item of its own.
		if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '|' || line[0] == '#' {
			continue
		}
		number := leadingNumber(line)
		if number == "" {
			continue
		}
		// The backlog numbers its items `1.` … `8.`; the evidence table keys
		// them `P0.1` … `P0.8` so the ecosystem items can sit in the same map.
		if !gated["P0."+number] {
			t.Errorf("P0.%s is in the P0 backlog but no evidence is gated for it in this file; add a row to p0Evidence so its exit condition is checked",
				number)
		}
	}
}

// TestFrozenSurfacesAreAllDeclared guards the freeze contract itself.
//
// CORE_FREEZE.md lists eleven frozen surfaces and requires a spec update for any
// change to them. A surface that drops out of the list stops being frozen without
// anyone deciding that, so the list is asserted against the eleven the core
// release depends on.
func TestFrozenSurfacesAreAllDeclared(t *testing.T) {
	root := repoRootForP0(t)
	freeze, err := os.ReadFile(filepath.Join(root, "CORE_FREEZE.md"))
	if err != nil {
		t.Fatalf("reading CORE_FREEZE.md failed: %v", err)
	}

	for _, surface := range []string{
		"Language syntax frozen",
		"Type semantics frozen",
		"Scope semantics frozen",
		"Assignment semantics frozen",
		"Function semantics frozen",
		"Module semantics frozen",
		"Error semantics frozen",
		"Concurrency semantics frozen",
		"Memory semantics frozen",
		"Effect semantics frozen",
		"Capability semantics frozen",
	} {
		if !strings.Contains(string(freeze), surface) {
			t.Errorf("CORE_FREEZE.md no longer declares %q; a surface that leaves the freeze stops being frozen by accident rather than by decision", surface)
		}
	}

	if !strings.Contains(string(freeze), "Status:** Active") {
		t.Error("CORE_FREEZE.md is not marked Active. Either the freeze has exited — which is a decision that belongs in an ADR — or the status line was lost.")
	}
}

func leadingNumber(s string) string {
	dot := strings.Index(s, ".")
	if dot < 1 || dot > 3 {
		return ""
	}
	for _, r := range s[:dot] {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return s[:dot]
}

func repoRootForP0(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("cannot determine the working directory: %v", err)
	}
	for d := dir; ; {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			t.Fatal("cannot find the repository root")
		}
		d = parent
	}
}
