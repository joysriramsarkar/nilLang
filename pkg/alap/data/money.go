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

// NewMoneyFromMajor creates Money from a major float amount (e.g. 12.50 -> 1250)
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

// Sub returns the exact difference of two Money values
func (m Money) Sub(other Money) Money {
	return NewMoney(m.Minor-other.Minor, m.Currency)
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

// ─── DECIMAL SUB-SYSTEM ─────────────────────────────────────────────────────

// Decimal provides fixed-point arithmetic with 4 decimal places (scale = 10000)
type Decimal struct {
	Value int64 `json:"raw"`
}

const DecimalScale int64 = 10000

// NewDecimal creates a Decimal from a float
func NewDecimal(f float64) Decimal {
	return Decimal{Value: int64(math.Round(f * float64(DecimalScale)))}
}

// NewDecimalFromInt creates a Decimal from an integer
func NewDecimalFromInt(i int64) Decimal {
	return Decimal{Value: i * DecimalScale}
}

// Add adds two Decimals
func (d Decimal) Add(other Decimal) Decimal {
	return Decimal{Value: d.Value + other.Value}
}

// Sub subtracts two Decimals
func (d Decimal) Sub(other Decimal) Decimal {
	return Decimal{Value: d.Value - other.Value}
}

// Mul multiplies two Decimals
func (d Decimal) Mul(other Decimal) Decimal {
	return Decimal{Value: (d.Value * other.Value) / DecimalScale}
}

// Div divides two Decimals
func (d Decimal) Div(other Decimal) (Decimal, error) {
	if other.Value == 0 {
		return Decimal{}, fmt.Errorf("decimal division by zero")
	}
	return Decimal{Value: (d.Value * DecimalScale) / other.Value}, nil
}

// Float64 converts Decimal to float64
func (d Decimal) Float64() float64 {
	return float64(d.Value) / float64(DecimalScale)
}

func (d Decimal) String() string {
	val := d.Value
	neg := val < 0
	if neg {
		val = -val
	}
	maj := val / DecimalScale
	min := val % DecimalScale
	res := fmt.Sprintf("%d.%04d", maj, min)
	if neg {
		return "-" + res
	}
	return res
}
