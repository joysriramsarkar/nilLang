package dap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestNilDAPProcess builds the real `nil` binary and drives a DAP session
// against `nil dap <file>` as a child process. This proves the whole chain —
// compiler position table, VM debug hooks, DAP server and the CLI wiring —
// works end to end, not just in-process.
//
// Skip with NIL_DAP_PROCESS_TEST=skip.
func TestNilDAPProcess(t *testing.T) {
	if os.Getenv("NIL_DAP_PROCESS_TEST") == "skip" {
		t.Skip("skipping process-level DAP test")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go toolchain not available: %v", err)
	}

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	work := t.TempDir()

	binName := "nil"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(work, binName)
	build := exec.Command(goBin, "build", "-o", binPath, "./cmd/nil")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build nil: %v\n%s", err, out)
	}

	srcPath := filepath.Join(work, "main.nil")
	src := "let a = 1;\nlet b = 2;\nlet c = a + b;\nc;\n"
	if err := os.WriteFile(srcPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := exec.Command(binPath, "dap", srcPath)
	srv.Dir = work
	stdin, err := srv.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := srv.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	srv.Stderr = os.Stderr
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = srv.Process.Kill()
		_ = srv.Wait()
	}()

	msgs := readMessages(stdout)
	send := func(seq int, command string, args map[string]any) {
		data, _ := json.Marshal(map[string]any{
			"seq": seq, "type": "request", "command": command, "arguments": args,
		})
		fmt.Fprintf(stdin, "Content-Length: %d\r\n\r\n%s", len(data), data)
	}

	gotCaps := false
	gotBreakpointLine3 := false
	globals := map[string]string{}
	var failures []string

	send(1, "initialize", map[string]any{})
	send(2, "launch", map[string]any{"program": srcPath})
	send(3, "setBreakpoints", map[string]any{
		"source":      map[string]any{"path": srcPath},
		"breakpoints": []any{map[string]any{"line": 3}},
	})
	send(4, "configurationDone", nil)
	send(5, "continue", map[string]any{"threadId": 1})

	// Generous timeouts: this test builds the real `nil` binary and drives a
	// DAP session against it as a child process. On a cold CI runner the
	// build alone can take tens of seconds, and the session handshake
	// (initialize → launch → setBreakpoints → configurationDone → continue)
	// crosses several process boundaries. A tight timeout here fails a
	// working chain for being observed slowly.
	drain(t, msgs, 90*time.Second, func(m map[string]any) bool {
		switch m["type"] {
		case "response":
			if m["success"] != true {
				failures = append(failures, fmt.Sprintf("%v: %v", m["command"], m["message"]))
			}
			if body, ok := m["body"].(map[string]any); ok {
				if _, ok := body["capabilities"]; ok {
					gotCaps = true
				}
			}
		case "event":
			if m["event"] == "stopped" {
				if b, ok := m["body"].(map[string]any); ok &&
					b["reason"] == "breakpoint" &&
					intOf(b["line"]) == 3 {
					gotBreakpointLine3 = true
					return true
				}
			}
		}
		return false
	})

	// Read the program state at the breakpoint.
	send(6, "variables", map[string]any{"variablesReference": 2})
	if !drain(t, msgs, dapStepBudget, func(m map[string]any) bool {
		if m["type"] != "response" || m["command"] != "variables" {
			return false
		}
		if body, ok := m["body"].(map[string]any); ok {
			if vars, ok := body["variables"].([]any); ok {
				for _, v := range vars {
					vm, _ := v.(map[string]any)
					globals[vm["name"].(string)] = vm["value"].(string)
				}
			}
		}
		return true
	}) {
		// Naming the request: without this the failure is a bare timeout and
		// the next reader opens the debug adapter looking for a fault that is
		// really a budget.
		t.Fatalf("the adapter did not answer the variables request within %s", dapStepBudget)
	}

	send(7, "disconnect", nil)
	if !drain(t, msgs, dapStepBudget, func(m map[string]any) bool {
		return m["type"] == "response" && m["command"] == "disconnect"
	}) {
		t.Fatalf("the adapter did not acknowledge disconnect within %s", dapStepBudget)
	}
	_ = stdin.Close()

	if len(failures) > 0 {
		t.Errorf("failed requests: %v", failures)
	}
	if !gotCaps {
		t.Error("initialize response missing capabilities")
	}
	if !gotBreakpointLine3 {
		t.Error("never stopped at breakpoint on line 3")
	}
	if globals["a"] != "1" || globals["b"] != "2" {
		t.Errorf("globals at breakpoint wrong: %v", globals)
	}
}

// readMessages turns a DAP stream into a channel of decoded messages.
func readMessages(r io.Reader) <-chan map[string]any {
	out := make(chan map[string]any, 64)
	go func() {
		defer close(out)
		br := bufio.NewReader(r)
		for {
			length := -1
			for {
				line, err := br.ReadString('\n')
				if err != nil {
					return
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
				return
			}
			buf := make([]byte, length)
			if _, err := io.ReadFull(br, buf); err != nil {
				return
			}
			var m map[string]any
			if err := json.Unmarshal(buf, &m); err != nil {
				return
			}
			out <- m
		}
	}()
	return out
}

// dapStepBudget is how long one DAP request may go unanswered.
//
// # Why it is generous
//
// The session this budget covers does about a second of real work: start a child
// process, set a breakpoint, run to it, read variables, disconnect. Thirty
// seconds was already thirty times that, and it still was not enough — the test
// failed five times out of five on a machine also running six CPU-bound
// processes, and passed in under three seconds on an idle one.
//
// A budget that only holds when nothing else is running does not test the debug
// adapter; it tests how idle the machine is. The number here is chosen so the
// test measures the adapter under contention, which is the condition CI runs it
// in.
//
// # Why there is a bound at all
//
// An unbounded wait turns a broken session into a hung test that holds a runner
// and reports nothing. Failing slowly is the price of failing informatively, and
// that is still the right trade: drain names the step it gave up on, so the
// report says which request went unanswered rather than just "timeout".
const dapStepBudget = 90 * time.Second

// drain reads messages until pred matches or the timeout elapses.
//
// A false return means the step was not answered in time; the caller is expected
// to say which one, because "timed out" on its own sends the next reader to look
// at the adapter instead of at the budget.
func drain(t *testing.T, msgs <-chan map[string]any, timeout time.Duration, pred func(map[string]any) bool) bool {
	t.Helper()
	if timeout <= 0 || timeout > dapStepBudget {
		timeout = dapStepBudget
	}
	end := time.Now().Add(timeout)
	for {
		remain := time.Until(end)
		if remain <= 0 {
			return false
		}
		select {
		case m, ok := <-msgs:
			if !ok {
				return false
			}
			if pred(m) {
				return true
			}
		case <-time.After(remain):
			return false
		}
	}
}
