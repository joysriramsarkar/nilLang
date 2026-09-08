package pos

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

var shiftSeq int64

// ShiftStatus indicates whether the register session is open or closed
type ShiftStatus string

const (
	ShiftOpen   ShiftStatus = "OPEN"
	ShiftClosed ShiftStatus = "CLOSED"
)

// DenominationCount breaks down physical cash count by denomination
type DenominationCount struct {
	Note1000 int `json:"note_1000"`
	Note500  int `json:"note_500"`
	Note200  int `json:"note_200"`
	Note100  int `json:"note_100"`
	Note50   int `json:"note_50"`
	Note20   int `json:"note_20"`
	Note10   int `json:"note_10"`
	Note5    int `json:"note_5"`
	Note2    int `json:"note_2"`
	Coin1    int `json:"coin_1"`
}

// TotalMinor computes the total minor currency sum of counted notes and coins
func (d DenominationCount) TotalMinor() int64 {
	return int64(d.Note1000)*100000 +
		int64(d.Note500)*50000 +
		int64(d.Note200)*20000 +
		int64(d.Note100)*10000 +
		int64(d.Note50)*5000 +
		int64(d.Note20)*2000 +
		int64(d.Note10)*1000 +
		int64(d.Note5)*500 +
		int64(d.Note2)*200 +
		int64(d.Coin1)*100
}

// MaxCashVarianceAllowedWithoutManagerMinor is ৳100.00 (10,000 minor)
const MaxCashVarianceAllowedWithoutManagerMinor int64 = 10000

// ErrShiftVarianceManagerApprovalRequired is returned when variance > ৳100 without manager role
var ErrShiftVarianceManagerApprovalRequired = fmt.Errorf("shift cash variance exceeds threshold (৳100.00) and requires manager approval")

// Shift represents a cashier session with float and cash reconciliation
type Shift struct {
	ID                  string         `json:"id"`
	RegisterID          string         `json:"register_id"`
	CashierID           string         `json:"cashier_id"`
	CashierName         string         `json:"cashier_name"`
	Status              ShiftStatus    `json:"status"`
	StartTime           time.Time      `json:"start_time"`
	EndTime             *time.Time     `json:"end_time,omitempty"`
	StartingCashMinor   int64          `json:"starting_cash_minor"` // Opening float
	CashSalesMinor      int64          `json:"cash_sales_minor"`
	CardSalesMinor      int64          `json:"card_sales_minor"`
	MFSSalesMinor       int64          `json:"mfs_sales_minor"` // bKash/Nagad
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

// ShiftManager manages register open/close workflows and DB persistence
type ShiftManager struct {
	mu           sync.RWMutex
	currentShift *Shift
	history      []*Shift
	db           *data.RealDBPool
	audit        *AuditTrail
}

// NewShiftManager creates a ShiftManager
func NewShiftManager() *ShiftManager {
	return &ShiftManager{
		history: make([]*Shift, 0),
	}
}

// SetDB configures the real database pool for persistent shift reconciliation.
func (sm *ShiftManager) SetDB(db *data.RealDBPool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.db = db
}

// SetAudit configures audit logging for shift actions.
func (sm *ShiftManager) SetAudit(audit *AuditTrail) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.audit = audit
}

