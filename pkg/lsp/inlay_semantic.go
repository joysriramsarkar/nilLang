package lsp

import (
	"regexp"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/token"
)

// Inlay hints and semantic tokens (L9.1), minimal and honest.

// InlayHint is a type annotation the editor renders inline.
type InlayHint struct {
	Position Position `json:"position"`
	Label    string   `json:"label"`
	Kind     int      `json:"kind,omitempty"` // 1 = type, 2 = parameter
}

var letLiteralPattern = regexp.MustCompile(`^\s*(?:let|const|state)\s+([\p{L}_][\p{L}\p{N}_]*)\s*(?::\s*[\w\[\]?|<>,\s]+)?\s*=\s*(.+?);?\s*$`)

// ComputeInlayHints annotates untyped `let x = <literal>` bindings with the
// literal's evident type. Bindings that already carry an annotation, or whose
// value is not a literal, get no hint — a hint that guesses is worse than
// none, because the editor renders it as fact.
func ComputeInlayHints(content string) []InlayHint {
	var out []InlayHint
	for i, ln := range strings.Split(content, "\n") {
		m := letLiteralPattern.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		// Skip when an explicit type annotation is present.
		decl := ln[:strings.Index(ln, "=")]
		if strings.Contains(decl, ":") {
			continue
		}
		typ := literalType(strings.TrimSpace(m[2]))
		if typ == "" {
			continue
		}
		out = append(out, InlayHint{
			Position: Position{Line: i, Character: len(ln)},
			Label:    ": " + typ,
			Kind:     1,
		})
	}
	return out
}

func literalType(expr string) string {
	switch {
	case expr == "true" || expr == "false":
		return "Bool"
	case expr == "null":
		return "null"
	case isIntLiteral(expr):
		return "Int"
	case isFloatLiteral(expr):
		return "Float"
	case len(expr) >= 2 && expr[0] == '"' && expr[len(expr)-1] == '"':
		return "String"
	case len(expr) >= 2 && expr[0] == '[':
		return "Array"
	case len(expr) >= 2 && expr[0] == '{':
		return "Hash"
	default:
		return ""
	}
}

func isIntLiteral(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && !(i == 0 && (c == '-' || c == '+')) {
			return false
		}
	}
	return true
}

func isFloatLiteral(s string) bool {
	dots := 0
	digits := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			digits = true
		case c == '.' && dots == 0:
			dots++
		case i == 0 && (c == '-' || c == '+'):
		default:
			return false
		}
	}
	return digits && dots == 1
}

// SemanticTokens legend indices (fixed order, shared with the client).
var SemanticTokenTypes = []string{
	"keyword", "string", "number", "comment", "variable", "function", "type",
}

// SemanticTokensResult is the LSP-encoded token stream (delta positions).
type SemanticTokensResult struct {
	Data []int `json:"data"`
}

// ComputeSemanticTokens lexes the document and encodes keywords, strings,
// numbers and comments. Identifiers are deliberately left unclassified: the
// lexer cannot tell a variable from a function, and a wrong classification
// colours a call as a declaration.
func ComputeSemanticTokens(content string) SemanticTokensResult {
	type tok struct {
		line, col, length, typ int
	}
	var toks []tok
	l := lexer.New(content)
	for {
		t := l.NextToken()
		if t.Type == token.EOF {
			break
		}
		tt := -1
		length := len(t.Literal)
		switch t.Type {
		case token.STRING:
			tt = 1
			length += 2 // Literal is the inner content; the quotes are on the line too.
		case token.INT, token.FLOAT:
			tt = 2
		}
		if tt >= 0 {
			toks = append(toks, tok{t.Line - 1, t.Column - 1, length, tt})
			continue
		}
		if isKeywordType(t.Type) {
			toks = append(toks, tok{t.Line - 1, t.Column - 1, len(t.Literal), 0})
		}
	}
	// Encode deltas; tokens must be ordered by (line, col).
	data := make([]int, 0, len(toks)*5)
	prevLine, prevCol := 0, 0
	for i, tk := range toks {
		dLine := tk.line - prevLine
		dCol := tk.col
		if dLine == 0 {
			dCol = tk.col - prevCol
		}
		if dLine < 0 || (dLine == 0 && dCol < 0) {
			continue
		}
		data = append(data, dLine, dCol, tk.length, tk.typ, 0)
		prevLine, prevCol = tk.line, tk.col
		_ = i
	}
	return SemanticTokensResult{Data: data}
}

func isKeywordType(t token.TokenType) bool {
	switch t {
	case token.LET, token.CONST, token.FN, token.IF, token.ELSE,
		token.WHILE, token.FOR, token.RETURN, token.IMPORT, token.EXPORT,
		token.TRUE, token.FALSE, token.NULL, token.TASK, token.AWAIT,
		token.BREAK, token.CONT, token.IN:
		return true
	default:
		return false
	}
}
