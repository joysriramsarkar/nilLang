package data

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Default currency configuration
const (
	DefaultCurrency = "BDT"
	DefaultSymbol   = "৳"
)

// Money represents exact financial amounts in minor units (paisa/cents)
// Preventing IEEE-754 floating point drift in enterprise applications
type Money struct {
	Minor    int64  `json:"minor"`
	Currency string `json:"currency"`
	Symbol   string `json:"symbol"`
}

// NewMoney creates a Money instance from minor units (e.g. 1250 for ৳12.50)
func NewMoney(minor int64, currency ...string) Money {
	curr := DefaultCurrency
	sym := DefaultSymbol
	if len(currency) > 0 && currency[0] != "" {
		curr = currency[0]
		sym = CurrencySymbol(curr)
	}
	return Money{
		Minor:    minor,
		Currency: curr,
		Symbol:   sym,
	}
}

var (
	ErrOverflow         = fmt.Errorf("financial arithmetic overflow")
	ErrDivisionByZero   = fmt.Errorf("financial division by zero")
	ErrCurrencyMismatch = fmt.Errorf("currency mismatch in financial operation")
	ErrInvalidScale     = fmt.Errorf("invalid scale value")
)

// FromMinor creates Money from minor units (e.g. 1250 for ৳12.50). Canonical constructor.
func FromMinor(minor int64, currency ...string) Money {
	return NewMoney(minor, currency...)
}

// FromMajorString parses standard currency string into Money without floating-point conversion.
func FromMajorString(s string, currency ...string) (Money, error) {
	return ParseMoney(s, currency...)
}

// Deprecated: NewMoneyFromMajor uses float64 which can introduce IEEE-754 drift.
// Use FromMinor or ParseMoney for financial paths.
func NewMoneyFromMajor(major float64, currency ...string) Money {
	minor := int64(math.Round(major * 100))
	return NewMoney(minor, currency...)
}


// ParseMoney parses standard currency strings such as "12.50", "৳1,250.00", "$49.99"
func ParseMoney(s string, currency ...string) (Money, error) {
	clean := strings.TrimSpace(s)
	clean = strings.ReplaceAll(clean, ",", "")
	clean = strings.TrimPrefix(clean, "৳")
	clean = strings.TrimPrefix(clean, "$")
	clean = strings.TrimPrefix(clean, "€")
	clean = strings.TrimPrefix(clean, "£")
	clean = strings.TrimSpace(clean)

	neg := false
	if strings.HasPrefix(clean, "-") {
		neg = true
		clean = clean[1:]
	}

	parts := strings.Split(clean, ".")
	var major, minor int64
	var err error

	major, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return Money{}, fmt.Errorf("invalid money major part: %w", err)
	}

	if len(parts) > 1 {
		minStr := parts[1]
		if len(minStr) == 1 {
			minStr += "0"
		} else if len(minStr) > 2 {
			minStr = minStr[:2]
		}
		minor, err = strconv.ParseInt(minStr, 10, 64)
		if err != nil {
			return Money{}, fmt.Errorf("invalid money minor part: %w", err)
		}
	}

	totalMinor := major*100 + minor
	if neg {
		totalMinor = -totalMinor
	}

	return NewMoney(totalMinor, currency...), nil
}

// CurrencySymbol returns the matching currency symbol
func CurrencySymbol(currency string) string {
	switch strings.ToUpper(currency) {
	case "BDT":
		return "৳"
	case "USD":
		return "$"
	case "EUR":
		return "€"
	case "GBP":
		return "£"
	case "INR":
		return "₹"
	case "JPY":
		return "¥"
	default:
		return currency + " "
	}
}

// Add returns the exact sum of two Money values
func (m Money) Add(other Money) Money {
	return NewMoney(m.Minor+other.Minor, m.Currency)
}

// AddChecked returns sum with currency mismatch check and 64-bit overflow detection
func (m Money) AddChecked(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, ErrCurrencyMismatch
	}
	res := m.Minor + other.Minor
	if (m.Minor > 0 && other.Minor > 0 && res < 0) || (m.Minor < 0 && other.Minor < 0 && res > 0) {
		return Money{}, ErrOverflow
	}
	return NewMoney(res, m.Currency), nil
}

// Sub returns the exact difference of two Money values
func (m Money) Sub(other Money) Money {
	return NewMoney(m.Minor-other.Minor, m.Currency)
}

// SubChecked returns difference with currency check and overflow detection
func (m Money) SubChecked(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, ErrCurrencyMismatch
	}
	res := m.Minor - other.Minor
	if (m.Minor > 0 && other.Minor < 0 && res < 0) || (m.Minor < 0 && other.Minor > 0 && res > 0) {
		return Money{}, ErrOverflow
	}
	return NewMoney(res, m.Currency), nil
}

