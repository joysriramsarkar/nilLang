// Package dap implements a Debug Adapter Protocol server for Nilang programs,
// backed by the real NABC virtual machine's debug hooks (breakpoints, single
// stepping, call stack and variable inspection).
package dap

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
	pkgcompiler "github.com/joysriramsarkar/nilLang/pkg/compiler"
)

// maxMessageBytes caps a single framed message. The length comes from the
// wire, so allocating it blindly lets one header exhaust memory.
const maxMessageBytes = 8 << 20 // 8 MiB

// Server is a DAP adapter over stdio (Content-Length framed JSON).
type Server struct {
	r   *bufio.Reader
	w   io.Writer
	mu  sync.Mutex
	seq int

	stopOnce sync.Once
	writeErr error // first transport write failure, surfaced by Serve

	sourceFile string
	source     string
	machine    *vm.VM
	ds         *vm.DebugState

	resumeCh chan struct{}
	stopCh   chan struct{}
	exited   chan struct{}

	started    bool
	stopReason string

	// conditions maps breakpoint lines to their condition expressions
	// (L9.3). Only single-identifier truthiness and `name ==/!= literal`
	// comparisons are evaluated; anything else is rejected at set-time.
	conditions map[int]string
	// exceptionFilters holds the active setExceptionBreakpoints filters.
	exceptionFilters []string
}

// NewServer creates a DAP server for the given source file/source text using
// os.Stdin/os.Stdout.
func NewServer(sourceFile, source string) *Server {
	return NewServerWithIO(sourceFile, source, os.Stdin, os.Stdout)
}

// NewServerWithIO creates a DAP server over arbitrary streams (used by tests).
func NewServerWithIO(sourceFile, source string, r io.Reader, w io.Writer) *Server {
	return &Server{
		r:          bufio.NewReader(r),
		w:          w,
		sourceFile: sourceFile,
		source:     source,
		resumeCh:   make(chan struct{}, 1),
		stopCh:     make(chan struct{}),
		exited:     make(chan struct{}),
	}
}

// ─── framing ─────────────────────────────────────────────────────────────

