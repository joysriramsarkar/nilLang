package data

import (
	"fmt"
	"strings"
	"sync"
)

// Dimension represents physical dimensional quantity types
type Dimension string

const (
	DimensionCount  Dimension = "count"
	DimensionMass   Dimension = "mass"
	DimensionVolume Dimension = "volume"
	DimensionLength Dimension = "length"
)

// Unit defines a standardized measurement unit with dimension and base multiplier
type Unit struct {
	Symbol         string    `json:"symbol"`
	Name           string    `json:"name"`
	NameBn         string    `json:"name_bn"`
	Dimension      Dimension `json:"dimension"`
	BaseMultiplier int64     `json:"base_multiplier"` // Multiplier to normalize to dimension base unit
	IsCustom       bool      `json:"is_custom,omitempty"`
}

// Dimension Base Units:
// Mass:   mg (1 g = 1,000 mg; 1 kg = 1,000,000 mg)
// Volume: ml (1 l = 1,000 ml)
// Length: mm (1 cm = 10 mm; 1 m = 1,000 mm)
// Count:  pcs (1 dozen = 12 pcs)

var (
	// Count Units
	UnitPcs = Unit{
		Symbol:         "pcs",
		Name:           "Piece",
		NameBn:         "পিস",
		Dimension:      DimensionCount,
		BaseMultiplier: 1,
	}
	UnitDozen = Unit{
		Symbol:         "dozen",
		Name:           "Dozen",
		NameBn:         "ডজন",
		Dimension:      DimensionCount,
		BaseMultiplier: 12,
	}
	UnitPacket = Unit{
		Symbol:         "pkt",
		Name:           "Packet",
		NameBn:         "প্যাকেট",
		Dimension:      DimensionCount,
		BaseMultiplier: 1,
	}
	UnitBag = Unit{
		Symbol:         "bag",
		Name:           "Bag",
		NameBn:         "বস্তা",
		Dimension:      DimensionCount,
		BaseMultiplier: 1,
	}
	UnitBottle = Unit{
		Symbol:         "btl",
		Name:           "Bottle",
		NameBn:         "বোতল",
		Dimension:      DimensionCount,
		BaseMultiplier: 1,
	}

	// Mass Units
	UnitMg = Unit{
		Symbol:         "mg",
		Name:           "Milligram",
		NameBn:         "মিলিগ্রাম",
		Dimension:      DimensionMass,
		BaseMultiplier: 1,
	}
	UnitGram = Unit{
		Symbol:         "g",
		Name:           "Gram",
		NameBn:         "গ্রাম",
		Dimension:      DimensionMass,
		BaseMultiplier: 1000,
	}
	UnitKg = Unit{
		Symbol:         "kg",
		Name:           "Kilogram",
		NameBn:         "কেজি",
		Dimension:      DimensionMass,
		BaseMultiplier: 1000000,
	}

	// Volume Units
	UnitMl = Unit{
		Symbol:         "ml",
		Name:           "Millilitre",
		NameBn:         "মিলি",
		Dimension:      DimensionVolume,
		BaseMultiplier: 1,
	}
	UnitLitre = Unit{
		Symbol:         "l",
		Name:           "Litre",
		NameBn:         "লিটার",
		Dimension:      DimensionVolume,
		BaseMultiplier: 1000,
	}

	// Length Units
	UnitMm = Unit{
		Symbol:         "mm",
		Name:           "Millimetre",
		NameBn:         "মিলিমিটার",
		Dimension:      DimensionLength,
		BaseMultiplier: 1,
	}
	UnitCm = Unit{
		Symbol:         "cm",
		Name:           "Centimetre",
		NameBn:         "সেন্টিমিটার",
		Dimension:      DimensionLength,
		BaseMultiplier: 10,
	}
	UnitMetre = Unit{
		Symbol:         "m",
		Name:           "Metre",
		NameBn:         "মিটার",
		Dimension:      DimensionLength,
		BaseMultiplier: 1000,
	}
)

var standardUnits = []Unit{
	UnitPcs, UnitDozen, UnitPacket, UnitBag, UnitBottle,
	UnitMg, UnitGram, UnitKg,
	UnitMl, UnitLitre,
	UnitMm, UnitCm, UnitMetre,
}

var (
	customUnitsLock sync.RWMutex
	customUnits     = make(map[string]Unit)
)

// RegisterUnit explicitly registers a custom unit of measurement.
func RegisterUnit(u Unit) {
	customUnitsLock.Lock()
	defer customUnitsLock.Unlock()
	u.IsCustom = true
	customUnits[strings.ToLower(strings.TrimSpace(u.Symbol))] = u
	if u.Name != "" {
		customUnits[strings.ToLower(strings.TrimSpace(u.Name))] = u
	}
	if u.NameBn != "" {
		customUnits[strings.TrimSpace(u.NameBn)] = u
	}
}

// IsStandardUnit returns true if the unit is a built-in canonical standard.
func IsStandardUnit(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	for _, u := range standardUnits {
		if strings.EqualFold(u.Symbol, s) || strings.EqualFold(u.Name, s) || u.NameBn == s {
			return true
		}
	}
	return false
}

