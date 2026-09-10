package object

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/code"
)

type ObjectType string

const (
	INTEGER_OBJ           = "INTEGER"
	FLOAT_OBJ             = "FLOAT"
	BOOLEAN_OBJ           = "BOOLEAN"
	NULL_OBJ              = "NULL"
	RETURN_VALUE_OBJ      = "RETURN_VALUE"
	ERROR_OBJ             = "ERROR"
	FUNCTION_OBJ          = "FUNCTION"
	STRING_OBJ            = "STRING"
	BUILTIN_OBJ           = "BUILTIN"
	ARRAY_OBJ             = "ARRAY"
	HASH_OBJ              = "HASH"
	COMPILED_FUNCTION_OBJ = "COMPILED_FUNCTION"
	CLOSURE_OBJ           = "CLOSURE"
	CAPTURE_CELL_OBJ      = "CAPTURE_CELL"
	ENTITY_OBJ            = "ENTITY"
	FUTURE_OBJ            = "FUTURE"
	CHANNEL_OBJ           = "CHANNEL"
	RESULT_OBJ            = "RESULT"
	OPTIONAL_OBJ          = "OPTIONAL"
)

type Object interface {
	Type() ObjectType
	Inspect() string
}

type HashKey struct {
	Type  ObjectType
	Value uint64
}

type Hashable interface {
	HashKey() HashKey
}

// ─── TYPES ──────────────────────────────────────────────────────────────────

type Integer struct {
	Value int64
}

func (i *Integer) Type() ObjectType { return INTEGER_OBJ }
func (i *Integer) Inspect() string  { return fmt.Sprintf("%d", i.Value) }
func (i *Integer) HashKey() HashKey {
	return HashKey{Type: i.Type(), Value: uint64(i.Value)}
}

type Float struct {
	Value float64
}

func (f *Float) Type() ObjectType { return FLOAT_OBJ }
func (f *Float) Inspect() string  { return fmt.Sprintf("%g", f.Value) }

type Boolean struct {
	Value bool
}

func (b *Boolean) Type() ObjectType { return BOOLEAN_OBJ }
func (b *Boolean) Inspect() string  { return fmt.Sprintf("%t", b.Value) }
func (b *Boolean) HashKey() HashKey {
	var value uint64
	if b.Value {
		value = 1
	} else {
		value = 0
	}
	return HashKey{Type: b.Type(), Value: value}
}

type Null struct{}

func (n *Null) Type() ObjectType { return NULL_OBJ }
func (n *Null) Inspect() string  { return "null" }

type ReturnValue struct {
	Value Object
}

func (rv *ReturnValue) Type() ObjectType { return RETURN_VALUE_OBJ }
func (rv *ReturnValue) Inspect() string  { return rv.Value.Inspect() }

type Error struct {
	Message string
}

func (e *Error) Type() ObjectType { return ERROR_OBJ }
func (e *Error) Inspect() string  { return "ERROR: " + e.Message }

type Function struct {
	Parameters []*ast.Identifier
	Body       *ast.BlockStatement
	Env        *Environment
}

func (f *Function) Type() ObjectType { return FUNCTION_OBJ }
func (f *Function) Inspect() string {
	var out bytes.Buffer
	params := []string{}
	for _, p := range f.Parameters {
		params = append(params, p.String())
	}
	out.WriteString("fn(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") {\n")
	out.WriteString(f.Body.String())
	out.WriteString("\n}")
	return out.String()
}

type String struct {
	Value string
}

func (s *String) Type() ObjectType { return STRING_OBJ }
func (s *String) Inspect() string  { return s.Value }
func (s *String) HashKey() HashKey {
	h := fnv.New64a()
	h.Write([]byte(s.Value))
	return HashKey{Type: s.Type(), Value: h.Sum64()}
}

type BuiltinFunction func(args ...Object) Object

type Builtin struct {
	Fn BuiltinFunction
}

func (b *Builtin) Type() ObjectType { return BUILTIN_OBJ }
func (b *Builtin) Inspect() string  { return "builtin function" }

type Array struct {
	Elements []Object
}

func (ao *Array) Type() ObjectType { return ARRAY_OBJ }
func (ao *Array) Inspect() string {
	var out bytes.Buffer
	elements := []string{}
	for _, e := range ao.Elements {
		elements = append(elements, e.Inspect())
	}
	out.WriteString("[")
	out.WriteString(strings.Join(elements, ", "))
	out.WriteString("]")
	return out.String()
}

type HashPair struct {
	Key   Object
	Value Object
}

type Hash struct {
	Pairs map[HashKey]HashPair
}

func (h *Hash) Type() ObjectType { return HASH_OBJ }
func (h *Hash) Inspect() string {
	var out bytes.Buffer
	pairs := []string{}
	for _, pair := range h.Pairs {
		pairs = append(pairs, fmt.Sprintf("%s: %s", pair.Key.Inspect(), pair.Value.Inspect()))
	}
	out.WriteString("{")
	out.WriteString(strings.Join(pairs, ", "))
	out.WriteString("}")
	return out.String()
}

type CompiledFunction struct {
	Instructions  code.Instructions
	NumLocals     int
	NumParameters int
}

func (cf *CompiledFunction) Type() ObjectType { return COMPILED_FUNCTION_OBJ }
func (cf *CompiledFunction) Inspect() string  { return fmt.Sprintf("CompiledFunction[%p]", cf) }

// CaptureCell is a heap-allocated mutable cell shared between closures.
// When a closure captures a variable that may be mutated, we wrap it in a
// CaptureCell so that all closures sharing the same captured variable see
// the same mutable cell rather than an immutable value copy.
type CaptureCell struct {
	Value Object
}

func (cc *CaptureCell) Type() ObjectType { return CAPTURE_CELL_OBJ }
func (cc *CaptureCell) Inspect() string  { return fmt.Sprintf("Cell(%s)", cc.Value.Inspect()) }

type Closure struct {
	Fn   *CompiledFunction
	Free []*CaptureCell // shared mutable cells, not value copies
}

func (c *Closure) Type() ObjectType { return CLOSURE_OBJ }
func (c *Closure) Inspect() string  { return fmt.Sprintf("Closure[%p]", c) }

type futureResult struct {
	Value Object
	Err   error
}

type Future struct {
	done   chan futureResult
	once   sync.Once
	result futureResult
}

func NewFuture() *Future {
	return &Future{done: make(chan futureResult, 1)}
}

func (f *Future) Type() ObjectType { return FUTURE_OBJ }
func (f *Future) Inspect() string  { return "future" }
func (f *Future) Complete(value Object, err error) {
	f.done <- futureResult{Value: value, Err: err}
}
func (f *Future) Await() (Object, error) {
	f.once.Do(func() { f.result = <-f.done })
	return f.result.Value, f.result.Err
}

type Channel struct {
	Values chan Object
}

func NewChannel(capacity int) *Channel {
	return &Channel{Values: make(chan Object, capacity)}
}

func (c *Channel) Type() ObjectType { return CHANNEL_OBJ }
func (c *Channel) Inspect() string  { return "channel" }

// ─── ENVIRONMENT ────────────────────────────────────────────────────────────

type Environment struct {
	store     map[string]Object
	constants map[string]bool
	outer     *Environment
}

func NewEnvironment() *Environment {
	s := make(map[string]Object)
	return &Environment{store: s, constants: make(map[string]bool), outer: nil}
}

func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	return env
}

