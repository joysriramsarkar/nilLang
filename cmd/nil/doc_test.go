package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDocFindBuiltin(t *testing.T) {
	entry, found := docFind("len")
	if !found {
		t.Fatal("expected to find builtin len")
	}
	if entry.Kind != "BUILTIN" {
		t.Fatalf("expected kind BUILTIN, got %q", entry.Kind)
	}
	if !strings.Contains(docMarkdown(entry), "`len`") {
		t.Fatal("expected markdown to name len")
	}
}

func TestDocFindStdModule(t *testing.T) {
	for _, name := range []string{"std/pos", "pos", "std/crypto/aes", "std/text/bn", "std/mobile"} {
		entry, found := docFind(name)
		if !found {
			t.Fatalf("expected to find module %q", name)
		}
		if entry.Kind != "MODULE" {
			t.Fatalf("%q: expected kind MODULE, got %q", name, entry.Kind)
		}
	}
	if _, found := docFind("std/does-not-exist"); found {
		t.Fatal("expected unknown module to be missing")
	}
}

func TestDocJSONRoundTrip(t *testing.T) {
	entries := docEntries()
	if len(entries) < 60 {
		t.Fatalf("expected at least 60 entries, got %d", len(entries))
	}
	// Sorted by name for deterministic output.
	for i := 1; i < len(entries); i++ {
		if entries[i-1].Name > entries[i].Name {
			t.Fatalf("entries not sorted: %q after %q", entries[i].Name, entries[i-1].Name)
		}
	}
	var decoded []docEntry
	if err := json.Unmarshal([]byte(docJSON(entries)), &decoded); err != nil {
		t.Fatalf("doc JSON does not round-trip: %v", err)
	}
	if len(decoded) != len(entries) {
		t.Fatalf("round trip lost entries: %d vs %d", len(decoded), len(entries))
	}
}

func TestDocIndexMentionsStdlib(t *testing.T) {
	index := docIndexMarkdown()
	for _, want := range []string{"std/pos", "std/crypto", "std/mobile", "len", "nil doc"} {
		if !strings.Contains(index, want) {
			t.Fatalf("expected index to mention %q", want)
		}
	}
}
