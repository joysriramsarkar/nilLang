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

// Server is a DAP adapter over stdio (Content-Length framed JSON).
type Server struct {
	r  *bufio.Reader
	w  io.Writer
	mu sync.Mutex
	seq int

	sourceFile string
	source     string
	machine    *vm.VM
	ds         *vm.DebugState

	resumeCh chan struct{}
	stopCh   chan struct{}
	exited   chan struct{}

	started    bool
	stopReason string
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

func (s *Server) write(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintf(s.w, "Content-Length: %d\r\n\r\n", len(data))
	s.w.Write(data)
}

func (s *Server) nextSeq() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	return s.seq
}

func (s *Server) respond(reqSeq int, command string, success bool, body any, message string) {
	msg := map[string]any{
		"seq": s.nextSeq(), "type": "response", "request_seq": reqSeq,
		"success": success, "command": command,
	}
	if body != nil {
		msg["body"] = body
	}
	if message != "" {
		msg["message"] = message
	}
	s.write(msg)
}

func (s *Server) event(name string, body any) {
	msg := map[string]any{"seq": s.nextSeq(), "type": "event", "event": name}
	if body != nil {
		msg["body"] = body
	}
	s.write(msg)
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
			close(s.stopCh)
			return err
		}
		command, _ := msg["command"].(string)
		reqSeq := intOf(msg["seq"])
		args, _ := msg["arguments"].(map[string]any)

		switch command {
		case "initialize":
			s.respond(reqSeq, command, true, map[string]any{
				"capabilities": map[string]any{
					"supportsConfigurationDoneRequest": true,
					"supportsEvaluateForHovers":        true,
					"supportsFunctionBreakpoints":      true,
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
			lines := breakpointLines(args)
			s.ds.SetBreakpoints(src, lines)
			out := make([]map[string]any, 0, len(lines))
			for i, ln := range lines {
				out = append(out, map[string]any{
					"id": i + 1, "verified": validLine(s.source, ln), "line": ln,
					"source": map[string]any{"name": src, "path": src},
				})
			}
			s.respond(reqSeq, command, true, map[string]any{"breakpoints": out}, "")
		case "setExceptionBreakpoints":
			s.respond(reqSeq, command, true, map[string]any{"breakpoints": []any{}}, "")
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
			close(s.stopCh)
			s.respond(reqSeq, command, true, nil, "")
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

func breakpointLines(args map[string]any) []int {
	var out []int
	if bps, ok := args["breakpoints"].([]any); ok {
		for _, b := range bps {
			if m, ok := b.(map[string]any); ok {
				out = append(out, intOf(m["line"]))
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
