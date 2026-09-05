package types

import (
	"fmt"
	"strings"
)

// TypeKind identifies the structural classification of a type
type TypeKind string

const (
	KindPrimitive  TypeKind = "PRIMITIVE"
	KindStruct     TypeKind = "STRUCT"
	KindEnum       TypeKind = "ENUM"
	KindGeneric    TypeKind = "GENERIC"
	KindUnion      TypeKind = "UNION"
	KindOptional   TypeKind = "OPTIONAL"
	KindResult     TypeKind = "RESULT"
	KindTrait      TypeKind = "TRAIT"
	KindFunction   TypeKind = "FUNCTION"
	KindCapability TypeKind = "CAPABILITY"
	KindEffect     TypeKind = "EFFECT"
	KindAny        TypeKind = "ANY"
)

// Type represents a NilLang static/semantic type
type Type interface {
	Kind() TypeKind
	String() string
	Equals(other Type) bool
	AssignableTo(target Type) bool
}

// ─── PRIMITIVES ─────────────────────────────────────────────────────────────

type PrimitiveType struct {
	Name string
}

func (p PrimitiveType) Kind() TypeKind { return KindPrimitive }
func (p PrimitiveType) String() string { return p.Name }
func (p PrimitiveType) Equals(other Type) bool {
	if o, ok := other.(PrimitiveType); ok {
		return p.Name == o.Name
	}
	return false
}
func (p PrimitiveType) AssignableTo(target Type) bool {
	if target == nil {
		return false
	}
	if _, ok := target.(AnyType); ok {
		return true
	}
	if u, ok := target.(*UnionType); ok {
		return u.Contains(p)
	}
	if opt, ok := target.(*OptionalType); ok {
		return p.AssignableTo(opt.Base) || p.Equals(Null)
	}
	return p.Equals(target)
}

var (
	Int    = PrimitiveType{Name: "Int"}
	Float  = PrimitiveType{Name: "Float"}
	String = PrimitiveType{Name: "String"}
	Bool   = PrimitiveType{Name: "Bool"}
	Byte   = PrimitiveType{Name: "Byte"}
	Null   = PrimitiveType{Name: "Null"}
	Void   = PrimitiveType{Name: "Void"}
)

// ─── ANY TYPE ───────────────────────────────────────────────────────────────

type AnyType struct{}

func (a AnyType) Kind() TypeKind                { return KindAny }
func (a AnyType) String() string                { return "Any" }
func (a AnyType) Equals(other Type) bool        { _, ok := other.(AnyType); return ok }
func (a AnyType) AssignableTo(target Type) bool { return true }

var Any = AnyType{}

// ─── STRUCT TYPE ────────────────────────────────────────────────────────────

type StructField struct {
	Name string
	Type Type
}

type StructType struct {
	Name   string
	Fields []StructField
}

func (s *StructType) Kind() TypeKind { return KindStruct }
func (s *StructType) String() string {
	if len(s.Fields) == 0 {
		return s.Name
	}
	var f []string
	for _, field := range s.Fields {
		f = append(f, fmt.Sprintf("%s: %s", field.Name, field.Type.String()))
	}
	return fmt.Sprintf("struct %s { %s }", s.Name, strings.Join(f, ", "))
}
func (s *StructType) Equals(other Type) bool {
	o, ok := other.(*StructType)
	if !ok || s.Name != o.Name || len(s.Fields) != len(o.Fields) {
		return false
	}
	for i := range s.Fields {
		if s.Fields[i].Name != o.Fields[i].Name || !s.Fields[i].Type.Equals(o.Fields[i].Type) {
			return false
		}
	}
	return true
}
func (s *StructType) AssignableTo(target Type) bool {
	if _, ok := target.(AnyType); ok {
		return true
	}
	return s.Equals(target)
}

func (s *StructType) GetField(name string) (Type, bool) {
	for _, f := range s.Fields {
		if f.Name == name {
			return f.Type, true
		}
	}
	return nil, false
}

// ─── ENUM TYPE ──────────────────────────────────────────────────────────────

type EnumVariant struct {
	Name    string
	Payload []Type
}

type EnumType struct {
	Name     string
	Variants []EnumVariant
}

func (e *EnumType) Kind() TypeKind { return KindEnum }
func (e *EnumType) String() string {
	var v []string
	for _, variant := range e.Variants {
		if len(variant.Payload) == 0 {
			v = append(v, variant.Name)
		} else {
			var p []string
			for _, pt := range variant.Payload {
				p = append(p, pt.String())
			}
			v = append(v, fmt.Sprintf("%s(%s)", variant.Name, strings.Join(p, ", ")))
		}
	}
	return fmt.Sprintf("enum %s { %s }", e.Name, strings.Join(v, ", "))
}
func (e *EnumType) Equals(other Type) bool {
	o, ok := other.(*EnumType)
	return ok && e.Name == o.Name
}
func (e *EnumType) AssignableTo(target Type) bool {
	if _, ok := target.(AnyType); ok {
		return true
	}
	return e.Equals(target)
}

