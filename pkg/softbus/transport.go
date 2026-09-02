package softbus

import (
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"
)

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

// Listen starts listening for incoming connections
func (t *Transport) Listen(port int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	var listener net.Listener
	var err error

	if t.config.EnableTLS {
		// In production: load TLS certificate
		// For now, use plain TCP
		listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
	} else {
		listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
	}

	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", port, err)
	}

	t.listener = listener
	t.running = true

	fmt.Printf("🌐 SoftBus Transport listening on port %d\n", port)

	// Accept connections in background
	go t.acceptLoop()

	return nil
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

		go t.handleConnection(conn)
	}
}

func (t *Transport) handleConnection(conn net.Conn) {
	remoteAddr := conn.RemoteAddr().String()
	fmt.Printf("🔗 New connection from %s\n", remoteAddr)

	connection := &Connection{
		Address:    remoteAddr,
		Conn:       conn,
		Connected:  true,
		LastActive: time.Now(),
	}

	// In production: perform handshake, authenticate, establish session
	// For now, just track the connection

	t.mu.Lock()
	t.connections[remoteAddr] = connection
	t.mu.Unlock()

	// Read messages
	buf := make([]byte, 65536)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			break
		}

		connection.BytesRecv += int64(n)
		connection.MessagesRecv++
		connection.LastActive = time.Now()

		// Parse and dispatch message
		msg, err := DeserializeMessage(buf[:n])
		if err != nil {
			continue
		}

		if t.onMessage != nil {
			t.onMessage(msg)
		}
	}

	// Connection closed
	t.mu.Lock()
	delete(t.connections, remoteAddr)
	t.mu.Unlock()
	conn.Close()
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