// LookupUnit finds a registered Unit by symbol, English name, or Bengali name
func LookupUnit(s string) (Unit, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return Unit{}, false
	}
	for _, u := range standardUnits {
		if strings.EqualFold(u.Symbol, s) ||
			strings.EqualFold(u.Name, s) ||
			u.NameBn == s {
			return u, true
		}
	}

	// Check registered custom units
	customUnitsLock.RLock()
	cu, exists := customUnits[s]
	customUnitsLock.RUnlock()
	if exists {
		return cu, true
	}

	// Fallback for custom count units
	return Unit{
		Symbol:         s,
		Name:           s,
		NameBn:         s,
		Dimension:      DimensionCount,
		BaseMultiplier: 1,
		IsCustom:       true,
	}, true
}

// CanConvert returns whether two units belong to the same dimension and are compatible
func CanConvert(u1, u2 Unit) bool {
	return u1.Dimension == u2.Dimension && u1.BaseMultiplier > 0 && u2.BaseMultiplier > 0
}

// ValidateUnitCompatibility checks if two unit strings can be converted / operated together
func ValidateUnitCompatibility(unit1, unit2 string) error {
	u1, ok1 := LookupUnit(unit1)
	u2, ok2 := LookupUnit(unit2)
	if !ok1 || !ok2 {
		return fmt.Errorf("unknown unit comparison: %q vs %q", unit1, unit2)
	}
	if !CanConvert(u1, u2) {
		return fmt.Errorf("incompatible units: cannot convert %s (%s) to %s (%s)",
			u1.Symbol, u1.Dimension, u2.Symbol, u2.Dimension)
	}
	return nil
}

// ConvertQuantity converts a Quantity into target unit
func ConvertQuantity(q Quantity, target Unit) (Quantity, error) {
	currentUnit, ok := LookupUnit(q.Unit)
	if !ok {
		return Quantity{}, fmt.Errorf("unrecognized source unit: %s", q.Unit)
	}
	if currentUnit.Symbol == target.Symbol {
		return q, nil
	}
	if !CanConvert(currentUnit, target) {
		return Quantity{}, fmt.Errorf("incompatible dimensions: cannot convert %s (%s) to %s (%s)",
			currentUnit.Symbol, currentUnit.Dimension, target.Symbol, target.Dimension)
	}

	// Normalize to base dimension units:
	// baseAmount = q.Amount * currentUnit.BaseMultiplier
	// targetAmount = baseAmount / target.BaseMultiplier
	ratioNum := currentUnit.BaseMultiplier
	ratioDen := target.BaseMultiplier

	convertedVal := (q.Amount.Value * ratioNum) / ratioDen
	return Quantity{
		Amount: Decimal{Value: convertedVal},
		Unit:   target.Symbol,
	}, nil
}

// AddQuantity adds two quantities with automatic unit conversion
func AddQuantity(q1, q2 Quantity) (Quantity, error) {
	u1, ok1 := LookupUnit(q1.Unit)
	u2, ok2 := LookupUnit(q2.Unit)
	if !ok1 || !ok2 {
		return Quantity{}, fmt.Errorf("unknown unit in addition: %s or %s", q1.Unit, q2.Unit)
	}
	if u1.Symbol == u2.Symbol {
		return Quantity{Amount: q1.Amount.Add(q2.Amount), Unit: q1.Unit}, nil
	}
	if !CanConvert(u1, u2) {
		return Quantity{}, fmt.Errorf("incompatible unit addition: %s and %s", q1.Unit, q2.Unit)
	}
	// Convert q2 to q1's unit
	q2Converted, err := ConvertQuantity(q2, u1)
	if err != nil {
		return Quantity{}, err
	}
	return Quantity{Amount: q1.Amount.Add(q2Converted.Amount), Unit: q1.Unit}, nil
}

// SubQuantity subtracts q2 from q1 with automatic unit conversion
func SubQuantity(q1, q2 Quantity) (Quantity, error) {
	u1, ok1 := LookupUnit(q1.Unit)
	u2, ok2 := LookupUnit(q2.Unit)
	if !ok1 || !ok2 {
		return Quantity{}, fmt.Errorf("unknown unit in subtraction: %s or %s", q1.Unit, q2.Unit)
	}
	if u1.Symbol == u2.Symbol {
		return Quantity{Amount: q1.Amount.Sub(q2.Amount), Unit: q1.Unit}, nil
	}
	if !CanConvert(u1, u2) {
		return Quantity{}, fmt.Errorf("incompatible unit subtraction: %s and %s", q1.Unit, q2.Unit)
	}
	q2Converted, err := ConvertQuantity(q2, u1)
	if err != nil {
		return Quantity{}, err
	}
	return Quantity{Amount: q1.Amount.Sub(q2Converted.Amount), Unit: q1.Unit}, nil
}

// CmpQuantity compares two quantities with automatic unit conversion
func CmpQuantity(q1, q2 Quantity) (int, error) {
	u1, ok1 := LookupUnit(q1.Unit)
	u2, ok2 := LookupUnit(q2.Unit)
	if !ok1 || !ok2 {
		return 0, fmt.Errorf("unknown unit in comparison: %s or %s", q1.Unit, q2.Unit)
	}
	if u1.Symbol == u2.Symbol {
		return q1.Amount.Cmp(q2.Amount), nil
	}
	if !CanConvert(u1, u2) {
		return 0, fmt.Errorf("cannot compare incompatible units: %s and %s", q1.Unit, q2.Unit)
	}
	q2Converted, err := ConvertQuantity(q2, u1)
	if err != nil {
		return 0, err
	}
	return q1.Amount.Cmp(q2Converted.Amount), nil
}
