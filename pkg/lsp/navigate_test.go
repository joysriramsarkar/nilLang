package lsp

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

const navDoc = "let counter = 0;\nfn inc() {\n  counter = counter + 1;\n}\nprintln(counter);\n"

func TestDefinitionFindsDeclaration(t *testing.T) {
	// `counter` on line 2 (0-indexed) must resolve to line 0.
	loc := ComputeDefinition(navDoc, Position{Line: 2, Character: 3}, "file:///a.nil")
	if loc == nil {
		t.Fatal("definition returned nil")
	}
	if loc.URI != "file:///a.nil" {
		t.Errorf("wrong URI: %s", loc.URI)
	}
	if loc.Range.Start.Line != 0 {
		t.Errorf("expected declaration on line 0, got %+v", loc.Range)
	}
	// Unknown name: no declaration anywhere.
	if got := ComputeDefinition(navDoc, Position{Line: 4, Character: 2}, "file:///a.nil"); got != nil {
		t.Errorf("println is not declared; expected nil, got %+v", got)
	}
	// Keyword with no declaration: nil, not a guess.
	if got := ComputeDefinition(navDoc, Position{Line: 0, Character: 1}, "file:///a.nil"); got != nil {
		t.Errorf("`let` is undeclared; expected nil, got %+v", got)
	}
}

func TestReferencesIncludeDeclAndUses(t *testing.T) {
	refs := ComputeReferences(navDoc, Position{Line: 0, Character: 5}, "file:///a.nil")
	if len(refs) != 4 { // decl + line2 use x2 + line4 use
		t.Fatalf("expected 4 references to counter, got %d: %+v", len(refs), refs)
	}
	for _, r := range refs {
		if r.URI != "file:///a.nil" {
			t.Errorf("wrong URI in reference: %s", r.URI)
		}
	}
}

func TestRenameProducesOneEditPerReference(t *testing.T) {
	edits := ComputeRename(navDoc, Position{Line: 0, Character: 5}, "total")
	if len(edits) != 4 {
		t.Fatalf("expected 4 edits, got %d", len(edits))
	}
	for _, e := range edits {
		if e.NewText != "total" {
			t.Errorf("wrong replacement: %q", e.NewText)
		}
	}
	if got := ComputeRename(navDoc, Position{Line: 0, Character: 5}, ""); got != nil {
		t.Errorf("empty new name must yield nil, got %+v", got)
	}
}

func TestInlayHintsAnnotateLiterals(t *testing.T) {
	doc := "let a = 5;\nlet b: Int = 6;\nlet s = \"hi\";\nlet f = fn() { return 1; };\n"
	hints := ComputeInlayHints(doc)
	labels := map[int]string{}
	for _, h := range hints {
		labels[h.Position.Line] = h.Label
	}
	if labels[0] != ": Int" {
		t.Errorf("line 0 hint: got %q", labels[0])
	}
	if _, ok := labels[1]; ok {
		t.Errorf("annotated binding must get no hint, got %q", labels[1])
	}
	if labels[2] != ": String" {
		t.Errorf("line 2 hint: got %q", labels[2])
	}
	if _, ok := labels[3]; ok {
		t.Errorf("non-literal value must get no hint, got %q", labels[3])
	}
}

func TestSemanticTokensEncode(t *testing.T) {
	res := ComputeSemanticTokens("let a = 5;\nprintln(\"hi\");\n")
	if len(res.Data)%5 != 0 {
		t.Fatalf("data length must be a multiple of 5, got %d", len(res.Data))
	}
	if len(res.Data) == 0 {
		t.Fatal("expected tokens for keywords/numbers/strings")
	}
	types := map[int]bool{}
	for i := 3; i < len(res.Data); i += 5 {
		types[res.Data[i]] = true
	}
	for _, want := range []int{0, 1, 2} { // keyword, string, number
		if !types[want] {
			t.Errorf("token type %d (%s) missing", want, SemanticTokenTypes[want])
		}
	}
}

func TestDiagnosticsCarryCodes(t *testing.T) {
	// Parse-error path must carry a stable E-code (L9.2). validateDocument
	// writes a framed notification to the server's writer; read it back.
	var buf strings.Builder
	_ = buf
	r, w := io.Pipe()
	var out bytes.Buffer
	s := NewServer(r, &out)
	s.validateDocument("file:///bad.nil", "let = ;\n", nil)
	_ = w
	raw := out.String()
	if !strings.Contains(raw, "E0001") {
		t.Errorf("syntax diagnostic must carry E0001, got: %s", raw)
	}
	if !strings.Contains(raw, "publishDiagnostics") {
		t.Errorf("expected publishDiagnostics notification, got: %s", raw)
	}
}
