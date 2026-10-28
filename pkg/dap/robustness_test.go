package dap

import (
	"bytes"
	"strings"
	"testing"
)

// A header line that never terminates must be refused at the cap instead
// of being buffered without bound; the 8 MiB body cap does not cover the
// header block.
func TestDAPRefusesOverlongHeaderLine(t *testing.T) {
	var out bytes.Buffer
	s := NewServerWithIO("main.nil", "let x = 1;", strings.NewReader(strings.Repeat("X", maxHeaderLineBytes+4096)), &out)
	err := s.Serve()
	if err == nil {
		t.Fatal("an over-long header line was accepted")
	}
	if !strings.Contains(err.Error(), "header line") {
		t.Fatalf("unexpected error: %v", err)
	}
}
