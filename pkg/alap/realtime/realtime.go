package realtime

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// WSMessage represents a message exchanged over WebSocket
type WSMessage struct {
	Type    string      `json:"type"`
	Room    string      `json:"room,omitempty"`
	Payload interface{} `json:"payload"`
}

// WSClient represents an active WebSocket client session
type WSClient struct {
	ID    string
	Send  chan []byte
	rooms map[string]bool
	mu    sync.Mutex
}

// NewWSClient creates a new client
func NewWSClient(id string) *WSClient {
	return &WSClient{
		ID:    id,
		Send:  make(chan []byte, 64),
		rooms: make(map[string]bool),
	}
}

// WSManager manages connected clients and room broadcasting
type WSManager struct {
	clients map[string]*WSClient
	rooms   map[string]map[string]*WSClient
	mu      sync.RWMutex
}

// NewWSManager creates a realtime WebSocket manager
func NewWSManager() *WSManager {
	return &WSManager{
		clients: make(map[string]*WSClient),
		rooms:   make(map[string]map[string]*WSClient),
	}
}

// Register adds a client
func (m *WSManager) Register(c *WSClient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[c.ID] = c
}

// Unregister removes a client from manager and all rooms
func (m *WSManager) Unregister(c *WSClient) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.clients, c.ID)
	for room := range c.rooms {
		if m.rooms[room] != nil {
			delete(m.rooms[room], c.ID)
			if len(m.rooms[room]) == 0 {
				delete(m.rooms, room)
			}
		}
	}
	close(c.Send)
}

// JoinRoom adds client to a room
func (m *WSManager) JoinRoom(clientID, room string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.clients[clientID]
	if !ok {
		return false
	}

	if m.rooms[room] == nil {
		m.rooms[room] = make(map[string]*WSClient)
	}
	m.rooms[room][clientID] = c

	c.mu.Lock()
	c.rooms[room] = true
	c.mu.Unlock()

	return true
}

// Broadcast sends payload to all connected clients
func (m *WSManager) Broadcast(msg WSMessage) {
	bytes, err := json.Marshal(msg)
	if err != nil {
		return
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, c := range m.clients {
		select {
		case c.Send <- bytes:
		default:
		}
	}
}

// BroadcastToRoom sends payload only to clients in a specific room
func (m *WSManager) BroadcastToRoom(room string, msg WSMessage) {
	msg.Room = room
	bytes, err := json.Marshal(msg)
	if err != nil {
		return
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if roomClients, ok := m.rooms[room]; ok {
		for _, c := range roomClients {
			select {
			case c.Send <- bytes:
			default:
			}
		}
	}
}

// ClientCount returns number of connected clients
func (m *WSManager) ClientCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}

// ─── SERVER-SENT EVENTS (SSE) ───────────────────────────────────────────────

// SSEWriter formats and streams Server-Sent Events over HTTP
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewSSEWriter initializes SSE response stream
func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming unsupported by response writer")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	return &SSEWriter{w: w, flusher: flusher}, nil
}

// SendEvent writes an SSE event
func (s *SSEWriter) SendEvent(event string, data interface{}, id string) error {
	var sb string
	if id != "" {
		sb += fmt.Sprintf("id: %s\n", id)
	}
	if event != "" {
		sb += fmt.Sprintf("event: %s\n", event)
	}

	var dataStr string
	if str, isStr := data.(string); isStr {
		dataStr = str
	} else {
		b, err := json.Marshal(data)
		if err != nil {
			return err
		}
		dataStr = string(b)
	}

	sb += fmt.Sprintf("data: %s\n\n", dataStr)

	_, err := io.WriteString(s.w, sb)
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}
