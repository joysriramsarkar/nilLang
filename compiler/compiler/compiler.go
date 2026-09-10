package compiler

import (
	"fmt"
	"sort"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/code"
	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/object"
)

type EmittedInstruction struct {
	Opcode   code.Opcode
	Position int
}

type CompilationScope struct {
	instructions        code.Instructions
	lastInstruction     EmittedInstruction
	previousInstruction EmittedInstruction
}

type Bytecode struct {
	Instructions code.Instructions
	Constants    []object.Object
}

type Compiler struct {
	constants   []object.Object
	symbolTable *SymbolTable

	scopes     []CompilationScope
	scopeIndex int
}

func New() *Compiler {
	mainScope := CompilationScope{
		instructions:        code.Instructions{},
		lastInstruction:     EmittedInstruction{},
		previousInstruction: EmittedInstruction{},
	}

	symbolTable := NewSymbolTable()

	for i, v := range getBuiltinNames() {
		symbolTable.DefineBuiltin(i, v)
	}

	return &Compiler{
		constants:   []object.Object{},
		symbolTable: symbolTable,
		scopes:      []CompilationScope{mainScope},
		scopeIndex:  0,
	}
}

func getBuiltinNames() []string {
	names := make([]string, 0, len(evaluator.Builtins))
	for k := range evaluator.Builtins {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func GetBuiltinNamesSorted() []string {
	return getBuiltinNames()
}

func NewWithState(s *SymbolTable, constants []object.Object) *Compiler {
	compiler := New()
	compiler.symbolTable = s
	compiler.constants = constants
	return compiler
}

func (c *Compiler) Compile(node ast.Node) error {
	switch node := node.(type) {
	case *ast.Program:
		for _, s := range node.Statements {
			err := c.Compile(s)
			if err != nil {
				return err
			}
		}

	case *ast.AppStatement:
		return c.Compile(node.Body)

	case *ast.ExpressionStatement:
		err := c.Compile(node.Expression)
		if err != nil {
			return err
		}
		c.emit(code.OpPop)

	case *ast.InfixExpression:
		if node.Operator == "<" || node.Operator == "<=" {
			err := c.Compile(node.Right)
			if err != nil {
				return err
			}
			err = c.Compile(node.Left)
			if err != nil {
				return err
			}
			if node.Operator == "<" {
				c.emit(code.OpGreaterThan)
			} else {
				c.emit(code.OpGreaterThanEqual)
			}
			return nil
		}

		err := c.Compile(node.Left)
		if err != nil {
			return err
		}

		err = c.Compile(node.Right)
		if err != nil {
			return err
		}

		switch node.Operator {
		case "+":
			c.emit(code.OpAdd)
		case "-":
			c.emit(code.OpSub)
		case "*":
			c.emit(code.OpMul)
		case "/":
			c.emit(code.OpDiv)
		case "%":
			c.emit(code.OpMod)
		case ">":
			c.emit(code.OpGreaterThan)
		case ">=":
			c.emit(code.OpGreaterThanEqual)
		case "==":
			c.emit(code.OpEqual)
		case "!=":
			c.emit(code.OpNotEqual)
		case "&&":
			c.emit(code.OpAnd)
		case "||":
			c.emit(code.OpOr)
		default:
			return fmt.Errorf("unknown operator %s", node.Operator)
		}

	case *ast.PrefixExpression:
		err := c.Compile(node.Right)
		if err != nil {
			return err
		}

		switch node.Operator {
		case "!":
			c.emit(code.OpBang)
		case "-":
			c.emit(code.OpMinus)
		default:
			return fmt.Errorf("unknown operator %s", node.Operator)
		}

	case *ast.IntegerLiteral:
		integer := &object.Integer{Value: node.Value}
		c.emit(code.OpConstant, c.addConstant(integer))

	case *ast.FloatLiteral:
		flt := &object.Float{Value: node.Value}
		c.emit(code.OpConstant, c.addConstant(flt))

	case *ast.Boolean:
		if node.Value {
			c.emit(code.OpTrue)
		} else {
			c.emit(code.OpFalse)
		}

	case *ast.NullLiteral:
		c.emit(code.OpNull)

	case *ast.StringLiteral:
		str := &object.String{Value: node.Value}
		c.emit(code.OpConstant, c.addConstant(str))

	case *ast.StringTemplate:
		for i, part := range node.Parts {
			if part.IsExpression {
				err := c.Compile(part.Expression)
				if err != nil {
					return err
				}
				c.emit(code.OpToString)
			} else {
				str := &object.String{Value: part.Literal}
				c.emit(code.OpConstant, c.addConstant(str))
			}
			if i > 0 {
				c.emit(code.OpStringConcat)
			}
		}

	case *ast.IfExpression:
		err := c.Compile(node.Condition)
		if err != nil {
			return err
		}

		// Emit OpJumpNotTruthy with placeholder offset
		jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

		err = c.Compile(node.Consequence)
		if err != nil {
			return err
		}

		if c.lastInstructionIs(code.OpPop) {
			c.removeLastPop()
		} else if !c.lastInstructionIs(code.OpReturnValue) && !c.lastInstructionIs(code.OpReturn) {
			c.emit(code.OpNull)
		}

		// Emit OpJump with placeholder offset
		jumpPos := c.emit(code.OpJump, 9999)

		afterConsequencePos := len(c.currentInstructions())
		c.changeOperand(jumpNotTruthyPos, afterConsequencePos)

		if node.Alternative == nil {
			c.emit(code.OpNull)
		} else {
			err := c.Compile(node.Alternative)
			if err != nil {
				return err
			}

			if c.lastInstructionIs(code.OpPop) {
				c.removeLastPop()
			} else if !c.lastInstructionIs(code.OpReturnValue) && !c.lastInstructionIs(code.OpReturn) {
				c.emit(code.OpNull)
			}
		}

		afterAlternativePos := len(c.currentInstructions())
		c.changeOperand(jumpPos, afterAlternativePos)

	case *ast.WhileStatement:
		conditionStart := len(c.currentInstructions())

		err := c.Compile(node.Condition)
		if err != nil {
			return err
		}

		jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

		err = c.Compile(node.Body)
		if err != nil {
			return err
		}

		// Jump back to condition
		c.emit(code.OpJump, conditionStart)

		afterBodyPos := len(c.currentInstructions())
		c.changeOperand(jumpNotTruthyPos, afterBodyPos)
		c.emit(code.OpNull)

	case *ast.BlockStatement:
		for _, s := range node.Statements {
			err := c.Compile(s)
			if err != nil {
				return err
			}
		}

	case *ast.LetStatement:
		var symbol Symbol
		if node.Constant {
			symbol = c.symbolTable.DefineConst(node.Name.Value)
		} else {
			symbol = c.symbolTable.Define(node.Name.Value)
		}
		if node.Value == nil {
			return fmt.Errorf("variable %s requires an initializer", node.Name.Value)
		}
		err := c.Compile(node.Value)
		if err != nil {
			return err
		}

		if symbol.Scope == GlobalScope {
			c.emit(code.OpSetGlobal, symbol.Index)
		} else {
			c.emit(code.OpSetLocal, symbol.Index)
		}

	case *ast.StateDeclaration:
		symbol := c.symbolTable.Define(node.Name.Value)
		if node.Value == nil {
			c.emit(code.OpNull)
		} else if err := c.Compile(node.Value); err != nil {
			return err
		}
		if symbol.Scope == GlobalScope {
			c.emit(code.OpSetGlobal, symbol.Index)
		} else {
			c.emit(code.OpSetLocal, symbol.Index)
		}

	case *ast.ComponentLiteral:
		if node.Name == nil {
			return fmt.Errorf("component declaration requires a name")
		}
		for _, state := range node.States {
			if err := c.Compile(state); err != nil {
				return err
			}
		}
		componentSymbol := c.symbolTable.Define(node.Name.Value)
		pairCount := 0
		emitKey := func(key string) {
			c.emit(code.OpConstant, c.addConstant(&object.String{Value: key}))
		}
		emitKey("__type")
		c.emit(code.OpConstant, c.addConstant(&object.String{Value: "Component"}))
		pairCount++
		emitKey("name")
		c.emit(code.OpConstant, c.addConstant(&object.String{Value: node.Name.Value}))
		pairCount++
		emitKey("state")
		for _, state := range node.States {
			emitKey(state.Name.Value)
			symbol, _ := c.symbolTable.Resolve(state.Name.Value)
			c.loadSymbol(symbol)
		}
		c.emit(code.OpHash, len(node.States)*2)
		pairCount++
		if node.Render != nil && node.Render.Body != nil {
			emitKey("render")
			if err := c.Compile(&ast.FunctionLiteral{Token: node.Render.Token, Body: node.Render.Body}); err != nil {
				return err
			}
			pairCount++
		}
		if node.Build != nil && node.Build.Body != nil {
			emitKey("build")
			if err := c.Compile(&ast.FunctionLiteral{Token: node.Build.Token, Body: node.Build.Body}); err != nil {
				return err
			}
			pairCount++
		}
		emitKey("events")
		for _, handler := range node.Handlers {
			emitKey(handler.Event.Value)
			if err := c.Compile(&ast.FunctionLiteral{Token: handler.Token, Parameters: handler.Parameters, Body: handler.Body}); err != nil {
				return err
			}
		}
		c.emit(code.OpHash, len(node.Handlers)*2)
		pairCount++
		c.emit(code.OpHash, pairCount*2)
		if componentSymbol.Scope == GlobalScope {
			c.emit(code.OpSetGlobal, componentSymbol.Index)
		} else {
			c.emit(code.OpSetLocal, componentSymbol.Index)
		}

	case *ast.AssignStatement:
		symbol, ok := c.symbolTable.Resolve(node.Name.Value)
		if !ok {
			return fmt.Errorf("undefined variable %s", node.Name.Value)
		}
		if symbol.Constant {
			return fmt.Errorf("cannot assign to constant %s", node.Name.Value)
		}
		if node.Operator == "+=" || node.Operator == "-=" {
			c.loadSymbol(symbol)
		}
		if err := c.Compile(node.Value); err != nil {
			return err
		}
		switch node.Operator {
		case "+=":
			c.emit(code.OpAdd)
		case "-=":
			c.emit(code.OpSub)
		}
		// Emit the correct store opcode for each variable scope.
		// FreeScope means the variable lives in an outer closure's
		// CaptureCell — we must write back through the cell pointer.
		switch symbol.Scope {
		case GlobalScope:
			c.emit(code.OpSetGlobal, symbol.Index)
		case LocalScope:
			c.emit(code.OpSetLocal, symbol.Index)
		case FreeScope:
			c.emit(code.OpSetFree, symbol.Index)
		}

	case *ast.Identifier:
		symbol, ok := c.symbolTable.Resolve(node.Value)
		if !ok {
			return fmt.Errorf("undefined variable %s", node.Value)
		}

		c.loadSymbol(symbol)

	case *ast.ArrayLiteral:
		for _, el := range node.Elements {
			err := c.Compile(el)
			if err != nil {
				return err
			}
		}

		c.emit(code.OpArray, len(node.Elements))

	case *ast.HashLiteral:
		keys := []ast.Expression{}
		for k := range node.Pairs {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].String() < keys[j].String()
		})

		for _, k := range keys {
			err := c.Compile(k)
			if err != nil {
				return err
			}
			err = c.Compile(node.Pairs[k])
			if err != nil {
				return err
			}
		}

		c.emit(code.OpHash, len(node.Pairs)*2)

	case *ast.IndexExpression:
		err := c.Compile(node.Left)
		if err != nil {
			return err
		}

		err = c.Compile(node.Index)
		if err != nil {
			return err
		}

		c.emit(code.OpIndex)

	case *ast.DotExpression:
		if err := c.Compile(node.Left); err != nil {
			return err
		}
		c.emit(code.OpConstant, c.addConstant(&object.String{Value: node.Member.Value}))
		c.emit(code.OpIndex)

	case *ast.FunctionLiteral:
		c.enterScope()

		if node.Name != "" {
			c.symbolTable.DefineFunctionName(node.Name)
		}

		for _, p := range node.Parameters {
			c.symbolTable.Define(p.Value)
		}

		err := c.Compile(node.Body)
		if err != nil {
			return err
		}

		if c.lastInstructionIs(code.OpPop) {
			c.replaceLastPopWithReturn()
		}
		if !c.lastInstructionIs(code.OpReturnValue) {
			c.emit(code.OpReturn)
		}

		freeSymbols := c.symbolTable.FreeSymbols
		numLocals := c.symbolTable.numDefinitions
		instructions := c.leaveScope()

		// When capturing free variables for a new closure, we must push the
		// raw CaptureCell pointer (not the unwrapped value) so the VM can
		// share the cell between parent and child closures. For non-free
		// (local/global) captures we push the value and the VM wraps it in a
		// new cell inside pushClosure.
		for _, s := range freeSymbols {
			c.loadSymbolForCapture(s)
		}

		compiledFn := &object.CompiledFunction{
			Instructions:  instructions,
			NumLocals:     numLocals,
			NumParameters: len(node.Parameters),
		}

		fnIndex := c.addConstant(compiledFn)
		c.emit(code.OpClosure, fnIndex, len(freeSymbols))

		if node.Name != "" {
			symbol := c.symbolTable.Define(node.Name)
			if symbol.Scope == GlobalScope {
				c.emit(code.OpSetGlobal, symbol.Index)
				c.emit(code.OpGetGlobal, symbol.Index)
			} else {
				c.emit(code.OpSetLocal, symbol.Index)
				c.emit(code.OpGetLocal, symbol.Index)
			}
		}

	case *ast.TaskExpression:
		function := &ast.FunctionLiteral{Token: node.Token, Parameters: []*ast.Identifier{}, Body: node.Body}
		if err := c.Compile(function); err != nil {
			return err
		}
		c.emit(code.OpTask)

	case *ast.AwaitExpression:
		if err := c.Compile(node.Right); err != nil {
			return err
		}
		c.emit(code.OpAwait)

	case *ast.ReturnStatement:
		err := c.Compile(node.ReturnValue)
		if err != nil {
			return err
		}

		c.emit(code.OpReturnValue)

	case *ast.CallExpression:
		err := c.Compile(node.Function)
		if err != nil {
			return err
		}

		for _, a := range node.Arguments {
			err := c.Compile(a)
			if err != nil {
				return err
			}
		}

		c.emit(code.OpCall, len(node.Arguments))
	}

	return nil
}

