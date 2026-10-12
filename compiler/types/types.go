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
	KindTypeVar    TypeKind = "TYPEVAR"
	KindEntity     TypeKind = "ENTITY"
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

// assignableToCompositeTarget is the one rule every concrete type needs for a
// target that *contains* other types: `?T` and `A | B`.
//
// # Why it is one function
//
// Leaving it to each type is how these rules end up half-implemented.
// `PrimitiveType` handled both and `StructType` handled neither, which made
// `?Address` a type nothing could be put into, and `P | Q` a type no `P` could
// satisfy — and an optional field is the ordinary reason a receiver can be null,
// so the first gap removed the use of `?.` entirely.
//
// # Why a source is checked directly rather than delegating
//
// `source.Equals(target)` is the caller's own identity check. Running it here
// rather than only after the composite cases keeps `T <: T | T` and `T <: ?T`
// from recursing: both would otherwise ask the source to assign to a base it is
// already equal to, which is the same question again.
func assignableToCompositeTarget(source Type, target Type, direct bool) (bool, bool) {
	// `Result<T, E>` and `Optional<T>` each exist as a dedicated Go type *and*
	// as a generic spelled `Result<...>`. An annotation resolves to one and a
	// value produces the other, so without a bridge between them a `let r:
	// Result<Int, String> = Ok(5)` compares two things that look the same in the
	// message and are different in the tree.
	//
	// The bridge is positional, because the two spellings carry the same
	// information in the same order: the Ok side, then the Err side.
	if params, other, ok := resultShaped(source, target); ok {
		return paramsAssignable(params, other) || direct, true
	}
	if params, other, ok := resultShaped(target, source); ok {
		return direct || paramsAssignable(other, params), true
	}

	switch t := target.(type) {
	case *TypeVar:
		// A type variable accepts whatever it is standing for. This is the
		// direction the checker asks about at a call site *before* inference —
		// `Int` against `T` — and getting it wrong here would reject every correct
		// call to a generic function.
		return true, true

	case *OptionalType:
		// `S <: ?T` when `S <: T` or `S` is `Null` (TYPE_SYSTEM §5 rule 3).
		if source.Equals(Null) || direct {
			return true, true
		}
		return source.AssignableTo(t.Base), true

	case *UnionType:
		// `S <: A | B` when `S` is one of the members. This is the case that was
		// missing entirely: nothing but a union source could satisfy a union
		// target, so `let x: P | Q = P { .. }` was rejected.
		if direct {
			return true, true
		}
		return t.Contains(source), true
	}

	return false, false
}

// resultShaped returns a type's Result or Optional parameters when the *other*
// type expresses the same thing by a different spelling.
//
// It is the bridge between the two representations, and it returns them in the
// same order so the two directions of the check can be written one after the
// other without either of them being the interesting one.
func resultShaped(a, b Type) (mine []Type, theirs []Type, ok bool) {
	switch v := a.(type) {
	case *ResultType:
		if _, isResult := b.(*GenericType); !isResult {
			return nil, nil, false
		}
		if g := b.(*GenericType); g.Base != "Result" {
			return nil, nil, false
		}
		return []Type{v.Ok, v.Err}, b.(*GenericType).Parameters, true

	case *GenericType:
		if v.Base == "Result" {
			if _, isResult := b.(*ResultType); !isResult {
				return nil, nil, false
			}
			return v.Parameters, []Type{b.(*ResultType).Ok, b.(*ResultType).Err}, true
		}
		if v.Base == "Optional" {
			if _, isOptional := b.(*OptionalType); !isOptional {
				return nil, nil, false
			}
			return v.Parameters, []Type{b.(*OptionalType).Base}, true
		}

	case *OptionalType:
		if g, isGeneric := b.(*GenericType); !isGeneric || g.Base != "Optional" {
			return nil, nil, false
		}
		return []Type{v.Base}, b.(*GenericType).Parameters, true
	}

	return nil, nil, false
}

