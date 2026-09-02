package softbus

import (
	"encoding/json"
	"time"
)

// Protocol version
const (
	ProtocolVersion = 1
	MagicNumber     = 0x4E494C42 // "NILB" in hex
)

// MessageType represents the type of SoftBus message
type MessageType uint8

const (
	MsgDiscover     MessageType = 0x01
	MsgDiscoverAck  MessageType = 0x02
	MsgConnect      MessageType = 0x10
	MsgConnectAck   MessageType = 0x11
	MsgDisconnect   MessageType = 0x12
	MsgData         MessageType = 0x20
	MsgDataAck      MessageType = 0x21
	MsgFileTransfer MessageType = 0x30
	MsgFileAck      MessageType = 0x31
	MsgRPC          MessageType = 0x40
	MsgRPCResponse  MessageType = 0x41
	MsgHeartbeat    MessageType = 0x50
	MsgHeartbeatAck MessageType = 0x51
	MsgError        MessageType = 0xFF
)

func (mt MessageType) String() string {
	names := map[MessageType]string{
		MsgDiscover:     "DISCOVER",
		MsgDiscoverAck:  "DISCOVER_ACK",
		MsgConnect:      "CONNECT",
		MsgConnectAck:   "CONNECT_ACK",
		MsgDisconnect:   "DISCONNECT",
		MsgData:         "DATA",
		MsgDataAck:      "DATA_ACK",
		MsgFileTransfer: "FILE_TRANSFER",
		MsgFileAck:      "FILE_ACK",
		MsgRPC:          "RPC",
		MsgRPCResponse:  "RPC_RESPONSE",
		MsgHeartbeat:    "HEARTBEAT",
		MsgHeartbeatAck: "HEARTBEAT_ACK",
		MsgError:        "ERROR",
	}
	if name, ok := names[mt]; ok {
		return name
	}
	return "UNKNOWN"
}

// Message represents a SoftBus protocol message
type Message struct {
	Version     uint8       `json:"version"`
	Type        MessageType `json:"type"`
	SourceID    string      `json:"source_id"`
	DestID      string      `json:"dest_id"`
	SessionID   string      `json:"session_id,omitempty"`
	SequenceNum uint64      `json:"seq"`
	Timestamp   int64       `json:"ts"`
	Payload     []byte      `json:"payload,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// NewMessage creates a new message
func NewMessage(msgType MessageType, sourceID, destID string) *Message {
	return &Message{
		Version:   ProtocolVersion,
		Type:      msgType,
		SourceID:  sourceID,
		DestID:    destID,
		Timestamp: time.Now().UnixMilli(),
		Metadata:  make(map[string]string),
	}
}

// SetPayload sets the message payload
func (m *Message) SetPayload(data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	m.Payload = payload
	return nil
}

// GetPayload deserializes the message payload
func (m *Message) GetPayload(target interface{}) error {
	return json.Unmarshal(m.Payload, target)
}

// Serialize serializes the message to bytes
func (m *Message) Serialize() ([]byte, error) {
	return json.Marshal(m)
}

// Deserialize deserializes bytes to a message
func DeserializeMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// DeviceInfo represents information about a device
type DeviceInfo struct {
	DeviceID    string   `json:"device_id"`
	DeviceName  string   `json:"device_name"`
	DeviceType  string   `json:"device_type"` // "phone", "tablet", "desktop", "watch"
	OSName      string   `json:"os_name"`
	OSVersion   string   `json:"os_version"`
	IPAddress   string   `json:"ip_address"`
	Port        int      `json:"port"`
	Capabilities []string `json:"capabilities"`
	PublicKey   string   `json:"public_key"`
	LastSeen    int64    `json:"last_seen"`
}

// DiscoveryRequest represents a device discovery request
type DiscoveryRequest struct {
	DeviceID   string   `json:"device_id"`
	DeviceName string   `json:"device_name"`
	DeviceType string   `json:"device_type"`
	Services   []string `json:"services"`
}

// DiscoveryResponse represents a device discovery response
type DiscoveryResponse struct {
	Devices []*DeviceInfo `json:"devices"`
}

// ConnectRequest represents a connection request
type ConnectRequest struct {
	DeviceID    string `json:"device_id"`
	SessionID   string `json:"session_id"`
	PublicKey   string `json:"public_key"`
	Challenge   string `json:"challenge"`
	ProtocolVer int    `json:"protocol_version"`
}

// ConnectResponse represents a connection response
type ConnectResponse struct {
	Accepted    bool   `json:"accepted"`
	SessionID   string `json:"session_id"`
	PublicKey   string `json:"public_key"`
	Challenge   string `json:"challenge"`
	Error       string `json:"error,omitempty"`
}

// Heartbeat represents a heartbeat message
type Heartbeat struct {
	DeviceID  string `json:"device_id"`
	Timestamp int64  `json:"timestamp"`
	Load      float64 `json:"load"` // 0.0 - 1.0
	Battery   int    `json:"battery"`
}

// ErrorPayload represents an error message
type ErrorPayload struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}