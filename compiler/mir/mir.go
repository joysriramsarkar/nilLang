package mir

import (
	"fmt"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/types"
)

// Operand represents a value, variable, or temporary in MIR
type Operand interface {
	String() string
}

type ConstOperand struct {
	Value interface{}
}

func (c ConstOperand) String() string {
	switch v := c.Value.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

type VarOperand struct {
	Name string
}

func (v VarOperand) String() string {
	return v.Name
}

type TempOperand struct {
	ID int
}

func (t TempOperand) String() string {
	return fmt.Sprintf("_t%d", t.ID)
}

// ─── INSTRUCTIONS ───────────────────────────────────────────────────────────

type Instruction interface {
	String() string
}

type AssignInst struct {
	Dest Operand
	Src  Operand
}

func (i AssignInst) String() string {
	return fmt.Sprintf("%s = %s", i.Dest.String(), i.Src.String())
}

type BinaryOpInst struct {
	Dest  Operand
	Op    string
	Left  Operand
	Right Operand
}

func (i BinaryOpInst) String() string {
	return fmt.Sprintf("%s = %s %s %s", i.Dest.String(), i.Left.String(), i.Op, i.Right.String())
}

// SelectInst is a two-way choice: the value of True when Cond holds, otherwise
// the value of False.
//
// # Why it is one instruction and not a branch
//
// A branch would be the CFG-level answer, and it is the wrong one here. `cond ? a
// : b` is an *expression* — it has a value that something else consumes — and a
// branch produces a value only through a phi at a merge block. That is the right
// shape for a statement, where the merge point exists anyway.
//
// Keeping the arms inside one instruction also keeps them *visible*. The previous
// lowering wrote the false arm and then discarded it, and the emitted MIR read
// `_t2 = _t1 ? "big"` — an operator with no second operand. A select that names
// both arms cannot be printed that way, so the loss becomes visible in the IR
// instead of showing up as a wrong answer much later.
type SelectInst struct {
	Dest  Operand
	Cond  Operand
	True  Operand
	False Operand
}

func (i SelectInst) String() string {
	return fmt.Sprintf("%s = %s ? %s : %s", i.Dest.String(), i.Cond.String(), i.True.String(), i.False.String())
}

// AggregateInst builds a list or a hash from its elements.
//
// # Why this is not a call
//
// The lowering used to emit `call array(1, 2, 3)` and `call map("k", 7)` — names
// invented here, for which the VM has no builtin. Every program containing a list
// literal or a map literal therefore failed with "undefined function array in MIR
// call".
//
// The VM has real opcodes for this: `OpArray n` and `OpHash 2n`. A literal is not
// a call to anything, and spelling it as one invented a function that had to be
// taught to a VM it did not belong to — an obligation that would have had to be
// paid again in every backend. An instruction is the same statement in every
// backend.
type AggregateInst struct {
	Dest  Operand
	Hash  bool
	Elems []Operand
}

func (i AggregateInst) String() string {
	var parts []string
	for _, e := range i.Elems {
		parts = append(parts, e.String())
	}
	kind := "list"
	if i.Hash {
		kind = "hash"
	}
	return fmt.Sprintf("%s = %s(%s)", i.Dest.String(), kind, strings.Join(parts, ", "))
}

type UnaryOpInst struct {
	Dest  Operand
	Op    string
	Right Operand
}

func (i UnaryOpInst) String() string {
	return fmt.Sprintf("%s = %s%s", i.Dest.String(), i.Op, i.Right.String())
}

type CallInst struct {
	Dest   Operand
	Callee string
	Args   []Operand
}

func (i CallInst) String() string {
	var a []string
	for _, arg := range i.Args {
		a = append(a, arg.String())
	}
	if i.Dest != nil {
		return fmt.Sprintf("%s = call %s(%s)", i.Dest.String(), i.Callee, strings.Join(a, ", "))
	}
	return fmt.Sprintf("call %s(%s)", i.Callee, strings.Join(a, ", "))
}

type LoadVarInst struct {
	Dest Operand
	Name string
}

func (i LoadVarInst) String() string {
	return fmt.Sprintf("%s = load %s", i.Dest.String(), i.Name)
}

type StoreVarInst struct {
	Name string
	Src  Operand
}

func (i StoreVarInst) String() string {
	return fmt.Sprintf("store %s, %s", i.Name, i.Src.String())
}

// SetIndexInst is `container[key] = value`.
//
// # Why it is not a StoreVarInst
//
// A store into a container changes the container, not the binding. `a[1] = 9` has
// to reach the array that `a` names; storing into a slot would replace `a` with the
// written value, which for `h["k"] = v` is the value and for `a[1] = 9` is a
// number where a list was.
//
// It is one instruction rather than two because by this point the two spellings
// are one operation: the parser normalises `m.name = v` into a subscript whose key
// is the string `"name"`, so a flag recording which syntax the program wrote would
// be false every time and would be a lie about what the IR knows. The read side
// *does* keep the distinction — MIR carries `.` and `[]` as separate operators and
// `nil_get_field` is a separate call — because there the parser has not folded
// them. One asymmetry is less bad than an always-false field.
//
// # It produces a value
//
// The assignment is an expression in Nilang — `y = (x = 1)` is legal — so the
// instruction writes the stored value to Dest. Reading `a[1] = 9` therefore
// answers 9, which is what the interpreter and the VM answer.
type SetIndexInst struct {
	Dest      Operand
	Container Operand
	Key       Operand
	Value     Operand
}

func (i SetIndexInst) String() string {
	return fmt.Sprintf("%s = set %s[%s], %s",
		i.Dest.String(), i.Container.String(), i.Key.String(), i.Value.String())
}

// ─── TERMINATORS ────────────────────────────────────────────────────────────

type Terminator interface {
	String() string
}

type ReturnTerminator struct {
	Value Operand
}

func (r ReturnTerminator) String() string {
	if r.Value == nil {
		return "return"
	}
	return fmt.Sprintf("return %s", r.Value.String())
}

type BranchTerminator struct {
	Cond        Operand
	TrueTarget  string
	FalseTarget string
}

func (b BranchTerminator) String() string {
	return fmt.Sprintf("branch %s ? %s : %s", b.Cond.String(), b.TrueTarget, b.FalseTarget)
}

type JumpTerminator struct {
	Target string
}

func (j JumpTerminator) String() string {
	return fmt.Sprintf("jump %s", j.Target)
}

// ─── BASIC BLOCK & FUNCTION ────────────────────────────────────────────────

type BasicBlock struct {
	ID           string
	Instructions []Instruction
	Terminator   Terminator
}

func (bb *BasicBlock) String() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s:\n", bb.ID))
	for _, inst := range bb.Instructions {
		b.WriteString(fmt.Sprintf("    %s\n", inst.String()))
	}
	if bb.Terminator != nil {
		b.WriteString(fmt.Sprintf("    %s\n", bb.Terminator.String()))
	}
	return b.String()
}

