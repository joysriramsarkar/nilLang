package data

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Decimal provides exact fixed-point arithmetic with 4 decimal places (scale = 10000)
// Eliminating IEEE-754 floating point drift for quantities, tax rates, weights, and POS operations.
type Decimal struct {
	Value int64 `json:"raw"`
}

const DecimalScale int64 = 10000

// Deprecated: NewDecimal uses float64 which can introduce precision drift.
// Prefer ParseDecimal or NewDecimalFromInt for exact arithmetic.
func NewDecimal(f float64) Decimal {
	return Decimal{Value: int64(math.Round(f * float64(DecimalScale)))}
}


// NewDecimalFromInt creates a Decimal from an integer
func NewDecimalFromInt(i int64) Decimal {
	return Decimal{Value: i * DecimalScale}
}

// DecimalFromMinor creates a Decimal from integer minor units (e.g. 1250 with scale 100 -> 12.5000)
func DecimalFromMinor(minor int64, scale int64) Decimal {
	if scale <= 0 {
		scale = 100 // Default 2 decimal places (cents / paisa)
	}
	return Decimal{Value: (minor * DecimalScale) / scale}
}

// DecimalFromString parses a numeric string into Decimal
func DecimalFromString(s string) (Decimal, error) {
	return ParseDecimal(s)
}

// ParseDecimal parses a numeric string into Decimal (e.g. "12.5", "0.05", "100")
func ParseDecimal(s string) (Decimal, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Decimal{}, fmt.Errorf("empty decimal string")
	}

	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	} else if strings.HasPrefix(s, "+") {
		s = s[1:]
	}

	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return Decimal{}, fmt.Errorf("invalid decimal format: %s", s)
	}

	intPart, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return Decimal{}, fmt.Errorf("invalid integer part: %w", err)
	}

	var fracPart int64
	if len(parts) == 2 {
		fracStr := parts[1]
		if len(fracStr) > 4 {
			fracStr = fracStr[:4]
		} else {
			for len(fracStr) < 4 {
				fracStr += "0"
			}
		}
		fracPart, err = strconv.ParseInt(fracStr, 10, 64)
		if err != nil {
			return Decimal{}, fmt.Errorf("invalid fraction part: %w", err)
		}
	}

	total := intPart*DecimalScale + fracPart
	if neg {
		total = -total
	}
	return Decimal{Value: total}, nil
}

// Add adds two Decimals
func (d Decimal) Add(other Decimal) Decimal {
	return Decimal{Value: d.Value + other.Value}
}

// AddChecked adds two Decimals with 64-bit signed overflow detection
func (d Decimal) AddChecked(other Decimal) (Decimal, error) {
	res := d.Value + other.Value
	if (d.Value > 0 && other.Value > 0 && res < 0) || (d.Value < 0 && other.Value < 0 && res > 0) {
		return Decimal{}, ErrOverflow
	}
	return Decimal{Value: res}, nil
}

// Sub subtracts two Decimals
func (d Decimal) Sub(other Decimal) Decimal {
	return Decimal{Value: d.Value - other.Value}
}

// SubChecked subtracts two Decimals with overflow detection
func (d Decimal) SubChecked(other Decimal) (Decimal, error) {
	res := d.Value - other.Value
	if (d.Value > 0 && other.Value < 0 && res < 0) || (d.Value < 0 && other.Value > 0 && res > 0) {
		return Decimal{}, ErrOverflow
	}
	return Decimal{Value: res}, nil
}

// Mul multiplies two Decimals
func (d Decimal) Mul(other Decimal) Decimal {
	return Decimal{Value: (d.Value * other.Value) / DecimalScale}
}

// MulChecked multiplies two Decimals with overflow detection
func (d Decimal) MulChecked(other Decimal) (Decimal, error) {
	if d.Value == 0 || other.Value == 0 {
		return Decimal{Value: 0}, nil
	}
	prod := d.Value * other.Value
	if prod/d.Value != other.Value {
		return Decimal{}, ErrOverflow
	}
	return Decimal{Value: prod / DecimalScale}, nil
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

// Cmp compares d and other (-1 if d < other, 1 if d > other, 0 if equal)
func (d Decimal) Cmp(other Decimal) int {
	if d.Value < other.Value {
		return -1
	}
	if d.Value > other.Value {
		return 1
	}
	return 0
}

func (d Decimal) IsZero() bool {
	return d.Value == 0
}

func (d Decimal) IsNegative() bool {
	return d.Value < 0
}

// Round rounds the Decimal to the specified number of decimal places (0 to 4) using Half-Up rounding
func (d Decimal) Round(decimals int) Decimal {
	if decimals >= 4 {
		return d
	}
	if decimals < 0 {
		decimals = 0
	}
	var step int64 = 1
	for i := 0; i < (4 - decimals); i++ {
		step *= 10
	}
	half := step / 2
	if d.Value >= 0 {
		return Decimal{Value: ((d.Value + half) / step) * step}
	}
	return Decimal{Value: ((d.Value - half) / step) * step}
}

// Abs returns absolute value
func (d Decimal) Abs() Decimal {
	if d.Value < 0 {
		return Decimal{Value: -d.Value}
	}
	return d
}

// Min returns minimum of two decimals
func MinDecimal(a, b Decimal) Decimal {
	if a.Cmp(b) <= 0 {
		return a
	}
	return b
}

// Max returns maximum of two decimals
func MaxDecimal(a, b Decimal) Decimal {
	if a.Cmp(b) >= 0 {
		return a
	}
	return b
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
	// Trim trailing zeros after 2 decimal places if clean
	res = strings.TrimRight(strings.TrimRight(res, "0"), ".")
	if !strings.Contains(res, ".") {
		res += ".00"
	} else {
		parts := strings.Split(res, ".")
		if len(parts[1]) < 2 {
			res += "0"
		}
	}
	if neg {
		return "-" + res
	}
	return res
}

func (d Decimal) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Decimal) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		dec, parseErr := ParseDecimal(s)
		if parseErr != nil {
			return parseErr
		}
		*d = dec
		return nil
	}

	var f float64
	if err := json.Unmarshal(b, &f); err == nil {
		*d = NewDecimal(f)
		return nil
	}

	var i int64
	if err := json.Unmarshal(b, &i); err == nil {
		*d = NewDecimalFromInt(i)
		return nil
	}

	return fmt.Errorf("invalid decimal json: %s", string(b))
}

// Quantity represents physical product quantity with exact Decimal amount and unit
type Quantity struct {
	Amount Decimal `json:"amount"`
	Unit   string  `json:"unit"`
}

// NewQuantity creates a new quantity
func NewQuantity(amount Decimal, unit string) Quantity {
	return Quantity{Amount: amount, Unit: unit}
}

// NewQuantityFromFloat creates a quantity from float
func NewQuantityFromFloat(f float64, unit string) Quantity {
	return Quantity{Amount: NewDecimal(f), Unit: unit}
}

func (q Quantity) String() string {
	if q.Unit != "" {
		return fmt.Sprintf("%s %s", q.Amount.String(), q.Unit)
	}
	return q.Amount.String()
}
