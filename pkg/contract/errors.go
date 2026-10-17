package contract

import (
	"fmt"
	"strings"
)

// Validation stages, in the order the contracts document requires.
//
// The order is load-bearing. Archive safety runs first because a validator that
// unpacks before checking paths is the vulnerability; identity runs before
// integrity because a digest of the wrong object is a meaningless answer; and
// trust runs before compatibility because "this package is not for this device"
// is a better message than "this key is not trusted" when both are true.
const (
	// StageArchive covers path safety, duplicate entries and size limits.
	StageArchive = 1
	// StageIdentity covers schema version and identity fields.
	StageIdentity = 2
	// StageIntegrity covers payload checksums.
	StageIntegrity = 3
	// StageTrust covers signer, key validity and signature verification.
	StageTrust = 4
	// StageCompatibility covers runtime ABI, backend and target.
	StageCompatibility = 5
	// StageCapability covers the declared capability set.
	StageCapability = 6
	// StageInstall covers install policy and launch verification.
	StageInstall = 7
)

// StageName returns the human name of a validation stage.
func StageName(stage int) string {
	switch stage {
	case StageArchive:
		return "archive-safety"
	case StageIdentity:
		return "identity"
	case StageIntegrity:
		return "integrity"
	case StageTrust:
		return "trust"
	case StageCompatibility:
		return "compatibility"
	case StageCapability:
		return "capability"
	case StageInstall:
		return "install"
	default:
		return "unknown"
	}
}

// Violation is a single contract failure.
//
// It carries the stage so that a caller can tell "this package is malformed"
// from "this package is not for this device" — the two produce very different
// responses from a user and very different responses from a package manager.
type Violation struct {
	// Stage is the validation stage that failed.
	Stage int
	// Field is the manifest member or archive entry at fault.
	Field string
	// Reason is a human sentence naming what was wrong.
	Reason string
	// Value is the offending value, when it is short enough to print.
	Value string
}

func (v *Violation) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "contract: stage %d (%s): ", v.Stage, StageName(v.Stage))
	if v.Field != "" {
		fmt.Fprintf(&b, "%s: ", v.Field)
	}
	b.WriteString(v.Reason)
	if v.Value != "" {
		fmt.Fprintf(&b, " (got %q)", v.Value)
	}
	return b.String()
}

// Violations is a list of failures from one validation pass.
//
// Validation collects rather than stops: a package that is both malformed and
// untrusted should say so once, not make the user fix issues one at a time.
type Violations []Violation

func (vs Violations) Error() string {
	lines := make([]string, 0, len(vs))
	for _, v := range vs {
		lines = append(lines, v.Error())
	}
	if len(lines) == 1 {
		return lines[0]
	}
	return fmt.Sprintf("contract: %d violations:\n  %s", len(lines), strings.Join(lines, "\n  "))
}

// Add records a violation.
func (vs *Violations) Add(stage int, field, value, reason string) {
	*vs = append(*vs, Violation{Stage: stage, Field: field, Value: value, Reason: reason})
}

// OrNil returns nil when there are no violations, and the list otherwise.
//
// Every caller gets the same nil-vs-non-nil shape, so a verifier that forgets to
// check still refuses rather than proceeding with a nil error it treated as
// success.
func (vs Violations) OrNil() error {
	if len(vs) == 0 {
		return nil
	}
	return vs
}

// HasStage reports whether any violation came from a given stage.
func (vs Violations) HasStage(stage int) bool {
	for _, v := range vs {
		if v.Stage == stage {
			return true
		}
	}
	return false
}
