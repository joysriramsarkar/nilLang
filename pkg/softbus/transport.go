package softbus

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// maxMessageBytes caps a single SoftBus message on the wire. Messages are
// JSON values in a stream; without a cap one peer can make the receiver
// buffer and allocate for an unbounded "message".
const maxMessageBytes = 1 << 20 // 1 MiB

// byteCounter wraps a connection to keep the cumulative byte count for
// connection statistics while a json.Decoder streams from it.
type byteCounter struct {
	r io.Reader
	n int64
}

func (c *byteCounter) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// TransportConfig holds transport configuration
type TransportConfig struct {
	MaxConnections    int
	ConnectionTimeout time.Duration
	KeepAliveInterval time.Duration
	EnableTLS         bool
	EnableQUIC        bool
	CertFile          string
	KeyFile           string
}

// DefaultTransportConfig returns default transport configuration
func DefaultTransportConfig() *TransportConfig {
	return &TransportConfig{
		MaxConnections:    16,
		ConnectionTimeout: 10 * time.Second,
		KeepAliveInterval: 30 * time.Second,
		EnableTLS:         true,
		EnableQUIC:        true,
	}
}

// Transport handles network communication
type Transport struct {
	mu          sync.RWMutex
	config      *TransportConfig
	deviceID    string
	connections map[string]*Connection
	listener    net.Listener
	running     bool
	stopChan    chan struct{}
	onMessage   func(*Message)
	// inbound counts accepted connections currently being handled, so
	// acceptLoop can enforce config.MaxConnections. Guarded by mu.
	inbound int
}

// Connection represents a connection to another device
type Connection struct {
	DeviceID     string
	Address      string
	Conn         net.Conn
	SessionID    string
	Connected    bool
	LastActive   time.Time
	BytesSent    int64
	BytesRecv    int64
	MessagesSent uint64
	MessagesRecv uint64
}

// NewTransport creates a new transport layer
func NewTransport(deviceID string, config *TransportConfig) *Transport {
	if config == nil {
		config = DefaultTransportConfig()
	}

	return &Transport{
		config:      config,
		deviceID:    deviceID,
		connections: make(map[string]*Connection),
		stopChan:    make(chan struct{}),
	}
}

// Listen starts listening for incoming connections.
//
// SECURITY: the wire is plaintext TCP with no peer authentication. The
// EnableTLS config flag exists, but TLS is NOT implemented — Listen warns
// loudly either way so the plaintext state can never be mistaken for a
// secure one. Do not expose this listener on an untrusted network.
func (t *Transport) Listen(port int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", port, err)
	}

	t.listener = listener
	t.running = true

	fmt.Printf("🌐 SoftBus Transport listening on port %d\n", port)
	if t.config.EnableTLS {
		fmt.Println("⚠️  WARNING: EnableTLS is set but TLS is not implemented — SoftBus traffic is PLAINTEXT and unauthenticated. Trusted networks only.")
	} else {
		fmt.Println("⚠️  WARNING: SoftBus traffic is PLAINTEXT and unauthenticated (TLS disabled and not implemented). Trusted networks only.")
	}

	// Accept connections in background
	go t.acceptLoop()

	return nil
}

