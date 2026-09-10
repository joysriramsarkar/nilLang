package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/joysriramsarkar/nilLang/compiler/diagnostics"
	"github.com/joysriramsarkar/nilLang/compiler/formatter"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/compiler/typecheck"
)

type Document struct {
	URI     string
	Version int
	Content string
}

type Server struct {
	reader io.Reader
	writer io.Writer
	mu     sync.Mutex // guards writes to writer

	docsMu    sync.RWMutex
	documents map[string]*Document

	isShutdown bool
	running    bool
}

func NewServer(r io.Reader, w io.Writer) *Server {
	return &Server{
		reader:    r,
		writer:    w,
		documents: make(map[string]*Document),
	}
}

// Serve starts the JSON-RPC read/eval/respond loop
func (s *Server) Serve() error {
	s.running = true
	br := bufio.NewReader(s.reader)

	for s.running {
		// Read headers
		contentLength := 0
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				// End of headers
				break
			}
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				headerKey := strings.TrimSpace(parts[0])
				headerVal := strings.TrimSpace(parts[1])
				if strings.EqualFold(headerKey, "Content-Length") {
					length, err := strconv.Atoi(headerVal)
					if err == nil {
						contentLength = length
					}
				}
			}
		}

		if contentLength <= 0 {
			continue
		}

		// Read payload
		body := make([]byte, contentLength)
		_, err := io.ReadFull(br, body)
		if err != nil {
			return err
		}

		var req Request
		if err := json.Unmarshal(body, &req); err != nil {
			continue
		}

		s.dispatch(req)
	}

	return nil
}

func (s *Server) dispatch(req Request) {
	switch req.Method {
	case "initialize":
		res := InitializeResult{
			Capabilities: ServerCapabilities{
				TextDocumentSync:           1, // Full sync
				DocumentFormattingProvider: true,
				HoverProvider:              true,
				CompletionProvider: &CompletionServerOptions{
					ResolveProvider:   false,
					TriggerCharacters: []string{".", ":"},
				},
			},
			ServerInfo: &ServerInfo{
				Name:    "nil-lsp",
				Version: "1.0.0",
			},
		}
		s.sendResponse(req.ID, res, nil)

	case "initialized":
		// Client notification, no reply needed

	case "shutdown":
		s.isShutdown = true
		s.sendResponse(req.ID, nil, nil)

	case "exit":
		s.running = false

	case "textDocument/didOpen":
		var params DidOpenTextDocumentParams
		if err := json.Unmarshal(req.Params, &params); err == nil {
			s.docsMu.Lock()
			s.documents[params.TextDocument.URI] = &Document{
				URI:     params.TextDocument.URI,
				Version: params.TextDocument.Version,
				Content: params.TextDocument.Text,
			}
			s.docsMu.Unlock()
			s.validateDocument(params.TextDocument.URI, params.TextDocument.Text, &params.TextDocument.Version)
		}

	case "textDocument/didChange":
		var params DidChangeTextDocumentParams
		if err := json.Unmarshal(req.Params, &params); err == nil {
			if len(params.ContentChanges) > 0 {
				lastChange := params.ContentChanges[len(params.ContentChanges)-1]
				s.docsMu.Lock()
				s.documents[params.TextDocument.URI] = &Document{
					URI:     params.TextDocument.URI,
					Version: params.TextDocument.Version,
					Content: lastChange.Text,
				}
				s.docsMu.Unlock()
				s.validateDocument(params.TextDocument.URI, lastChange.Text, &params.TextDocument.Version)
			}
		}

	case "textDocument/didClose":
		var params DidCloseTextDocumentParams
		if err := json.Unmarshal(req.Params, &params); err == nil {
			s.docsMu.Lock()
			delete(s.documents, params.TextDocument.URI)
			s.docsMu.Unlock()
		}

	case "textDocument/formatting":
		var params DocumentFormattingParams
		if err := json.Unmarshal(req.Params, &params); err == nil {
			s.docsMu.RLock()
			doc, ok := s.documents[params.TextDocument.URI]
			s.docsMu.RUnlock()

			if !ok {
				s.sendResponse(req.ID, []TextEdit{}, nil)
				return
			}

			formatted, err := formatter.Format(doc.Content)
			if err != nil {
				// Don't format if there are syntax errors
				s.sendResponse(req.ID, []TextEdit{}, nil)
				return
			}

			lines := strings.Split(doc.Content, "\n")
			lastLine := len(lines) - 1
			if lastLine < 0 {
				lastLine = 0
			}
			lastChar := len(lines[lastLine])

			edits := []TextEdit{
				{
					Range: Range{
						Start: Position{Line: 0, Character: 0},
						End:   Position{Line: lastLine, Character: lastChar},
					},
					NewText: formatted,
				},
			}
			s.sendResponse(req.ID, edits, nil)
		}

	case "textDocument/hover":
		var params HoverParams
		if err := json.Unmarshal(req.Params, &params); err == nil {
			s.docsMu.RLock()
			doc, ok := s.documents[params.TextDocument.URI]
			s.docsMu.RUnlock()

			if !ok {
				s.sendResponse(req.ID, nil, nil)
				return
			}

			hover := ComputeHover(doc.Content, params.Position)
			s.sendResponse(req.ID, hover, nil)
		}

	case "textDocument/completion":
		var params CompletionParams
		if err := json.Unmarshal(req.Params, &params); err == nil {
			s.docsMu.RLock()
			doc, ok := s.documents[params.TextDocument.URI]
			s.docsMu.RUnlock()

			content := ""
			if ok {
				content = doc.Content
			}

			completions := ComputeCompletions(content)
			s.sendResponse(req.ID, completions, nil)
		}

	default:
		// Unknown request, reply method not found if it has an ID
		if len(req.ID) > 0 {
			s.sendResponse(req.ID, nil, &ResponseError{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			})
		}
	}
}

