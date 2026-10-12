package object

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"sort"
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

	// Loop control signals.
	//
	// They exist so that `break` and `continue` can unwind the tree-walking
	// evaluator through its ordinary value channel, the same way `return`
	// already does. The stack VM compiles a loop into real branches and never
	// produces these.
	BREAK_SIGNAL_OBJ    = "BREAK_SIGNAL"
	CONTINUE_SIGNAL_OBJ = "CONTINUE_SIGNAL"
)

// The three values every engine and every builtin agrees on.
//
// They live here, rather than in one engine package, because a Go builtin
// returns them and the value crosses into whichever engine is running. When each
// package made its own, a boolean produced by `isOk` was a different pointer from
// the VM's `False`, so `if (isOk(x)) { ... } else { ... }` took the wrong branch
// under `--vm` while the evaluator was correct.
//
// They are exported so that code creating these values can create *these* ones.
// Prefer them over `&Boolean{...}` for exactly that reason: the truthiness tests
// compare by type and value, but a shared pointer also keeps identity
// comparisons and the collector's mark set honest.
var (
	TRUE  = &Boolean{Value: true}
	FALSE = &Boolean{Value: false}
	NULL  = &Null{}
)

// IsTrue reports whether obj is the language's true value.
func IsTrue(obj Object) bool {
	b, ok := obj.(*Boolean)
	return ok && b.Value
}

// IsFalse reports whether obj is the language's false value.
func IsFalse(obj Object) bool {
	b, ok := obj.(*Boolean)
	return ok && !b.Value
}

// IsNull reports whether obj is the language's null value.
func IsNull(obj Object) bool {
	_, ok := obj.(*Null)
	return ok
}

// LoopSignal is the evaluator's marker for `break` or `continue`.
//
// It is a distinct type rather than a sentinel Integer so that a program cannot
// forge one: no builtin produces a LoopSignal, and none accepts one.
//
// The signal kind lives in `kind` rather than in a field named Type, because
// Type is the method every Object must provide.
type LoopSignal struct {
	kind ObjectType
}

// NewLoopSignal builds a signal of the given kind.
func NewLoopSignal(kind ObjectType) *LoopSignal {
	return &LoopSignal{kind: kind}
}

// Type reports which loop control signal this is.
func (l *LoopSignal) Type() ObjectType {
	if l == nil {
		return CONTINUE_SIGNAL_OBJ
	}
	return l.kind
}

func (l *LoopSignal) Inspect() string {
	if l == nil || l.kind == BREAK_SIGNAL_OBJ {
		return "break"
	}
	return "continue"
}

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

	// IsPure records a `pure fn` declaration. The evaluator refuses an
	// effectful call made while a pure function is on the call stack
	// (runtime E0202), which is the dynamic half of the static check the
	// type checker performs. It defaults to false, so functions built by
	// hand or by older lowering passes are treated as effectful.
	IsPure bool
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

	// order preserves insertion order for Pairs.
	//
	// Pairs alone cannot report a stable order, because a Go map has none. That
	// matters wherever a hash is observed rather than only looked up: iterating
	// `for k, v in hash`, printing a hash, and rendering a JSON response would
	// otherwise produce a different order on every run. See SEMANTICS.md §1.4.
	//
	// order may be empty for a hash assembled programmatically; OrderedPairs
	// then falls back to a sorted key order rather than a random one.
	order []HashKey
}

// NewHash creates an empty hash with room for size entries.
//
// Construction always goes through this rather than a struct literal, because a
// literal cannot initialise the order slice and would leave the hash in the
// degraded state where iteration falls back to sorted keys.
func NewHash(size int) *Hash {
	if size < 0 {
		size = 0
	}
	return &Hash{
		Pairs: make(map[HashKey]HashPair, size),
		order: make([]HashKey, 0, size),
	}
}

// SetPair records a key and value, preserving insertion order.
//
// Every mutation goes through here so that the two views cannot diverge: a pair
// present in Pairs but missing from order would make iteration incomplete.
func (h *Hash) SetPair(key, value Object) {
	hashKey, ok := key.(Hashable)
	if !ok {
		// A non-hashable key can never be looked up, so storing it would create
		// an entry that is unreachable. The evaluator rejects such keys before
		// reaching here.
		return
	}

	hk := hashKey.HashKey()
	if _, exists := h.Pairs[hk]; !exists {
		h.order = append(h.order, hk)
	}
	h.Pairs[hk] = HashPair{Key: key, Value: value}
}

