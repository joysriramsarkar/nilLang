package softbus

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// RPCRequest represents a remote procedure call request
type RPCRequest struct {
	ID        string          `json:"id"`
	Method    string          `json:"method"`
	Params    json.RawMessage `json:"params,omitempty"`
	Timeout   time.Duration   `json:"timeout,omitempty"`
}

// RPCResponse represents a remote procedure call response
type RPCResponse struct {
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents an RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// RPCHandler is a function that handles an RPC call
type RPCHandler func(params json.RawMessage) (interface{}, error)

// RPCServer handles incoming RPC calls
type RPCServer struct {
	mu       sync.RWMutex
	handlers map[string]RPCHandler
}

// NewRPCServer creates a new RPC server
func NewRPCServer() *RPCServer {
	server := &RPCServer{
		handlers: make(map[string]RPCHandler),
	}

	// Register built-in handlers
	server.Register("ping", func(params json.RawMessage) (interface{}, error) {
		return "pong", nil
	})

	server.Register("device.info", func(params json.RawMessage) (interface{}, error) {
		return map[string]string{
			"status": "online",
		}, nil
	})

	return server
}

// Register registers an RPC handler
func (s *RPCServer) Register(method string, handler RPCHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[method] = handler
}

// HandleRequest processes an incoming RPC request
func (s *RPCServer) HandleRequest(req *RPCRequest) *RPCResponse {
	s.mu.RLock()
	handler, exists := s.handlers[req.Method]
	s.mu.RUnlock()

	if !exists {
		return &RPCResponse{
			ID: req.ID,
			Error: &RPCError{
				Code:    -32601,
				Message: fmt.Sprintf("method not found: %s", req.Method),
			},
		}
	}

	result, err := handler(req.Params)
	if err != nil {
		return &RPCResponse{
			ID: req.ID,
			Error: &RPCError{
				Code:    -32000,
				Message: err.Error(),
			},
		}
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return &RPCResponse{
			ID: req.ID,
			Error: &RPCError{
				Code:    -32603,
				Message: "failed to marshal result",
			},
		}
	}

	return &RPCResponse{
		ID:     req.ID,
		Result: resultJSON,
	}
}

// RPCClient makes outgoing RPC calls
type RPCClient struct {
	transport *Transport
	deviceID  string
	timeout   time.Duration
}

// NewRPCClient creates a new RPC client
func NewRPCClient(transport *Transport, deviceID string) *RPCClient {
	return &RPCClient{
		transport: transport,
		deviceID:  deviceID,
		timeout:   10 * time.Second,
	}
}

// Call makes an RPC call to a remote device
func (c *RPCClient) Call(targetDevice, method string, params interface{}) (*RPCResponse, error) {
	// Generate request ID
	reqID, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	// Marshal params
	var paramsJSON json.RawMessage
	if params != nil {
		paramsJSON, err = json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal params: %w", err)
		}
	}

	// Create RPC request
	req := &RPCRequest{
		ID:      reqID,
		Method:  method,
		Params:  paramsJSON,
		Timeout: c.timeout,
	}

	// Create message
	msg := NewMessage(MsgRPC, c.deviceID, targetDevice)
	if err := msg.SetPayload(req); err != nil {
		return nil, err
	}

	// Send message
	if err := c.transport.Send(targetDevice, msg); err != nil {
		return nil, fmt.Errorf("failed to send RPC request: %w", err)
	}

	// In production: wait for response with timeout
	// For now, return a simulated response
	return &RPCResponse{
		ID:     reqID,
		Result: json.RawMessage(`{"status":"ok"}`),
	}, nil
}