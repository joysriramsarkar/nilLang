package contract

import (
	"fmt"
	"sort"

	langcap "github.com/joysriramsarkar/nilLang/compiler/capability"
)

// CapabilityEntry is one canonical capability name.
type CapabilityEntry struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

// AliasEntry maps a resource-oriented spelling onto a canonical name.
type AliasEntry struct {
	Alias     string `json:"alias"`
	Canonical string `json:"canonical"`
	Reason    string `json:"reason"`
}

// RecognisedEntry is a name the manifest vocabulary accepts that no builtin
// currently requires.
type RecognisedEntry struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// PlannedEntry is a name that is documented as a future refinement. It is
// refused today, and the reason it is listed is so that the refusal can be
// explained rather than merely enforced.
type PlannedEntry struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// EnforcementBlock records where capabilities are enforced and where they are not.
type EnforcementBlock struct {
	LanguageGate     string   `json:"language_gate"`
	KnownLimitations []string `json:"known_limitations"`
	OSGate           string   `json:"os_gate"`
}

// Vocabulary is contracts/capabilities/vocabulary.json.
type Vocabulary struct {
	Contract           string            `json:"contract"`
	Version            string            `json:"version"`
	Status             string            `json:"status"`
	Owner              string            `json:"owner"`
	Spec               string            `json:"spec"`
	SourceOfTruth      string            `json:"source_of_truth"`
	DefaultDeny        string            `json:"default_deny"`
	Capabilities       []CapabilityEntry `json:"capabilities"`
	RecognisedUnlisted []RecognisedEntry `json:"recognised_unlisted"`
	Aliases            []AliasEntry      `json:"aliases"`
	Planned            []PlannedEntry    `json:"planned"`
	Enforcement        EnforcementBlock  `json:"enforcement"`

	canonical map[string]CapabilityEntry
	alias     map[string]string
}

// CanonicalNames returns every canonical capability name, sorted.
func (v *Vocabulary) CanonicalNames() []string {
	out := make([]string, 0, len(v.Capabilities))
	for _, c := range v.Capabilities {
		out = append(out, c.Name)
	}
	sort.Strings(out)
	return out
}

// Lookup returns the canonical entry for a name that is already canonical.
func (v *Vocabulary) Lookup(name string) (CapabilityEntry, bool) {
	if v.canonical == nil {
		v.canonical = make(map[string]CapabilityEntry, len(v.Capabilities))
		for _, c := range v.Capabilities {
			v.canonical[c.Name] = c
		}
	}
	entry, ok := v.canonical[name]
	return entry, ok
}

// Canonicalize maps a manifest's spelling of a capability onto its canonical
// name, and refuses a name the vocabulary does not carry.
//
// Normalisation is the compiler's, not this package's: two implementations of
// "how do I read File-System" that disagree about '-' and '_' would each accept
// manifests the other rejects, which is the same defect as two signature
// implementations. So the table has exactly one owner.
func (v *Vocabulary) Canonicalize(name string) (string, error) {
	normalized := langcap.NormalizeName(name)
	if normalized == "" {
		return "", fmt.Errorf("empty capability name")
	}
	if _, ok := v.Lookup(normalized); ok {
		return normalized, nil
	}
	for _, planned := range v.Planned {
		if langcap.NormalizeName(planned.Name) == normalized {
			return "", fmt.Errorf("capability %q is a planned refinement, not implemented: %s", name, planned.Reason)
		}
	}
	return "", fmt.Errorf("capability %q is not in the canonical vocabulary", name)
}

// Recognised reports whether a name is accepted in a manifest, whether or not
// any operation currently requires it.
//
// This is weaker than Canonicalize on purpose and is used only where the
// question really is "is this spelling understood" — for example to give a
// better message for a capability that is recognised but gates nothing yet.
func (v *Vocabulary) Recognised(name string) bool {
	normalized := langcap.NormalizeName(name)
	if _, ok := v.Lookup(normalized); ok {
		return true
	}
	if v.alias == nil {
		v.alias = make(map[string]string, len(v.Aliases))
		for _, a := range v.Aliases {
			v.alias[a.Alias] = a.Canonical
		}
	}
	if _, ok := v.alias[normalized]; ok {
		return true
	}
	for _, r := range v.RecognisedUnlisted {
		if langcap.NormalizeName(r.Name) == normalized {
			return true
		}
	}
	return false
}

// RequirementTable is contracts/capabilities/requirements.json.
//
// It is generated from compiler/capability/manifest.go, and the test
// TestRequirementsMatchSource fails if the two disagree. The table exists so
// that a verifier in another language can ask "what does this operation need"
// without linking against Go.
type RequirementTable struct {
	Contract      string              `json:"contract"`
	Version       string              `json:"version"`
	Status        string              `json:"status"`
	Owner         string              `json:"owner"`
	Spec          string              `json:"spec"`
	SourceOfTruth string              `json:"source_of_truth"`
	Description   string              `json:"description"`
	DefaultDeny   string              `json:"default_deny"`
	UnknownOp     string              `json:"unknown_operation"`
	Requirements  map[string][]string `json:"requirements"`
}

// RequirementsFor returns the capabilities an operation needs.
//
// The bool reports whether the operation is known. An unknown operation is
// never returned as "needs nothing": the caller gets the same refusal the
// runtime gate gives, because a validator that answered "no requirement" for a
// typo would be a validator that approves typos.
func (t *RequirementTable) RequirementsFor(operation string) (names []string, known bool) {
	req, ok := t.Requirements[operation]
	if !ok {
		return nil, false
	}
	return append([]string(nil), req...), true
}

// Operations lists every gated operation, sorted.
func (t *RequirementTable) Operations() []string {
	out := make([]string, 0, len(t.Requirements))
	for op := range t.Requirements {
		out = append(out, op)
	}
	sort.Strings(out)
	return out
}

// RequireCapability reports whether an operation may run given a grant set.
//
// It is the contract-side restatement of the runtime gate: an operation needs
// its requirement satisfied, an unknown operation is refused, and holding extra
// capabilities is harmless. The names are normalised first, so a manifest may
// spell a grant "Filesystem".
func (t *RequirementTable) RequireCapability(operation string, granted []string) error {
	required, known := t.RequirementsFor(operation)
	if !known {
		return fmt.Errorf("capability %q refused: unknown operation (%s)", operation, t.UnknownOp)
	}

	have := make(map[string]bool, len(granted))
	for _, name := range granted {
		if canonical := langcap.NormalizeName(name); canonical != "" {
			have[canonical] = true
		}
	}
	for _, need := range required {
		if !have[need] {
			return fmt.Errorf("capability %q refused: requires %q, which the program was not granted", operation, need)
		}
	}
	return nil
}

// CheckCapabilities validates a manifest's declared capability list against the
// vocabulary, collecting every problem rather than stopping at the first.
func (s *Set) CheckCapabilities(declared []string) Violations {
	var vs Violations
	seen := make(map[string]bool, len(declared))

	for _, raw := range declared {
		canonical, err := s.vocab.Canonicalize(raw)
		if err != nil {
			vs.Add(StageCapability, "permissions", raw, err.Error())
			continue
		}
		if seen[canonical] {
			vs.Add(StageCapability, "permissions", raw,
				fmt.Sprintf("duplicate capability: %q and an earlier entry both normalise to %q", raw, canonical))
		}
		seen[canonical] = true
	}

	return vs
}
