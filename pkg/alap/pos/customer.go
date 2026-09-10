package pos

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

var ledgerSeq int64

// Customer represents a retail or wholesale client
type Customer struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Phone               string `json:"phone"`
	Email               string `json:"email,omitempty"`
	Address             string `json:"address,omitempty"`
	TotalPurchasesMinor int64  `json:"total_purchases_minor"`
	DueBalanceMinor     int64  `json:"due_balance_minor"` // Outstanding debt
}

// CustomerLedgerEntry tracks credit transactions and due payments
type CustomerLedgerEntry struct {
	ID           string    `json:"id"`
	CustomerID   string    `json:"customer_id"`
	Type         string    `json:"type"` // "CREDIT_SALE" or "DUE_PAYMENT"
	AmountMinor  int64     `json:"amount_minor"`
	BalanceAfter int64     `json:"balance_after"`
	Reference    string    `json:"reference"`
	Timestamp    time.Time `json:"timestamp"`
	Notes        string    `json:"notes,omitempty"`
}

// CustomerRepository manages customer records and credit ledgers
type CustomerRepository struct {
	mu        sync.RWMutex
	customers map[string]*Customer
	entries   []*CustomerLedgerEntry
}

// NewCustomerRepository creates a customer repository
func NewCustomerRepository() *CustomerRepository {
	return &CustomerRepository{
		customers: make(map[string]*Customer),
		entries:   make([]*CustomerLedgerEntry, 0),
	}
}

// AddCustomer registers a customer
func (cr *CustomerRepository) AddCustomer(c *Customer) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cp := *c
	cr.customers[c.ID] = &cp
}

// FindByID finds customer by ID
func (cr *CustomerRepository) FindByID(id string) (*Customer, bool) {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	c, ok := cr.customers[id]
	if !ok {
		return nil, false
	}
	cp := *c
	return &cp, true
}

// AllCustomers returns list of customers
func (cr *CustomerRepository) AllCustomers() []*Customer {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	list := make([]*Customer, 0, len(cr.customers))
	for _, c := range cr.customers {
		cp := *c
		list = append(list, &cp)
	}
	return list
}

// RecordCreditSale increases customer due balance
func (cr *CustomerRepository) RecordCreditSale(customerID string, amountMinor int64, invoiceNo string) (*CustomerLedgerEntry, error) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	c, exists := cr.customers[customerID]
	if !exists {
		return nil, fmt.Errorf("customer not found: %s", customerID)
	}

	c.DueBalanceMinor += amountMinor
	c.TotalPurchasesMinor += amountMinor

	entry := &CustomerLedgerEntry{
		ID:           fmt.Sprintf("cled-%d-%d", time.Now().UnixNano(), atomic.AddInt64(&ledgerSeq, 1)),
		CustomerID:   customerID,
		Type:         "CREDIT_SALE",
		AmountMinor:  amountMinor,
		BalanceAfter: c.DueBalanceMinor,
		Reference:    invoiceNo,
		Timestamp:    time.Now(),
		Notes:        fmt.Sprintf("Baki on Invoice %s", invoiceNo),
	}

	cr.entries = append(cr.entries, entry)
	return entry, nil
}

// RecordDuePayment decreases customer due balance
func (cr *CustomerRepository) RecordDuePayment(customerID string, amountMinor int64, reference, notes string) (*CustomerLedgerEntry, error) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	c, exists := cr.customers[customerID]
	if !exists {
		return nil, fmt.Errorf("customer not found: %s", customerID)
	}

	if amountMinor <= 0 {
		return nil, fmt.Errorf("payment amount must be greater than zero")
	}

	c.DueBalanceMinor -= amountMinor

	entry := &CustomerLedgerEntry{
		ID:           fmt.Sprintf("cled-%d-%d", time.Now().UnixNano(), atomic.AddInt64(&ledgerSeq, 1)),
		CustomerID:   customerID,
		Type:         "DUE_PAYMENT",
		AmountMinor:  amountMinor,
		BalanceAfter: c.DueBalanceMinor,
		Reference:    reference,
		Timestamp:    time.Now(),
		Notes:        strings.TrimSpace(notes),
	}

	cr.entries = append(cr.entries, entry)
	return entry, nil
}

// EntriesForCustomer returns ledger history for a customer
func (cr *CustomerRepository) EntriesForCustomer(customerID string) []*CustomerLedgerEntry {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	res := make([]*CustomerLedgerEntry, 0)
	for _, e := range cr.entries {
		if e.CustomerID == customerID {
			res = append(res, e)
		}
	}
	return res
}

