package realtime

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWSManagerBroadcastAndRooms(t *testing.T) {
	mgr := NewWSManager()

	c1 := NewWSClient("client-1")
	c2 := NewWSClient("client-2")
	c3 := NewWSClient("client-3")

	mgr.Register(c1)
	mgr.Register(c2)
	mgr.Register(c3)

	if mgr.ClientCount() != 3 {
		t.Fatalf("expected 3 clients, got %d", mgr.ClientCount())
	}

	// Join rooms
	mgr.JoinRoom("client-1", "chat-dev")
	mgr.JoinRoom("client-2", "chat-dev")
	// client-3 is NOT in chat-dev

	// Broadcast to room
	mgr.BroadcastToRoom("chat-dev", WSMessage{
		Type:    "message",
		Payload: "Hello Devs",
	})

	select {
	case msgBytes := <-c1.Send:
		var msg WSMessage
		_ = json.Unmarshal(msgBytes, &msg)
		if msg.Payload != "Hello Devs" {
			t.Errorf("unexpected payload: %v", msg.Payload)
		}
	default:
		t.Error("client-1 did not receive message")
	}

	select {
	case msgBytes := <-c2.Send:
		var msg WSMessage
		_ = json.Unmarshal(msgBytes, &msg)
		if msg.Payload != "Hello Devs" {
			t.Errorf("unexpected payload: %v", msg.Payload)
		}
	default:
		t.Error("client-2 did not receive message")
	}

	// client-3 should not receive message
	select {
	case <-c3.Send:
		t.Error("client-3 should not have received room message")
	default:
		// OK
	}

	// Unregister
	mgr.Unregister(c1)
	if mgr.ClientCount() != 2 {
		t.Errorf("expected 2 clients after unregister")
	}
}

func TestSSEWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	sse, err := NewSSEWriter(rec)
	if err != nil {
		t.Fatalf("failed to create SSE writer: %v", err)
	}

	err = sse.SendEvent("price-update", map[string]float64{"NIL": 142.50}, "msg-1")
	if err != nil {
		t.Fatalf("failed to send SSE event: %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "id: msg-1\n") {
		t.Errorf("missing id in SSE: %s", body)
	}
	if !strings.Contains(body, "event: price-update\n") {
		t.Errorf("missing event in SSE: %s", body)
	}
	if !strings.Contains(body, `"NIL":142.5`) {
		t.Errorf("missing data payload in SSE: %s", body)
	}
}
