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

// labelFixup is a jump whose destination is not a block but a position further
// down the same instruction stream.
//
// It is what makes a *select* — `cond ? a : b` — expressible. A select cannot be
// two instructions, because only one side may be evaluated; it is a jump over the
// side that was not taken, and the destination is therefore a point in the stream
// rather than the entry of another block. Blocks cannot serve here, because an
// expression's arms live inside the instruction that holds the expression, not
// beside it.
type labelFixup struct {
	pos int
}

// Label is a position in the instruction stream that a jump can name before its
// offset is known.
//
// Naming it up front is what lets both arms of a select be written in the order
// the expression reads — true arm, false arm — while the jump that skips the true
// arm still targets the correct place. Writing the false arm first would produce
// the same bytecode and a much harder-to-read emitter.
type Label struct {
	id int
}

// tempSlotFor returns the slot a MIR temporary lives in, allocating one on first
// use.
//
// # Why the temp needs a home
//
// The VM's operand stack is a stack. A MIR temporary is an SSA-style *name*, and a
// name outlives the expression that pushed it: `_t0 = load n; _t1 = _t0 > 3;`
// pushes two values, and reading `_t0` back has to mean the first one, not
// whatever is on top. Reading a temp as "already on the stack" — which is what the
// emitter used to do — is right exactly once, for the immediately following
// instruction, and wrong for every use after it.
//
// BytecodeEmitter converts MIR program into NABC VM bytecode
type BytecodeEmitter struct {
	constants    []object.Object
	symbolTable  *compiler.SymbolTable
	instructions code.Instructions

	// temps maps a MIR temporary name to the slot holding it.
	temps map[string]int
	// nextLabel numbers the forward-jump destinations a select needs.
	nextLabel int
}

// tempSlotFor returns the slot a MIR temporary lives in, allocating one on first
// use.
//
// A temp is allocated through the symbol table rather than a counter of its own,
// which means it inherits whatever scope that table assigns — a local inside a
// function, a global in `main` — and needs no separate bookkeeping to keep the two
// apart.
func (e *BytecodeEmitter) tempSlotFor(op Operand) (int, bool) {
	t, ok := op.(TempOperand)
	if !ok {
		return 0, false
	}
	name := t.String()
	if sym, found := e.symbolTable.Resolve(name); found {
		return sym.Index, true
	}
	return e.symbolTable.Define(name).Index, true
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
		temps:        map[string]int{},
	}
}

// tempSlotFor returns the local a MIR temporary lives in, allocating one on first
// use.
//
// Allocation is by definition, which is what makes it a local rather than a
// register: a temporary's slot has to be the same every time the name appears, and
// the symbol table is already the place where a name becomes an index.
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
			Name:          fn.Name,
			IsPure:        fn.IsPure,
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

		// Global names are what make a variable readable by name. Omitting them
		// does not fail — the bytecode runs and every value is correct — it
		// leaves a debugger that can only say "slot 3", which is not a debugger
		// a person can use. A backend that leaves them blank has quietly disabled
		// a feature nobody asked it to disable.
		//
		// Source positions are a separate matter and are *not* faked here. MIR
		// instructions carry no token, so there is nothing to record; inventing
		// positions would put a breakpoint on a plausible-looking wrong line,
		// which is worse than admitting the step is unlocated. Lifting
		// positions through MIR is the open half of `L3.4`.
		GlobalNames: e.collectGlobalNames(),
	}, nil
}

// collectGlobalNames maps each global slot index to the name written for it.
//
// It reads the symbol table rather than being recorded during emission, because a
// name's slot is only known once `Define` has run, and forcing every emission site
// to report it would put the bookkeeping in the wrong place.
func (e *BytecodeEmitter) collectGlobalNames() []string {
	names := make([]string, e.symbolTable.NumDefinitions())
	for name, sym := range e.symbolTable.Names() {
		if sym.Scope != compiler.GlobalScope {
			continue
		}
		if sym.Index >= 0 && sym.Index < len(names) {
			names[sym.Index] = name
		}
	}
	return names
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
		// A temp is a *name*, and the name is a slot. Emitting a load is what
		// makes a temporary usable more than once, or after another value has
		// been pushed on top of it.
		idx, ok := e.tempSlotFor(o)
		if !ok {
			return fmt.Errorf("internal error: temporary %s has no slot", o.String())
		}
		if sym, found := e.symbolTable.Resolve(o.String()); found && sym.Scope == compiler.GlobalScope {
			e.emitOp(code.OpGetGlobal, idx)
		} else {
			e.emitOp(code.OpGetLocal, idx)
		}
	}
	return nil
}

func (e *BytecodeEmitter) emitInstruction(inst Instruction) error {
	if err := e.emitOne(inst); err != nil {
		return err
	}
	// An instruction with a destination writes its result into that destination's
	// slot. Every consumer reads an operand back out of a slot — variables,
	// temporaries and constants alike — so leaving the value on the stack as well
	// would push it twice, and the stack would drift one deep for every
	// instruction that computes something.
	return e.storeDest(inst)
}