// ─── UNION TYPE ─────────────────────────────────────────────────────────────

type UnionType struct {
	Types []Type
}

func NewUnion(types ...Type) *UnionType {
	u := &UnionType{}
	for _, t := range types {
		u.Add(t)
	}
	return u
}

func (u *UnionType) Add(t Type) {
	if t == nil {
		return
	}
	if otherU, ok := t.(*UnionType); ok {
		for _, sub := range otherU.Types {
			u.Add(sub)
		}
		return
	}
	if !u.Contains(t) {
		u.Types = append(u.Types, t)
	}
}

func (u *UnionType) Contains(t Type) bool {
	for _, existing := range u.Types {
		if existing.Equals(t) {
			return true
		}
	}
	return false
}

func (u *UnionType) Kind() TypeKind { return KindUnion }
func (u *UnionType) String() string {
	var s []string
	for _, t := range u.Types {
		s = append(s, t.String())
	}
	return strings.Join(s, " | ")
}
func (u *UnionType) Equals(other Type) bool {
	o, ok := other.(*UnionType)
	if !ok || len(u.Types) != len(o.Types) {
		return false
	}
	for _, t := range u.Types {
		if !o.Contains(t) {
			return false
		}
	}
	return true
}
func (u *UnionType) AssignableTo(target Type) bool {
	if _, ok := target.(AnyType); ok {
		return true
	}
	if targetU, ok := target.(*UnionType); ok {
		for _, t := range u.Types {
			if !targetU.Contains(t) {
				return false
			}
		}
		return true
	}
	return false
}

// ─── OPTIONAL TYPE ──────────────────────────────────────────────────────────

type OptionalType struct {
	Base Type
}

func (o *OptionalType) Kind() TypeKind { return KindOptional }
func (o *OptionalType) String() string { return "?" + o.Base.String() }
func (o *OptionalType) Equals(other Type) bool {
	if opt, ok := other.(*OptionalType); ok {
		return o.Base.Equals(opt.Base)
	}
	return false
}
func (o *OptionalType) AssignableTo(target Type) bool {
	if _, ok := target.(AnyType); ok {
		return true
	}
	if opt, ok := target.(*OptionalType); ok {
		return o.Base.AssignableTo(opt.Base)
	}
	return false
}

// ─── RESULT TYPE ────────────────────────────────────────────────────────────

type ResultType struct {
	Ok  Type
	Err Type
}

func (r *ResultType) Kind() TypeKind { return KindResult }
func (r *ResultType) String() string {
	return fmt.Sprintf("Result<%s, %s>", r.Ok.String(), r.Err.String())
}
func (r *ResultType) Equals(other Type) bool {
	o, ok := other.(*ResultType)
	return ok && r.Ok.Equals(o.Ok) && r.Err.Equals(o.Err)
}
func (r *ResultType) AssignableTo(target Type) bool {
	if _, ok := target.(AnyType); ok {
		return true
	}
	o, ok := target.(*ResultType)
	return ok && r.Ok.AssignableTo(o.Ok) && r.Err.AssignableTo(o.Err)
}

// ─── GENERIC TYPE ───────────────────────────────────────────────────────────

type GenericType struct {
	Base       string
	Parameters []Type
}

func (g *GenericType) Kind() TypeKind { return KindGeneric }
func (g *GenericType) String() string {
	var p []string
	for _, param := range g.Parameters {
		p = append(p, param.String())
	}
	return fmt.Sprintf("%s<%s>", g.Base, strings.Join(p, ", "))
}
func (g *GenericType) Equals(other Type) bool {
	o, ok := other.(*GenericType)
	if !ok || g.Base != o.Base || len(g.Parameters) != len(o.Parameters) {
		return false
	}
	for i := range g.Parameters {
		if !g.Parameters[i].Equals(o.Parameters[i]) {
			return false
		}
	}
	return true
}
func (g *GenericType) AssignableTo(target Type) bool {
	if _, ok := target.(AnyType); ok {
		return true
	}
	return g.Equals(target)
}

// ─── FUNCTION TYPE ──────────────────────────────────────────────────────────

type FunctionType struct {
	Params     []Type
	ReturnType Type
	Effects    []string
}

func (f *FunctionType) Kind() TypeKind { return KindFunction }
func (f *FunctionType) String() string {
	var p []string
	for _, param := range f.Params {
		if param != nil {
			p = append(p, param.String())
		} else {
			p = append(p, "Any")
		}
	}
	eff := ""
	if len(f.Effects) > 0 {
		eff = fmt.Sprintf(" [%s]", strings.Join(f.Effects, ", "))
	}
	ret := "Void"
	if f.ReturnType != nil {
		ret = f.ReturnType.String()
	}
	return fmt.Sprintf("fn(%s) -> %s%s", strings.Join(p, ", "), ret, eff)
}
func (f *FunctionType) Equals(other Type) bool {
	o, ok := other.(*FunctionType)
	if !ok || len(f.Params) != len(o.Params) {
		return false
	}
	for i := range f.Params {
		if !f.Params[i].Equals(o.Params[i]) {
			return false
		}
	}
	if (f.ReturnType == nil) != (o.ReturnType == nil) {
		return false
	}
	if f.ReturnType != nil && !f.ReturnType.Equals(o.ReturnType) {
		return false
	}
	return true
}
func (f *FunctionType) AssignableTo(target Type) bool {
	if _, ok := target.(AnyType); ok {
		return true
	}
	return f.Equals(target)
}