func (c *Compiler) loadSymbol(s Symbol) {
	switch s.Scope {
	case GlobalScope:
		c.emit(code.OpGetGlobal, s.Index)
	case LocalScope:
		c.emit(code.OpGetLocal, s.Index)
	case BuiltinScope:
		c.emit(code.OpGetBuiltin, s.Index)
	case FreeScope:
		// OpGetFree reads the cell's .Value. During closure creation we
		// need the raw cell pointer so the inner closure can share it.
		// We always emit OpGetFree here; the pushClosure logic in the VM
		// detects whether the stack value is already a CaptureCell and
		// re-uses it, avoiding double-wrapping.
		c.emit(code.OpGetFree, s.Index)
	case FunctionScope:
		c.emit(code.OpCurrentClosure)
	}
}

// loadSymbolForCapture is used only when building the free-variable list
// for a new OpClosure instruction. For free-scoped symbols we need to push
// the raw CaptureCell pointer (not the unwrapped value) so the inner closure
// can share mutation with the outer one. The VM's pushClosure detects the
// CaptureCell type and re-uses the pointer instead of wrapping a new cell.
func (c *Compiler) loadSymbolForCapture(s Symbol) {
	if s.Scope == FreeScope {
		// OpGetFreeCell pushes the raw *CaptureCell (not .Value).
		// The VM's pushClosure will then detect it's already a cell and
		// share the pointer, giving both closures the same mutable cell.
		c.emit(code.OpGetFreeCell, s.Index)
	} else {
		// For locals/globals: push the current value; VM wraps a new cell.
		c.loadSymbol(s)
	}
}