type Function struct {
	Name   string
	Params []string
	Blocks []*BasicBlock

	// IsPure carries a `pure fn` declaration to the backends for the
	// runtime E0202 check.
	IsPure bool

	// Foreign marks a function this program does not define: an `extern "C"`
	// declaration resolved by the linker.
	//
	// # Why it is a flag and not a separate kind
	//
	// A foreign function has a name, parameters and a return type — everything a
	// call site needs — and differs only in that there is no body to emit. Every
	// pass that reads a function's *name* or *parameters* treats the two the same,
	// and the one pass that does not is code generation. A second node type would
	// mean every one of those passes needed a case that did the same thing.
	Foreign bool

	// Abi is the foreign calling convention, carried from the declaration so a
	// back end that supports more than one can branch on it rather than on the
	// absence of a body.
	Abi string

	// ReturnType is the written return annotation. It is not in `Params` and is
	// not a Nilang type: a C function's return is part of its signature to the
	// linker, and the backend needs it to write the declaration correctly.
	ReturnType string

	// ParamTypes is each parameter's written annotation, in order, for a foreign
	// function. Empty for one this program defines, whose parameters are whatever
	// the program passes.
	//
	// # Why the annotations rather than resolved types
	//
	// A backend needs to know how to *cross* the boundary, and that is decided by
	// the declared type: `Int` crosses as `int64_t`, `String` as a pointer and a
	// length. Resolving them to Nilang types would lose the distinction — `Any`
	// and `Int` are both "some value" on this side and different things on the
	// other — so the text the programmer wrote is what travels.
	ParamTypes []string
}

