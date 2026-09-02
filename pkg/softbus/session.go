package softbus

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// SessionState represents the state of a session
type SessionState int

const (
	SessionPending SessionState = iota
	SessionActive
	SessionClosed
	SessionExpired
)

func (s SessionState) String() string {
	switch s {
	case SessionPending:
		return "PENDING"
	case SessionActive:
		return "ACTIVE"
	case SessionClosed:
		return "CLOSED"
	case SessionExpired:
		return "EXPIRED"
	default:
		return "UNKNOWN"
	}
}

// Session represents a communication session between devices
type Session struct {
	ID           string
	SourceDevice string
	DestDevice   string
	State        SessionState
	CreatedAt    time.Time
	LastActive   time.Time
	SequenceNum  uint64
	EncryptionKey []byte
	Metadata     map[string]string
}

// SessionManager manages sessions
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	deviceID string
	timeout  time.Duration
}

// NewSessionManager creates a new session manager
func NewSessionManager(deviceID string) *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
		deviceID: deviceID,
		timeout:  5 * time.Minute,
	}
}

// CreateSession creates a new session
func (sm *SessionManager) CreateSession(destDevice string) (*Session, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	encryptionKey := make([]byte, 32)
	if _, err := rand.Read(encryptionKey); err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}

	session := &Session{
		ID:            sessionID,
		SourceDevice:  sm.deviceID,
		DestDevice:    destDevice,
		State:         SessionPending,
		CreatedAt:     time.Now(),
		LastActive:    time.Now(),
		EncryptionKey: encryptionKey,
		Metadata:      make(map[string]string),
	}

	sm.mu.Lock()
	sm.sessions[sessionID] = session
	sm.mu.Unlock()

	return session, nil
}

// GetSession returns a session by ID
func (sm *SessionManager) GetSession(sessionID string) (*Session, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return session, nil
}

// ActivateSession activates a pending session
func (sm *SessionManager) ActivateSession(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.State = SessionActive
	session.LastActive = time.Now()
	return nil
}

// CloseSession closes a session
func (sm *SessionManager) CloseSession(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.State = SessionClosed
	return nil
}

// GetNextSequence returns the next sequence number for a session
func (sm *SessionManager) GetNextSequence(sessionID string) uint64 {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, exists := sm.sessions[sessionID]; exists {
		session.SequenceNum++
		session.LastActive = time.Now()
		return session.SequenceNum
	}
	return 0
}

// CleanupExpired removes expired sessions
func (sm *SessionManager) CleanupExpired() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	count := 0
	now := time.Now()
	for id, session := range sm.sessions {
		if now.Sub(session.LastActive) > sm.timeout {
			session.State = SessionExpired
			delete(sm.sessions, id)
			count++
		}
	}
	return count
}

// GetActiveSessions returns all active sessions
func (sm *SessionManager) GetActiveSessions() []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]*Session, 0)
	for _, session := range sm.sessions {
		if session.State == SessionActive {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

func generateSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}