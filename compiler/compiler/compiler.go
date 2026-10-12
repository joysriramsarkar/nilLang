package compiler

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/builtin"
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

	// positions is the per-scope line table (offset -> source location).
	positions []object.SourcePos
}

type Bytecode struct {
	Instructions code.Instructions
	Constants    []object.Object

	// Positions is the top-level line table; SourceFile is the file it maps.
	Positions  []object.SourcePos
	SourceFile string
	// GlobalNames maps global slot indices to their debug names.
	GlobalNames []string
}

type Compiler struct {
	constants   []object.Object
	symbolTable *SymbolTable

	scopes     []CompilationScope
	scopeIndex int

	// SourceFile is recorded in the debug line tables.
	SourceFile string
	// SourceDir is the directory of the file being compiled. Relative
	// module imports ("./domain/cart.nil") resolve against it.
	SourceDir string

	// LoadModule resolves a file-based import path to its parsed AST.
	// It returns the program and a canonical path used for caching and
	// cycle detection. When nil, only native modules can be imported.
	LoadModule func(importPath, sourceDir string) (*ast.Program, string, error)

	// moduleCache remembers inlined file modules (canonical path -> globals).
	moduleCache map[string][]Symbol

	// importStack tracks the active import chain for cycle detection.
	importStack []string

	// nextModuleGen hands out fresh import generations, one per inlined
	// module. Zero is the main program's generation; every import takes the
	// next number and never reuses it, so two modules declaring the same name
	// can never compare equal in Define's generation check.
	nextModuleGen int

	// loops is the stack of enclosing loops, one entry per `while` currently
	// being compiled. `break` and `continue` patch the innermost entry, which is
	// what makes a nested loop's control flow independent of the loops around it.
	loops []*loopContext
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
		moduleCache: make(map[string][]Symbol),
	}
}

var (
	sortedBuiltinNamesOnce sync.Once
	sortedBuiltinNames     []string
)

func getBuiltinNames() []string {
	sortedBuiltinNamesOnce.Do(func() {
		names := make([]string, 0, len(evaluator.Builtins))
		for k := range evaluator.Builtins {
			names = append(names, k)
		}
		sort.Strings(names)
		sortedBuiltinNames = names
	})
	return sortedBuiltinNames
}

func GetBuiltinNamesSorted() []string {
	return getBuiltinNames()
}

func NewWithState(s *SymbolTable, constants []object.Object) *Compiler {
	compiler := New()
	compiler.symbolTable = s
	compiler.constants = constants
	// Continue the shared table's import generations rather than restarting
	// them, so a module inlined through this compiler can never reuse a slot
	// an earlier compiler gave another module in the same table.
	for _, sym := range s.Names() {
		if sym.Module >= compiler.nextModuleGen {
			compiler.nextModuleGen = sym.Module + 1
		}
	}
	return compiler
}