func (s *Server) read() (map[string]any, error) {
	length := -1
	for {
		line, err := s.r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			v := strings.TrimSpace(line[len("content-length:"):])
			n, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("bad Content-Length %q", v)
			}
			length = n
		}
	}
	if length < 0 {
		return nil, errors.New("missing Content-Length")
	}
	if length > maxMessageBytes {
		return nil, fmt.Errorf("Content-Length %d exceeds the %d byte limit", length, maxMessageBytes)
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(s.r, buf); err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(buf, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// stop signals the runVM goroutine to exit. Disconnect and a read failure can
// race; closing the channel twice panics, so the close happens once.
func (s *Server) stop() {
	s.stopOnce.Do(func() { close(s.stopCh) })
}

// emit stamps the sequence number, frames and writes one message atomically.
// Sequence assignment and the write share the lock so the runVM goroutine and
// the serve loop cannot interleave sequence numbers out of order. The first
// transport failure sticks and is surfaced by Serve.
func (s *Server) emit(msg map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.writeErr != nil {
		return
	}
	s.seq++
	msg["seq"] = s.seq
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	if _, err := fmt.Fprintf(s.w, "Content-Length: %d\r\n\r\n", len(data)); err != nil {
		s.writeErr = err
		return
	}
	if _, err := s.w.Write(data); err != nil {
		s.writeErr = err
	}
}

func (s *Server) stickyWriteErr() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeErr
}

func (s *Server) respond(reqSeq int, command string, success bool, body any, message string) {
	msg := map[string]any{
		"type": "response", "request_seq": reqSeq,
		"success": success, "command": command,
	}
	if body != nil {
		msg["body"] = body
	}
	if message != "" {
		msg["message"] = message
	}
	s.emit(msg)
}

func (s *Server) event(name string, body any) {
	msg := map[string]any{"type": "event", "event": name}
	if body != nil {
		msg["body"] = body
	}
	s.emit(msg)
}

// ─── program load ────────────────────────────────────────────────────────

func (s *Server) compileProgram() error {
	res := frontend.ParseAndCheck(s.source)
	if len(res.ParseErrors) > 0 {
		return fmt.Errorf("parse errors: %v", res.ParseErrors)
	}
	if !res.OK {
		msgs := make([]string, 0, len(res.Diagnostics))
		for _, d := range res.Diagnostics {
			msgs = append(msgs, d.String())
		}
		return fmt.Errorf("type errors: %s", strings.Join(msgs, "; "))
	}
	comp := compiler.New()
	comp.SourceFile = s.sourceFile
	if dir := filepath.Dir(s.sourceFile); dir != "" && dir != "." {
		pkgcompiler.ConfigureCompiler(comp, dir)
	}
	if err := comp.Compile(res.Program); err != nil {
		return fmt.Errorf("compile: %w", err)
	}
	s.machine = vm.New(comp.Bytecode())
	s.ds = vm.NewDebugState()
	s.machine.Debug = s.ds
	return nil
}

// ─── run loop ────────────────────────────────────────────────────────────

func (s *Server) startVM() {
	if s.started || s.machine == nil || s.ds == nil {
		return
	}
	s.started = true
	// Stop at the entry point first.
	s.stopReason = "entry"
	s.ds.Step()
	go s.runVM()
}

func (s *Server) runVM() {
	defer close(s.exited)
	for {
		err := s.machine.Run()
		if errors.Is(err, vm.ErrPaused) {
			loc := s.machine.Location()
			// L9.3: conditional breakpoint — a false condition resumes
			// silently instead of reporting a stop. Continue() skips the
			// current location so the same breakpoint does not re-trigger.
			if cond, ok := s.conditions[loc.Line]; ok && cond != "" {
				if s.evalCondition(cond) {
					// fall through to the stop report below
				} else {
					s.ds.Continue()
					continue
				}
			}
			reason := s.stopReason
			if reason == "" {
				reason = "breakpoint"
			}
			s.stopReason = ""
			s.event("stopped", map[string]any{
				"reason": reason, "threadId": 1, "allThreadsStopped": true,
				"line": loc.Line, "column": loc.Column,
			})
			select {
			case <-s.resumeCh:
			case <-s.stopCh:
				return
			}
			continue
		}
		if err != nil {
			if s.exceptionStop() {
				s.event("stopped", map[string]any{
					"reason": "exception", "threadId": 1, "allThreadsStopped": true,
					"text": err.Error(),
				})
				select {
				case <-s.resumeCh:
				case <-s.stopCh:
					return
				}
			}
			s.event("output", map[string]any{"category": "stderr", "output": err.Error() + "\n"})
			s.event("exited", map[string]any{"exitCode": 1})
			return
		}
		s.event("exited", map[string]any{"exitCode": 0})
		return
	}
}

func (s *Server) resume() {
	select {
	case s.resumeCh <- struct{}{}:
	default:
	}
}

// ─── dispatch ────────────────────────────────────────────────────────────

// Serve runs the DAP loop until disconnect or EOF.
func (s *Server) Serve() error {
	for {
		msg, err := s.read()
		if err != nil {
			s.stop()
			return err
		}
		command, _ := msg["command"].(string)
		reqSeq := intOf(msg["seq"])
		args, _ := msg["arguments"].(map[string]any)

		switch command {
		case "initialize":
			s.respond(reqSeq, command, true, map[string]any{
				"capabilities": map[string]any{
					"supportsConfigurationDoneRequest":  true,
					"supportsEvaluateForHovers":         true,
					"supportsFunctionBreakpoints":       true,
					"supportsConditionalBreakpoints":    true,
					"supportsHitConditionalBreakpoints": false,
					"exceptionBreakpointFilters": []map[string]any{
						{"filter": "all", "label": "All errors", "default": false},
						{"filter": "uncaught", "label": "Uncaught errors", "default": false},
					},
				},
				"serverInfo": map[string]any{"name": "nil dap", "version": "1.0.0"},
			}, "")
			s.event("initialized", nil)
		case "launch", "attach":
			if s.machine == nil {
				if err := s.compileProgram(); err != nil {
					s.respond(reqSeq, command, false, nil, err.Error())
					continue
				}
			}
			s.respond(reqSeq, command, true, nil, "")
		case "configurationDone":
			s.respond(reqSeq, command, true, nil, "")
			s.startVM()
		case "setBreakpoints":
			if s.machine == nil {
				if err := s.compileProgram(); err != nil {
					s.respond(reqSeq, command, false, nil, err.Error())
					continue
				}
			}
			src := sourcePath(args)
			bps := breakpointSpecs(args)
			lines := make([]int, 0, len(bps))
			for _, b := range bps {
				lines = append(lines, b.line)
			}
			s.ds.SetBreakpoints(src, lines)
			// L9.3: record per-line conditions; a condition the evaluator
			// cannot handle marks its breakpoint unverified with a message
			// rather than failing silently at pause time.
			if s.conditions == nil {
				s.conditions = map[int]string{}
			} else {
				for k := range s.conditions {
					delete(s.conditions, k)
				}
			}
			out := make([]map[string]any, 0, len(bps))
			for i, b := range bps {
				entry := map[string]any{
					"id": i + 1, "line": b.line,
					"source": map[string]any{"name": src, "path": src},
				}
				if b.condition == "" {
					entry["verified"] = validLine(s.source, b.line)
				} else if validCondition(b.condition) {
					s.conditions[b.line] = b.condition
					entry["verified"] = validLine(s.source, b.line)
				} else {
					entry["verified"] = false
					entry["message"] = "unsupported condition: use a variable name or `name ==/!= literal`"
				}
				out = append(out, entry)
			}
			s.respond(reqSeq, command, true, map[string]any{"breakpoints": out}, "")
		case "setExceptionBreakpoints":
			filters := exceptionFilters(args)
			s.exceptionFilters = filters
			out := make([]map[string]any, 0, len(filters))
			for _, f := range filters {
				out = append(out, map[string]any{"filter": f, "verified": true})
			}
			s.respond(reqSeq, command, true, map[string]any{"breakpoints": out}, "")
		case "threads":
			s.respond(reqSeq, command, true, map[string]any{"threads": []map[string]any{{"id": 1, "name": "main"}}}, "")
		case "stackTrace":
			if s.machine == nil {
				s.respond(reqSeq, command, false, nil, "program not loaded")
				continue
			}
			frames := s.machine.CallStack()
			out := make([]map[string]any, 0, len(frames))
			for _, f := range frames {
				out = append(out, map[string]any{
					"id": f.Index + 1, "name": f.Function, "line": f.Line, "column": f.Column,
					"source": map[string]any{"name": filepath.Base(f.File), "path": f.File},
				})
			}
			s.respond(reqSeq, command, true, map[string]any{"stackFrames": out, "totalFrames": len(out)}, "")
		case "scopes":
			s.respond(reqSeq, command, true, map[string]any{"scopes": []map[string]any{
				{"name": "Locals", "variablesReference": 1, "expensive": false},
				{"name": "Globals", "variablesReference": 2, "expensive": false},
				{"name": "Captured", "variablesReference": 3, "expensive": false},
			}}, "")
		case "variables":
			if s.machine == nil {
				s.respond(reqSeq, command, false, nil, "program not loaded")
				continue
			}
			var vars []vm.Variable
			switch intOf(args["variablesReference"]) {
			case 1:
				vars = s.machine.Locals()
			case 2:
				vars = s.machine.Globals()
			case 3:
				vars = s.machine.FreeVars()
			}
			out := make([]map[string]any, 0, len(vars))
			for _, v := range vars {
				out = append(out, map[string]any{"name": v.Name, "value": v.Value, "type": v.Type, "variablesReference": 0})
			}
			s.respond(reqSeq, command, true, map[string]any{"variables": out}, "")
		case "evaluate":
			if s.machine == nil {
				s.respond(reqSeq, command, false, nil, "program not loaded")
				continue
			}
			expr, _ := args["expression"].(string)
			if v, ok := s.machine.EvaluateName(strings.TrimSpace(expr)); ok {
				s.respond(reqSeq, command, true, map[string]any{"result": v.Value, "type": v.Type, "variablesReference": 0}, "")
			} else {
				s.respond(reqSeq, command, true, map[string]any{"result": "(nil)", "variablesReference": 0}, "")
			}
		case "continue":
			if s.ds != nil {
				s.stopReason = "breakpoint"
				s.ds.Continue()
				s.resume()
			}
			s.respond(reqSeq, command, true, map[string]any{"allThreadsContinued": true}, "")
		case "next":
			if s.ds != nil {
				s.stopReason = "step"
				s.ds.Step()
				s.resume()
			}
			s.respond(reqSeq, command, true, nil, "")
		case "stepIn":
			if s.ds != nil {
				s.stopReason = "step"
				s.ds.Step()
				s.resume()
			}
			s.respond(reqSeq, command, true, nil, "")
		case "stepOut":
			if s.ds != nil {
				s.stopReason = "step"
				s.ds.Step()
				s.resume()
			}
			s.respond(reqSeq, command, true, nil, "")
		case "pause":
			if s.ds != nil {
				s.stopReason = "pause"
				s.ds.Pause()
			}
			s.respond(reqSeq, command, true, nil, "")
		case "disconnect", "terminate":
			s.stop()
			s.respond(reqSeq, command, true, nil, "")
			if err := s.stickyWriteErr(); err != nil {
				return err
			}
			return nil
		default:
			s.respond(reqSeq, command, false, nil, "unsupported command: "+command)
		}
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────

func sourcePath(args map[string]any) string {
	if src, ok := args["source"].(map[string]any); ok {
		if p, ok := src["path"].(string); ok && p != "" {
			return p
		}
		if n, ok := src["name"].(string); ok && n != "" {
			return n
		}
	}
	return "main.nil"
}

type breakpointSpec struct {
	line      int
	condition string
}

func breakpointSpecs(args map[string]any) []breakpointSpec {
	var out []breakpointSpec
	if bps, ok := args["breakpoints"].([]any); ok {
		for _, b := range bps {
			if m, ok := b.(map[string]any); ok {
				spec := breakpointSpec{line: intOf(m["line"])}
				if c, ok := m["condition"].(string); ok {
					spec.condition = strings.TrimSpace(c)
				}
				out = append(out, spec)
			}
		}
	}
	return out
}

func breakpointLines(args map[string]any) []int {
	var out []int
	for _, b := range breakpointSpecs(args) {
		out = append(out, b.line)
	}
	return out
}

// validCondition accepts a bare identifier or `name == literal` /
// `name != literal`. Anything richer is refused at set-time (L9.3) so the
// client sees the limit instead of a breakpoint that never fires.
func validCondition(cond string) bool {
	cond = strings.TrimSpace(cond)
	if cond == "" {
		return true
	}
	for _, op := range []string{"==", "!="} {
		if parts := strings.SplitN(cond, op, 2); len(parts) == 2 {
			return isIdent(strings.TrimSpace(parts[0])) && strings.TrimSpace(parts[1]) != ""
		}
	}
	return isIdent(cond)
}

func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9' && i > 0) {
			continue
		}
		return false
	}
	return true
}

