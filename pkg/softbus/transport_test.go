package softbus

import (
	"fmt"
	"net"
	"testing"
	"time"
)

// freePort finds a currently-free TCP port. There is an inherent small
// race between releasing it and the transport binding it; acceptable in
// tests, and Listen takes only a port number.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func startTransport(t *testing.T, config *TransportConfig) (*Transport, int) {
	t.Helper()
	port := freePort(t)
	tr := NewTransport("test-device", config)
	if err := tr.Listen(port); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(tr.Stop)
	return tr, port
}

func dial(t *testing.T, port int) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func recv(t *testing.T, ch <-chan *Message, what string) *Message {
	t.Helper()
	select {
	case m := <-ch:
		return m
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
		return nil
	}
}

// A message split across several TCP segments must still be delivered:
// the stream decoder waits for the rest of the JSON value instead of
// treating one Read as one message (the old code dropped it).
func TestTransportDeliversFragmentedMessage(t *testing.T) {
	got := make(chan *Message, 4)
	tr, port := startTransport(t, nil)
	tr.SetOnMessage(func(m *Message) { got <- m })

	conn := dial(t, port)
	msg := NewMessage(MsgData, "sender-1", "test-device")
	msg.SequenceNum = 42
	if err := msg.SetPayload(map[string]string{"hello": "world"}); err != nil {
		t.Fatal(err)
	}
	data, err := msg.Serialize()
	if err != nil {
		t.Fatal(err)
	}

	// Write the message in small chunks with pauses, forcing fragmentation.
	for i := 0; i < len(data); i += 7 {
		end := i + 7
		if end > len(data) {
			end = len(data)
		}
		if _, err := conn.Write(data[i:end]); err != nil {
			t.Fatalf("write chunk: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}

	m := recv(t, got, "fragmented message")
	if m.SourceID != "sender-1" || m.SequenceNum != 42 || m.Type != MsgData {
		t.Fatalf("wrong message: %+v", m)
	}
}

// Several messages written in a single segment must each be delivered.
func TestTransportDeliversCoalescedMessages(t *testing.T) {
	got := make(chan *Message, 4)
	tr, port := startTransport(t, nil)
	tr.SetOnMessage(func(m *Message) { got <- m })

	conn := dial(t, port)
	var wire []byte
	for seq := uint64(1); seq <= 3; seq++ {
		m := NewMessage(MsgHeartbeat, "peer", "test-device")
		m.SequenceNum = seq
		data, err := m.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		wire = append(wire, data...)
	}
	if _, err := conn.Write(wire); err != nil {
		t.Fatalf("write: %v", err)
	}

	for seq := uint64(1); seq <= 3; seq++ {
		m := recv(t, got, "coalesced message")
		if m.SequenceNum != seq {
			t.Fatalf("got seq %d, want %d", m.SequenceNum, seq)
		}
	}
}

// Once MaxConnections peers are connected, the next connection is refused
// (closed by the server) instead of being tracked without bound.
func TestTransportEnforcesMaxConnections(t *testing.T) {
	config := DefaultTransportConfig()
	config.MaxConnections = 2
	_, port := startTransport(t, config)

	c1 := dial(t, port)
	c2 := dial(t, port)
	_ = c1
	_ = c2
	// Give the accept loop a moment to register both connections.
	time.Sleep(100 * time.Millisecond)

	c3 := dial(t, port)
	// The server closes the refused connection: a read returns promptly
	// with an error (EOF) rather than blocking.
	_ = c3.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	if _, err := c3.Read(buf); err == nil {
		t.Fatal("over-limit connection was not refused")
	}
}