// reserveInbound claims a slot under the configured connection ceiling.
// A non-positive MaxConnections means no ceiling.
func (t *Transport) reserveInbound() bool {
	if t.config.MaxConnections <= 0 {
		return true
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.inbound >= t.config.MaxConnections {
		return false
	}
	t.inbound++
	return true
}

func (t *Transport) releaseInbound() {
	if t.config.MaxConnections <= 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.inbound > 0 {
		t.inbound--
	}
}

func (t *Transport) acceptLoop() {
	for {
		select {
		case <-t.stopChan:
			return
		default:
		}

		conn, err := t.listener.Accept()
		if err != nil {
			continue
		}

		if !t.reserveInbound() {
			fmt.Printf("⚠️  SoftBus: connection limit (%d) reached, refusing %s\n", t.config.MaxConnections, conn.RemoteAddr())
			conn.Close()
			continue
		}

		go t.handleConnection(conn)
	}
}

func (t *Transport) handleConnection(conn net.Conn) {
	defer t.releaseInbound()

	remoteAddr := conn.RemoteAddr().String()
	fmt.Printf("🔗 New connection from %s\n", remoteAddr)

	connection := &Connection{
		Address:    remoteAddr,
		Conn:       conn,
		Connected:  true,
		LastActive: time.Now(),
	}

	// NOTE: no handshake or peer authentication is performed — the wire
	// protocol trusts any peer that can connect (see Listen's warning).

	t.mu.Lock()
	t.connections[remoteAddr] = connection
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		delete(t.connections, remoteAddr)
		t.mu.Unlock()
		conn.Close()
	}()

	// Read messages as a JSON stream. Senders write one JSON value per
	// message with no delimiter, and TCP owes us no message boundaries:
	// a value may arrive split across segments (fragmentation) or several
	// may arrive coalesced into one segment. A json.Decoder over the
	// connection handles both; the previous one-Read-one-message code
	// silently dropped fragmented messages and could not parse coalesced
	// ones at all.
	counter := &byteCounter{r: conn}
	decoder := json.NewDecoder(counter)
	for {
		// Bound how long a peer may hold the connection idle between
		// messages; heartbeats arrive every KeepAliveInterval, so three
		// missed beats mean the peer is gone or stuck.
		if idle := t.readIdleTimeout(); idle > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(idle))
		}

		start := decoder.InputOffset()
		var msg Message
		if err := decoder.Decode(&msg); err != nil {
			// EOF, timeout, malformed JSON, or a closed connection:
			// the stream cannot be resynchronised, so the connection ends.
			break
		}
		if decoder.InputOffset()-start > maxMessageBytes {
			fmt.Printf("⚠️  SoftBus: dropping %s — message exceeds the %d byte limit\n", remoteAddr, maxMessageBytes)
			break
		}

		connection.BytesRecv = counter.n
		connection.MessagesRecv++
		connection.LastActive = time.Now()

		if t.onMessage != nil {
			t.onMessage(&msg)
		}
	}
}

// readIdleTimeout derives the per-message read deadline from the config:
// three keepalive intervals. Zero disables the deadline.
func (t *Transport) readIdleTimeout() time.Duration {
	if t.config.KeepAliveInterval <= 0 {
		return 0
	}
	return 3 * t.config.KeepAliveInterval
}

// Connect connects to a remote device
func (t *Transport) Connect(address string, deviceID string) (*Connection, error) {
	t.mu.RLock()
	if conn, exists := t.connections[deviceID]; exists && conn.Connected {
		t.mu.RUnlock()
		return conn, nil
	}
	t.mu.RUnlock()

	conn, err := net.DialTimeout("tcp", address, t.config.ConnectionTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	connection := &Connection{
		DeviceID:   deviceID,
		Address:    address,
		Conn:       conn,
		Connected:  true,
		LastActive: time.Now(),
	}

	t.mu.Lock()
	t.connections[deviceID] = connection
	t.mu.Unlock()

	fmt.Printf("🔗 Connected to %s (%s)\n", deviceID, address)

	return connection, nil
}

// Send sends a message to a device
func (t *Transport) Send(deviceID string, msg *Message) error {
	t.mu.RLock()
	conn, exists := t.connections[deviceID]
	t.mu.RUnlock()

	if !exists || !conn.Connected {
		return fmt.Errorf("device not connected: %s", deviceID)
	}

	data, err := msg.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	n, err := conn.Conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	conn.BytesSent += int64(n)
	conn.MessagesSent++
	conn.LastActive = time.Now()

	return nil
}

// Broadcast sends a message to all connected devices
func (t *Transport) Broadcast(msg *Message) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for deviceID := range t.connections {
		if err := t.Send(deviceID, msg); err != nil {
			// Log error but continue
			fmt.Printf("⚠️  Failed to send to %s: %v\n", deviceID, err)
		}
	}

	return nil
}

// Disconnect disconnects from a device
func (t *Transport) Disconnect(deviceID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	conn, exists := t.connections[deviceID]
	if !exists {
		return fmt.Errorf("device not connected: %s", deviceID)
	}

	conn.Conn.Close()
	conn.Connected = false
	delete(t.connections, deviceID)

	return nil
}

// SetOnMessage sets the message handler
func (t *Transport) SetOnMessage(handler func(*Message)) {
	t.onMessage = handler
}

// GetConnections returns all active connections
func (t *Transport) GetConnections() []*Connection {
	t.mu.RLock()
	defer t.mu.RUnlock()

	connections := make([]*Connection, 0, len(t.connections))
	for _, conn := range t.connections {
		connections = append(connections, conn)
	}
	return connections
}

// Stop stops the transport
func (t *Transport) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.running {
		return
	}

	t.running = false
	close(t.stopChan)

	if t.listener != nil {
		t.listener.Close()
	}

	for _, conn := range t.connections {
		conn.Conn.Close()
	}

	fmt.Println("🌐 SoftBus Transport stopped")
}

// GetTLSConfig returns TLS configuration
func (t *Transport) GetTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS13,
		// In production: load certificates
	}
}