// emitSelect compiles a two-way choice into a jump over the arm not taken.
//
// # The shape
//
//	<cond>
//	jump-not-truthy -> falseArm
//	<true arm>          ; pushes one value
//	jump -> end
//	falseArm: <false arm> ; pushes one value
//	end:
//
// Both arms push exactly one value and neither may run, which is the whole
// contract. It is why the arms are expressions rather than blocks: a block could
// leave the stack unbalanced, and the arm that did not run would leave it wrong
// for the instruction after.
//
// # Why it is patched here and not by emitBlocks
//
// `emitBlocks` backpatches to *block* offsets, recorded before a block is emitted.
// An arm is not a block — it lives inside the instruction that holds it — so its
// destination is a position in the stream that only exists once the arm has been
// written. Patching at the point of emission is what makes both arms expressible
// in the order the expression reads.
func (e *BytecodeEmitter) emitSelect(i SelectInst) error {
	if err := e.emitOperand(i.Cond); err != nil {
		return err
	}

	skipTrue := e.emitOp(code.OpJumpNotTruthy, 0)

	if err := e.emitOperand(i.True); err != nil {
		return err
	}
	jumpToEnd := e.emitOp(code.OpJump, 0)

	falseArm := len(e.instructions)
	if err := e.emitOperand(i.False); err != nil {
		return err
	}
	end := len(e.instructions)

	binary.BigEndian.PutUint16(e.instructions[skipTrue+1:], uint16(falseArm))
	binary.BigEndian.PutUint16(e.instructions[jumpToEnd+1:], uint16(end))
	return nil
}

// storeDest writes an instruction's result into its destination temp's slot.
func (e *BytecodeEmitter) storeDest(inst Instruction) error {
	var dest Operand
	switch i := inst.(type) {
	case AssignInst:
		dest = i.Dest
	case BinaryOpInst:
		dest = i.Dest
	case UnaryOpInst:
		dest = i.Dest
	case CallInst:
		dest = i.Dest
	case AggregateInst:
		dest = i.Dest
	case SelectInst:
		dest = i.Dest
	case LoadVarInst:
		// A load has a destination like any other instruction. Leaving it out is
		// what made every temp read as nil: `_t0 = load a` put the value on the
		// stack but never wrote `_t0`, so the next instruction's `OpGetLocal _t0`
		// found an empty slot.
		dest = i.Dest
	}

	if dest == nil {
		return nil
	}
	idx, ok := e.tempSlotFor(dest)
	if !ok {
		return nil
	}

	if sym, found := e.symbolTable.Resolve(dest.(TempOperand).String()); found && sym.Scope == compiler.GlobalScope {
		e.emitOp(code.OpSetGlobal, idx)
	} else {
		e.emitOp(code.OpSetLocal, idx)
	}
	return nil
}

func (e *BytecodeEmitter) emitOne(inst Instruction) error {
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
			e.emitOp(code.OpLessThan)
		case "<=":
			e.emitOp(code.OpLessThanEqual)
		case "&&":
			e.emitOp(code.OpAnd)
		case "||":
			e.emitOp(code.OpOr)
		case "[]":
			// Subscript and field access are spelled as binary operations in MIR
			// because they take two operands, but the VM has a dedicated opcode for
			// each and no arithmetic opcode for them.
			//
			// They used to fall through this switch with no case, which emitted
			// nothing: both operands stayed on the operand stack and the *index* was
			// what ended up in the destination. So `xs[1]` produced `1` and
			// `m["k"]` produced `"k"` — the right shape of answer, answering the
			// wrong question. Nothing in MIR's text showed it.
			e.emitOp(code.OpIndex)
		case ".":
			e.emitOp(code.OpGetField)
		default:
			return fmt.Errorf("operator %q has no bytecode in the MIR emitter", i.Op)
		}

	case AggregateInst:
		// Elements go on the stack in order, and the opcode's operand is the
		// count. `OpHash` takes *twice* the count because it reads key, value,
		// key, value — passing the pair count would build a hash of half its
		// entries and fail at run time with something that looks like a bug in
		// the literal rather than a miscounted operand.
		for _, elem := range i.Elems {
			if err := e.emitOperand(elem); err != nil {
				return err
			}
		}
		if i.Hash {
			e.emitOp(code.OpHash, len(i.Elems))
		} else {
			e.emitOp(code.OpArray, len(i.Elems))
		}

	case SelectInst:
		if err := e.emitSelect(i); err != nil {
			return err
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
			// `OpCall` expects `[callee, arg...]`, so the builtin is pushed
			// *before* its arguments.
			//
			// It used to be pushed after them, which left the first argument on
			// top where the callee belonged. Every builtin call then failed at run
			// time with "calling non-function and non-built-in: STRING" when the
			// first argument was a string, and with something just as confusing
			// when it was not — a message that names the argument rather than the
			// ordering, which is why the fault was not obvious from it.
			e.emitOp(code.OpGetBuiltin, sym.Index)
			for _, arg := range i.Args {
				if err := e.emitOperand(arg); err != nil {
					return err
				}
			}
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
