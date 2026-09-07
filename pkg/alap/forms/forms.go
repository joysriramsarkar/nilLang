package forms

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

// RuleType defines the validator rule type
type RuleType string

const (
	RuleRequired  RuleType = "required"
	RuleMin       RuleType = "min"
	RuleMax       RuleType = "max"
	RuleMinLength RuleType = "minLength"
	RuleMaxLength RuleType = "maxLength"
	RuleEmail     RuleType = "email"
	RuleRegex     RuleType = "regex"
	RuleNumeric   RuleType = "numeric"
	RuleDecimal   RuleType = "decimal"
	RuleMoney     RuleType = "money"
	RuleCustom    RuleType = "custom"
)

// Rule represents a single field validation constraint
type Rule struct {
	Type    RuleType
	Param   interface{}
	Message string
	Custom  func(val interface{}) error
}

// FieldDef defines validation metadata for an individual form input
type FieldDef struct {
	Name  string
	Label string
	Rules []Rule
}

// FormSchema defines a declarative form specification
type FormSchema struct {
	Name   string
	Fields map[string]*FieldDef
}

// NewForm creates a new FormSchema
func NewForm(name string) *FormSchema {
	return &FormSchema{
		Name:   name,
		Fields: make(map[string]*FieldDef),
	}
}

// Field adds or retrieves a FieldDef on the form
func (f *FormSchema) Field(name, label string) *FieldDef {
	fd := &FieldDef{
		Name:  name,
		Label: label,
		Rules: make([]Rule, 0),
	}
	f.Fields[name] = fd
	return fd
}

// Required enforces non-empty value
func (fd *FieldDef) Required(msg ...string) *FieldDef {
	m := fmt.Sprintf("%s is required", fd.Label)
	if len(msg) > 0 && msg[0] != "" {
		m = msg[0]
	}
	fd.Rules = append(fd.Rules, Rule{Type: RuleRequired, Message: m})
	return fd
}

// MinLength enforces minimum string length
func (fd *FieldDef) MinLength(min int, msg ...string) *FieldDef {
	m := fmt.Sprintf("%s must be at least %d characters", fd.Label, min)
	if len(msg) > 0 && msg[0] != "" {
		m = msg[0]
	}
	fd.Rules = append(fd.Rules, Rule{Type: RuleMinLength, Param: min, Message: m})
	return fd
}

// MaxLength enforces maximum string length
func (fd *FieldDef) MaxLength(max int, msg ...string) *FieldDef {
	m := fmt.Sprintf("%s must be at most %d characters", fd.Label, max)
	if len(msg) > 0 && msg[0] != "" {
		m = msg[0]
	}
	fd.Rules = append(fd.Rules, Rule{Type: RuleMaxLength, Param: max, Message: m})
	return fd
}

// Min enforces minimum numeric or decimal value
func (fd *FieldDef) Min(min float64, msg ...string) *FieldDef {
	m := fmt.Sprintf("%s must be at least %v", fd.Label, min)
	if len(msg) > 0 && msg[0] != "" {
		m = msg[0]
	}
	fd.Rules = append(fd.Rules, Rule{Type: RuleMin, Param: min, Message: m})
	return fd
}

// Max enforces maximum numeric or decimal value
func (fd *FieldDef) Max(max float64, msg ...string) *FieldDef {
	m := fmt.Sprintf("%s must be at most %v", fd.Label, max)
	if len(msg) > 0 && msg[0] != "" {
		m = msg[0]
	}
	fd.Rules = append(fd.Rules, Rule{Type: RuleMax, Param: max, Message: m})
	return fd
}

// Email enforces valid email format
func (fd *FieldDef) Email(msg ...string) *FieldDef {
	m := fmt.Sprintf("%s must be a valid email", fd.Label)
	if len(msg) > 0 && msg[0] != "" {
		m = msg[0]
	}
	fd.Rules = append(fd.Rules, Rule{Type: RuleEmail, Message: m})
	return fd
}

// Regex enforces pattern matching
func (fd *FieldDef) Regex(pattern string, msg ...string) *FieldDef {
	m := fmt.Sprintf("%s format is invalid", fd.Label)
	if len(msg) > 0 && msg[0] != "" {
		m = msg[0]
	}
	fd.Rules = append(fd.Rules, Rule{Type: RuleRegex, Param: pattern, Message: m})
	return fd
}

// Custom adds an arbitrary custom validator function
func (fd *FieldDef) Custom(fn func(val interface{}) error, msg ...string) *FieldDef {
	m := "Invalid value"
	if len(msg) > 0 && msg[0] != "" {
		m = msg[0]
	}
	fd.Rules = append(fd.Rules, Rule{Type: RuleCustom, Message: m, Custom: fn})
	return fd
}

// ValidationResult holds validation errors keyed by field name
type ValidationResult struct {
	IsValid bool
	Errors  map[string]string
}

// Validate checks given form payload against the schema
func (f *FormSchema) Validate(data map[string]interface{}) *ValidationResult {
	res := &ValidationResult{
		IsValid: true,
		Errors:  make(map[string]string),
	}

	for fieldName, fieldDef := range f.Fields {
		val := data[fieldName]

		for _, rule := range fieldDef.Rules {
			err := checkRule(rule, val)
			if err != nil {
				res.IsValid = false
				res.Errors[fieldName] = rule.Message
				break // Stop on first error for this field
			}
		}
	}

	return res
}

func checkRule(rule Rule, val interface{}) error {
	switch rule.Type {
	case RuleRequired:
		if val == nil {
			return fmt.Errorf("required")
		}
		if s, ok := val.(string); ok && strings.TrimSpace(s) == "" {
			return fmt.Errorf("required")
		}

	case RuleMinLength:
		min, _ := rule.Param.(int)
		if s, ok := val.(string); ok {
			if len(strings.TrimSpace(s)) < min {
				return fmt.Errorf("too short")
			}
		}

	case RuleMaxLength:
		max, _ := rule.Param.(int)
		if s, ok := val.(string); ok {
			if len(strings.TrimSpace(s)) > max {
				return fmt.Errorf("too long")
			}
		}

	case RuleMin:
		min, _ := rule.Param.(float64)
		num := toFloat(val)
		if num < min {
			return fmt.Errorf("too small")
		}

	case RuleMax:
		max, _ := rule.Param.(float64)
		num := toFloat(val)
		if num > max {
			return fmt.Errorf("too large")
		}

	case RuleEmail:
		if s, ok := val.(string); ok && s != "" {
			_, err := mail.ParseAddress(s)
			if err != nil {
				return fmt.Errorf("invalid email")
			}
		}

	case RuleRegex:
		pat, _ := rule.Param.(string)
		if s, ok := val.(string); ok && s != "" {
			matched, err := regexp.MatchString(pat, s)
			if err != nil || !matched {
				return fmt.Errorf("regex mismatch")
			}
		}

	case RuleCustom:
		if rule.Custom != nil {
			return rule.Custom(val)
		}
	}

	return nil
}

func toFloat(val interface{}) float64 {
	switch v := val.(type) {
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case float64:
		return v
	case data.Decimal:
		return v.Float64()
	case string:
		d, err := data.ParseDecimal(v)
		if err == nil {
			return d.Float64()
		}
	}
	return 0
}
