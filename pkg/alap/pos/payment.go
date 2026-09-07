package pos

import (
	"fmt"
	"time"
)

// PaymentMethod specifies the tender method
type PaymentMethod string

const (
	MethodCash   PaymentMethod = "CASH"
	MethodCard   PaymentMethod = "CARD"
	MethodBKash  PaymentMethod = "BKASH"
	MethodNagad  PaymentMethod = "NAGAD"
	MethodUPI    PaymentMethod = "UPI"
	MethodCredit PaymentMethod = "CREDIT" // Due balance on customer account
)

// PaymentRecord represents an individual payment transaction line
type PaymentRecord struct {
	ID          string        `json:"id"`
	Method      PaymentMethod `json:"method"`
	AmountMinor int64         `json:"amount_minor"`
	Reference   string        `json:"reference,omitempty"` // TrxID, Card Last4, etc.
	Timestamp   time.Time     `json:"timestamp"`
}

// TenderResult calculates change and balance
type TenderResult struct {
	TotalDueMinor   int64 `json:"total_due_minor"`
	TotalPaidMinor  int64 `json:"total_paid_minor"`
	ChangeDueMinor  int64 `json:"change_due_minor"`
	BalanceDueMinor int64 `json:"balance_due_minor"`
	IsComplete      bool  `json:"is_complete"`
}

// CalculateTender evaluates given payment against total due
func CalculateTender(totalDueMinor int64, payments []PaymentRecord) TenderResult {
	var totalPaid int64 = 0
	for _, p := range payments {
		totalPaid += p.AmountMinor
	}

	res := TenderResult{
		TotalDueMinor:  totalDueMinor,
		TotalPaidMinor: totalPaid,
	}

	if totalPaid >= totalDueMinor {
		res.ChangeDueMinor = totalPaid - totalDueMinor
		res.BalanceDueMinor = 0
		res.IsComplete = true
	} else {
		res.ChangeDueMinor = 0
		res.BalanceDueMinor = totalDueMinor - totalPaid
		res.IsComplete = false
	}

	return res
}

// ValidatePayments checks whether payments fully satisfy the sale total
func ValidatePayments(totalDueMinor int64, payments []PaymentRecord) error {
	if len(payments) == 0 && totalDueMinor > 0 {
		return fmt.Errorf("no payment received for non-zero sale")
	}

	tender := CalculateTender(totalDueMinor, payments)
	if !tender.IsComplete {
		return fmt.Errorf("insufficient payment: due ৳%.2f, received ৳%.2f (remaining ৳%.2f)",
			float64(totalDueMinor)/100.0, float64(tender.TotalPaidMinor)/100.0, float64(tender.BalanceDueMinor)/100.0)
	}

	return nil
}
