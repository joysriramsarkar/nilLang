package code

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type Instructions []byte

func (ins Instructions) String() string {
	var out bytes.Buffer

	i := 0
	for i < len(ins) {
		def, err := Lookup(ins[i])
		if err != nil {
			fmt.Fprintf(&out, "ERROR: %s\n", err)
			continue
		}

		operands, read := ReadOperands(def, ins[i+1:])

		fmt.Fprintf(&out, "%04d %s\n", i, ins.fmtInstruction(def, operands))

		i += 1 + read
	}

	return out.String()
}

func (ins Instructions) fmtInstruction(def *Definition, operands []int) string {
	operandCount := len(def.OperandWidths)

	if len(operands) != operandCount {
		return fmt.Sprintf("ERROR: operand len %d does not match defined %d\n",
			len(operands), operandCount)
	}

	switch operandCount {
	case 0:
		return def.Name
	case 1:
		return fmt.Sprintf("%s %d", def.Name, operands[0])
	case 2:
		return fmt.Sprintf("%s %d %d", def.Name, operands[0], operands[1])
	}

	return fmt.Sprintf("ERROR: unhandled operandCount for %s\n", def.Name)
}

type Opcode byte

const (
	OpConstant Opcode = iota
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpMod
	OpPop
	OpTrue
	OpFalse
	OpEqual
	OpNotEqual
	OpGreaterThan
	OpGreaterThanEqual
	OpLessThan
	OpLessThanEqual
	OpMinus
	OpBang
	OpJumpNotTruthy
	OpJump
	OpNull
	OpSetGlobal
	OpGetGlobal
	OpArray
	OpHash
	OpIndex
	// OpGetField reads `x.k`; OpIndex reads `x[k]`.
	//
	// They are separate instructions because they answer different questions.
	// Indexing asks "what is at this key", and a key that is not there has the
	// answer null. Dot access asks for a named member, and a member that does not
	// exist is a mistake worth reporting: silently answering null turns a typo in
	// a field name into a null that surfaces somewhere else entirely.
	//
	// The two used to compile to the same instruction, which is how the VM came
	// to answer null for a dot access the evaluator refused.
	OpGetField
	OpCall
	OpReturnValue
	OpReturn
	OpSetLocal
	OpGetLocal
	OpGetBuiltin
	OpClosure
	OpGetFree     // read cell.Value — normal execution path
	OpGetFreeCell // push raw *CaptureCell pointer — used only during closure creation
	OpSetFree     // write back to a shared mutable capture cell

	// OpBoxLocal turns a local slot into a shared *CaptureCell, in place.
	//
	// Mutable capture by reference needs the *enclosing* frame and the closure
	// to see the same cell. Boxing in place achieves that: the local slot stops
	// holding the value and starts holding the cell, so every later read and
	// write in the enclosing frame goes through it too.
	OpBoxLocal
	OpCurrentClosure
	OpToString
	OpStringConcat
	OpNativeCall
	OpAnd
	OpOr
	OpTask
	OpAwait
	OpSetIndex

	// OpSetLocalFresh / OpSetGlobalFresh store a `let` (re-)declaration
	// inside a loop body.
	//
	// A loop re-executes its body, so each pass declares a fresh binding
	// (SEMANTICS.md §4.2 rule 2: a closure created in the body captures that
	// pass's value). A plain store would write through the shared cell and
	// merge every pass into one binding. The Fresh variants replace the cell
	// when the slot already holds one, and store plainly otherwise — so
	// passes without any capture pay one type assertion and nothing else.
	OpSetLocalFresh
	OpSetGlobalFresh

	// OpBoxGlobal is OpBoxLocal for the globals array: wrap the slot in a
	// shared *CaptureCell in place (idempotently) and push the cell. Only
	// top-level loop-body bindings (Symbol.LoopFresh) ever take this path.
	OpBoxGlobal
)

type Definition struct {
	Name          string
	OperandWidths []int
}

