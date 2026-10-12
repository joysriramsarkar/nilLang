package lsp

import (
	"regexp"
	"strings"
)

// Symbol navigation (L9.1): go-to-definition, references, rename.
//
// The implementation is deliberately line-based rather than AST-based: the
// language server must answer even while the file does not parse (the common
// case mid-keystroke), and an AST position is only meaningful for a file
// that compiled. A declaration is a line matching
// `let|const|fn|entity|component|state NAME`; a reference is any
// word-boundary occurrence of NAME.

var declPattern = regexp.MustCompile(`^\s*(?:let|const|fn|entity|component|state|struct|enum|trait|type)\s+([\p{L}_][\p{L}\p{N}_]*)`)

func isWordChar(r rune) bool { return isIdentChar(r) }

// declarationOf returns the 0-indexed line and name range declaring word, or
// -1 when the file declares no such name.
func declarationOf(content, word string) (line int, start, end int, found bool) {
	for i, ln := range strings.Split(content, "\n") {
		m := declPattern.FindStringSubmatchIndex(ln)
		if m == nil {
			continue
		}
		name := ln[m[2]:m[3]]
		if name == word {
			return i, m[2], m[3], true
		}
	}
	return 0, 0, 0, false
}

// occurrences returns every word-boundary occurrence of word with ranges.
func occurrences(content, word string) []Range {
	var out []Range
	for i, ln := range strings.Split(content, "\n") {
		for col := 0; col < len(ln); {
			idx := strings.Index(ln[col:], word)
			if idx < 0 {
				break
			}
			s := col + idx
			e := s + len(word)
			beforeOK := s == 0 || !isWordChar(rune(ln[s-1]))
			afterOK := e >= len(ln) || !isWordChar(rune(ln[e]))
			col = e
			if !beforeOK || !afterOK {
				continue
			}
			out = append(out, Range{
				Start: Position{Line: i, Character: s},
				End:   Position{Line: i, Character: e},
			})
		}
	}
	return out
}

// ComputeDefinition resolves the word under pos to its declaration.
func ComputeDefinition(content string, pos Position, uri string) *Location {
	word, _ := GetWordAtPosition(content, pos.Line, pos.Character)
	if word == "" {
		return nil
	}
	line, s, e, found := declarationOf(content, word)
	if !found {
		return nil
	}
	return &Location{
		URI: uri,
		Range: Range{
			Start: Position{Line: line, Character: s},
			End:   Position{Line: line, Character: e},
		},
	}
}

// ComputeReferences returns the declaration plus every use of the word.
func ComputeReferences(content string, pos Position, uri string) []Location {
	word, _ := GetWordAtPosition(content, pos.Line, pos.Character)
	if word == "" {
		return nil
	}
	var out []Location
	for _, r := range occurrences(content, word) {
		out = append(out, Location{URI: uri, Range: r})
	}
	return out
}

// ComputeRename turns every reference into a TextEdit replacing it.
func ComputeRename(content string, pos Position, newName string) []TextEdit {
	word, _ := GetWordAtPosition(content, pos.Line, pos.Character)
	if word == "" || newName == "" {
		return nil
	}
	var out []TextEdit
	for _, r := range occurrences(content, word) {
		out = append(out, TextEdit{Range: r, NewText: newName})
	}
	return out
}