// MulDecimalChecked multiplies Money by Decimal with overflow detection
func (m Money) MulDecimalChecked(d Decimal) (Money, error) {
	if m.Minor == 0 || d.Value == 0 {
		return NewMoney(0, m.Currency), nil
	}
	prod := m.Minor * d.Value
	if prod/m.Minor != d.Value {
		return Money{}, ErrOverflow
	}
	return NewMoney(prod/DecimalScale, m.Currency), nil
}


// Mul multiplies Money by an exact floating scalar and rounds to nearest minor unit
func (m Money) Mul(factor float64) Money {
	res := math.Round(float64(m.Minor) * factor)
	return NewMoney(int64(res), m.Currency)
}

// MulQty multiplies unit price by quantity given in scaled minor units (e.g. 2.5 kg = 2500 with scale=1000)
func (m Money) MulQty(qtyMinor int64, scale int64) Money {
	if scale == 0 {
		scale = 1000
	}
	return NewMoney((m.Minor*qtyMinor)/scale, m.Currency)
}

// Div divides Money evenly by an integer divisor
func (m Money) Div(divisor int64) (Money, error) {
	if divisor == 0 {
		return Money{}, fmt.Errorf("division by zero in Money")
	}
	return NewMoney(m.Minor/divisor, m.Currency), nil
}

// IsZero returns true if the money value is 0
func (m Money) IsZero() bool {
	return m.Minor == 0
}

// IsNegative returns true if the value is less than zero
func (m Money) IsNegative() bool {
	return m.Minor < 0
}

// Major returns the float representation (for display or JSON)
func (m Money) Major() float64 {
	return float64(m.Minor) / 100.0
}

// Format formats the money amount with currency symbol, grouping commas, and 2 decimal digits
func (m Money) Format() string {
	n := m.Minor
	neg := n < 0
	if neg {
		n = -n
	}

	maj := n / 100
	min := n % 100

	// Format major part with commas
	majStr := strconv.FormatInt(maj, 10)
	var formattedMaj strings.Builder
	l := len(majStr)
	for i, c := range majStr {
		if i > 0 && (l-i)%3 == 0 {
			formattedMaj.WriteRune(',')
		}
		formattedMaj.WriteRune(c)
	}

	res := fmt.Sprintf("%s%s.%02d", m.Symbol, formattedMaj.String(), min)
	if neg {
		return "-" + res
	}
	return res
}

func (m Money) String() string {
	return m.Format()
}

func (m Money) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"minor":     m.Minor,
		"major":     m.Major(),
		"formatted": m.Format(),
		"currency":  m.Currency,
	})
}

// ─── DECIMAL & RATIO INTEGRATION ─────────────────────────────────────────────

// MulDecimal multiplies Money by an exact Decimal quantity with Half-Up rounding to nearest minor unit
func (m Money) MulDecimal(d Decimal) Money {
	prod := m.Minor * d.Value
	var newMinor int64
	if prod >= 0 {
		newMinor = (prod + DecimalScale/2) / DecimalScale
	} else {
		newMinor = (prod - DecimalScale/2) / DecimalScale
	}
	return NewMoney(newMinor, m.Currency)
}

// MulRatio multiplies Money by an exact integer fraction (numerator / denominator) with Half-Up rounding
func (m Money) MulRatio(numerator, denominator int64) (Money, error) {
	if denominator == 0 {
		return Money{}, fmt.Errorf("division by zero in MulRatio")
	}
	prod := m.Minor * numerator
	var newMinor int64
	if (prod >= 0 && denominator > 0) || (prod < 0 && denominator < 0) {
		newMinor = (prod + denominator/2) / denominator
	} else {
		newMinor = (prod - denominator/2) / denominator
	}
	return NewMoney(newMinor, m.Currency), nil
}

// Round returns the Money value (already canonical exact minor units)
func (m Money) Round() Money {
	return m
}

// Allocate splits Money across multiple ratio weights without losing any minor units (Fowler's allocation)
func (m Money) Allocate(ratios ...int64) ([]Money, error) {
	if len(ratios) == 0 {
		return nil, fmt.Errorf("no ratios provided for allocation")
	}
	var totalRatio int64
	for _, r := range ratios {
		if r < 0 {
			return nil, fmt.Errorf("ratio weight cannot be negative: %d", r)
		}
		totalRatio += r
	}
	if totalRatio == 0 {
		return nil, fmt.Errorf("sum of ratios must be greater than zero")
	}

	results := make([]Money, len(ratios))
	remainder := m.Minor
	for i, r := range ratios {
		share := (m.Minor * r) / totalRatio
		results[i] = NewMoney(share, m.Currency)
		remainder -= share
	}

	// Distribute leftover minor units one by one
	step := int64(1)
	if remainder < 0 {
		step = -1
		remainder = -remainder
	}
	for i := 0; remainder > 0 && i < len(results); i++ {
		results[i].Minor += step
		remainder--
	}

	return results, nil
}