var definitions = map[Opcode]*Definition{
	OpConstant:         {"OpConstant", []int{2}},
	OpAdd:              {"OpAdd", []int{}},
	OpSub:              {"OpSub", []int{}},
	OpMul:              {"OpMul", []int{}},
	OpDiv:              {"OpDiv", []int{}},
	OpMod:              {"OpMod", []int{}},
	OpPop:              {"OpPop", []int{}},
	OpTrue:             {"OpTrue", []int{}},
	OpFalse:            {"OpFalse", []int{}},
	OpEqual:            {"OpEqual", []int{}},
	OpNotEqual:         {"OpNotEqual", []int{}},
	OpGreaterThan:      {"OpGreaterThan", []int{}},
	OpGreaterThanEqual: {"OpGreaterThanEqual", []int{}},
	OpLessThan:         {"OpLessThan", []int{}},
	OpLessThanEqual:    {"OpLessThanEqual", []int{}},
	OpMinus:            {"OpMinus", []int{}},
	OpBang:             {"OpBang", []int{}},
	OpJumpNotTruthy:    {"OpJumpNotTruthy", []int{2}},
	OpJump:             {"OpJump", []int{2}},
	OpNull:             {"OpNull", []int{}},
	OpSetGlobal:        {"OpSetGlobal", []int{2}},
	OpGetGlobal:        {"OpGetGlobal", []int{2}},
	OpArray:            {"OpArray", []int{2}},
	OpHash:             {"OpHash", []int{2}},
	OpIndex:            {"OpIndex", []int{}},
	OpGetField:         {"OpGetField", []int{}},
	OpCall:             {"OpCall", []int{1}},
	OpReturnValue:      {"OpReturnValue", []int{}},
	OpReturn:           {"OpReturn", []int{}},
	OpSetLocal:         {"OpSetLocal", []int{1}},
	OpGetLocal:         {"OpGetLocal", []int{1}},
	OpGetBuiltin:       {"OpGetBuiltin", []int{1}},
	OpClosure:          {"OpClosure", []int{2, 1}},
	OpGetFree:          {"OpGetFree", []int{1}},
	OpGetFreeCell:      {"OpGetFreeCell", []int{1}},
	OpSetFree:          {"OpSetFree", []int{1}},
	OpBoxLocal:         {"OpBoxLocal", []int{1}},
	OpCurrentClosure:   {"OpCurrentClosure", []int{}},
	OpToString:         {"OpToString", []int{}},
	OpStringConcat:     {"OpStringConcat", []int{}},
	OpNativeCall:       {"OpNativeCall", []int{2, 1}},
	OpAnd:              {"OpAnd", []int{}},
	OpOr:               {"OpOr", []int{}},
	OpTask:             {"OpTask", []int{}},
	OpAwait:            {"OpAwait", []int{}},
	OpSetIndex:         {"OpSetIndex", []int{}},
	OpSetLocalFresh:    {"OpSetLocalFresh", []int{1}},
	OpSetGlobalFresh:   {"OpSetGlobalFresh", []int{2}},
	OpBoxGlobal:        {"OpBoxGlobal", []int{2}},
}

func Lookup(op byte) (*Definition, error) {
	def, ok := definitions[Opcode(op)]
	if !ok {
		return nil, fmt.Errorf("opcode %d undefined", op)
	}
	return def, nil
}

func Make(op Opcode, operands ...int) []byte {
	def, ok := definitions[op]
	if !ok {
		return []byte{}
	}

	instructionLen := 1
	for _, w := range def.OperandWidths {
		instructionLen += w
	}

	instruction := make([]byte, instructionLen)
	instruction[0] = byte(op)

	offset := 1
	for i, o := range operands {
		width := def.OperandWidths[i]
		switch width {
		case 1:
			instruction[offset] = byte(o)
		case 2:
			binary.BigEndian.PutUint16(instruction[offset:], uint16(o))
		}
		offset += width
	}

	return instruction
}

func ReadOperands(def *Definition, ins Instructions) ([]int, int) {
	operands := make([]int, len(def.OperandWidths))
	offset := 0

	for i, width := range def.OperandWidths {
		switch width {
		case 1:
			operands[i] = int(ReadUint8(ins[offset:]))
		case 2:
			operands[i] = int(ReadUint16(ins[offset:]))
		}
		offset += width
	}

	return operands, offset
}

func ReadUint16(ins Instructions) uint16 {
	return binary.BigEndian.Uint16(ins)
}

func ReadUint8(ins Instructions) uint8 {
	return uint8(ins[0])
}