// evalCondition evaluates a validated condition against the paused VM.
func (s *Server) evalCondition(cond string) bool {
	for _, op := range []string{"==", "!="} {
		if parts := strings.SplitN(cond, op, 2); len(parts) == 2 {
			name := strings.TrimSpace(parts[0])
			want := strings.TrimSpace(parts[1])
			v, ok := s.machine.EvaluateName(name)
			if !ok {
				return false
			}
			eq := strings.TrimSpace(v.Value) == want ||
				strings.Trim(strings.TrimSpace(v.Value), `"`) == strings.Trim(want, `"`)
			if op == "!=" {
				return !eq
			}
			return eq
		}
	}
	v, ok := s.machine.EvaluateName(strings.TrimSpace(cond))
	if !ok {
		return false
	}
	switch strings.TrimSpace(v.Value) {
	case "", "0", "false", "null", "(nil)":
		return false
	default:
		return true
	}
}

// exceptionStop reports whether a failing run should first surface as an
// exception stop (L9.3): any active filter ("all" or "uncaught") enables it,
// because the VM has no catch boundary — every error is uncaught.
func (s *Server) exceptionStop() bool { return len(s.exceptionFilters) > 0 }

func exceptionFilters(args map[string]any) []string {
	var out []string
	if fs, ok := args["filters"].([]any); ok {
		for _, f := range fs {
			if name, ok := f.(string); ok && name != "" {
				out = append(out, name)
			}
		}
	}
	return out
}

func validLine(source string, line int) bool {
	if line <= 0 {
		return false
	}
	return line <= strings.Count(source, "\n")+1
}

func intOf(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}