// paramsAssignable compares two parameter lists position by position, with an
// `Any` on either side counting as "not yet known" rather than as a mismatch.
//
// It is the same rule `GenericType.AssignableTo` uses for its own parameters,
// lifted out so the Result bridge can use it too. Two copies of a rule is how they
// disagree, so it is one function called from both.
func paramsAssignable(mine, theirs []Type) bool {
	if len(mine) != len(theirs) {
		return false
	}
	for i := range mine {
		if mine[i] == nil || theirs[i] == nil {
			continue
		}
		if mine[i].Equals(Any) || theirs[i].Equals(Any) {
			continue
		}
		if !mine[i].AssignableTo(theirs[i]) {
			return false
		}
	}
	return true
}

func (p PrimitiveType) AssignableTo(target Type) bool {
	if target == nil {
		return false
	}
	if _, ok := target.(AnyType); ok {
		return true
	}
	if result, handled := assignableToCompositeTarget(p, target, p.Equals(target)); handled {
		return result
	}
	return p.Equals(target)
}

// Numeric widths.
//
// Each is a distinct type, not an alias. That is the whole point: `u8` and `Int`
// being the same type is what made `let x: u8 = 300;` compile, and a width that
// changes nothing is worse than no width at all — a developer who writes it
// reasonably believes a range is being checked.
//
// The run-time representation is still a 64-bit integer or float. A width here is
// a *static* constraint: literals are range-checked against it, and a width
// carries through arithmetic. A value computed at run time that exceeds its
// declared width is not caught, because catching it would mean a narrowing
// instruction in both engines — and a check that only sometimes fires is worse
// than one that is documented as compile-time.
var (
	Int8    = PrimitiveType{Name: "Int8"}
	Int16   = PrimitiveType{Name: "Int16"}
	Int32   = PrimitiveType{Name: "Int32"}
	Int64   = PrimitiveType{Name: "Int64"}
	Uint8   = PrimitiveType{Name: "Uint8"}
	Uint16  = PrimitiveType{Name: "Uint16"}
	Uint32  = PrimitiveType{Name: "Uint32"}
	Uint64  = PrimitiveType{Name: "Uint64"}
	Float32 = PrimitiveType{Name: "Float32"}
	Float64 = PrimitiveType{Name: "Float64"}
)

