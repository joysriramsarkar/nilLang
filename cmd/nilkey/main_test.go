package main

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/signing"
)

func TestChooseKeyID(t *testing.T) {
	keys := []*signing.KeyInfo{
		{KeyID: "11111111111111111111111111111111", Owner: "Alice"},
		{KeyID: "22222222222222222222222222222222", Owner: "Bob"},
	}

	// Explicit selection wins
	id, err := chooseKeyID(keys, "22222222222222222222222222222222")
	if err != nil || id != "22222222222222222222222222222222" {
		t.Fatalf("explicit selection failed: %v %q", err, id)
	}

	// Unknown key must error
	if _, err := chooseKeyID(keys, "33333333333333333333333333333333"); err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}

	// Multiple keys without a request must error (never guess)
	if _, err := chooseKeyID(keys, ""); err == nil {
		t.Fatal("expected error for ambiguous selection, got nil")
	}

	// Single key is used automatically
	single := keys[:1]
	id, err = chooseKeyID(single, "")
	if err != nil || id != "11111111111111111111111111111111" {
		t.Fatalf("single-key selection failed: %v %q", err, id)
	}
}
