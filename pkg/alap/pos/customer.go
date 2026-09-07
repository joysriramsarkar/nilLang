package pos

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

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
	cr.customers[c.ID] = c
}

// FindByID finds customer by ID
func (cr *CustomerRepository) FindByID(id string) (*Customer, bool) {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	c, ok := cr.customers[id]
	return c, ok
}

// AllCustomers returns list of customers
func (cr *CustomerRepository) AllCustomers() []*Customer {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	list := make([]*Customer, 0, len(cr.customers))
	for _, c := range cr.customers {
		list = append(list, c)
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
		ID:           fmt.Sprintf("cled-%d", time.Now().UnixNano()),
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
		ID:           fmt.Sprintf("cled-%d", time.Now().UnixNano()),
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