var (
	Int      = PrimitiveType{Name: "Int"}
	Float    = PrimitiveType{Name: "Float"}
	String   = PrimitiveType{Name: "String"}
	Bool     = PrimitiveType{Name: "Bool"}
	Byte     = PrimitiveType{Name: "Byte"}
	Null     = PrimitiveType{Name: "Null"}
	Void     = PrimitiveType{Name: "Void"}
	UUID     = PrimitiveType{Name: "UUID"}
	Email    = PrimitiveType{Name: "Email"}
	Date     = PrimitiveType{Name: "Date"}
	Money    = PrimitiveType{Name: "Money"}
	Quantity = PrimitiveType{Name: "Quantity"}
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
	if result, handled := assignableToCompositeTarget(s, target, s.Equals(target)); handled {
		return result
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
	if result, handled := assignableToCompositeTarget(e, target, e.Equals(target)); handled {
		return result
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
	if result, handled := assignableToCompositeTarget(u, target, u.Equals(target)); handled {
		return result
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
	if result, handled := assignableToCompositeTarget(o, target, o.Equals(target)); handled {
		return result
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
	if result, handled := assignableToCompositeTarget(r, target, false); handled {
		return result
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

// canonicalGenericBase maps the two names for the keyed-collection type onto one.
//
// Nilang has a single keyed collection at run time; `Map` and `Hash` are two
// names for it, borrowed from different vocabularies. Comparing the written names
// would make `fn f() -> Map<String, Int>` reject a body returning `Hash<String,
// Int>` — the same value under the other name — which is a distinction the
// language does not actually make.
func canonicalGenericBase(base string) string {
	switch base {
	case "Map", "Hash":
		return "Hash"
	case "Array", "List":
		// `Array<T>` is the name in every document and `List<T>` is the name the
		// checker infers for an array literal. They are one type, and leaving them
		// apart would make `fn first<T>(items: Array<T>)` reject `first([1, 2])` —
		// a generic function that cannot be called on the only kind of array the
		// language produces.
		return "Array"
	default:
		return base
	}
}

func (g *GenericType) Equals(other Type) bool {
	o, ok := other.(*GenericType)
	if !ok || canonicalGenericBase(g.Base) != canonicalGenericBase(o.Base) || len(g.Parameters) != len(o.Parameters) {
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
	if target == nil {
		return false
	}
	if _, ok := target.(AnyType); ok {
		return true
	}
	if u, ok := target.(*UnionType); ok {
		return u.Contains(g)
	}
	if result, handled := assignableToCompositeTarget(g, target, g.Equals(target)); handled {
		return result
	}
	o, ok := target.(*GenericType)
	if !ok || canonicalGenericBase(g.Base) != canonicalGenericBase(o.Base) || len(g.Parameters) != len(o.Parameters) {
		return false
	}
	for i := range g.Parameters {
		// An `Any` parameter in *either* position does not block the assignment.
		//
		// The target side already had this: an array literal of mixed elements is
		// `Array<Any>` and must be usable where `Array<Int>` is wanted. The source
		// side is the same situation reached from the other direction —
		// `Ok(5)` is `Result<Int, Any>` because a checker that has not read every
		// `Err` in a program cannot know the error type, and refusing to store it
		// in a `Result<Int, String>` would make `Ok` unusable with any annotated
		// Result.
		//
		// What it costs is that a genuinely `Any`-typed container can be stored as
		// a specific one without complaint. That is the price of inference, and it
		// is the same price the language already pays in the other direction.
		if o.Parameters[i].Equals(Any) || g.Parameters[i].Equals(Any) {
			continue
		}
		if !g.Parameters[i].AssignableTo(o.Parameters[i]) {
			return false
		}
	}
	return true
}

// ─── FUNCTION TYPE ──────────────────────────────────────────────────────────

type FunctionType struct {
	Params     []Type
	ReturnType Type
	Effects    []string
	MinArgs    int // minimum argument count required
	MaxArgs    int // maximum argument count allowed (-1 for variadic / unlimited)

	// Requires names the capabilities a call to this function needs.
	//
	// It lives on the signature rather than on the declaration so that a call
	// site can check it without looking the declaration up again. A function
	// whose capability is missing cannot do its job, so the failure belongs at
	// the call, before the program runs.
	Requires []string

	// IsPure records that the function was declared `pure fn`.
	IsPure bool

	// IsUnsafe records that the function was declared `unsafe fn`.
	//
	// Inside such a function a capability-gated operation does not require its
	// capability to be granted to the project. The exemption is per-function and
	// has to be written down, so a reviewer can see where it applies.
	IsUnsafe bool
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

// MinExpectedArgs returns the effective minimum argument count.
func (f *FunctionType) MinExpectedArgs() int {
	if f.MinArgs > 0 {
		return f.MinArgs
	}
	if f.MaxArgs == -1 {
		return 0
	}
	return len(f.Params)
}

// MaxExpectedArgs returns the effective maximum argument count (-1 for variadic / unlimited).
func (f *FunctionType) MaxExpectedArgs() int {
	if f.MaxArgs != 0 || len(f.Params) == 0 {
		return f.MaxArgs
	}
	return len(f.Params)
}

// CheckArity verifies if argCount is within [MinExpectedArgs(), MaxExpectedArgs()].
func (f *FunctionType) CheckArity(argCount int) bool {
	min := f.MinExpectedArgs()
	max := f.MaxExpectedArgs()
	if argCount < min {
		return false
	}
	if max >= 0 && argCount > max {
		return false
	}
	return true
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
	if target == nil {
		return false
	}
	if result, handled := assignableToCompositeTarget(f, target, f.Equals(target)); handled {
		return result
	}
	if target == nil {
		return false
	}
	if _, ok := target.(AnyType); ok {
		return true
	}
	if u, ok := target.(*UnionType); ok {
		return u.Contains(f)
	}
	o, ok := target.(*FunctionType)
	if !ok || len(f.Params) != len(o.Params) {
		return false
	}
	for i := range f.Params {
		if f.Params[i].Equals(Any) || o.Params[i].Equals(Any) || o.Params[i].AssignableTo(f.Params[i]) {
			continue
		}
		return false
	}
	if f.ReturnType != nil && o.ReturnType != nil {
		if !f.ReturnType.Equals(Any) && !o.ReturnType.Equals(Any) && !f.ReturnType.AssignableTo(o.ReturnType) {
			return false
		}
	}
	return true
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
func (c CapabilityType) AssignableTo(target Type) bool {
	if result, handled := assignableToCompositeTarget(c, target, c.Equals(target)); handled {
		return result
	}
	return c.Equals(target)
}

type EffectType struct {
	Effect string
}

func (e EffectType) Kind() TypeKind { return KindEffect }
func (e EffectType) String() string { return fmt.Sprintf("Effect<%s>", e.Effect) }
func (e EffectType) Equals(other Type) bool {
	o, ok := other.(EffectType)
	return ok && e.Effect == o.Effect
}
func (e EffectType) AssignableTo(target Type) bool {
	if result, handled := assignableToCompositeTarget(e, target, e.Equals(target)); handled {
		return result
	}
	return e.Equals(target)
}

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
	case "Int", "int", "i64":
		return Int, nil
	case "Float", "float", "f64":
		return Float, nil
	case "String", "string":
		return String, nil
	case "Bool", "bool":
		return Bool, nil
	case "Byte":
		return Byte, nil
	case "Null":
		return Null, nil
	case "Void":
		return Void, nil
	case "Int8", "i8":
		return Int8, nil
	case "Int16", "i16":
		return Int16, nil
	case "Int32", "i32":
		return Int32, nil
	case "Uint8", "u8":
		return Uint8, nil
	case "Uint16", "u16":
		return Uint16, nil
	case "Uint32", "u32":
		return Uint32, nil
	case "Uint64", "u64":
		return Uint64, nil
	case "Float32", "f32":
		return Float32, nil
	case "UUID":
		return UUID, nil
	case "Email":
		return Email, nil
	case "Date":
		return Date, nil
	case "Money":
		return Money, nil
	case "Quantity":
		return Quantity, nil
	case "Any":
		return Any, nil
	default:
		return &StructType{Name: s}, nil
	}
}

// ─── ENTITY TYPE (web-implications.md Section 26) ───────────────────────────

type EntityFieldDef struct {
	Name         string
	Type         Type
	IsPrimary    bool
	IsRequired   bool
	IsUnique     bool
	TargetEntity string
}

type EntityType struct {
	Name   string
	Fields []EntityFieldDef
}

func (e *EntityType) Kind() TypeKind { return KindEntity }
func (e *EntityType) String() string {
	if len(e.Fields) == 0 {
		return "entity " + e.Name
	}
	var f []string
	for _, field := range e.Fields {
		f = append(f, fmt.Sprintf("%s: %s", field.Name, field.Type.String()))
	}
	return fmt.Sprintf("entity %s { %s }", e.Name, strings.Join(f, ", "))
}
func (e *EntityType) Equals(other Type) bool {
	if o, ok := other.(*EntityType); ok {
		return e.Name == o.Name
	}
	return false
}
func (e *EntityType) AssignableTo(target Type) bool {
	if target == nil {
		return false
	}
	if _, ok := target.(AnyType); ok {
		return true
	}
	if result, handled := assignableToCompositeTarget(e, target, e.Equals(target)); handled {
		return result
	}
	return e.Equals(target)
}
func (e *EntityType) GetField(name string) (EntityFieldDef, bool) {
	for _, f := range e.Fields {
		if f.Name == name {
			return f, true
		}
	}
	return EntityFieldDef{}, false
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

// ─── TYPE VARIABLE ──────────────────────────────────────────────────────────

// TypeVar is a type parameter: the `T` in `fn map<T>(...)`.
//
// # Why it exists rather than reusing a placeholder struct
//
// `types.Parse` already produces a `*StructType` with no fields for a name it
// does not recognise, and reusing that would make a type parameter
// indistinguishable from a type that was never declared. The two are opposites:
// a placeholder means "the checker has not seen this type", and a type variable
// means "this is deliberately not fixed yet". Reporting E0502 for the second
// because of the first would reject every generic function.
//
// # Why it is assignable in both directions
//
// A type variable stands for whatever it is instantiated with, so before a call
// site binds it, there is nothing it can be checked against. The checker applies
// its inferences *before* asking about assignability, so by the time this method
// is reached in a meaningful case the variable has been replaced.
//
// Where it has not — a body that uses `T` without a call site to bind it — the
// permissive answer is right, because the alternative is rejecting a correct
// function body on the grounds that the caller has not been read yet.
type TypeVar struct {
	Name string
}

func (t *TypeVar) Kind() TypeKind { return KindTypeVar }
func (t *TypeVar) String() string { return t.Name }

func (t *TypeVar) Equals(other Type) bool {
	if t == nil {
		return false
	}
	if v, ok := other.(*TypeVar); ok {
		return v.Name == t.Name
	}
	// A variable matches nothing else on its own. The checker applies its
	// inferences *before* asking about equality, so a variable still standing in
	// a comparison is one the call site could not work out — and reporting that as
	// unequal would reject a correct function body on the grounds that nobody has
	// called it yet.
	return false
}

func (t *TypeVar) AssignableTo(target Type) bool {
	if t == nil || target == nil {
		return false
	}
	if _, ok := target.(AnyType); ok {
		return true
	}
	if t.Equals(target) {
		return true
	}
	// Anything else is accepted, because nothing is decided until a call site
	// binds the variable.
	//
	// That sounds like the hole it is, and it is — but the checker instantiates
	// the signature *before* asking, so this answer is only reached for a variable
	// a call site could not work out. Refusing here would reject the correct
	// function it appears in, on the grounds that its caller has not been read.
	//
	// What it does cost is that `first(5)` against `fn first<T>(items: Array<T>)`
	// is caught by the *structural* mismatch — an integer is not an array — and
	// not by this method. The cases worth catching are caught; the ones that are
	// not are cases where the variable was never pinned down at all.
	return true
}

// HasTypeVar reports whether a type mentions any type variable.
//
// It exists because the two things that must know — instantiation at a call
// site, and whether a signature is worth caching under a concrete type — both
// need the same walk, and a second walk would eventually disagree with the first.
func HasTypeVar(t Type) bool {
	switch v := t.(type) {
	case *TypeVar:
		return true
	case *GenericType:
		for _, param := range v.Parameters {
			if HasTypeVar(param) {
				return true
			}
		}
	case *OptionalType:
		return HasTypeVar(v.Base)
	case *UnionType:
		for _, member := range v.Types {
			if HasTypeVar(member) {
				return true
			}
		}
	case *ResultType:
		return HasTypeVar(v.Ok) || HasTypeVar(v.Err)
	case *FunctionType:
		for _, param := range v.Params {
			if HasTypeVar(param) {
				return true
			}
		}
		return HasTypeVar(v.ReturnType)
	}
	return false
}

// SubstituteTypeVars replaces every type variable in t using bindings.
//
// A variable with no binding is left as it is: the function is still legal, the
// call site just could not work out what it was.
func SubstituteTypeVars(t Type, bindings map[string]Type) Type {
	if t == nil || len(bindings) == 0 {
		return t
	}

	switch v := t.(type) {
	case *TypeVar:
		if bound, ok := bindings[v.Name]; ok {
			return bound
		}
		return v

	case *GenericType:
		params := make([]Type, 0, len(v.Parameters))
		for _, param := range v.Parameters {
			params = append(params, SubstituteTypeVars(param, bindings))
		}
		return &GenericType{Base: v.Base, Parameters: params}

	case *OptionalType:
		return &OptionalType{Base: SubstituteTypeVars(v.Base, bindings)}

	case *UnionType:
		members := make([]Type, 0, len(v.Types))
		for _, member := range v.Types {
			members = append(members, SubstituteTypeVars(member, bindings))
		}
		return &UnionType{Types: members}

	case *ResultType:
		return &ResultType{
			Ok:  SubstituteTypeVars(v.Ok, bindings),
			Err: SubstituteTypeVars(v.Err, bindings),
		}

	case *FunctionType:
		params := make([]Type, 0, len(v.Params))
		for _, param := range v.Params {
			params = append(params, SubstituteTypeVars(param, bindings))
		}
		return &FunctionType{
			Params:     params,
			ReturnType: SubstituteTypeVars(v.ReturnType, bindings),
			Effects:    v.Effects,
			MinArgs:    v.MinArgs,
			MaxArgs:    v.MaxArgs,
		}
	}

	return t
}
