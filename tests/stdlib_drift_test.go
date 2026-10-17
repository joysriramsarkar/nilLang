package tests

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Standard library drift — Phase 2 item 5 ("std/ ও pkg/stdlib/modules/
// ডুপ্লিকেশন ... পরিষ্কার করুন").
//
// # The inversion this file exists to make visible
//
// The Go toolchain embeds `pkg/stdlib/modules` (see pkg/stdlib/stdlib.go's
// `//go:embed modules`), so that is the copy an `import "std/core"` resolves at
// run time. `std/` is the copy a developer opens and edits, and the copy the
// installer and the repository ship.
//
// Those are not the same file. They are supposed to be, and they are not:
//
//	std/core.nil                     110 lines
//	pkg/stdlib/modules/core.nil      171 lines
//
// Every one of the 53 modules differs, and not one pair is byte-identical. So
// the standard library a program gets depends on which copy it reached, and the
// one nobody edits is the one that runs.
//
// # Why a baseline instead of a plain gate
//
// A plain gate fails today on all 53 modules, which means it is deleted on the
// first day and the divergence goes back to being invisible. Reconciling 53
// divergent modules is a migration, not a commit, and a migration needs the
// count to stay from getting worse while it happens.
//
// So the divergence is recorded in testdata/stdlib-drift.txt as approved debt.
// The test fails when the debt grows, when debt is silently retired without the
// entry being removed, and when a module appears in one tree and not the other.
// When the last entry goes, delete the baseline file and flip TestStdlibDriftIs
// from reporting to enforcing — that flip is the only thing that proves the
// migration finished.

func stdModuleSet(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".nil") {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		sum := sha256.Sum256(data)
		out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s failed: %v", root, err)
	}
	return out
}

// stdDrift describes one disagreement between the two trees.
type stdDrift struct {
	Module string
	// Status is one of: only-in-std, only-in-embedded, content-differs.
	Status string
	// SHA is the embedded copy's digest, which is what identifies the baseline
	// entry. The std/ side is not pinned: the whole point is that it is about to
	// change, and pinning both sides would freeze the problem.
	SHA string
}

func collectStdDrift(t *testing.T) []stdDrift {
	t.Helper()
	repo := repoRootForP0(t)

	source := stdModuleSet(t, filepath.Join(repo, "std"))
	embedded := stdModuleSet(t, filepath.Join(repo, "pkg", "stdlib", "modules"))

	names := map[string]bool{}
	for name := range source {
		names[name] = true
	}
	for name := range embedded {
		names[name] = true
	}

	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)

	var drift []stdDrift
	for _, name := range sorted {
		src, inSource := source[name]
		emb, inEmbedded := embedded[name]
		switch {
		case inSource && !inEmbedded:
			drift = append(drift, stdDrift{Module: name, Status: "only-in-std"})
		case !inSource && inEmbedded:
			drift = append(drift, stdDrift{Module: name, Status: "only-in-embedded", SHA: emb})
		case src != emb:
			drift = append(drift, stdDrift{Module: name, Status: "content-differs", SHA: emb})
		}
	}
	return drift
}

// formatStdDrift renders the drift as the baseline file's lines.
func formatStdDrift(drift []stdDrift) string {
	var sb strings.Builder
	for _, d := range drift {
		fmt.Fprintf(&sb, "%s\t%s\t%s\n", d.Module, d.Status, d.SHA)
	}
	return sb.String()
}

func parseStdDrift(text string) map[string]stdDrift {
	out := map[string]stdDrift{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		d := stdDrift{Module: parts[0], Status: parts[1]}
		if len(parts) > 2 {
			d.SHA = parts[2]
		}
		out[d.Module] = d
	}
	return out
}

