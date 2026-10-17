package diagnostics

import (
	"fmt"
	"strings"
)

type Severity string

// CodeLoweringFailed is reported when a program parses and type checks but a
// construct cannot be translated into an executable form.
//
// It is a source error, not an internal fault: `break` outside a loop is
// something a developer writes and needs told about precisely.
const CodeLoweringFailed = "E0005"

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
		if indent > 256 {
			indent = 256
		}
		b.WriteString(strings.Repeat(" ", indent))

		length := d.Span.EndCol - d.Span.StartCol
		if length <= 0 {
			length = 1
		}
		if length > 256 {
			length = 256
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
	"E0000": "Internal Compiler Error: the toolchain itself failed on checked source. This is a bug in the compiler, not in the program — please report it with the source attached.",
	"E0001": "Unexpected Token: A character or token appeared where the grammar does not allow it.",
	"E0002": "Unterminated String: A string literal is missing its closing quote.",
	"E0003": "Unexpected EOF: The file ended in the middle of a statement, expression, or block.",
	"E0004": "Syntax Error: A required token is missing, or an unexpected token appears in its place.",
	"E0005": "Lowering Failed: The source parses and type checks, but a construct could not be translated into executable form, such as `break` outside a loop.",
	"E0101": "Type Mismatch: Expression type does not conform to the expected target type.",
	"E0102": "Undefined Symbol: The requested identifier is not found in the current or enclosing lexical scopes.",
	"E0103": "Missing Initializer: Variable requires an initializer until definite assignment is supported.",
	"E0104": "Const Mutation: Cannot assign to constant.",
	"E0105": "Arity Mismatch: Function called with incorrect number of arguments.",
	"E0106": "Non Callable Type: Attempted to call an expression whose type is not a function.",
	"E0107": "Invalid Member Access: Accessing non-existent field or member on struct or object.",
	"E0108": "Incompatible Binary Op: Binary operator applied to incompatible operand types.",
	"E0201": "Capability Violation: Operation requires a system capability not declared in the project manifest.",
	"E0202": "Effect Disallowed: Function performs side-effects (IO, mutation, async) within a declared pure context.",
	"E0301": "Circular Dependency: Module import graph contains a cycle.",
	"E0302": "Module Not Found: No module exists at the requested import path.",
	"E0303": "Symbol Not Exported: The imported module does not export the requested symbol.",
	"E0401": "Invalid Entity Name: Entity declaration must carry a valid identifier name.",
	"E0402": "Duplicate Entity: An entity with this name is already declared in scope.",
	"E0403": "Duplicate Entity Field: The entity declares the same field name twice.",
	"E0404": "Invalid Entity Field Type: The declared field type could not be resolved.",
	"E0405": "Multiple Primary Keys: An entity may declare at most one primary key.",
	"E0406": "Invalid Component Name: Component declaration must have a name.",
	"E0502": "Unknown Type: Unknown type name in a parameter, return, field, or alias annotation.",
	"E0110": "Width Overflow: An integer literal does not fit the width it was written against.",
	"E0111": "Unknown Member: A member chain names something the receiver's declared type does not have.",
	"E0112": "Not Narrowable: A narrowing builtin applied to a value that is neither a `Result` nor an optional.",
	"E0113": "Width Mismatch: A numeric width used where a different width or Int is expected, with no implicit conversion.",
	"E0203": "Unsafe Without Block: Unsafe operation invoked without `unsafe` block.",
	"E0204": "Generic Mismatch: A call cannot satisfy a generic signature's structure.",
	"E0501": "Duplicate Declaration: A type name, enum variant, trait, or trait method redeclared.",
	"E0503": "Unknown Field: Unknown field on a struct.",
	"E0504": "Duplicate Field: Duplicate field in a struct literal.",
	"E0505": "Unknown Variant: Unknown enum variant in a `match` or a call.",
	"E0506": "Missing Field: Missing field in a struct literal.",
	"E0507": "Non-exhaustive Match: Non-exhaustive `match` over an enum, `bool`, `Result`, or `Optional`.",
	"E0508": "Duplicate Arm: Duplicate `match` arm.",
	"E0509": "Variant Arity: Wrong number of arguments for an enum variant.",
	"E0510": "Pattern Arity: Wrong arity for a `match` pattern.",
	"E0511": "Unbound Variable: Variable not bound in the arm that would read it.",
	"E0601": "Impl Missing Method: An `impl` does not provide a method its trait declares.",
	"E0602": "Impl Extra Method: An `impl` provides a method its trait does not declare.",
	"E0603": "Unresolved Method: A method call cannot be resolved on the receiver's type.",
}

// ExplainCode returns human and AI explanation for a given code
func ExplainCode(code string) string {
	if exp, ok := DiagnosticCatalog[code]; ok {
		return exp
	}
	return "Compiler diagnostic."
}
