package capability

import (
	"errors"
	"fmt"
	"strings"
)

// Code is the stable diagnostic code reported when an operation runs without
// the capabilities it requires.
//
// It matches ERROR_MODEL.md's capability band so that a runtime refusal and a
// compile time refusal are recognisable as the same class of problem.
const Code = "E0201"

// DiagnosticCode is the code carried by this specific error.
func (e *DeniedError) DiagnosticCode() string {
	return Code
}

// DeniedError reports that an operation was refused for lack of a capability.
//
// It is returned rather than panicking, and carries enough structure for a
// front end to render a precise message or to map it onto an LSP diagnostic.
type DeniedError struct {
	// Operation is the builtin or std module function that was refused.
	Operation string

	// Required lists the capabilities the operation needs.
	Required Requirement

	// Granted lists the capabilities the program actually holds.
	Granted Requirement

	// Reason distinguishes an outright unknown operation from a missing grant.
	Reason string
}

func (e *DeniedError) Error() string {
	var b strings.Builder
	b.WriteString("capability denied: ")
	b.WriteString(e.Operation)
	b.WriteString(" requires [")
	b.WriteString(e.Required.String())
	b.WriteString("]")
	if len(e.Granted) > 0 {
		b.WriteString(" but the program was granted [")
		b.WriteString(e.Granted.String())
		b.WriteString("]")
	}
	if e.Reason != "" {
		b.WriteString(": ")
		b.WriteString(e.Reason)
	}
	return b.String()
}

// Suggestion returns actionable help text for the refusal.
func (e *DeniedError) Suggestion() string {
	return fmt.Sprintf(
		"add the %s capability to the manifest, for example {\"capabilities\": [%s]}",
		e.Required.String(), quoteAll(e.Required))
}

func quoteAll(req Requirement) string {
	quoted := make([]string, 0, len(req))
	for _, name := range req {
		quoted = append(quoted, fmt.Sprintf("%q", name))
	}
	return strings.Join(quoted, ", ")
}

// NewDeniedError builds a refusal for an operation whose requirements are not
// satisfied.
func NewDeniedError(operation string, required, granted Requirement, reason string) *DeniedError {
	return &DeniedError{
		Operation: operation,
		Required:  required,
		Granted:   granted,
		Reason:    reason,
	}
}

// AsDenied extracts a DeniedError from err, if there is one.
//
// It unwraps, so a refusal that an engine wrapped in a categorised
// object.RuntimeError is still recognised. The VM does exactly that: the
// category satisfies the cross-engine error contract, and the wrapped cause
// keeps this extraction working.
func AsDenied(err error) (*DeniedError, bool) {
	var denied *DeniedError
	if errors.As(err, &denied) {
		return denied, true
	}
	return nil, false
}