func (f *Function) String() string {
	var b strings.Builder
	if f.Foreign {
		signature := fmt.Sprintf("extern %q fn %s(%s)", f.Abi, f.Name, strings.Join(f.Params, ", "))
		if f.ReturnType != "" {
			signature += " -> " + f.ReturnType
		}
		b.WriteString(signature + ";\n")
		return b.String()
	}

	b.WriteString(fmt.Sprintf("fn %s(%s) {\n", f.Name, strings.Join(f.Params, ", ")))
	for _, bb := range f.Blocks {
		b.WriteString(bb.String())
	}
	b.WriteString("}\n")
	return b.String()
}

// ─── ENTITY DEFINITION (web-implications.md Section 26) ──────────────────────

type EntityFieldDef struct {
	Name         string
	Type         string
	IsPrimary    bool
	IsRequired   bool
	IsUnique     bool
	TargetEntity string
}

type EntityDef struct {
	Name   string
	Fields []EntityFieldDef
}

type ComponentDef struct {
	Name       string
	States     []string
	RenderFunc string
	BuildFunc  string
	Events     map[string]string
}

func (c *ComponentDef) String() string {
	return fmt.Sprintf("component %s { states=%s render=%s build=%s events=%d }\n",
		c.Name, strings.Join(c.States, ","), c.RenderFunc, c.BuildFunc, len(c.Events))
}

func (e *EntityDef) String() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("entity %s {\n", e.Name))
	for _, f := range e.Fields {
		b.WriteString(fmt.Sprintf("    %s: %s\n", f.Name, f.Type))
	}
	b.WriteString("}\n")
	return b.String()
}

type Program struct {
	Functions  map[string]*Function
	Entities   map[string]*EntityDef
	Components map[string]*ComponentDef
	Main       *Function

	// Types are the struct, enum and type-alias declarations the program made, by
	// name.
	//
	// # Why a declaration survives when its value does not
	//
	// Because a struct's *value* is a hash at run time — `P { x: 7 }` becomes
	// `hash("x", 7)` — and that is the whole of it. The declaration is not in the
	// value, so a lowerer that emitted only the value produced a program in which
	// `Point` had stopped meaning anything.
	//
	// That was fine while nothing downstream needed to know, and then three things
	// needed to at once: the FFI cannot lay out a struct it cannot see, the generated
	// C header cannot declare one, and a back end cannot reason about a field it does
	// not know exists. All three read this one map, so all three get the same answer
	// rather than three readers of the source text.
	//
	// Kept beside the program rather than folded into it: it is a *type* fact, and a
	// program is a set of instructions. Folding them together is what made the
	// information unrecoverable in the first place.
	Types map[string]*TypeDef
}

// TypeDef is one declared type: a struct, an enum, or an alias.
type TypeDef struct {
	Kind TypeKind
	Name string

	// Fields is the struct's, in declaration order.
	//
	// Order is load-bearing. A struct crossing a boundary is laid out in it, and a
	// map would make that layout depend on iteration order — which differs between
	// two runs of the same program, and between two machines.
	Fields []FieldDef

	// Variants is the enum's, in declaration order, for the same reason.
	Variants []VariantDef
}

// FieldDef is one field of a declared struct.
type FieldDef struct {
	Name string

	// TypeString is the annotation as the program wrote it; Type is what it resolved
	// to.
	//
	// Both, because they answer different questions. Something laying a struct out
	// wants the *resolved* type — `std.Int` and `Int` are one field of eight bytes —
	// while a diagnostic wants the spelling the programmer wrote.
	TypeString string
	Type       types.Type
}

// VariantDef is one case of a declared enum.
type VariantDef struct {
	Name    string
	Payload string
}

// TypeKind is which sort of declaration a TypeDef is.
type TypeKind string

const (
	KindStruct TypeKind = "struct"
	KindEnum   TypeKind = "enum"
	KindAlias  TypeKind = "alias"
)

func (p *Program) String() string {
	var b strings.Builder
	for _, ent := range p.Entities {
		b.WriteString(ent.String())
	}
	for _, component := range p.Components {
		b.WriteString(component.String())
	}
	if p.Main != nil {
		b.WriteString(p.Main.String())
	}
	for _, f := range p.Functions {
		b.WriteString(f.String())
	}
	return b.String()
}
