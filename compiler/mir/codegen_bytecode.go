package mir

import (
	"encoding/binary"
	"fmt"

	"github.com/joysriramsarkar/nilLang/compiler/code"
	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/object"
)

type jumpFixup struct {
	pos    int
	target string
}

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

	// Step 1: Emit any standalone functions into constants
	for name, fn := range prog.Functions {
		subEmitter := &BytecodeEmitter{
			constants:    e.constants,
			symbolTable:  compiler.NewEnclosedSymbolTable(e.symbolTable),
			instructions: code.Instructions{},
		}
		for _, param := range fn.Params {
			subEmitter.symbolTable.Define(param)
		}

		if err := subEmitter.emitBlocks(fn.Blocks); err != nil {
			return nil, err
		}

		compiledFn := &object.CompiledFunction{
			Instructions:  subEmitter.instructions,
			NumLocals:     subEmitter.symbolTable.NumDefinitions(),
			NumParameters: len(fn.Params),
		}

		e.constants = subEmitter.constants
		fnIdx := e.addConstant(compiledFn)
		sym := e.symbolTable.Define(name)
		if sym.Scope == compiler.GlobalScope {
			e.emitOp(code.OpClosure, fnIdx, 0)
			e.emitOp(code.OpSetGlobal, sym.Index)
		}
	}

	// Step 2: Emit main function blocks with jump backpatching
	if err := e.emitBlocks(prog.Main.Blocks); err != nil {
		return nil, err
	}

	// Ensure final instruction returns
	if len(e.instructions) == 0 || (e.instructions[len(e.instructions)-1] != byte(code.OpReturnValue) && e.instructions[len(e.instructions)-1] != byte(code.OpReturn)) {
		e.emitOp(code.OpReturn)
	}

	return &compiler.Bytecode{
		Instructions: e.instructions,
		Constants:    e.constants,
	}, nil
}

func (e *BytecodeEmitter) emitBlocks(blocks []*BasicBlock) error {
	blockOffsets := make(map[string]int)
	var fixups []jumpFixup

	// Pass 1: Emit instructions and record block entry offsets
	for _, block := range blocks {
		blockOffsets[block.ID] = len(e.instructions)

		for _, inst := range block.Instructions {
			if err := e.emitInstruction(inst); err != nil {
				return err
			}
		}

		if block.Terminator != nil {
			if err := e.emitTerminator(block.Terminator, &fixups); err != nil {
				return err
			}
		}
	}

	// Pass 2: Backpatch jump targets
	for _, f := range fixups {
		targetOffset, ok := blockOffsets[f.target]
		if !ok {
			return fmt.Errorf("unknown branch target block: %s", f.target)
		}
		binary.BigEndian.PutUint16(e.instructions[f.pos+1:], uint16(targetOffset))
	}

	return nil
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
		// Temporary values already reside at top of VM operand stack
	}
	return nil
}

func (e *BytecodeEmitter) emitInstruction(inst Instruction) error {
	switch i := inst.(type) {
	case AssignInst:
		if err := e.emitOperand(i.Src); err != nil {
			return err
		}

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
		case ">=":
			e.emitOp(code.OpGreaterThanEqual)
		case "<":
			// < is reversed >
			e.emitOp(code.OpGreaterThan)
		case "<=":
			e.emitOp(code.OpGreaterThanEqual)
		case "&&":
			e.emitOp(code.OpAnd)
		case "||":
			e.emitOp(code.OpOr)
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
		sym, ok := e.symbolTable.Resolve(i.Callee)
		if ok && sym.Scope == compiler.BuiltinScope {
			for _, arg := range i.Args {
				if err := e.emitOperand(arg); err != nil {
					return err
				}
			}
			e.emitOp(code.OpGetBuiltin, sym.Index)
			e.emitOp(code.OpCall, len(i.Args))
		} else if ok {
			// User defined function call
			if sym.Scope == compiler.GlobalScope {
				e.emitOp(code.OpGetGlobal, sym.Index)
			} else {
				e.emitOp(code.OpGetLocal, sym.Index)
			}
			for _, arg := range i.Args {
				if err := e.emitOperand(arg); err != nil {
					return err
				}
			}
			e.emitOp(code.OpCall, len(i.Args))
		} else {
			return fmt.Errorf("undefined function %s in MIR call", i.Callee)
		}
	}
	return nil
}

func (e *BytecodeEmitter) emitTerminator(term Terminator, fixups *[]jumpFixup) error {
	switch t := term.(type) {
	case ReturnTerminator:
		if t.Value != nil {
			if err := e.emitOperand(t.Value); err != nil {
				return err
			}
			e.emitOp(code.OpReturnValue)
		} else {
			e.emitOp(code.OpReturn)
		}

	case JumpTerminator:
		pos := e.emitOp(code.OpJump, 9999)
		*fixups = append(*fixups, jumpFixup{pos: pos, target: t.Target})

	case BranchTerminator:
		if err := e.emitOperand(t.Cond); err != nil {
			return err
		}
		// If condition is false, jump to FalseTarget
		posFalse := e.emitOp(code.OpJumpNotTruthy, 9999)
		*fixups = append(*fixups, jumpFixup{pos: posFalse, target: t.FalseTarget})

		// Otherwise, jump to TrueTarget
		posTrue := e.emitOp(code.OpJump, 9999)
		*fixups = append(*fixups, jumpFixup{pos: posTrue, target: t.TrueTarget})
	}
	return nil
}