// ─── TRANSACTIONAL DATABASE LEDGER METHODS ──────────────────────────────────

// RecordCreditSaleTx records a credit sale atomically in the database inside tx,
// updates customer totals, and records a customer_ledger entry with exact balance_after.
func (cr *CustomerRepository) RecordCreditSaleTx(
	tx *data.RealTx, customerID string, amountMinor int64, invoiceNo, cashierID string,
) (*CustomerLedgerEntry, error) {
	if amountMinor <= 0 {
		return nil, fmt.Errorf("credit sale amount must be positive")
	}
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	res, err := tx.Exec(
		`UPDATE customers SET due_balance_minor=due_balance_minor+?, total_purchases_minor=total_purchases_minor+?, updated_at=? WHERE id=?`,
		amountMinor, amountMinor, nowStr, customerID,
	)
	if err != nil {
		return nil, fmt.Errorf("update customer credit sale: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return nil, fmt.Errorf("customer not found: %s", customerID)
	}

	var newDueBalance int64
	if err := tx.QueryRow(`SELECT due_balance_minor FROM customers WHERE id=?`, customerID).Scan(&newDueBalance); err != nil {
		return nil, fmt.Errorf("read updated customer due balance: %w", err)
	}

	ledgerID := fmt.Sprintf("cled-%d-%d", now.UnixNano(), atomic.AddInt64(&ledgerSeq, 1))
	notes := fmt.Sprintf("Baki on Invoice %s", invoiceNo)
	_, err = tx.Exec(
		`INSERT INTO customer_ledger (id,customer_id,type,amount_minor,balance_after,reference,notes,created_by,timestamp) VALUES (?,?,?,?,?,?,?,?,?)`,
		ledgerID, customerID, "CREDIT_SALE", amountMinor, newDueBalance,
		invoiceNo, notes, cashierID, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("insert customer_ledger: %w", err)
	}

	entry := &CustomerLedgerEntry{
		ID:           ledgerID,
		CustomerID:   customerID,
		Type:         "CREDIT_SALE",
		AmountMinor:  amountMinor,
		BalanceAfter: newDueBalance,
		Reference:    invoiceNo,
		Timestamp:    now,
		Notes:        notes,
	}

	// Sync in-memory cache if present
	cr.mu.Lock()
	if c, ok := cr.customers[customerID]; ok {
		c.DueBalanceMinor = newDueBalance
		c.TotalPurchasesMinor += amountMinor
	}
	cr.entries = append(cr.entries, entry)
	cr.mu.Unlock()

	return entry, nil
}

// RecordDuePaymentTx records a payment toward outstanding balance atomically inside tx.
func (cr *CustomerRepository) RecordDuePaymentTx(
	tx *data.RealTx, customerID string, amountMinor int64, reference, notes, cashierID string,
) (*CustomerLedgerEntry, error) {
	if amountMinor <= 0 {
		return nil, fmt.Errorf("payment amount must be greater than zero")
	}
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	res, err := tx.Exec(
		`UPDATE customers SET due_balance_minor=due_balance_minor-?, updated_at=? WHERE id=?`,
		amountMinor, nowStr, customerID,
	)
	if err != nil {
		return nil, fmt.Errorf("update customer due payment: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return nil, fmt.Errorf("customer not found: %s", customerID)
	}

	var newDueBalance int64
	if err := tx.QueryRow(`SELECT due_balance_minor FROM customers WHERE id=?`, customerID).Scan(&newDueBalance); err != nil {
		return nil, fmt.Errorf("read updated customer due balance: %w", err)
	}

	ledgerID := fmt.Sprintf("cled-%d-%d", now.UnixNano(), atomic.AddInt64(&ledgerSeq, 1))
	_, err = tx.Exec(
		`INSERT INTO customer_ledger (id,customer_id,type,amount_minor,balance_after,reference,notes,created_by,timestamp) VALUES (?,?,?,?,?,?,?,?,?)`,
		ledgerID, customerID, "DUE_PAYMENT", amountMinor, newDueBalance,
		reference, notes, cashierID, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("insert customer_ledger: %w", err)
	}

	entry := &CustomerLedgerEntry{
		ID:           ledgerID,
		CustomerID:   customerID,
		Type:         "DUE_PAYMENT",
		AmountMinor:  amountMinor,
		BalanceAfter: newDueBalance,
		Reference:    reference,
		Timestamp:    now,
		Notes:        notes,
	}

	cr.mu.Lock()
	if c, ok := cr.customers[customerID]; ok {
		c.DueBalanceMinor = newDueBalance
	}
	cr.entries = append(cr.entries, entry)
	cr.mu.Unlock()

	return entry, nil
}

// RecordAdjustmentTx records a ledger balance adjustment (e.g. debit/credit reconciliation).
func (cr *CustomerRepository) RecordAdjustmentTx(
	tx *data.RealTx, customerID string, deltaMinor int64, reference, notes, cashierID string,
) (*CustomerLedgerEntry, error) {
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	res, err := tx.Exec(
		`UPDATE customers SET due_balance_minor=due_balance_minor+?, updated_at=? WHERE id=?`,
		deltaMinor, nowStr, customerID,
	)
	if err != nil {
		return nil, fmt.Errorf("update customer adjustment: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return nil, fmt.Errorf("customer not found: %s", customerID)
	}

	var newDueBalance int64
	if err := tx.QueryRow(`SELECT due_balance_minor FROM customers WHERE id=?`, customerID).Scan(&newDueBalance); err != nil {
		return nil, fmt.Errorf("read updated customer due balance: %w", err)
	}

	adjType := "ADJUSTMENT_DEBIT"
	if deltaMinor < 0 {
		adjType = "ADJUSTMENT_CREDIT"
	}

	ledgerID := fmt.Sprintf("cled-%d-%d", now.UnixNano(), atomic.AddInt64(&ledgerSeq, 1))
	_, err = tx.Exec(
		`INSERT INTO customer_ledger (id,customer_id,type,amount_minor,balance_after,reference,notes,created_by,timestamp) VALUES (?,?,?,?,?,?,?,?,?)`,
		ledgerID, customerID, adjType, deltaMinor, newDueBalance,
		reference, notes, cashierID, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("insert customer_ledger adjustment: %w", err)
	}

	entry := &CustomerLedgerEntry{
		ID:           ledgerID,
		CustomerID:   customerID,
		Type:         adjType,
		AmountMinor:  deltaMinor,
		BalanceAfter: newDueBalance,
		Reference:    reference,
		Timestamp:    now,
		Notes:        notes,
	}

	cr.mu.Lock()
	if c, ok := cr.customers[customerID]; ok {
		c.DueBalanceMinor = newDueBalance
	}
	cr.entries = append(cr.entries, entry)
	cr.mu.Unlock()

	return entry, nil
}

// RecordRefundCreditTx decrements customer due balance and total purchases when a credit sale is refunded.
func (cr *CustomerRepository) RecordRefundCreditTx(
	tx *data.RealTx, customerID string, amountMinor int64, invoiceNo, cashierID string,
) (*CustomerLedgerEntry, error) {
	if amountMinor <= 0 {
		return nil, fmt.Errorf("refund credit amount must be positive")
	}
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	res, err := tx.Exec(
		`UPDATE customers SET due_balance_minor=due_balance_minor-?, total_purchases_minor=total_purchases_minor-?, updated_at=? WHERE id=?`,
		amountMinor, amountMinor, nowStr, customerID,
	)
	if err != nil {
		return nil, fmt.Errorf("update customer refund credit: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return nil, fmt.Errorf("customer not found: %s", customerID)
	}

	var newDueBalance int64
	if err := tx.QueryRow(`SELECT due_balance_minor FROM customers WHERE id=?`, customerID).Scan(&newDueBalance); err != nil {
		return nil, fmt.Errorf("read updated customer due balance: %w", err)
	}

	ledgerID := fmt.Sprintf("cled-%d-%d", now.UnixNano(), atomic.AddInt64(&ledgerSeq, 1))
	notes := fmt.Sprintf("Refund on Invoice %s", invoiceNo)
	_, err = tx.Exec(
		`INSERT INTO customer_ledger (id,customer_id,type,amount_minor,balance_after,reference,notes,created_by,timestamp) VALUES (?,?,?,?,?,?,?,?,?)`,
		ledgerID, customerID, "REFUND_CREDIT", amountMinor, newDueBalance,
		invoiceNo, notes, cashierID, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("insert customer_ledger refund: %w", err)
	}

	entry := &CustomerLedgerEntry{
		ID:           ledgerID,
		CustomerID:   customerID,
		Type:         "REFUND_CREDIT",
		AmountMinor:  amountMinor,
		BalanceAfter: newDueBalance,
		Reference:    invoiceNo,
		Timestamp:    now,
		Notes:        notes,
	}

	cr.mu.Lock()
	if c, ok := cr.customers[customerID]; ok {
		c.DueBalanceMinor = newDueBalance
		c.TotalPurchasesMinor -= amountMinor
	}
	cr.entries = append(cr.entries, entry)
	cr.mu.Unlock()

	return entry, nil
}
