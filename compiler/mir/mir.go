package mir

import (
	"fmt"
	"strings"
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
}

func (f *Function) String() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("fn %s(%s) {\n", f.Name, strings.Join(f.Params, ", ")))
	for _, bb := range f.Blocks {
		b.WriteString(bb.String())
	}
	b.WriteString("}\n")
	return b.String()
}

type Program struct {
	Functions map[string]*Function
	Main      *Function
}

func (p *Program) String() string {
	var b strings.Builder
	if p.Main != nil {
		b.WriteString(p.Main.String())
	}
	for _, f := range p.Functions {
		b.WriteString(f.String())
	}
	return b.String()
}