// OrderedPairs returns the hash's entries in insertion order.
//
// A hash built without SetPair, which is possible when a caller constructs the
// Pairs map directly, falls back to sorted-by-inspection order so that output is
// deterministic rather than dependent on Go's randomised map iteration.
func (h *Hash) OrderedPairs() []HashPair {
	if len(h.order) > 0 {
		out := make([]HashPair, 0, len(h.order))
		for _, hk := range h.order {
			if pair, ok := h.Pairs[hk]; ok {
				out = append(out, pair)
			}
		}
		return out
	}

	keys := make([]HashKey, 0, len(h.Pairs))
	for hk := range h.Pairs {
		keys = append(keys, hk)
	}
	sort.Slice(keys, func(i, j int) bool {
		return h.Pairs[keys[i]].Key.Inspect() < h.Pairs[keys[j]].Key.Inspect()
	})

	out := make([]HashPair, 0, len(keys))
	for _, hk := range keys {
		out = append(out, h.Pairs[hk])
	}
	return out
}

func (h *Hash) Type() ObjectType { return HASH_OBJ }

// Inspect renders the hash in insertion order.
//
// It used to sort the rendered pairs, which is a different thing: sorting the
// *text* reorders keys and values independently, so `{ b: 1, a: 2 }` printed as
// `{ a: 2, b: 1 }`. A struct's printed form then disagreed with the order its
// fields were declared in, which is the one order a reader has to be able to
// rely on. SEMANTICS.md §1.4 requires insertion order wherever a hash is
// observed rather than only looked up.
func (h *Hash) Inspect() string {
	var out bytes.Buffer
	pairs := make([]string, 0, len(h.Pairs))
	for _, pair := range h.OrderedPairs() {
		pairs = append(pairs, fmt.Sprintf("%s: %s", pair.Key.Inspect(), pair.Value.Inspect()))
	}
	out.WriteString("{")
	out.WriteString(strings.Join(pairs, ", "))
	out.WriteString("}")
	return out.String()
}

// SourcePos maps a bytecode offset to a source location. It is the debug
// line table used by the VM debugger to translate an instruction pointer back
// to a file/line/column.
type SourcePos struct {
	Offset int `json:"offset"`
	Line   int `json:"line"`
	Column int `json:"column"`
}

type CompiledFunction struct {
	Instructions  code.Instructions
	NumLocals     int
	NumParameters int

	// IsPure records a `pure fn` declaration through compilation, so the VM
	// can refuse an effectful call made while a pure function is on the call
	// stack (runtime E0202). It travels in the NABC image (v2) beside the
	// locals and parameter counts.
	IsPure bool

	// Name is the function name ("" for anonymous closures; "<main>" for the
	// top-level script). Used by the debugger's call stack.
	Name string
	// SourceFile is the source file this function was compiled from.
	SourceFile string
	// Positions is a sorted offset->source line table.
	Positions []SourcePos
	// LocalNames holds the debug name of each local slot (index -> name).
	LocalNames []string
}

func (cf *CompiledFunction) Type() ObjectType { return COMPILED_FUNCTION_OBJ }
func (cf *CompiledFunction) Inspect() string  { return fmt.Sprintf("CompiledFunction[%p]", cf) }

// LineAt returns the source line and column for a bytecode offset using the
// position table (the greatest offset <= target). Returns (0,0) if unknown.
func (cf *CompiledFunction) LineAt(offset int) (int, int) {
	if len(cf.Positions) == 0 {
		return 0, 0
	}
	line, col := cf.Positions[0].Line, cf.Positions[0].Column
	for _, p := range cf.Positions {
		if p.Offset > offset {
			break
		}
		line, col = p.Line, p.Column
	}
	return line, col
}

// LocalName returns the debug name for a local slot.
func (cf *CompiledFunction) LocalName(index int) string {
	if index >= 0 && index < len(cf.LocalNames) && cf.LocalNames[index] != "" {
		return cf.LocalNames[index]
	}
	return fmt.Sprintf("local[%d]", index)
}

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
	mu sync.Mutex
	// done is closed when the Future completes. A closed channel reports
	// readiness to every receiver without consuming anything, which is what
	// lets Resolved ask without stealing the result Await is owed.
	done      chan struct{}
	result    futureResult
	completed bool
}

func NewFuture() *Future {
	return &Future{done: make(chan struct{})}
}