// TestStdlibDriftIsApprovedKnown is the gate.
//
// It is a budget, not a description: new drift fails, and so does drift that
// disappears without its entry being removed, because a stale baseline entry is
// how a "fixed" module keeps looking broken.
func TestStdlibDriftIsApprovedKnown(t *testing.T) {
	repo := repoRootForP0(t)
	baselinePath := filepath.Join(repo, "tests", "testdata", "stdlib-drift.txt")
	drift := collectStdDrift(t)

	// Rewriting the baseline is how a deliberate change to either copy becomes
	// recorded debt rather than a failure nobody can reproduce. It is an
	// environment variable inside the test rather than a separate test because a
	// test named "update" is something CI could eventually run by accident.
	if updateStdlibDriftBaseline() {
		if err := os.WriteFile(baselinePath, []byte(stdlibDriftBaselineHeader+formatStdDrift(drift)), 0o644); err != nil {
			t.Fatalf("writing the baseline failed: %v", err)
		}
		t.Logf("baseline regenerated: %d drifting modules recorded", len(drift))
		return
	}

	raw, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("the drift baseline is missing: %v\n"+
			"Without it the divergence between std/ and the embedded copy is invisible again.\n"+
			"Generate it once with:\n"+
			"  NILANG_UPDATE_STDLIB_DRIFT=1 go test ./tests/ -run TestStdlibDriftIsApprovedKnown", err)
	}
	approved := parseStdDrift(string(raw))
	current := drift

	var newDrift, retired []string
	for _, d := range current {
		want, known := approved[d.Module]
		switch {
		case !known:
			newDrift = append(newDrift, fmt.Sprintf("%s (%s)", d.Module, d.Status))
		case want.Status != d.Status:
			newDrift = append(newDrift, fmt.Sprintf("%s changed kind: was %s, now %s", d.Module, want.Status, d.Status))
		case want.SHA != "" && d.SHA != "" && want.SHA != d.SHA:
			// The embedded copy changed. That is either a fix in progress or a
			// regression in the standard library; either way the baseline has to
			// be regenerated deliberately rather than drifting silently.
			newDrift = append(newDrift, fmt.Sprintf("%s changed content in the embedded copy (baseline %s, now %s)",
				d.Module, short(want.SHA), short(d.SHA)))
		}
	}
	for name := range approved {
		if !containsDrift(current, name) {
			retired = append(retired, name)
		}
	}
	sort.Strings(newDrift)
	sort.Strings(retired)

	if len(newDrift) > 0 {
		t.Errorf("the standard library gained drift that the baseline does not record:\n  %s\n"+
			"If the change is intended, regenerate the baseline deliberately; if not, the two copies moved apart again.",
			strings.Join(newDrift, "\n  "))
	}
	if len(retired) > 0 {
		t.Errorf("these baseline entries no longer correspond to any drift: %s\n"+
			"A module that was reconciled must have its entry removed, or the baseline lies about what is still broken.",
			strings.Join(retired, ", "))
	}

	t.Logf("%d modules drift between std/ and the embedded copy, all recorded as approved debt",
		len(current))
	if len(current) == 0 {
		t.Log("drift is fully reconciled — delete tests/testdata/stdlib-drift.txt and flip this test to a plain equality check")
	}
}

// TestEmbeddedStdlibIsTheOneThatRuns pins which copy is canonical today.
//
// The migration in the roadmap makes `std/` the source of truth and the embedded
// copy a generated mirror. Until then the embedded copy is authoritative, because
// it is what `pkg/stdlib.Read` returns — so a reconciliation that edits the
// wrong one produces a change that does not affect any program.
//
// If this test ever fails, the migration has happened and the other test's
// baseline can go.
func TestEmbeddedStdlibIsTheOneThatRuns(t *testing.T) {
	repo := repoRootForP0(t)

	data, err := os.ReadFile(filepath.Join(repo, "pkg", "stdlib", "stdlib.go"))
	if err != nil {
		t.Fatalf("reading pkg/stdlib/stdlib.go failed: %v", err)
	}
	if !strings.Contains(string(data), "//go:embed modules") {
		t.Error("pkg/stdlib no longer embeds modules/; the embedded standard library moved and this file's assumptions need revisiting")
	}

	drift := collectStdDrift(t)
	sourceOnly := 0
	for _, d := range drift {
		if d.Status == "only-in-std" {
			sourceOnly++
		}
	}
	if sourceOnly > 0 {
		t.Errorf("%d modules exist in std/ but are not embedded, so an import of them fails at run time:\n  %s",
			sourceOnly, strings.Join(driftNames(drift, "only-in-std"), "\n  "))
	}
}

func containsDrift(drift []stdDrift, name string) bool {
	for _, d := range drift {
		if d.Module == name {
			return true
		}
	}
	return false
}

func driftNames(drift []stdDrift, status string) []string {
	var out []string
	for _, d := range drift {
		if d.Status == status {
			out = append(out, d.Module)
		}
	}
	return out
}

// stdlibDriftBaselineHeader is written above the recorded drift so that the file
// explains itself to whoever opens it in a diff.
//
// It lives here rather than being preserved across regenerations because a
// preserved header is a header that quietly goes stale; regenerating the file
// writes the current truth and this text with it.
const stdlibDriftBaselineHeader = `# Approved standard-library drift — Phase 2 item 5.
#
# std/ is the tree a developer edits and the installer ships.
# pkg/stdlib/modules/ is what //go:embed modules puts inside every binary, and so
# is what an import of "std/..." resolves to at run time.
#
# Every line below is a module where those two disagree. New drift fails CI, and
# so does a baseline entry whose module has been reconciled — a stale entry is how
# a fixed module keeps looking broken.
#
# Format: <module>\t<status>\t<embedded-sha256>
#   only-in-std      — exists in std/, not embedded, so an import of it fails
#   only-in-embedded — embedded but absent from std/, so editors cannot see it
#   content-differs  — in both, different bytes, so behaviour depends on the copy
#
# Regenerate deliberately:
#   NILANG_UPDATE_STDLIB_DRIFT=1 go test ./tests/ -run TestStdlibDriftIsApprovedKnown
`

// updateStdlibDriftBaseline reports whether the caller asked for the baseline to
// be rewritten from what is on disk.
//
// An environment variable rather than a flag because the Go test binary rejects
// flags it does not define, and a second flag-parsing package for one boolean is
// more machinery than the boolean is worth.
func updateStdlibDriftBaseline() bool {
	switch strings.ToLower(os.Getenv("NILANG_UPDATE_STDLIB_DRIFT")) {
	case "1", "true", "yes":
		return true
	}
	return false
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