// ─── TRAIT TYPE ─────────────────────────────────────────────────────────────

type TraitType struct {
	Name    string
	Methods map[string]*FunctionType
}

func (t *TraitType) Kind() TypeKind { return KindTrait }
func (t *TraitType) String() string { return fmt.Sprintf("trait %s", t.Name) }
func (t *TraitType) Equals(other Type) bool {
	o, ok := other.(*TraitType)
	return ok && t.Name == o.Name
}
func (t *TraitType) AssignableTo(target Type) bool {
	if _, ok := target.(AnyType); ok {
		return true
	}
	return t.Equals(target)
}

// ─── CAPABILITY & EFFECT TYPES ──────────────────────────────────────────────

type CapabilityType struct {
	Capability string
}

func (c CapabilityType) Kind() TypeKind { return KindCapability }
func (c CapabilityType) String() string { return fmt.Sprintf("Cap<%s>", c.Capability) }
func (c CapabilityType) Equals(other Type) bool {
	o, ok := other.(CapabilityType)
	return ok && c.Capability == o.Capability
}
func (c CapabilityType) AssignableTo(target Type) bool { return c.Equals(target) }

type EffectType struct {
	Effect string
}

func (e EffectType) Kind() TypeKind { return KindEffect }
func (e EffectType) String() string { return fmt.Sprintf("Effect<%s>", e.Effect) }
func (e EffectType) Equals(other Type) bool {
	o, ok := other.(EffectType)
	return ok && e.Effect == o.Effect
}
func (e EffectType) AssignableTo(target Type) bool { return e.Equals(target) }

// ─── TYPE PARSER ────────────────────────────────────────────────────────────

// Parse parses a type string into a Type object
func Parse(s string) (Type, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Void, nil
	}

	// Optionals: ?T
	if strings.HasPrefix(s, "?") {
		inner, err := Parse(s[1:])
		if err != nil {
			return nil, err
		}
		return &OptionalType{Base: inner}, nil
	}

	// Unions: A | B
	if strings.Contains(s, "|") {
		parts := strings.Split(s, "|")
		u := &UnionType{}
		for _, p := range parts {
			pt, err := Parse(p)
			if err != nil {
				return nil, err
			}
			u.Add(pt)
		}
		return u, nil
	}

	// Generics: List<T>, Result<T, E>
	if strings.Contains(s, "<") && strings.HasSuffix(s, ">") {
		idx := strings.Index(s, "<")
		base := strings.TrimSpace(s[:idx])
		inner := s[idx+1 : len(s)-1]

		if base == "Result" {
			parts := splitGenericArgs(inner)
			if len(parts) == 2 {
				okT, _ := Parse(parts[0])
				errT, _ := Parse(parts[1])
				return &ResultType{Ok: okT, Err: errT}, nil
			}
		}

		parts := splitGenericArgs(inner)
		var args []Type
		for _, p := range parts {
			t, err := Parse(p)
			if err != nil {
				return nil, err
			}
			args = append(args, t)
		}
		return &GenericType{Base: base, Parameters: args}, nil
	}

	// Function: fn(A, B) -> C
	if strings.HasPrefix(s, "fn(") {
		arrowIdx := strings.Index(s, "->")
		if arrowIdx != -1 {
			paramsPart := strings.TrimSpace(s[3:arrowIdx])
			paramsPart = strings.TrimSuffix(paramsPart, ")")
			paramsPart = strings.TrimSpace(paramsPart)
			retPart := strings.TrimSpace(s[arrowIdx+2:])

			var params []Type
			if paramsPart != "" {
				for _, p := range strings.Split(paramsPart, ",") {
					pt, err := Parse(p)
					if err != nil {
						return nil, err
					}
					params = append(params, pt)
				}
			}
			retT, err := Parse(retPart)
			if err != nil {
				return nil, err
			}
			return &FunctionType{Params: params, ReturnType: retT}, nil
		}
	}

	// Primitives
	switch s {
	case "Int":
		return Int, nil
	case "Float":
		return Float, nil
	case "String":
		return String, nil
	case "Bool":
		return Bool, nil
	case "Byte":
		return Byte, nil
	case "Null":
		return Null, nil
	case "Void":
		return Void, nil
	case "Any":
		return Any, nil
	default:
		return &StructType{Name: s}, nil
	}
}

func splitGenericArgs(s string) []string {
	var parts []string
	depth := 0
	current := strings.Builder{}

	for _, ch := range s {
		switch ch {
		case '<':
			depth++
			current.WriteRune(ch)
		case '>':
			depth--
			current.WriteRune(ch)
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(current.String()))
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, strings.TrimSpace(current.String()))
	}
	return parts
}
