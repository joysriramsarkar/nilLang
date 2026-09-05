package diagnostics

import (
	"fmt"
	"strings"
)

type Severity string

const (
	SeverityError   Severity = "ERROR"
	SeverityWarning Severity = "WARNING"
	SeverityInfo    Severity = "INFO"
	SeverityHint    Severity = "HINT"
)

type Span struct {
	Filename  string
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
}

type Diagnostic struct {
	Code          string
	Severity      Severity
	Message       string
	Span          Span
	ContextLine   string
	Suggestion    string
	AIExplanation string
}

func (d *Diagnostic) String() string {
	return d.Format(false)
}

func (d *Diagnostic) Format(useColor bool) string {
	var b strings.Builder

	prefix := string(d.Severity)
	if useColor {
		switch d.Severity {
		case SeverityError:
			prefix = "\033[1;31mERROR\033[0m"
		case SeverityWarning:
			prefix = "\033[1;33mWARNING\033[0m"
		case SeverityInfo:
			prefix = "\033[1;34mINFO\033[0m"
		case SeverityHint:
			prefix = "\033[1;36mHINT\033[0m"
		}
	}

	loc := ""
	if d.Span.Filename != "" || d.Span.StartLine > 0 {
		loc = fmt.Sprintf("%s:%d:%d: ", d.Span.Filename, d.Span.StartLine, d.Span.StartCol)
	}

	codeTag := ""
	if d.Code != "" {
		codeTag = fmt.Sprintf("[%s] ", d.Code)
	}

	b.WriteString(fmt.Sprintf("%s%s%s%s\n", loc, prefix, ": ", codeTag+d.Message))

	if d.ContextLine != "" {
		lineNum := fmt.Sprintf("%4d | ", d.Span.StartLine)
		b.WriteString(lineNum)
		b.WriteString(d.ContextLine)
		b.WriteString("\n")

		indent := len(lineNum) + d.Span.StartCol - 1
		if indent < len(lineNum) {
			indent = len(lineNum)
		}
		b.WriteString(strings.Repeat(" ", indent))

		length := d.Span.EndCol - d.Span.StartCol
		if length <= 0 {
			length = 1
		}
		pointer := strings.Repeat("^", length)
		if useColor {
			pointer = "\033[1;31m" + pointer + "\033[0m"
		}
		b.WriteString(pointer)
		b.WriteString("\n")
	}

	if d.Suggestion != "" {
		hintPrefix := "  = help: "
		if useColor {
			hintPrefix = "\033[1;32m  = help: \033[0m"
		}
		b.WriteString(hintPrefix)
		b.WriteString(d.Suggestion)
		b.WriteString("\n")
	}

	if d.AIExplanation != "" {
		aiPrefix := "  = ai truth: "
		if useColor {
			aiPrefix = "\033[1;35m  = ai truth: \033[0m"
		}
		b.WriteString(aiPrefix)
		b.WriteString(d.AIExplanation)
		b.WriteString("\n")
	}

	return b.String()
}

// DiagnosticCatalog provides standard descriptions for compiler error codes
var DiagnosticCatalog = map[string]string{
	"E0101": "Type Mismatch: Expression type does not conform to the expected target type.",
	"E0102": "Undefined Symbol: The requested identifier is not found in the current or enclosing lexical scopes.",
	"E0103": "Arity Mismatch: Function called with incorrect number of arguments.",
	"E0201": "Capability Violation: Operation requires a system capability not declared in the project manifest.",
	"E0202": "Effect Disallowed: Function performs side-effects (IO, mutation, async) within a declared pure context.",
	"E0301": "AI Hallucination Detected: Referenced component member does not exist in standard truth tables.",
}

// ExplainCode returns human and AI explanation for a given code
func ExplainCode(code string) string {
	if exp, ok := DiagnosticCatalog[code]; ok {
		return exp
	}
	return "Compiler diagnostic."
}
