package parser

import (
	"fmt"

	"github.com/joysriramsarkar/nilLang/compiler/diagnostics"
	"github.com/joysriramsarkar/nilLang/compiler/token"
)

// Syntactic diagnostic codes, per docs/spec/ERROR_MODEL.md §2 ("Lexical &
// Syntactic", E0001–E0099). Every parser failure must carry one of these so
// that editors, the LSP, and `nil check` can react to a stable identifier
// instead of parsing an English message.
const (
	CodeUnexpectedToken = "E0001" // unexpected character or invalid token
	CodeUnterminatedStr = "E0002" // unterminated string literal
	CodeUnexpectedEOF   = "E0003" // unexpected end of file
	CodeSyntaxError     = "E0004" // expected token X, got Y
)

// Syntax-level codes that belong to a later grammar family but are detected
// while parsing rather than while type checking.
const (
	CodeEntityDuplicateField = "E0403"
	CodeComponentDuplicate   = "E0406"

	// Declarative type declarations: struct, enum, trait, match.
	CodeDuplicateType    = "E0501"
	CodeUnknownType      = "E0502"
	CodeUnknownField     = "E0503"
	CodeDuplicateField   = "E0504"
	CodeUnknownVariant   = "E0505"
	CodeMissingField     = "E0506"
	CodeNonExhaustive    = "E0507"
	CodeNonStructLiteral = "E0508"
	CodeNonEnumVariant   = "E0509"
	CodeWrongArity       = "E0510"
	CodeUndeclaredField  = "E0511"
)

// SetFilename records the source file name used in diagnostic spans.
// Front ends should call this immediately after New so that editor tooling
// can navigate from a diagnostic back to the file.
func (p *Parser) SetFilename(name string) {
	p.filename = name
}

// Diagnostics returns the structured parse diagnostics in source order.
//
// This is the machine-readable counterpart of Errors(). Errors() is retained
// for the CLI's existing human-readable output and for the conformance suite;
// new front ends should prefer Diagnostics().
func (p *Parser) Diagnostics() []*diagnostics.Diagnostic {
	return p.diagnostics
}

// recordError appends a structured diagnostic plus its legacy string form.
//
// The string form keeps the historic "line L:C: message" shape so that no
// existing test or CLI output changes shape, while the diagnostic gives the
// stable code and a navigable span that tooling can rely on.
func (p *Parser) recordError(code string, tok token.Token, msg, suggestion string) {
	p.errors = append(p.errors, fmt.Sprintf("line %d:%d: %s", tok.Line, tok.Column, msg))

	span := diagnostics.Span{
		Filename:  p.filename,
		StartLine: tok.Line,
		StartCol:  tok.Column,
		EndLine:   tok.Line,
		EndCol:    tok.Column + tokenDisplayWidth(tok),
	}
	if span.EndCol <= span.StartCol {
		span.EndCol = span.StartCol + 1
	}

	p.diagnostics = append(p.diagnostics, &diagnostics.Diagnostic{
		Code:          code,
		Severity:      diagnostics.SeverityError,
		Message:       msg,
		Span:          span,
		ContextLine:   p.lineFor(tok.Line),
		Suggestion:    suggestion,
		AIExplanation: diagnostics.ExplainCode(code),
	})
}

// tokenDisplayWidth returns how many columns a token should underline.
// EOF and ILLEGAL carry no literal text, so they get a single caret.
func tokenDisplayWidth(tok token.Token) int {
	if tok.Literal == "" {
		return 1
	}
	n := len([]rune(tok.Literal))
	if n == 0 {
		return 1
	}
	return n
}

// lineFor returns the 1-indexed source line, or "" when unavailable.
func (p *Parser) lineFor(line int) string {
	if line <= 0 || line > len(p.sourceLines) {
		return ""
	}
	return p.sourceLines[line-1]
}

// captureSource stores the source text so diagnostics can quote the offending
// line. Front ends call this once; without it spans are still correct but the
// rendered diagnostic omits the context line.
func (p *Parser) captureSource(src string) {
	if src == "" {
		return
	}
	p.sourceLines = splitLines(src)
}

func splitLines(src string) []string {
	var out []string
	start := 0
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			out = append(out, src[start:i])
			start = i + 1
		}
	}
	out = append(out, src[start:])
	return out
}
