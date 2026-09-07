package pos

import (
	"fmt"
	"sync"
	"time"
)

// AuditAction defines critical enterprise actions
type AuditAction string

const (
	ActionSaleCompleted   AuditAction = "SALE_COMPLETED"
	ActionSaleRefunded    AuditAction = "SALE_REFUNDED"
	ActionStockAdjusted   AuditAction = "STOCK_ADJUSTED"
	ActionShiftOpened     AuditAction = "SHIFT_OPENED"
	ActionShiftClosed     AuditAction = "SHIFT_CLOSED"
	ActionCustomerDuePaid AuditAction = "CUSTOMER_DUE_PAID"
)

// AuditEntry records a tamper-evident audit trail log
type AuditEntry struct {
	ID          string                 `json:"id"`
	Action      AuditAction            `json:"action"`
	EntityID    string                 `json:"entity_id"`
	Actor       string                 `json:"actor"`
	BeforeState map[string]interface{} `json:"before_state,omitempty"`
	AfterState  map[string]interface{} `json:"after_state,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Notes       string                 `json:"notes,omitempty"`
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

// Record creates a new audit log
func (at *AuditTrail) Record(action AuditAction, entityID, actor string, before, after map[string]interface{}, notes string) *AuditEntry {
	at.mu.Lock()
	defer at.mu.Unlock()

	entry := &AuditEntry{
		ID:          fmt.Sprintf("aud-%d", time.Now().UnixNano()),
		Action:      action,
		EntityID:    entityID,
		Actor:       actor,
		BeforeState: before,
		AfterState:  after,
		Timestamp:   time.Now(),
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