func (c *Compiler) Compile(node ast.Node) error {
	// Record a debug line-table entry for this node's source position before
	// emitting its instructions. This powers VM source-level debugging.
	c.recordNodePosition(node)

	// A visibility marker changes what leaves the module, not what the
	// declaration does inside it, so the wrapper compiles to the declaration.
	if wrapped, ok := node.(*ast.ExportStatement); ok && wrapped.Decl != nil {
		return c.Compile(wrapped.Decl)
	}

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

	case *ast.ImportStatement:
		return c.compileImport(node)

	case *ast.ExpressionStatement:
		err := c.Compile(node.Expression)
		if err != nil {
			return err
		}
		c.emit(code.OpPop)

	case *ast.InfixExpression:
		if node.Operator == "&&" {
			if err := c.Compile(node.Left); err != nil {
				return err
			}
			jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)
			if err := c.Compile(node.Right); err != nil {
				return err
			}
			c.emit(code.OpBang)
			c.emit(code.OpBang)
			jumpEnd := c.emit(code.OpJump, 9999)

			afterRightPos := len(c.currentInstructions())
			c.changeOperand(jumpNotTruthyPos, afterRightPos)
			c.emit(code.OpFalse)

			endPos := len(c.currentInstructions())
			c.changeOperand(jumpEnd, endPos)
			return nil
		}
		if node.Operator == "||" {
			if err := c.Compile(node.Left); err != nil {
				return err
			}
			jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)
			c.emit(code.OpTrue)
			jumpEnd := c.emit(code.OpJump, 9999)

			afterLeftPos := len(c.currentInstructions())
			c.changeOperand(jumpNotTruthyPos, afterLeftPos)
			if err := c.Compile(node.Right); err != nil {
				return err
			}
			c.emit(code.OpBang)
			c.emit(code.OpBang)

			endPos := len(c.currentInstructions())
			c.changeOperand(jumpEnd, endPos)
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
		case "<":
			c.emit(code.OpLessThan)
		case "<=":
			c.emit(code.OpLessThanEqual)
		case ">":
			c.emit(code.OpGreaterThan)
		case ">=":
			c.emit(code.OpGreaterThanEqual)
		case "==":
			c.emit(code.OpEqual)
		case "!=":
			c.emit(code.OpNotEqual)
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

		// Register the loop before compiling the body, so that a `break` or
		// `continue` inside it has a target. The continue target is the loop's
		// own condition, which makes `continue` mean "re-test" for a while loop.
		loop := &loopContext{continueTarget: conditionStart}
		c.loops = append(c.loops, loop)

		err := c.Compile(node.Condition)
		if err != nil {
			c.loops = c.loops[:len(c.loops)-1]
			return err
		}

		jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

		err = c.Compile(node.Body)
		if err != nil {
			c.loops = c.loops[:len(c.loops)-1]
			return err
		}

		// Jump back to condition
		c.emit(code.OpJump, conditionStart)

		afterBodyPos := len(c.currentInstructions())
		c.changeOperand(jumpNotTruthyPos, afterBodyPos)

		// Every `break` inside this loop lands after the exit jump.
		for _, patch := range loop.breakPatches {
			c.changeOperand(patch, afterBodyPos)
		}
		c.loops = c.loops[:len(c.loops)-1]
		// A while statement is a statement, never an expression, so it must not
		// leave a value behind: pushing one here leaks a stack slot for every
		// loop and unbalances blocks whose last statement is a loop.

	case *ast.StructDecl, *ast.EnumDecl, *ast.TypeAliasDecl:
		// Declarations have no run-time effect: a struct's name is a type and
		// its fields are checked statically. Emitting nothing is correct, and
		// emitting a value would leave one on the stack for the enclosing
		// statement to account for.

	case *ast.LetExpression:
		// `let name = value; body` in expression position.
		//
		// The binding is defined in the *current* scope rather than a new one.
		// A fresh compilation scope would give the body its own instruction
		// buffer, and every jump inside it would then be relative to the wrong
		// base; splicing the buffer back in means rewriting every operand, which
		// is more machinery than the lifetime is worth. Defining in the current
		// scope reuses one slot per name, so a second let expression with the
		// same hidden subject simply overwrites it.
		symbol := c.symbolTable.Define(node.Name.Value)

		if err := c.Compile(node.Value); err != nil {
			return err
		}
		c.emitSymbolStore(symbol)

		// The body's value is the expression's value, so nothing is popped.
		return c.Compile(node.Body)

	case *ast.MatchExpression:
		return fmt.Errorf("internal error: a `match` reached the bytecode compiler without being lowered")

	case *ast.CoalesceExpression:
		return fmt.Errorf("internal error: `??` reached the bytecode compiler without being lowered")

	case *ast.SafeAccessExpression:
		return fmt.Errorf("internal error: `?.` reached the bytecode compiler without being lowered")

	case *ast.EnumVariantExpr:
		return fmt.Errorf("internal error: an enum variant reached the bytecode compiler without being lowered")

	case *ast.StructLiteral:
		return fmt.Errorf("internal error: a struct literal reached the bytecode compiler without being lowered")

	case *ast.ForStatement:
		// for is lowered to a while before any backend sees it, so reaching
		// here means the lowering pass did not run.
		return fmt.Errorf("internal error: a `for` loop reached the bytecode compiler without being lowered")

	case *ast.BreakStatement:
		return c.compileBreak()

	case *ast.ContinueStatement:
		return c.compileContinue()

	case *ast.RangeExpression:
		// Ranges have no runtime form of their own; they exist only in a
		// for header, which the lowering pass consumes.
		return fmt.Errorf("a range expression is only valid in a `for` loop header")

	case *ast.BlockStatement:
		for _, s := range node.Statements {
			err := c.Compile(s)
			if err != nil {
				return err
			}
		}

	case *ast.LetStatement:
		if node.Value == nil {
			return fmt.Errorf("variable %s requires an initializer", node.Name.Value)
		}

		isFunc := false
		if _, ok := node.Value.(*ast.FunctionLiteral); ok {
			isFunc = true
			if node.Constant {
				c.symbolTable.DefineConst(node.Name.Value)
			} else {
				c.symbolTable.Define(node.Name.Value)
			}
		}

		err := c.Compile(node.Value)
		if err != nil {
			return err
		}

		var symbol Symbol
		if !isFunc {
			if node.Constant {
				symbol = c.symbolTable.DefineConst(node.Name.Value)
			} else {
				symbol = c.symbolTable.Define(node.Name.Value)
			}
		} else {
			symbol, _ = c.symbolTable.Resolve(node.Name.Value)
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
			if err := c.Compile(&ast.FunctionLiteral{Token: handler.Token, Parameters: identifiersToParameters(handler.Parameters), Body: handler.Body}); err != nil {
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

	case *ast.IndexAssignStatement:
		if err := c.Compile(node.Left); err != nil {
			return err
		}
		if err := c.Compile(node.Index); err != nil {
			return err
		}
		if err := c.Compile(node.Value); err != nil {
			return err
		}
		c.emit(code.OpSetIndex)

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
		// Emit in source order so that side effects in key or value
		// expressions are observable in the order written, matching the
		// tree-walking evaluator. Previously the keys were sorted by their
		// printed form, which happened to be deterministic but was not the
		// order the developer wrote.
		for _, entry := range orderedHashPairs(node) {
			if err := c.Compile(entry.Key); err != nil {
				return err
			}
			if err := c.Compile(entry.Value); err != nil {
				return err
			}
		}

		c.emit(code.OpHash, node.EntryCount()*2)

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
		// Dot access, not indexing: a member that is not there is a mistake, and
		// the VM has to be able to say so rather than answer null.
		c.emit(code.OpGetField)

	case *ast.FunctionLiteral:
		c.enterScope()

		if node.Name != "" {
			c.symbolTable.DefineFunctionName(node.Name)
		}

		for _, p := range node.Parameters {
			if p != nil && p.Name != nil {
				c.symbolTable.Define(p.Name.Value)
			}
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
		localNames := c.collectLocalNames(numLocals)
		instructions, positions := c.leaveScope()

		// When capturing free variables for a new closure, we must push the
		// raw CaptureCell pointer (not the unwrapped value) so the VM can
		// share the cell between parent and child closures. For non-free
		// (local/global) captures we push the value and the VM wraps it in a
		// new cell inside pushClosure.
		for _, s := range freeSymbols {
			c.loadSymbolForCapture(s)
		}

		fnName := node.Name
		if fnName == "" {
			fnName = "<func>"
		}
		compiledFn := &object.CompiledFunction{
			Instructions:  instructions,
			NumLocals:     numLocals,
			NumParameters: len(node.Parameters),
			Name:          fnName,
			SourceFile:    c.SourceFile,
			Positions:     positions,
			LocalNames:    localNames,
			IsPure:        node.IsPure,
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
		function := &ast.FunctionLiteral{Token: node.Token, Parameters: []*ast.Parameter{}, Body: node.Body}
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
		// A direct call to a gated builtin is emitted as OpNativeCall so that
		// the VM's capability checker sees it. Previously every builtin call was
		// an ordinary OpCall, which meant the gate never fired and the sandbox
		// was inert.
		if name, ok := c.gatedBuiltinCall(node); ok {
			for _, a := range node.Arguments {
				if err := c.Compile(a); err != nil {
					return err
				}
			}
			nameIndex := c.addConstant(&object.String{Value: name})
			c.emit(code.OpNativeCall, nameIndex, len(node.Arguments))
			break
		}

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

// compileImport resolves an import statement during bytecode compilation,
// giving the bytecode VM the same module semantics as the tree-walking
// evaluator (evaluator.evalImportStatement):
//
//   - Native modules (web, data, money, security, realtime, ...) are bound
//     as a constant Hash of builtins.
//   - File modules ("./domain/cart.nil", "std/...") are inlined into the
//     current compilation scope: their top-level symbols become globals of
//     the main program (so the VM can call them as closures), and the
//     import alias is bound to a runtime Hash built from those globals.
func (c *Compiler) compileImport(node *ast.ImportStatement) error {
	if node.Path == nil {
		return fmt.Errorf("import path cannot be empty")
	}
	importPath := node.Path.Value

	// Case 1: native module (registered via evaluator.RegisterNativeModule)
	if mod, ok := evaluator.GetNativeModule(importPath); ok {
		return c.bindNativeImport(node, importPath, mod)
	}

	// Case 2: file-based module
	if c.LoadModule == nil {
		return fmt.Errorf("E0302: module not found: %q (no module loader configured)", importPath)
	}

	prog, canonical, err := c.LoadModule(importPath, c.SourceDir)
	if err != nil {
		return fmt.Errorf("E0302: %s", err)
	}

	// Circular dependency detection (mirrors evaluator E0301)
	for _, visiting := range c.importStack {
		if visiting == canonical {
			chain := append(append([]string{}, c.importStack...), canonical)
			return fmt.Errorf("E0301: circular dependency detected: %s", strings.Join(chain, " -> "))
		}
	}

	// Cached module: rebind without re-executing module init code
	if syms, ok := c.moduleCache[canonical]; ok {
		return c.bindInlinedImport(node, importPath, syms)
	}

	c.importStack = append(c.importStack, canonical)
	prevDir := c.SourceDir
	c.SourceDir = filepath.Dir(canonical)
	// Each inlined module compiles in its own import generation so that a
	// name declared by two modules (for example `sha256` in both
	// `std/crypto/hash` and `std/crypto/hmac`) takes two global slots instead
	// of the second silently reusing the first. Without this, the reusing
	// module's alias snapshot misses the name (the shared slot predates the
	// import) and internal calls compiled against the slot resolve to
	// whichever module wrote it last — while the evaluator, which gives every
	// module its own environment, keeps them apart.
	prevGen := c.symbolTable.ModuleGen
	c.symbolTable.ModuleGen = c.nextModuleGen + 1
	c.nextModuleGen++
	defer func() {
		c.importStack = c.importStack[:len(c.importStack)-1]
		c.SourceDir = prevDir
		c.symbolTable.ModuleGen = prevGen
	}()

	// Inline-compile the module into the current scope. Module top-level
	// symbols become globals shared with the main program, so functions
	// defined in the module can call each other (OpGetGlobal) at VM runtime.
	before := c.symbolTable.NumDefinitions()
	for _, stmt := range prog.Statements {
		if err := c.Compile(stmt); err != nil {
			return fmt.Errorf("module %s: %w", canonical, err)
		}
	}

	// Collect the globals defined by the module
	all := make([]Symbol, 0, 8)
	for _, sym := range c.symbolTable.store {
		if sym.Scope == GlobalScope && sym.Index >= before {
			all = append(all, sym)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Index < all[j].Index })

	// Only the module's exported names are offered to the importer. The private
	// ones stay compiled and reachable from inside the module — that is what
	// private means — they are simply not bound here. Filtering at this point
	// rather than at the bind site keeps the cached form and the bound form the
	// same list, so a module imported twice cannot get two different answers.
	exported, _ := ast.ExportedNames(prog)
	syms := all
	if len(exported) != len(all) {
		syms = make([]Symbol, 0, len(exported))
		for _, sym := range all {
			if exported[sym.Name] {
				syms = append(syms, sym)
			}
		}
	}

	c.moduleCache[canonical] = syms
	return c.bindInlinedImport(node, importPath, syms)
}

// bindNativeImport binds an imported native module (web, data, money, ...).
// Native module hashes contain Go closures that cannot be serialized into a
// NABC bytecode image, so the module is fetched by name at runtime through
// the __native_module__ builtin — the same registry the evaluator uses.
func (c *Compiler) bindNativeImport(node *ast.ImportStatement, importPath string, mod *object.Hash) error {
	bindName := ""
	if node.Alias != nil {
		bindName = node.Alias.Value
	}
	if bindName == "" {
		bindName = moduleBaseName(importPath)
	}

	// <alias> = __native_module__("<path>")
	if err := c.emitNativeModuleFetch(importPath); err != nil {
		return err
	}
	moduleSymbol := c.symbolTable.Define(bindName)
	c.emitSymbolStore(moduleSymbol)

	// import {x, y} from "mod" -> <name> = <alias>["x"]
	for _, ident := range node.Names {
		c.emit(code.OpGetGlobal, moduleSymbol.Index)
		c.emit(code.OpConstant, c.addConstant(&object.String{Value: ident.Value}))
		c.emit(code.OpIndex)
		nameSymbol := c.symbolTable.Define(ident.Value)
		c.emitSymbolStore(nameSymbol)
	}
	return nil
}

func (c *Compiler) emitNativeModuleFetch(importPath string) error {
	sym, ok := c.symbolTable.Resolve(nativeModuleBuiltinName)
	if !ok || sym.Scope != BuiltinScope {
		return fmt.Errorf("E0302: native module loader %q unavailable", nativeModuleBuiltinName)
	}
	// OpCall expects [callee, arg...]: push the builtin first,
	// then the module name argument.
	c.emit(code.OpGetBuiltin, sym.Index)
	c.emit(code.OpConstant, c.addConstant(&object.String{Value: importPath}))
	c.emit(code.OpCall, 1)
	return nil
}

// nativeModuleBuiltinName is the runtime resolver for native modules.
const nativeModuleBuiltinName = "__native_module__"

// bindInlinedImport binds an inlined file module. For `import {x} from "m"`
// the requested symbols are bound directly to their globals; otherwise the
// alias is bound to a runtime Hash containing every module symbol.
func (c *Compiler) bindInlinedImport(node *ast.ImportStatement, importPath string, syms []Symbol) error {
	if len(node.Names) > 0 {
		byName := make(map[string]Symbol, len(syms))
		for _, s := range syms {
			byName[s.Name] = s
		}

		// Every missing name is collected before anything is emitted, so an
		// import that is missing two names says both. Failing on the first would
		// make fixing one error reveal the next one, which is a needless round
		// trip through the compiler for a mistake the whole list already answers.
		var missing []string
		for _, ident := range node.Names {
			if _, ok := byName[ident.Value]; !ok {
				missing = append(missing, strconv.Quote(ident.Value))
			}
		}
		if len(missing) > 0 {
			return fmt.Errorf("E0303: module %s does not export %s; a module only exports what it marks with export",
				importPath, strings.Join(missing, ", "))
		}

		for _, ident := range node.Names {
			c.emit(code.OpGetGlobal, byName[ident.Value].Index)
			symbol := c.symbolTable.Define(ident.Value)
			c.emitSymbolStore(symbol)
		}
		return nil
	}

	// Build {name: value, ...} from the module globals at runtime
	for _, s := range syms {
		c.emit(code.OpConstant, c.addConstant(&object.String{Value: s.Name}))
		c.emit(code.OpGetGlobal, s.Index)
	}
	c.emit(code.OpHash, len(syms)*2)

	bindName := ""
	if node.Alias != nil {
		bindName = node.Alias.Value
	}
	if bindName == "" {
		bindName = moduleBaseName(importPath)
	}
	symbol := c.symbolTable.Define(bindName)
	c.emitSymbolStore(symbol)
	return nil
}

func (c *Compiler) emitSymbolStore(symbol Symbol) {
	if symbol.Scope == GlobalScope {
		c.emit(code.OpSetGlobal, symbol.Index)
	} else {
		c.emit(code.OpSetLocal, symbol.Index)
	}
}

// moduleBaseName derives the implicit binding name of an import path,
// matching the evaluator: base name without extension ("./a/b.nil" -> "b").
func moduleBaseName(importPath string) string {
	base := filepath.Base(importPath)
	return strings.TrimSuffix(base, filepath.Ext(base))
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
		return
	}

	if s.Scope == LocalScope {
		// A captured local must become a shared cell *in place*, so that the
		// enclosing frame's later reads and writes go through the same cell the
		// closure holds. That is what makes mutable capture by reference work:
		// without it the closure writes to a cell the parent never sees.
		//
		// Globals need no boxing: they live in one array shared by every frame,
		// so a closure capturing a global already observes writes.
		c.emit(code.OpBoxLocal, s.Index)
		return
	}

	// Globals: push the current value; the VM wraps it in a new cell. The
	// closure captures a snapshot, which matches the evaluator's treatment of
	// globals.
	c.loadSymbol(s)
}

func (c *Compiler) Bytecode() *Bytecode {
	return &Bytecode{
		Instructions: c.currentInstructions(),
		Constants:    c.constants,
		Positions:    c.scopes[c.scopeIndex].positions,
		SourceFile:   c.SourceFile,
		GlobalNames:  c.collectGlobalNames(),
	}
}

// collectGlobalNames maps global slot indices to their debug names.
func (c *Compiler) collectGlobalNames() []string {
	max := 0
	for _, sym := range c.symbolTable.store {
		if sym.Scope == GlobalScope && sym.Index+1 > max {
			max = sym.Index + 1
		}
	}
	if max == 0 {
		return nil
	}
	names := make([]string, max)
	for name, sym := range c.symbolTable.store {
		if sym.Scope == GlobalScope && sym.Index >= 0 && sym.Index < max {
			names[sym.Index] = name
		}
	}
	return names
}

// SourceFileName returns the file recorded in the debug line table.
func (c *Compiler) SourceFileName() string { return c.SourceFile }

// collectLocalNames maps local slot indices to their debug names using the
// current (innermost) symbol table.
func (c *Compiler) collectLocalNames(numLocals int) []string {
	names := make([]string, numLocals)
	for name, sym := range c.symbolTable.store {
		if sym.Scope == LocalScope && sym.Index >= 0 && sym.Index < numLocals {
			names[sym.Index] = name
		}
	}
	return names
}

// recordNodePosition appends a line-table entry for node's source position.
func (c *Compiler) recordNodePosition(node ast.Node) {
	line, col, ok := nodePosition(node)
	if !ok {
		return
	}
	c.recordPosition(line, col)
}

func (c *Compiler) recordPosition(line, col int) {
	offset := len(c.currentInstructions())
	sc := &c.scopes[c.scopeIndex]
	if n := len(sc.positions); n > 0 && sc.positions[n-1].Offset == offset {
		sc.positions[n-1].Line = line
		sc.positions[n-1].Column = col
		return
	}
	sc.positions = append(sc.positions, object.SourcePos{Offset: offset, Line: line, Column: col})
}

// nodePosition extracts the source position from an AST node that carries a
// token. It uses reflection so every statement/expression type is supported
// without a large type switch.
func nodePosition(node ast.Node) (int, int, bool) {
	if node == nil {
		return 0, 0, false
	}
	v := reflect.ValueOf(node)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return 0, 0, false
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return 0, 0, false
	}
	tok := v.FieldByName("Token")
	if !tok.IsValid() || tok.Kind() != reflect.Struct {
		return 0, 0, false
	}
	lineF := tok.FieldByName("Line")
	colF := tok.FieldByName("Column")
	if !lineF.IsValid() || !colF.IsValid() {
		return 0, 0, false
	}
	line := int(lineF.Int())
	if line <= 0 {
		return 0, 0, false
	}
	return line, int(colF.Int()), true
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

func (c *Compiler) leaveScope() (code.Instructions, []object.SourcePos) {
	instructions := c.currentInstructions()
	positions := c.scopes[c.scopeIndex].positions

	c.scopes = c.scopes[:len(c.scopes)-1]
	c.scopeIndex--
	c.symbolTable = c.symbolTable.Outer

	return instructions, positions
}

func (c *Compiler) replaceLastPopWithReturn() {
	lastPos := c.scopes[c.scopeIndex].lastInstruction.Position
	c.replaceInstruction(lastPos, code.Make(code.OpReturnValue))

	c.scopes[c.scopeIndex].lastInstruction.Opcode = code.OpReturnValue
}

// orderedHashPairs returns a hash literal's entries in source order.
//
// The parser records that order in ast.HashLiteral.OrderedPairs. For a node
// built by hand without an order slice, entries are sorted by their printed
// form so that emission stays deterministic rather than depending on Go map
// iteration order.
func orderedHashPairs(node *ast.HashLiteral) []ast.HashPairExpr {
	if len(node.OrderedPairs) > 0 {
		return node.OrderedPairs
	}

	entries := make([]ast.HashPairExpr, 0, len(node.Pairs))
	for k, v := range node.Pairs {
		entries = append(entries, ast.HashPairExpr{Key: k, Value: v})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key.String() < entries[j].Key.String()
	})
	return entries
}

// gatedBuiltinCall reports whether a call expression is a direct call to a
// builtin that the runtime must capability-check.
//
// Only the direct, statically named form `writeFile(path, data)` qualifies.
// A call through a variable (`let f = writeFile; f(...)`) cannot be recognised
// here, which is a real limitation of bytecode level gating rather than a
// choice: see docs/spec/CAPABILITIES.md. Intercepting that form needs either a
// `requires` clause on the function (planned) or a wrapper object that carries
// the requirement, and either way the check must move from the call site to
// the binding.
//
// A name that resolves to a non-builtin binding is NOT a builtin call, even
// when a builtin of the same name exists: the module (or local) shadows it,
// and the call must reach the shadow exactly as the evaluator resolves it.
// Without this, a module defining `exec` (like `std/db`, whose `insertMap`
// calls its own `exec`) has those calls compiled to process-spawns on the VM
// while the evaluator calls the module function — and because the process
// builtin reports failure as exit code 1 rather than an error, the divergence
// is silent data loss rather than a loud failure.
func (c *Compiler) gatedBuiltinCall(node *ast.CallExpression) (string, bool) {
	ident, ok := node.Function.(*ast.Identifier)
	if !ok {
		return "", false
	}
	class, known := builtin.Classify(ident.Value)
	if !known || !class.Gated {
		return "", false
	}
	if sym, found := c.symbolTable.Resolve(ident.Value); found && sym.Scope != BuiltinScope {
		return "", false
	}
	return ident.Value, true
}

// loopContext records what `break` and `continue` need to patch while a loop is
// being compiled.
//
// The compiler keeps a stack of these so that a nested loop patches only its own
// jumps: an inner `break` must leave the enclosing loop running.
type loopContext struct {
	// breakPatches are the jump operands that `break` must rewrite to the loop's
	// exit. They are patched once the body, and therefore the exit position, is
	// known.
	breakPatches []int

	// continueTarget is where `continue` jumps: the position of the loop's
	// increment, for a `for` loop, or of its condition re-test, for a `while`.
	continueTarget int
}

// Errors reported when `break` or `continue` appears with no enclosing loop.
//
// Catching these while compiling, rather than letting a jump patch list stay
// empty and silently do nothing, is the difference between a clear message and a
// loop that never terminates.
var (
	ErrBreakOutsideLoop    = errors.New("`break` used outside of a loop")
	ErrContinueOutsideLoop = errors.New("`continue` used outside of a loop")
)

// compileBreak emits a jump to the innermost loop's exit.
func (c *Compiler) compileBreak() error {
	if len(c.loops) == 0 {
		return ErrBreakOutsideLoop
	}

	loop := c.loops[len(c.loops)-1]
	patch := c.emit(code.OpJump, 0)
	loop.breakPatches = append(loop.breakPatches, patch)
	return nil
}

// compileContinue emits a jump to the innermost loop's continue target.
func (c *Compiler) compileContinue() error {
	if len(c.loops) == 0 {
		return ErrContinueOutsideLoop
	}

	loop := c.loops[len(c.loops)-1]
	c.emit(code.OpJump, loop.continueTarget)
	return nil
}

// identifiersToParameters wraps plain names as untyped parameters.
//
// It is used where the AST node holds names rather than parameters — an event
// handler's payload, whose type is whatever the event carries — so that the rest
// of the compiler sees one shape.
func identifiersToParameters(names []*ast.Identifier) []*ast.Parameter {
	out := make([]*ast.Parameter, 0, len(names))
	for _, name := range names {
		if name == nil {
			continue
		}
		out = append(out, &ast.Parameter{Token: name.Token, Name: name})
	}
	return out
}
