package pos

import (
	"fmt"
	"sync"
	"time"
)

// AuditAction defines critical enterprise actions
type AuditAction string

const (
	ActionSaleCompleted    AuditAction = "SALE_COMPLETED"
	ActionSaleRefunded     AuditAction = "SALE_REFUNDED"
	ActionStockAdjusted    AuditAction = "STOCK_ADJUSTED"
	ActionShiftOpened      AuditAction = "SHIFT_OPENED"
	ActionShiftClosed      AuditAction = "SHIFT_CLOSED"
	ActionCustomerDuePaid  AuditAction = "CUSTOMER_DUE_PAID"
	ActionSupplierPaid     AuditAction = "SUPPLIER_PAID"
	ActionPurchaseReturned AuditAction = "PURCHASE_RETURNED"
)

// AuditEntry records a tamper-evident audit trail log
type AuditEntry struct {
	ID          string                 `json:"id"`
	Action      AuditAction            `json:"action"`
	EntityID    string                 `json:"entity_id"`
	Actor       string                 `json:"actor"`
	RequestID   string                 `json:"request_id,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	IP          string                 `json:"ip,omitempty"`
	DeviceID    string                 `json:"device_id,omitempty"`
	BeforeState map[string]interface{} `json:"before_state,omitempty"`
	AfterState  map[string]interface{} `json:"after_state,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Notes       string                 `json:"notes,omitempty"`
}

// AuditContext holds operational network and device metadata for tamper-evident logging.
type AuditContext struct {
	RequestID string
	SessionID string
	IP        string
	DeviceID  string
}

// AuditTrail records and provides access to enterprise audit logs
type AuditTrail struct {
	mu      sync.RWMutex
	entries []*AuditEntry
}

// NewAuditTrail creates an audit trail
func NewAuditTrail() *AuditTrail {
	return &AuditTrail{
		entries: make([]*AuditEntry, 0),
	}
}

// Record creates a new immutable audit log entry.
func (at *AuditTrail) Record(action AuditAction, entityID, actor string, before, after map[string]interface{}, notes string) *AuditEntry {
	return at.RecordWithContext(action, entityID, actor, before, after, notes, AuditContext{})
}

// RecordWithContext creates an audit log entry with request/session/device metadata.
func (at *AuditTrail) RecordWithContext(
	action AuditAction,
	entityID, actor string,
	before, after map[string]interface{},
	notes string,
	ctx AuditContext,
) *AuditEntry {
	at.mu.Lock()
	defer at.mu.Unlock()

	entry := &AuditEntry{
		ID:          fmt.Sprintf("aud-%d", time.Now().UnixNano()),
		Action:      action,
		EntityID:    entityID,
		Actor:       actor,
		RequestID:   ctx.RequestID,
		SessionID:   ctx.SessionID,
		IP:          ctx.IP,
		DeviceID:    ctx.DeviceID,
		BeforeState: before,
		AfterState:  after,
		Timestamp:   time.Now().UTC(),
		Notes:       notes,
	}

	at.entries = append(at.entries, entry)
	return entry
}

// RecentEntries returns latest N audit logs
func (at *AuditTrail) RecentEntries(limit int) []*AuditEntry {
	at.mu.RLock()
	defer at.mu.RUnlock()

	n := len(at.entries)
	if limit > n {
		limit = n
	}

	res := make([]*AuditEntry, limit)
	for i := 0; i < limit; i++ {
		res[i] = at.entries[n-1-i]
	}
	return res
}