func (c *Compiler) Bytecode() *Bytecode {
	return &Bytecode{
		Instructions: c.currentInstructions(),
		Constants:    c.constants,
	}
}

func (c *Compiler) addConstant(obj object.Object) int {
	c.constants = append(c.constants, obj)
	return len(c.constants) - 1
}

func (c *Compiler) emit(op code.Opcode, operands ...int) int {
	ins := code.Make(op, operands...)
	pos := c.addInstruction(ins)

	c.setLastInstruction(op, pos)

	return pos
}

func (c *Compiler) addInstruction(ins []byte) int {
	posNewInstruction := len(c.currentInstructions())
	updatedInstructions := append(c.currentInstructions(), ins...)

	c.scopes[c.scopeIndex].instructions = updatedInstructions

	return posNewInstruction
}

func (c *Compiler) setLastInstruction(op code.Opcode, pos int) {
	previous := c.scopes[c.scopeIndex].lastInstruction
	last := EmittedInstruction{Opcode: op, Position: pos}

	c.scopes[c.scopeIndex].previousInstruction = previous
	c.scopes[c.scopeIndex].lastInstruction = last
}

func (c *Compiler) lastInstructionIs(op code.Opcode) bool {
	if len(c.currentInstructions()) == 0 {
		return false
	}

	return c.scopes[c.scopeIndex].lastInstruction.Opcode == op
}