// OpenShift opens a new shift session
func (sm *ShiftManager) OpenShift(registerID, cashierID, cashierName string, startingCashMinor int64) (*Shift, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.currentShift != nil && sm.currentShift.Status == ShiftOpen {
		return nil, fmt.Errorf("a shift is already open for register %s", sm.currentShift.RegisterID)
	}

	seq := atomic.AddInt64(&shiftSeq, 1)
	now := time.Now()
	nowStr := now.UTC().Format(time.RFC3339)
	shiftID := fmt.Sprintf("shift-%d-%d", now.UnixNano(), seq)

	shift := &Shift{
		ID:                shiftID,
		RegisterID:        registerID,
		CashierID:         cashierID,
		CashierName:       cashierName,
		Status:            ShiftOpen,
		StartTime:         now,
		StartingCashMinor: startingCashMinor,
		ExpectedCashMinor: startingCashMinor,
	}

	if sm.db != nil {
		_, err := sm.db.Exec(
			`INSERT INTO shifts (id, register_id, cashier_id, cashier_name, status, start_time, starting_cash_minor, expected_cash_minor, created_at, updated_at)
			 VALUES (?, ?, ?, ?, 'OPEN', ?, ?, ?, ?, ?)`,
			shift.ID, shift.RegisterID, shift.CashierID, shift.CashierName,
			nowStr, shift.StartingCashMinor, shift.ExpectedCashMinor, nowStr, nowStr,
		)
		if err != nil {
			return nil, fmt.Errorf("persist open shift: %w", err)
		}
	}

	if sm.audit != nil {
		sm.audit.Record(ActionShiftOpened, shift.ID, cashierID, nil,
			map[string]interface{}{"starting_cash_minor": startingCashMinor},
			fmt.Sprintf("Opened shift on register %s with float ৳%.2f", registerID, float64(startingCashMinor)/100.0))
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

	if sm.db != nil {
		nowStr := time.Now().UTC().Format(time.RFC3339)
		_, _ = sm.db.Exec(
			`UPDATE shifts SET total_orders=?, total_sales_minor=?, cash_sales_minor=?, card_sales_minor=?, mfs_sales_minor=?, expected_cash_minor=?, updated_at=?
			 WHERE id=?`,
			shift.TotalOrders, shift.TotalSalesMinor, shift.CashSalesMinor, shift.CardSalesMinor, shift.MFSSalesMinor, shift.ExpectedCashMinor, nowStr, shift.ID,
		)
	}

	return nil
}

// CloseShift closes the register with physical cash count and enforces manager approval on variance
func (sm *ShiftManager) CloseShift(actualCashMinor int64, notes string, managerRole ...string) (*Shift, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.currentShift == nil || sm.currentShift.Status != ShiftOpen {
		return nil, fmt.Errorf("no open shift to close")
	}

	shift := sm.currentShift
	variance := actualCashMinor - shift.ExpectedCashMinor
	absVariance := variance
	if absVariance < 0 {
		absVariance = -absVariance
	}

	if absVariance > MaxCashVarianceAllowedWithoutManagerMinor {
		role := RoleCashier
		if len(managerRole) > 0 && managerRole[0] != "" {
			role = managerRole[0]
		}
		if role != RoleManager && role != RoleAdmin {
			return nil, ErrShiftVarianceManagerApprovalRequired
		}
	}

	now := time.Now()
	nowStr := now.UTC().Format(time.RFC3339)
	shift.EndTime = &now
	shift.Status = ShiftClosed
	shift.ActualCashMinor = actualCashMinor
	shift.CashDifferenceMinor = variance
	shift.Notes = notes

	if sm.db != nil {
		_, err := sm.db.Exec(
			`UPDATE shifts SET status='CLOSED', end_time=?, actual_cash_minor=?, cash_difference_minor=?, notes=?, updated_at=?
			 WHERE id=?`,
			nowStr, actualCashMinor, variance, notes, nowStr, shift.ID,
		)
		if err != nil {
			return nil, fmt.Errorf("persist close shift: %w", err)
		}
	}

	if sm.audit != nil {
		sm.audit.Record(ActionShiftClosed, shift.ID, shift.CashierID,
			map[string]interface{}{"expected_cash_minor": shift.ExpectedCashMinor},
			map[string]interface{}{"actual_cash_minor": actualCashMinor, "difference_minor": variance},
			fmt.Sprintf("Closed shift on register %s: Expected ৳%.2f, Counted ৳%.2f (Variance: ৳%.2f)",
				shift.RegisterID, float64(shift.ExpectedCashMinor)/100.0, float64(actualCashMinor)/100.0, float64(variance)/100.0))
	}

	sm.history = append(sm.history, shift)
	sm.currentShift = nil

	return shift, nil
}

// CloseShiftWithDenominations closes the shift using counted notes and coins
func (sm *ShiftManager) CloseShiftWithDenominations(denoms DenominationCount, notes string, managerRole ...string) (*Shift, error) {
	return sm.CloseShift(denoms.TotalMinor(), notes, managerRole...)
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

	seq := atomic.AddInt64(&shiftSeq, 1)
	now := time.Now()
	nowStr := now.UTC().Format(time.RFC3339)
	mov := CashMovement{
		ID:          fmt.Sprintf("cmov-%d-%d", now.UnixNano(), seq),
		ShiftID:     sm.currentShift.ID,
		Type:        moveType,
		AmountMinor: amountMinor,
		Reason:      reason,
		Timestamp:   now,
	}

	if moveType == CashMovementIn {
		sm.currentShift.ExpectedCashMinor += amountMinor
	} else {
		sm.currentShift.ExpectedCashMinor -= amountMinor
	}

	if sm.db != nil {
		_, err := sm.db.Exec(
			`INSERT INTO cash_movements (id, shift_id, type, amount_minor, reason, timestamp)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			mov.ID, mov.ShiftID, string(mov.Type), mov.AmountMinor, mov.Reason, nowStr,
		)
		if err != nil {
			return nil, fmt.Errorf("persist cash movement: %w", err)
		}
		_, _ = sm.db.Exec(
			`UPDATE shifts SET expected_cash_minor=?, updated_at=? WHERE id=?`,
			sm.currentShift.ExpectedCashMinor, nowStr, sm.currentShift.ID,
		)
	}

	sm.currentShift.Movements = append(sm.currentShift.Movements, mov)
	return &mov, nil
}
