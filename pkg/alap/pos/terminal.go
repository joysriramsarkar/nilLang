package pos

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

// IntentStatus defines the operational lifecycle states of an integrated payment terminal intent
type IntentStatus string

const (
	IntentCreated    IntentStatus = "CREATED"
	IntentSent       IntentStatus = "SENT_TO_TERMINAL"
	IntentAuthorized IntentStatus = "AUTHORIZED"
	IntentCaptured   IntentStatus = "CAPTURED"
	IntentFailed     IntentStatus = "FAILED"
	IntentTimedOut   IntentStatus = "TIMED_OUT"
	IntentCancelled  IntentStatus = "CANCELLED"
)

// PaymentIntent tracks the end-to-end lifecycle of an electronic tender (Card, UPI, MFS)
type PaymentIntent struct {
	ID             string        `json:"id"`
	IdempotencyKey string        `json:"idempotency_key"`
	SaleID         string        `json:"sale_id,omitempty"`
	Method         PaymentMethod `json:"method"`
	AmountMinor    int64         `json:"amount_minor"`
	Status         IntentStatus  `json:"status"`
	TerminalID     string        `json:"terminal_id,omitempty"`
	ProviderRef    string        `json:"provider_ref,omitempty"`
	CardLast4      string        `json:"card_last4,omitempty"`
	AuthCode       string        `json:"auth_code,omitempty"`
	LastError      string        `json:"last_error,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

// TerminalGateway defines communication with physical payment terminals or payment switches
type TerminalGateway interface {
	SendToTerminal(intent *PaymentIntent) error
	Capture(intentID string) (providerRef, authCode string, err error)
	Cancel(intentID string) error
	ReconcileTimeout(intentID string) (IntentStatus, string, error)
}

// MockTerminalGateway simulates physical card/UPI terminal interactions, timeouts, and network reconciliation
type MockTerminalGateway struct {
	mu            sync.Mutex
	ShouldFail    bool
	ShouldTimeOut bool
	FailOnCapture bool
	TerminalLogs  []string
}

// NewMockTerminalGateway creates a mock terminal gateway
func NewMockTerminalGateway() *MockTerminalGateway {
	return &MockTerminalGateway{
		TerminalLogs: make([]string, 0),
	}
}

func (m *MockTerminalGateway) SendToTerminal(intent *PaymentIntent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TerminalLogs = append(m.TerminalLogs, fmt.Sprintf("SEND: %s (%d minor)", intent.ID, intent.AmountMinor))
	if m.ShouldFail {
		return fmt.Errorf("terminal connection error: card reader unresponsive")
	}
	if m.ShouldTimeOut {
		return fmt.Errorf("terminal timeout: no response after 30s")
	}
	return nil
}

func (m *MockTerminalGateway) Capture(intentID string) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.FailOnCapture {
		return "", "", fmt.Errorf("card declined by issuer (insufficient funds)")
	}
	ref := fmt.Sprintf("TXN-%d", time.Now().UnixNano())
	auth := "AUTH9988"
	m.TerminalLogs = append(m.TerminalLogs, fmt.Sprintf("CAPTURE: %s -> Ref:%s", intentID, ref))
	return ref, auth, nil
}

func (m *MockTerminalGateway) Cancel(intentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TerminalLogs = append(m.TerminalLogs, fmt.Sprintf("CANCEL: %s", intentID))
	return nil
}

func (m *MockTerminalGateway) ReconcileTimeout(intentID string) (IntentStatus, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Simulated provider reconciliation: looks up provider switch to see if transaction actually went through
	ref := fmt.Sprintf("REC-%d", time.Now().UnixNano())
	m.TerminalLogs = append(m.TerminalLogs, fmt.Sprintf("RECONCILE: %s -> Ref:%s", intentID, ref))
	return IntentCaptured, ref, nil
}

// TerminalService coordinates terminal intents, idempotency, execution, and timeout reconciliation
type TerminalService struct {
	mu      sync.RWMutex
	gateway TerminalGateway
	intents map[string]*PaymentIntent // keyed by ID
	byKey   map[string]*PaymentIntent // keyed by IdempotencyKey
	db      *data.RealDBPool
}

// NewTerminalService constructs a TerminalService
func NewTerminalService(gateway TerminalGateway) *TerminalService {
	return &TerminalService{
		gateway: gateway,
		intents: make(map[string]*PaymentIntent),
		byKey:   make(map[string]*PaymentIntent),
	}
}

// SetDB configures the real database pool
func (ts *TerminalService) SetDB(db *data.RealDBPool) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.db = db
}

// CreateIntent creates or idempotently retrieves a payment intent
func (ts *TerminalService) CreateIntent(
	saleID string,
	method PaymentMethod,
	amountMinor int64,
	terminalID string,
	idempotencyKey string,
) (*PaymentIntent, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if idempotencyKey != "" {
		if existing, ok := ts.byKey[idempotencyKey]; ok {
			return existing, nil
		}
	}

	if amountMinor <= 0 {
		return nil, fmt.Errorf("intent amount must be > 0")
	}

	b := make([]byte, 12)
	_, _ = rand.Read(b)
	intentID := fmt.Sprintf("pi-%s", hex.EncodeToString(b))

	now := time.Now()
	intent := &PaymentIntent{
		ID:             intentID,
		IdempotencyKey: idempotencyKey,
		SaleID:         saleID,
		Method:         method,
		AmountMinor:    amountMinor,
		Status:         IntentCreated,
		TerminalID:     terminalID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	ts.intents[intentID] = intent
	if idempotencyKey != "" {
		ts.byKey[idempotencyKey] = intent
	}

	return intent, nil
}

// ExecuteIntent pushes the intent to the terminal hardware, authorizes, and captures the payment
func (ts *TerminalService) ExecuteIntent(intentID string) (*PaymentIntent, error) {
	ts.mu.Lock()
	intent, ok := ts.intents[intentID]
	ts.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("payment intent not found: %s", intentID)
	}

	intent.Status = IntentSent
	intent.UpdatedAt = time.Now()

	// 1. Send to hardware terminal
	if err := ts.gateway.SendToTerminal(intent); err != nil {
		intent.Status = IntentTimedOut
		intent.LastError = err.Error()
		intent.UpdatedAt = time.Now()

		// Automatic Reconciliation Attempt on timeout
		recStatus, recRef, recErr := ts.gateway.ReconcileTimeout(intentID)
		if recErr == nil && recStatus == IntentCaptured {
			intent.Status = IntentCaptured
			intent.ProviderRef = recRef
			intent.LastError = ""
			return intent, nil
		}

		intent.Status = IntentFailed
		return intent, fmt.Errorf("terminal transaction failed: %w", err)
	}

	intent.Status = IntentAuthorized

	// 2. Capture authorized tender
	ref, auth, err := ts.gateway.Capture(intentID)
	if err != nil {
		intent.Status = IntentFailed
		intent.LastError = err.Error()
		intent.UpdatedAt = time.Now()
		_ = ts.gateway.Cancel(intentID)
		return intent, fmt.Errorf("capture failed: %w", err)
	}

	intent.Status = IntentCaptured
	intent.ProviderRef = ref
	intent.AuthCode = auth
	intent.UpdatedAt = time.Now()

	return intent, nil
}

// GetIntent retrieves intent by ID
func (ts *TerminalService) GetIntent(intentID string) (*PaymentIntent, bool) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	intent, ok := ts.intents[intentID]
	return intent, ok
}