func (c *Compiler) removeLastPop() {
	last := c.scopes[c.scopeIndex].lastInstruction
	previous := c.scopes[c.scopeIndex].previousInstruction

	old := c.currentInstructions()
	c.scopes[c.scopeIndex].instructions = old[:last.Position]
	c.scopes[c.scopeIndex].lastInstruction = previous
}

func (c *Compiler) replaceInstruction(pos int, newInstruction []byte) {
	ins := c.currentInstructions()

	for i := 0; i < len(newInstruction); i++ {
		ins[pos+i] = newInstruction[i]
	}
}

func (c *Compiler) changeOperand(opPos int, operand int) {
	op := code.Opcode(c.currentInstructions()[opPos])
	newInstruction := code.Make(op, operand)

	c.replaceInstruction(opPos, newInstruction)
}

func (c *Compiler) currentInstructions() code.Instructions {
	return c.scopes[c.scopeIndex].instructions
}

func (c *Compiler) enterScope() {
	scope := CompilationScope{
		instructions:        code.Instructions{},
		lastInstruction:     EmittedInstruction{},
		previousInstruction: EmittedInstruction{},
	}
	c.scopes = append(c.scopes, scope)
	c.scopeIndex++
	c.symbolTable = NewEnclosedSymbolTable(c.symbolTable)
}

func (c *Compiler) leaveScope() code.Instructions {
	instructions := c.currentInstructions()

	c.scopes = c.scopes[:len(c.scopes)-1]
	c.scopeIndex--
	c.symbolTable = c.symbolTable.Outer

	return instructions
}

func (c *Compiler) replaceLastPopWithReturn() {
	lastPos := c.scopes[c.scopeIndex].lastInstruction.Position
	c.replaceInstruction(lastPos, code.Make(code.OpReturnValue))

	c.scopes[c.scopeIndex].lastInstruction.Opcode = code.OpReturnValue
}
