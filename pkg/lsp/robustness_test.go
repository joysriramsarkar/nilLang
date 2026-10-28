package lsp

import (
	"io"
	"strings"
	"testing"
	"time"
)

// A header line that never terminates must be refused at the cap instead
// of being buffered without bound (ReadString had no limit). The body cap
// alone did not cover the header block.
func TestFramingRefusesOverlongHeaderLine(t *testing.T) {
	clientToServerR, clientToServerW := io.Pipe()
	serverToClientR, serverToClientW := io.Pipe()
	defer serverToClientR.Close()

	server := NewServer(clientToServerR, serverToClientW)
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.Serve()
	}()

	// One "header line" far beyond the cap, no newline. Written from a
	// goroutine: the pipe is unbuffered and the server stops reading once
	// it refuses, so this writer may block — that is fine.
	go func() {
		_, _ = io.WriteString(clientToServerW, strings.Repeat("X", maxHeaderLineBytes+4096))
	}()

	select {
	case err := <-serverDone:
		if err == nil {
			t.Fatal("an over-long header line was accepted")
		}
		if !strings.Contains(err.Error(), "header line") {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not refuse the over-long header line")
	}
}