func (s *Server) validateDocument(uri string, content string, version *int) {
	var diags []Diagnostic

	l := lexer.New(content)
	p := parser.New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, parseErr := range p.Errors() {
			diags = append(diags, Diagnostic{
				Range: Range{
					Start: Position{Line: 0, Character: 0},
					End:   Position{Line: 0, Character: 1},
				},
				Severity: SeverityError,
				Source:   "nil-parser",
				Message:  parseErr,
			})
		}
	} else if prog != nil {
		tc := typecheck.NewChecker()
		tc.CheckProgram(prog)

		for _, d := range tc.Diagnostics {
			startL := d.Span.StartLine - 1
			if startL < 0 {
				startL = 0
			}
			startC := d.Span.StartCol - 1
			if startC < 0 {
				startC = 0
			}
			endL := d.Span.EndLine - 1
			if endL < 0 {
				endL = startL
			}
			endC := d.Span.EndCol - 1
			if endC <= startC && endL == startL {
				endC = startC + 1
			}

			sev := SeverityError
			switch d.Severity {
			case diagnostics.SeverityWarning:
				sev = SeverityWarning
			case diagnostics.SeverityInfo:
				sev = SeverityInformation
			case diagnostics.SeverityHint:
				sev = SeverityHint
			}

			msg := d.Message
			if d.Suggestion != "" {
				msg += "\nSuggestion: " + d.Suggestion
			}

			diags = append(diags, Diagnostic{
				Range: Range{
					Start: Position{Line: startL, Character: startC},
					End:   Position{Line: endL, Character: endC},
				},
				Severity: sev,
				Code:     d.Code,
				Source:   "nil-typecheck",
				Message:  msg,
			})
		}
	}

	if diags == nil {
		diags = []Diagnostic{}
	}

	s.sendNotification("textDocument/publishDiagnostics", PublishDiagnosticsParams{
		URI:         uri,
		Version:     version,
		Diagnostics: diags,
	})
}

func (s *Server) sendResponse(id any, result any, rerr *ResponseError) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
		Error:   rerr,
	}
	s.writeMessage(resp)
}

func (s *Server) sendNotification(method string, params any) {
	notif := Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	s.writeMessage(notif)
}

func (s *Server) writeMessage(msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Content-Length: %d\r\n\r\n", len(data))
	buf.Write(data)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.writer.Write(buf.Bytes())
}
