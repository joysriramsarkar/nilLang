package pos

import (
	"fmt"
	"sync"
	"time"
)

// ShiftStatus indicates whether the register session is open or closed
type ShiftStatus string

const (
	ShiftOpen   ShiftStatus = "OPEN"
	ShiftClosed ShiftStatus = "CLOSED"
)

// Shift represents a cashier session with float and cash reconciliation
type Shift struct {
	ID                  string      `json:"id"`
	RegisterID          string      `json:"register_id"`
	CashierID           string      `json:"cashier_id"`
	CashierName         string      `json:"cashier_name"`
	Status              ShiftStatus `json:"status"`
	StartTime           time.Time   `json:"start_time"`
	EndTime             *time.Time  `json:"end_time,omitempty"`
	StartingCashMinor   int64       `json:"starting_cash_minor"` // Opening float
	CashSalesMinor      int64       `json:"cash_sales_minor"`
	CardSalesMinor      int64       `json:"card_sales_minor"`
	MFSSalesMinor       int64       `json:"mfs_sales_minor"` // bKash/Nagad
	TotalSalesMinor     int64          `json:"total_sales_minor"`
	TotalOrders         int64          `json:"total_orders"`
	ExpectedCashMinor   int64          `json:"expected_cash_minor"`
	ActualCashMinor     int64          `json:"actual_cash_minor"`
	CashDifferenceMinor int64          `json:"cash_difference_minor"` // (Actual - Expected)
	Movements           []CashMovement `json:"movements,omitempty"`
	Notes               string         `json:"notes,omitempty"`
}

type CashMovementType string

const (
	CashMovementIn  CashMovementType = "CASH_IN"  // Cash float addition
	CashMovementOut CashMovementType = "CASH_OUT" // Cash drop to safe / payout
)

// CashMovement logs cash drops or additions during an open shift
type CashMovement struct {
	ID          string           `json:"id"`
	ShiftID     string           `json:"shift_id"`
	Type        CashMovementType `json:"type"`
	AmountMinor int64            `json:"amount_minor"`
	Reason      string           `json:"reason"`
	Timestamp   time.Time        `json:"timestamp"`
}

// ShiftManager manages register open/close workflows
type ShiftManager struct {
	mu           sync.RWMutex
	currentShift *Shift
	history      []*Shift
}

// NewShiftManager creates a ShiftManager
func NewShiftManager() *ShiftManager {
	return &ShiftManager{
		history: make([]*Shift, 0),
	}
}

// OpenShift opens a new shift session
func (sm *ShiftManager) OpenShift(registerID, cashierID, cashierName string, startingCashMinor int64) (*Shift, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.currentShift != nil && sm.currentShift.Status == ShiftOpen {
		return nil, fmt.Errorf("a shift is already open for register %s", sm.currentShift.RegisterID)
	}

	shift := &Shift{
		ID:                fmt.Sprintf("shift-%d", time.Now().Unix()),
		RegisterID:        registerID,
		CashierID:         cashierID,
		CashierName:       cashierName,
		Status:            ShiftOpen,
		StartTime:         time.Now(),
		StartingCashMinor: startingCashMinor,
		ExpectedCashMinor: startingCashMinor,
	}

	sm.currentShift = shift
	return shift, nil
}

// CurrentShift returns active open shift
func (sm *ShiftManager) CurrentShift() (*Shift, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.currentShift == nil || sm.currentShift.Status != ShiftOpen {
		return nil, fmt.Errorf("no active open shift")
	}
	s := *sm.currentShift
	return &s, nil
}

// HasActiveShift returns true if a shift is currently open.
func (sm *ShiftManager) HasActiveShift() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentShift != nil && sm.currentShift.Status == ShiftOpen
}

// RecordSale adds payment totals to current shift metrics
func (sm *ShiftManager) RecordSale(sale *Sale) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.currentShift == nil || sm.currentShift.Status != ShiftOpen {
		return fmt.Errorf("cannot record sale: no active shift")
	}

	shift := sm.currentShift
	shift.TotalOrders++
	shift.TotalSalesMinor += sale.TotalMinor

	for _, p := range sale.Payments {
		switch p.Method {
		case MethodCash:
			// Net cash added = amount paid minus change returned
			netCash := p.AmountMinor - sale.ChangeMinor
			shift.CashSalesMinor += netCash
			shift.ExpectedCashMinor += netCash
		case MethodCard:
			shift.CardSalesMinor += p.AmountMinor
		case MethodBKash, MethodNagad, MethodUPI:
			shift.MFSSalesMinor += p.AmountMinor
		}
	}

	return nil
}

// CloseShift closes the register with physical cash count
func (sm *ShiftManager) CloseShift(actualCashMinor int64, notes string) (*Shift, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.currentShift == nil || sm.currentShift.Status != ShiftOpen {
		return nil, fmt.Errorf("no open shift to close")
	}

	shift := sm.currentShift
	now := time.Now()
	shift.EndTime = &now
	shift.Status = ShiftClosed
	shift.ActualCashMinor = actualCashMinor
	shift.CashDifferenceMinor = actualCashMinor - shift.ExpectedCashMinor
	shift.Notes = notes

	sm.history = append(sm.history, shift)
	sm.currentShift = nil

	return shift, nil
}

// RecordCashMovement records cash addition or drop in active shift
func (sm *ShiftManager) RecordCashMovement(moveType CashMovementType, amountMinor int64, reason string) (*CashMovement, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.currentShift == nil || sm.currentShift.Status != ShiftOpen {
		return nil, fmt.Errorf("no active open shift for cash movement")
	}

	if amountMinor <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	mov := CashMovement{
		ID:          fmt.Sprintf("cmov-%d", time.Now().UnixNano()),
		ShiftID:     sm.currentShift.ID,
		Type:        moveType,
		AmountMinor: amountMinor,
		Reason:      reason,
		Timestamp:   time.Now(),
	}

	sm.currentShift.Movements = append(sm.currentShift.Movements, mov)

	if moveType == CashMovementIn {
		sm.currentShift.ExpectedCashMinor += amountMinor
	} else {
		sm.currentShift.ExpectedCashMinor -= amountMinor
	}

	return &mov, nil
}