func (e *Environment) Get(name string) (Object, bool) {
	obj, ok := e.store[name]
	if !ok && e.outer != nil {
		obj, ok = e.outer.Get(name)
	}
	return obj, ok
}

func (e *Environment) Set(name string, val Object) Object {
	e.store[name] = val
	return val
}

func (e *Environment) SetConst(name string, val Object) Object {
	e.store[name] = val
	e.constants[name] = true
	return val
}

func (e *Environment) IsConst(name string) bool {
	if _, ok := e.store[name]; ok {
		return e.constants[name]
	}
	if e.outer != nil {
		return e.outer.IsConst(name)
	}
	return false
}

func (e *Environment) Assign(name string, val Object) bool {
	if _, ok := e.store[name]; ok {
		if e.constants[name] {
			return false
		}
		e.store[name] = val
		return true
	}
	if e.outer != nil {
		return e.outer.Assign(name, val)
	}
	return false
}

func (e *Environment) Store() map[string]Object {
	return e.store
}

func (e *Environment) Snapshot() *Environment {
	snapshot := NewEnvironment()
	if e.outer != nil {
		for name, value := range e.outer.Snapshot().store {
			snapshot.store[name] = value
		}
	}
	for name, value := range e.store {
		snapshot.store[name] = value
	}
	return snapshot
}

// ─── ENTITY OBJECT (web-implications.md Section 26) ─────────────────────────

type EntityFieldObj struct {
	Name         string
	Type         string
	IsPrimary    bool
	IsRequired   bool
	IsUnique     bool
	TargetEntity string
}

type Entity struct {
	Name   string
	Fields []EntityFieldObj
}

func (e *Entity) Type() ObjectType { return ENTITY_OBJ }
func (e *Entity) Inspect() string {
	var out bytes.Buffer
	out.WriteString(fmt.Sprintf("entity %s {\n", e.Name))
	for _, f := range e.Fields {
		out.WriteString(fmt.Sprintf("  %s: %s\n", f.Name, f.Type))
	}
	out.WriteString("}")
	return out.String()
}

// ─── RESULT OBJECT (web-implications.md Section 1) ──────────────────────────

type Result struct {
	IsOk  bool
	Value Object
	Error Object
}

func (r *Result) Type() ObjectType { return RESULT_OBJ }
func (r *Result) Inspect() string {
	if r.IsOk {
		val := "null"
		if r.Value != nil {
			val = r.Value.Inspect()
		}
		return fmt.Sprintf("Ok(%s)", val)
	}
	err := "null"
	if r.Error != nil {
		err = r.Error.Inspect()
	}
	return fmt.Sprintf("Err(%s)", err)
}

// ─── OPTIONAL OBJECT (web-implications.md Section 1) ────────────────────────

type Optional struct {
	HasValue bool
	Value    Object
}

func (o *Optional) Type() ObjectType { return OPTIONAL_OBJ }
func (o *Optional) Inspect() string {
	if o.HasValue {
		val := "null"
		if o.Value != nil {
			val = o.Value.Inspect()
		}
		return fmt.Sprintf("Some(%s)", val)
	}
	return "None"
}
