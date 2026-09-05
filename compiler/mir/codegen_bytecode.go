package mir

import (
	"fmt"

	"github.com/joysriramsarkar/nilLang/compiler/code"
	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/object"
)

// BytecodeEmitter converts MIR program into NABC VM bytecode
type BytecodeEmitter struct {
	constants    []object.Object
	symbolTable  *compiler.SymbolTable
	instructions code.Instructions
}

func NewBytecodeEmitter(symbolTable *compiler.SymbolTable) *BytecodeEmitter {
	if symbolTable == nil {
		symbolTable = compiler.NewSymbolTable()
		for i, v := range compiler.GetBuiltinNamesSorted() {
			symbolTable.DefineBuiltin(i, v)
		}
	}
	return &BytecodeEmitter{
		constants:    []object.Object{},
		symbolTable:  symbolTable,
		instructions: code.Instructions{},
	}
}

func (e *BytecodeEmitter) EmitProgram(prog *Program) (*compiler.Bytecode, error) {
	if prog.Main == nil {
		return nil, fmt.Errorf("MIR program has no main function")
	}

	for _, block := range prog.Main.Blocks {
		for _, inst := range block.Instructions {
			if err := e.emitInstruction(inst); err != nil {
				return nil, err
			}
		}
		if block.Terminator != nil {
			if err := e.emitTerminator(block.Terminator); err != nil {
				return nil, err
			}
		}
	}

	return &compiler.Bytecode{
		Instructions: e.instructions,
		Constants:    e.constants,
	}, nil
}

func (e *BytecodeEmitter) addConstant(obj object.Object) int {
	e.constants = append(e.constants, obj)
	return len(e.constants) - 1
}

func (e *BytecodeEmitter) emitOp(op code.Opcode, operands ...int) int {
	ins := code.Make(op, operands...)
	pos := len(e.instructions)
	e.instructions = append(e.instructions, ins...)
	return pos
}

func (e *BytecodeEmitter) emitOperand(op Operand) error {
	switch o := op.(type) {
	case ConstOperand:
		switch v := o.Value.(type) {
		case int64:
			idx := e.addConstant(&object.Integer{Value: v})
			e.emitOp(code.OpConstant, idx)
		case float64:
			idx := e.addConstant(&object.Float{Value: v})
			e.emitOp(code.OpConstant, idx)
		case string:
			idx := e.addConstant(&object.String{Value: v})
			e.emitOp(code.OpConstant, idx)
		case bool:
			if v {
				e.emitOp(code.OpTrue)
			} else {
				e.emitOp(code.OpFalse)
			}
		case nil:
			e.emitOp(code.OpNull)
		}
	case VarOperand:
		sym, ok := e.symbolTable.Resolve(o.Name)
		if !ok {
			sym = e.symbolTable.Define(o.Name)
		}
		if sym.Scope == compiler.GlobalScope {
			e.emitOp(code.OpGetGlobal, sym.Index)
		} else {
			e.emitOp(code.OpGetLocal, sym.Index)
		}
	case TempOperand:
		// Temporaries are held on evaluation stack
	}
	return nil
}

func (e *BytecodeEmitter) emitInstruction(inst Instruction) error {
	switch i := inst.(type) {
	case StoreVarInst:
		if err := e.emitOperand(i.Src); err != nil {
			return err
		}
		sym, ok := e.symbolTable.Resolve(i.Name)
		if !ok {
			sym = e.symbolTable.Define(i.Name)
		}
		if sym.Scope == compiler.GlobalScope {
			e.emitOp(code.OpSetGlobal, sym.Index)
		} else {
			e.emitOp(code.OpSetLocal, sym.Index)
		}

	case LoadVarInst:
		sym, ok := e.symbolTable.Resolve(i.Name)
		if !ok {
			sym = e.symbolTable.Define(i.Name)
		}
		if sym.Scope == compiler.GlobalScope {
			e.emitOp(code.OpGetGlobal, sym.Index)
		} else {
			e.emitOp(code.OpGetLocal, sym.Index)
		}

	case BinaryOpInst:
		if err := e.emitOperand(i.Left); err != nil {
			return err
		}
		if err := e.emitOperand(i.Right); err != nil {
			return err
		}
		switch i.Op {
		case "+":
			e.emitOp(code.OpAdd)
		case "-":
			e.emitOp(code.OpSub)
		case "*":
			e.emitOp(code.OpMul)
		case "/":
			e.emitOp(code.OpDiv)
		case "%":
			e.emitOp(code.OpMod)
		case "==":
			e.emitOp(code.OpEqual)
		case "!=":
			e.emitOp(code.OpNotEqual)
		case ">":
			e.emitOp(code.OpGreaterThan)
		}

	case UnaryOpInst:
		if err := e.emitOperand(i.Right); err != nil {
			return err
		}
		switch i.Op {
		case "-":
			e.emitOp(code.OpMinus)
		case "!":
			e.emitOp(code.OpBang)
		}

	case CallInst:
		// Push function
		sym, ok := e.symbolTable.Resolve(i.Callee)
		if !ok {
			return fmt.Errorf("undefined function %s in MIR call", i.Callee)
		}
		if sym.Scope == compiler.BuiltinScope {
			for _, arg := range i.Args {
				if err := e.emitOperand(arg); err != nil {
					return err
				}
			}
			e.emitOp(code.OpCall, len(i.Args))
		}
	}
	return nil
}

func (e *BytecodeEmitter) emitTerminator(term Terminator) error {
	switch t := term.(type) {
	case ReturnTerminator:
		if t.Value != nil {
			if err := e.emitOperand(t.Value); err != nil {
				return err
			}
		}
	}
	return nil
}