func (f *Future) Type() ObjectType { return FUTURE_OBJ }
func (f *Future) Inspect() string  { return "future" }

// Resolved returns the value a completed Future settled on, and whether it has
// completed at all.
//
// It exists for the garbage collector: a Future holds a reference to its result,
// and that reference must be traced or the result would be swept while the
// Future is still live. The query does not block, so a collector can ask during
// a mark phase without deadlocking on the task that owns the Future.
func (f *Future) Resolved() (Object, bool) {
	select {
	case <-f.done:
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.result.Value, true
	default:
		return nil, false
	}
}

func (f *Future) Complete(value Object, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.completed {
		return
	}
	f.result = futureResult{Value: value, Err: err}
	f.completed = true
	close(f.done)
}
func (f *Future) Await() (Object, error) {
	<-f.done
	f.mu.Lock()
	defer f.mu.Unlock()
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

	// CallDepth counts the function frames on this evaluation chain (L6.3).
	//
	// It lives here rather than in a package counter because task bodies run
	// on their own goroutines: a shared counter is a data race, and worse, a
	// shared budget — one task's depth would spend another's. Each function
	// call extends the caller's environment with depth+1, so concurrent chains
	// count independently and sequential calls reuse the parent's count.
	// Only freshly created environments are ever written; a shared parent is
	// read, never mutated, which is what keeps this race-free.
	CallDepth int

	// PureDepth counts the pure functions on this evaluation chain (L6.3).
	// Same placement rationale as CallDepth: purity is a property of the
	// dynamic call stack, and two tasks have two stacks. A task spawned
	// inside a pure function inherits its purity, so an effectful call
	// inside still reports E0202 — while a concurrent impure task is
	// unaffected by it.
	PureDepth int

	// DepthLimit bounds this chain; 0 selects evalCallDepthLimit. Carried so
	// an embedder can bound one evaluation without moving a global that
	// every other evaluation shares.
	DepthLimit int

	// mu guards the store because task bodies run on their own goroutines
	// while sharing their parent's bindings (L6.2). A task reads its
	// parent's variables and assigns through to them; without the lock a
	// concurrent map read and write is a fatal runtime error, not a data
	// race the program could observe and recover from.
	mu sync.RWMutex
}

func NewEnvironment() *Environment {
	s := make(map[string]Object)
	return &Environment{store: s, constants: make(map[string]bool), outer: nil}
}

func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	if outer != nil {
		// The chain's counters travel with it: a child starts where its
		// parent is, and only the child's own frame or purity adds to them.
		env.CallDepth = outer.CallDepth
		env.PureDepth = outer.PureDepth
		env.DepthLimit = outer.DepthLimit
	}
	return env
}

func (e *Environment) Get(name string) (Object, bool) {
	e.mu.RLock()
	obj, ok := e.store[name]
	e.mu.RUnlock()
	if !ok && e.outer != nil {
		obj, ok = e.outer.Get(name)
	}
	return obj, ok
}

func (e *Environment) Set(name string, val Object) Object {
	e.mu.Lock()
	e.store[name] = val
	e.mu.Unlock()
	return val
}

func (e *Environment) SetConst(name string, val Object) Object {
	e.mu.Lock()
	e.store[name] = val
	e.constants[name] = true
	e.mu.Unlock()
	return val
}

func (e *Environment) IsConst(name string) bool {
	e.mu.RLock()
	_, ok := e.store[name]
	isConst := e.constants[name]
	e.mu.RUnlock()
	if ok {
		return isConst
	}
	if e.outer != nil {
		return e.outer.IsConst(name)
	}
	return false
}

func (e *Environment) Assign(name string, val Object) bool {
	e.mu.Lock()
	if _, ok := e.store[name]; ok {
		if e.constants[name] {
			e.mu.Unlock()
			return false
		}
		e.store[name] = val
		e.mu.Unlock()
		return true
	}
	e.mu.Unlock()
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
		outerSnap := e.outer.Snapshot()
		outerSnap.mu.RLock()
		for name, value := range outerSnap.store {
			snapshot.store[name] = value
		}
		for name := range outerSnap.constants {
			snapshot.constants[name] = true
		}
		outerSnap.mu.RUnlock()
	}
	e.mu.RLock()
	for name, value := range e.store {
		snapshot.store[name] = value
	}
	for name := range e.constants {
		snapshot.constants[name] = true
	}
	e.mu.RUnlock()
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
