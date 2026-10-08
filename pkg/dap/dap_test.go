package dap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// dapClient is a minimal DAP client over a TCP connection.
type dapClient struct {
	conn net.Conn
	r    *bufio.Reader
}

func dialDAP(t *testing.T, sourceFile, source string) *dapClient {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan net.Conn, 1)
	go func() {
		c, err := ln.Accept()
		if err == nil {
			accepted <- c
		}
	}()
	cli, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	srv := <-accepted
	ln.Close()

	s := NewServerWithIO(sourceFile, source, srv, srv)
	go func() { _ = s.Serve() }()
	t.Cleanup(func() { srv.Close(); cli.Close() })
	return &dapClient{conn: cli, r: bufio.NewReader(cli)}
}

func (c *dapClient) send(t *testing.T, seq int, command string, args map[string]any) {
	t.Helper()
	msg := map[string]any{"seq": seq, "type": "request", "command": command}
	if args != nil {
		msg["arguments"] = args
	}
	data, _ := json.Marshal(msg)
	fmt.Fprintf(c.conn, "Content-Length: %d\r\n\r\n%s", len(data), data)
}

func (c *dapClient) read(t *testing.T) map[string]any {
	t.Helper()
	c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	length := -1
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			t.Fatalf("read header: %v", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			length, _ = strconv.Atoi(strings.TrimSpace(line[len("content-length:"):]))
		}
	}
	if length < 0 {
		t.Fatal("missing content-length")
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(c.r, buf); err != nil {
		t.Fatalf("read body: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

// waitForResponse reads until the response for reqSeq arrives, buffering any
// events it encounters.
func (c *dapClient) waitForResponse(t *testing.T, reqSeq int) (map[string]any, []map[string]any) {
	t.Helper()
	var events []map[string]any
	for i := 0; i < 50; i++ {
		m := c.read(t)
		if m["type"] == "event" {
			events = append(events, m)
			continue
		}
		if m["type"] == "response" && int(m["request_seq"].(float64)) == reqSeq {
			return m, events
		}
	}
	t.Fatalf("no response for seq %d", reqSeq)
	return nil, nil
}

func (c *dapClient) waitForEvent(t *testing.T, name string) map[string]any {
	t.Helper()
	for i := 0; i < 50; i++ {
		m := c.read(t)
		if m["type"] == "event" && m["event"] == name {
			return m
		}
	}
	t.Fatalf("event %q not seen", name)
	return nil
}

func body(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	b, _ := resp["body"].(map[string]any)
	if b == nil {
		t.Fatalf("response has no body: %+v", resp)
	}
	return b
}

const testProgram = "let a = 1;\nlet b = 2;\nlet c = a + b;\nc;"

func TestDAPRealVMBreakpointAndVariables(t *testing.T) {
	c := dialDAP(t, "test.nil", testProgram)

	c.send(t, 1, "initialize", map[string]any{})
	resp, _ := c.waitForResponse(t, 1)
	if resp["success"] != true {
		t.Fatalf("initialize failed: %+v", resp)
	}
	if _, ok := body(t, resp)["capabilities"].(map[string]any); !ok {
		t.Fatalf("initialize response missing capabilities: %+v", resp)
	}
	c.waitForEvent(t, "initialized")

	c.send(t, 2, "setBreakpoints", map[string]any{
		"source":      map[string]any{"path": "test.nil"},
		"breakpoints": []any{map[string]any{"line": 3}},
	})
	resp, _ = c.waitForResponse(t, 2)
	bps, _ := body(t, resp)["breakpoints"].([]any)
	if len(bps) != 1 {
		t.Fatalf("expected 1 breakpoint, got %+v", bps)
	}
	if bp := bps[0].(map[string]any); bp["verified"] != true {
		t.Fatalf("breakpoint not verified: %+v", bp)
	}

	c.send(t, 3, "configurationDone", nil)
	if resp, _ := c.waitForResponse(t, 3); resp["success"] != true {
		t.Fatal("configurationDone failed")
	}
	// The VM stops at the entry point first.
	entry := c.waitForEvent(t, "stopped")
	if entryBody, _ := entry["body"].(map[string]any); entryBody["reason"] != "entry" {
		t.Fatalf("expected entry stop, got %+v", entryBody)
	}

	c.send(t, 4, "continue", nil)
	c.waitForResponse(t, 4)
	stop := c.waitForEvent(t, "stopped")
	stopBody, _ := stop["body"].(map[string]any)
	if stopBody["reason"] != "breakpoint" || int(stopBody["line"].(float64)) != 3 {
		t.Fatalf("expected breakpoint at line 3, got %+v", stopBody)
	}

	c.send(t, 5, "stackTrace", map[string]any{"threadId": 1})
	resp, _ = c.waitForResponse(t, 5)
	frames, _ := body(t, resp)["stackFrames"].([]any)
	if len(frames) == 0 {
		t.Fatal("no stack frames")
	}
	f0 := frames[0].(map[string]any)
	if f0["name"] != "<main>" || int(f0["line"].(float64)) != 3 {
		t.Fatalf("unexpected top frame: %+v", f0)
	}

	c.send(t, 6, "variables", map[string]any{"variablesReference": 2})
	resp, _ = c.waitForResponse(t, 6)
	vars, _ := body(t, resp)["variables"].([]any)
	found := map[string]string{}
	for _, v := range vars {
		m := v.(map[string]any)
		found[m["name"].(string)] = m["value"].(string)
	}
	if found["a"] != "1" || found["b"] != "2" {
		t.Fatalf("globals wrong at breakpoint: %+v", found)
	}

	c.send(t, 7, "evaluate", map[string]any{"expression": "a"})
	resp, _ = c.waitForResponse(t, 7)
	if body(t, resp)["result"] != "1" {
		t.Fatalf("evaluate a: %+v", body(t, resp))
	}

	c.send(t, 8, "disconnect", nil)
	c.waitForResponse(t, 8)
}

func TestDAPStepAndFunctionFrame(t *testing.T) {
	src := "fn inc(x) {\n  let y = x + 1;\n  return y;\n}\nlet r = inc(41);\nr;"
	c := dialDAP(t, "func.nil", src)

	c.send(t, 1, "initialize", nil)
	c.waitForResponse(t, 1)
	c.waitForEvent(t, "initialized")

	c.send(t, 2, "setBreakpoints", map[string]any{
		"source":      map[string]any{"path": "func.nil"},
		"breakpoints": []any{map[string]any{"line": 2}},
	})
	c.waitForResponse(t, 2)

	c.send(t, 3, "configurationDone", nil)
	c.waitForResponse(t, 3)
	c.waitForEvent(t, "stopped") // entry

	c.send(t, 4, "continue", nil)
	c.waitForResponse(t, 4)
	stop := c.waitForEvent(t, "stopped")
	stopBody, _ := stop["body"].(map[string]any)
	if int(stopBody["line"].(float64)) != 2 {
		t.Fatalf("expected to stop at line 2, got %+v", stopBody)
	}

	c.send(t, 5, "stackTrace", map[string]any{"threadId": 1})
	resp, _ := c.waitForResponse(t, 5)
	frames, _ := body(t, resp)["stackFrames"].([]any)
	if len(frames) < 2 {
		t.Fatalf("expected nested frames, got %+v", frames)
	}
	if frames[0].(map[string]any)["name"] != "inc" {
		t.Fatalf("expected innermost frame inc, got %+v", frames[0])
	}

	c.send(t, 6, "evaluate", map[string]any{"expression": "x"})
	resp, _ = c.waitForResponse(t, 6)
	if body(t, resp)["result"] != "41" {
		t.Fatalf("evaluate x: %+v", body(t, resp))
	}

	// Single-step should advance to line 3.
	c.send(t, 7, "next", nil)
	c.waitForResponse(t, 7)
	stop = c.waitForEvent(t, "stopped")
	stopBody, _ = stop["body"].(map[string]any)
	if l := int(stopBody["line"].(float64)); l < 2 {
		t.Fatalf("step went backwards: %+v", stopBody)
	}

	c.send(t, 8, "disconnect", nil)
	c.waitForResponse(t, 8)
}
