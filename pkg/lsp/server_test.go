package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Helper to write a request with Content-Length header
func writeLSPMessage(w io.Writer, payload string) {
	fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(payload), payload)
}

// Helper to read an LSP message
func readLSPMessage(br *bufio.Reader) ([]byte, error) {
	contentLength := 0
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[0]), "Content-Length") {
			contentLength, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
		}
	}
	body := make([]byte, contentLength)
	_, err := io.ReadFull(br, body)
	return body, err
}

func TestLSPLifecycleAndFeatures(t *testing.T) {
	clientToServerR, clientToServerW := io.Pipe()
	serverToClientR, serverToClientW := io.Pipe()

	server := NewServer(clientToServerR, serverToClientW)

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.Serve()
	}()

	clientReader := bufio.NewReader(serverToClientR)

	// 1. Initialize
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	writeLSPMessage(clientToServerW, initReq)

	initRespBytes, err := readLSPMessage(clientReader)
	if err != nil {
		t.Fatalf("failed to read initialize response: %v", err)
	}

	var initResp Response
	if err := json.Unmarshal(initRespBytes, &initResp); err != nil {
		t.Fatalf("failed to unmarshal init response: %v", err)
	}
	if initResp.Error != nil {
		t.Fatalf("initialize error: %v", initResp.Error)
	}

	// 2. Open Document (with deliberate type error: undeclared identifier assignment)
	openNotif := `{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"file:///test.nil","languageId":"nil","version":1,"text":"x = 10;\n"}}}`
	writeLSPMessage(clientToServerW, openNotif)

	// Expect publishDiagnostics notification
	diagBytes, err := readLSPMessage(clientReader)
	if err != nil {
		t.Fatalf("failed to read diagnostics notification: %v", err)
	}

	var diagNotif Notification
	if err := json.Unmarshal(diagBytes, &diagNotif); err != nil {
		t.Fatalf("failed to unmarshal diagnostics notification: %v", err)
	}
	if diagNotif.Method != "textDocument/publishDiagnostics" {
		t.Fatalf("expected textDocument/publishDiagnostics, got %s", diagNotif.Method)
	}

	// 3. Hover Request
	hoverReq := `{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":"file:///test.nil"},"position":{"line":0,"character":1}}}`
	writeLSPMessage(clientToServerW, hoverReq)

	hoverRespBytes, err := readLSPMessage(clientReader)
	if err != nil {
		t.Fatalf("failed to read hover response: %v", err)
	}
	var hoverResp Response
	if err := json.Unmarshal(hoverRespBytes, &hoverResp); err != nil {
		t.Fatalf("failed to unmarshal hover response: %v", err)
	}
	if hoverResp.Error != nil {
		t.Fatalf("hover error: %v", hoverResp.Error)
	}

	// 4. Change Document to valid code
	changeNotif := `{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":"file:///test.nil","version":2},"contentChanges":[{"text":"let  a=  5 ; let b =  10 ;\n"}]}}`
	writeLSPMessage(clientToServerW, changeNotif)

	// Read diagnostics for changed document
	_, err = readLSPMessage(clientReader)
	if err != nil {
		t.Fatalf("failed to read change diagnostics: %v", err)
	}

	// 5. Formatting Request
	fmtReq := `{"jsonrpc":"2.0","id":3,"method":"textDocument/formatting","params":{"textDocument":{"uri":"file:///test.nil"}}}`
	writeLSPMessage(clientToServerW, fmtReq)

	fmtRespBytes, err := readLSPMessage(clientReader)
	if err != nil {
		t.Fatalf("failed to read format response: %v", err)
	}
	var fmtResp Response
	if err := json.Unmarshal(fmtRespBytes, &fmtResp); err != nil {
		t.Fatalf("failed to unmarshal format response: %v", err)
	}
	if fmtResp.Error != nil {
		t.Fatalf("format error: %v", fmtResp.Error)
	}

	// 6. Completion Request
	compReq := `{"jsonrpc":"2.0","id":4,"method":"textDocument/completion","params":{"textDocument":{"uri":"file:///test.nil"},"position":{"line":0,"character":2}}}`
	writeLSPMessage(clientToServerW, compReq)

	compRespBytes, err := readLSPMessage(clientReader)
	if err != nil {
		t.Fatalf("failed to read completion response: %v", err)
	}
	var compResp Response
	if err := json.Unmarshal(compRespBytes, &compResp); err != nil {
		t.Fatalf("failed to unmarshal completion response: %v", err)
	}
	if compResp.Error != nil {
		t.Fatalf("completion error: %v", compResp.Error)
	}

	// 7. Shutdown
	shutdownReq := `{"jsonrpc":"2.0","id":5,"method":"shutdown","params":{}}`
	writeLSPMessage(clientToServerW, shutdownReq)

	shutRespBytes, err := readLSPMessage(clientReader)
	if err != nil {
		t.Fatalf("failed to read shutdown response: %v", err)
	}
	var shutResp Response
	if err := json.Unmarshal(shutRespBytes, &shutResp); err != nil {
		t.Fatalf("failed to unmarshal shutdown response: %v", err)
	}

	// 8. Exit
	exitNotif := `{"jsonrpc":"2.0","method":"exit","params":{}}`
	writeLSPMessage(clientToServerW, exitNotif)

	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("server exited with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not exit within timeout")
	}
}

func TestLSPUnknownMethodReturnsMethodNotFound(t *testing.T) {
	clientToServerR, clientToServerW := io.Pipe()
	serverToClientR, serverToClientW := io.Pipe()

	server := NewServer(clientToServerR, serverToClientW)

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.Serve()
	}()

	clientReader := bufio.NewReader(serverToClientR)

	// Send an unknown method request
	unknownReq := `{"jsonrpc":"2.0","id":99,"method":"custom/unknownMethod","params":{}}`
	writeLSPMessage(clientToServerW, unknownReq)

	respBytes, err := readLSPMessage(clientReader)
	if err != nil {
		t.Fatalf("failed to read unknown method response: %v", err)
	}

	var resp Response
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error == nil || resp.Error.Code != -32601 {
		t.Fatalf("expected code -32601 (Method not found), got %+v", resp.Error)
	}

	// Exit
	writeLSPMessage(clientToServerW, `{"jsonrpc":"2.0","method":"exit","params":{}}`)
	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("server exited with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not exit within timeout")
	}
}
