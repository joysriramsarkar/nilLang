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
	TotalSalesMinor     int64       `json:"total_sales_minor"`
	TotalOrders         int64       `json:"total_orders"`
	ExpectedCashMinor   int64       `json:"expected_cash_minor"`
	ActualCashMinor     int64       `json:"actual_cash_minor"`
	CashDifferenceMinor int64       `json:"cash_difference_minor"` // (Actual - Expected)
	Notes               string      `json:"notes,omitempty"`
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
