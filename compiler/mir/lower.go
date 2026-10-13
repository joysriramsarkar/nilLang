package mir

import (
	"fmt"

	"github.com/joysriramsarkar/nilLang/compiler/hir"
)

type Lowerer struct {
	tempCounter  int
	blockCounter int
	currentProg  *Program
	currentFn    *Function
	currentBlock *BasicBlock

	// scopes maps source names to the MIR names currently bound, innermost
	// frame last. A `let` that shadows an outer binding gets a versioned
	// name (`x$1`); every other declaration keeps its spelling, so programs
	// without shadowing lower exactly as before.
	//
	// `$` can never collide with user code: Nilang identifiers hold letters,
	// digits and `_` only. It is also safe for the backends: `$` is legal in
	// LLVM identifiers and idiomatic in WAT names, and the VM treats MIR
	// names as opaque strings.
	scopes []map[string]string

	// versionCounter numbers shadow versions. It never resets: two shadows
	// of one name must stay distinct even after both scopes close.
	versionCounter int

	// lastStatement is the operand an expression statement produced, and nil for
	// any other kind of statement.
	//
	// It exists for one reason: a program's top-level expression *is* the program's
	// value, and MIR is where that fact belongs. See `LowerHIR`.
	lastStatement Operand
}

func NewLowerer() *Lowerer {
	return &Lowerer{}
}

func (l *Lowerer) newTemp() Operand {
	id := l.tempCounter
	l.tempCounter++
	return TempOperand{ID: id}
}

// pushScope opens a `{ ... }` scope: lets inside shadow instead of
// overwriting, matching the type checker and the two Go engines.
func (l *Lowerer) pushScope() {
	l.scopes = append(l.scopes, map[string]string{})
}

// popScope closes the innermost scope, revealing shadowed bindings again.
func (l *Lowerer) popScope() {
	l.scopes = l.scopes[:len(l.scopes)-1]
}

// lookup resolves a source name to the MIR name currently bound, or "" when
// no scope binds it (a global lowered before any scope opened, or a name the
// backend resolves itself, such as a function).
func (l *Lowerer) lookup(name string) (string, bool) {
	for i := len(l.scopes) - 1; i >= 0; i-- {
		if v, ok := l.scopes[i][name]; ok {
			return v, true
		}
	}
	return "", false
}

// declare binds a freshly declared name in the current scope. A name already
// bound in a strictly-outer scope is shadowed under a versioned name; every
// other declaration — including same-scope redeclaration, which reuses one
// binding exactly like the direct compiler — keeps its spelling.
func (l *Lowerer) declare(name string) string {
	for i := len(l.scopes) - 2; i >= 0; i-- {
		if _, ok := l.scopes[i][name]; ok {
			v := fmt.Sprintf("%s$%d", name, l.versionCounter)
			l.versionCounter++
			l.scopes[len(l.scopes)-1][name] = v
			return v
		}
	}
	l.scopes[len(l.scopes)-1][name] = name
	return name
}

// bind records an already-lowered name (a function parameter) in the current
// scope without versioning: parameters are the ABI, not shadowing.
func (l *Lowerer) bind(name string) {
	l.scopes[len(l.scopes)-1][name] = name
}

func (l *Lowerer) newBlock(name string) *BasicBlock {
	id := fmt.Sprintf("%s_%d", name, l.blockCounter)
	l.blockCounter++
	bb := &BasicBlock{
		ID:           id,
		Instructions: []Instruction{},
	}
	if l.currentFn != nil {
		l.currentFn.Blocks = append(l.currentFn.Blocks, bb)
	}
	return bb
}

func (l *Lowerer) emit(inst Instruction) {
	if l.currentBlock != nil {
		l.currentBlock.Instructions = append(l.currentBlock.Instructions, inst)
	}
}

func (l *Lowerer) terminate(term Terminator) {
	if l.currentBlock != nil && l.currentBlock.Terminator == nil {
		l.currentBlock.Terminator = term
	}
}

func (l *Lowerer) LowerHIR(p *hir.Program) *Program {
	prog := NewProgram()
	l.currentProg = prog

	// The root scope: top-level declarations bind here, and every nested
	// scope resolves through it.
	l.pushScope()

	mainFn := &Function{
		Name:   "main",
		Params: []string{},
		Blocks: []*BasicBlock{},
	}
	l.currentFn = mainFn
	l.currentBlock = l.newBlock("bb_entry")

	var programValue Operand

	for _, stmt := range p.Statements {
		l.lastStatement = nil
		l.lowerStatement(stmt)

		// The value of the last expression statement is what the program evaluates
		// to.
		//
		// All three engines answer "what does `1 + 2` evaluate to" the same way:
		// the interpreter returns the object's value, the VM leaves it as the last
		// thing on the stack, and a compiled program has to return it from its
		// entry. MIR recorded none of that — it computed the answer into a
		// temporary and returned nothing, which left the two Go engines to recover
		// it from a stack and left every other backend with nothing to return at
		// all.
		//
		// Recording it here rather than in each backend is what makes the three
		// agree by construction instead of three implementations agreeing by hand.
		// Assigned unconditionally, including when it is nil.
		//
		// Keeping the *previous* expression's value would be wrong: the evaluator
		// answers a `let` statement with null, so a program ending
		// `fn f() {...} let x = 1;` evaluates to nothing, and a MIR that returned the
		// function would make the three engines disagree about the simplest program
		// containing a declaration.
		programValue = l.lastStatement
	}

	// The final block needs a terminator. When one is already there — the program's
	// last statement was a `return` — it is left alone: a `return` is the program
	// saying what its value is, and overwriting it with the previous expression
	// would discard that.
	if l.currentBlock != nil && l.currentBlock.Terminator == nil {
		l.terminate(ReturnTerminator{Value: programValue})
	}

	prog.Main = mainFn
	return prog
}

// typeDefOf converts a declared type into the form the rest of the compiler reads.
//
// Fields keep their order and their resolved type, because both are load-bearing: a
// struct laid out for the boundary is laid out in declaration order, and a lookup
// wants what `Int` *is* rather than how it was spelled.
func typeDefOf(stmt *hir.TypeDeclStmt) *TypeDef {
	definition := &TypeDef{Name: stmt.Name}
	switch stmt.Kind {
	case hir.KindStruct:
		definition.Kind = KindStruct
		definition.Fields = make([]FieldDef, 0, len(stmt.Fields))
		for _, field := range stmt.Fields {
			definition.Fields = append(definition.Fields, FieldDef{
				Name:       field.Name,
				TypeString: field.TypeString,
				Type:       field.Type,
			})
		}
	case hir.KindEnum:
		definition.Kind = KindEnum
		definition.Variants = make([]VariantDef, 0, len(stmt.Variants))
		for _, variant := range stmt.Variants {
			definition.Variants = append(definition.Variants, VariantDef{
				Name:    variant.Name,
				Payload: variant.Payload,
			})
		}
	default:
		definition.Kind = KindAlias
	}
	return definition
}

// NewProgram is an empty program, with every map ready.
//
// The maps rather than nil values because every caller that adds to one would
// otherwise have to check, and a program with a nil `Types` map reads as a program
// that declared no types — which is true of some programs and a bug in the rest.
func NewProgram() *Program {
	return &Program{
		Functions:  make(map[string]*Function),
		Entities:   make(map[string]*EntityDef),
		Components: make(map[string]*ComponentDef),
		Types:      make(map[string]*TypeDef),
	}
}

func (l *Lowerer) lowerStatement(stmt hir.Statement) {
	switch s := stmt.(type) {
	case *hir.TypeDeclStmt:
		// A declaration emits no instruction.
		//
		// That is the whole of what it used to do, and it is why `Point` meant
		// nothing downstream: the value `Point { x: 7 }` becomes is a hash, and the
		// hash carries the *contents* rather than the shape. Recording the
		// declaration here is what makes the shape available to the FFI, to the
		// generated header, and to anything else that has to know a field exists.
		if l.currentProg == nil {
			return
		}
		l.currentProg.Types[s.Name] = typeDefOf(s)

	case *hir.ComponentDeclStmt:
		definition := &ComponentDef{Name: s.Name, Events: make(map[string]string)}
		for _, state := range s.States {
			definition.States = append(definition.States, state.Name)
			l.lowerStatement(state)
		}
		if s.Render != nil {
			definition.RenderFunc = l.lowerComponentFunction(s.Name+".render", nil, s.Render)
		}
		if s.Build != nil {
			definition.BuildFunc = l.lowerComponentFunction(s.Name+".build", nil, s.Build)
		}
		for _, event := range s.Handlers {
			functionName := s.Name + ".on." + event.Name
			definition.Events[event.Name] = l.lowerComponentFunction(functionName, event.Params, event.Body)
		}
		l.currentProg.Components[s.Name] = definition

	case *hir.EntityDeclStmt:
		fields := make([]EntityFieldDef, 0, len(s.Fields))
		for _, f := range s.Fields {
			fields = append(fields, EntityFieldDef{
				Name:         f.Name,
				Type:         f.Type.String(),
				IsPrimary:    f.IsPrimary,
				IsRequired:   f.IsRequired,
				IsUnique:     f.IsUnique,
				TargetEntity: f.TargetEntity,
			})
		}
		ent := &EntityDef{
			Name:   s.Name,
			Fields: fields,
		}
		if l.currentProg != nil {
			l.currentProg.Entities[s.Name] = ent
		}

	case *hir.LetStmt:
		valOp := l.lowerExpression(s.Value)
		l.emit(StoreVarInst{Name: l.declare(s.Name), Src: valOp})

	case *hir.BlockStmt:
		// A block is a scope: lets inside shadow instead of overwriting,
		// matching the type checker and the two Go engines.
		l.pushScope()
		defer l.popScope()
		// A block that *is* the program's value — what the HIR optimizer leaves
		// behind after folding `if (10 > 5) { 100 } else { 200 }` down to the
		// surviving arm — has to report that arm's value as its own.
		//
		// Without this the optimizer's own optimisation deleted the answer: the
		// block was lowered, its statements ran, and its last expression was
		// discarded, so the whole program evaluated to nothing. That is a bug an
		// optimiser introduced and a backend noticed, which is the worst order for
		// two bugs to appear in.
		l.lastStatement = nil
		for _, inner := range s.Statements {
			l.lowerStatement(inner)
		}

	case *hir.AssignStmt:
		l.lowerAssignment(s)
		l.lastStatement = nil
	case *hir.ReturnStmt:
		valOp := l.lowerExpression(s.Value)
		l.terminate(ReturnTerminator{Value: valOp})

	case *hir.ExprStmt:
		l.lastStatement = l.lowerExpression(s.Expr)

	case *hir.IfStmt:
		condOp := l.lowerExpression(s.Condition)
		thenBB := l.newBlock("bb_then")
		elseBB := l.newBlock("bb_else")
		mergeBB := l.newBlock("bb_merge")

		l.terminate(BranchTerminator{
			Cond:        condOp,
			TrueTarget:  thenBB.ID,
			FalseTarget: elseBB.ID,
		})

		// Each arm's last expression is what the `if` evaluates to, and it has to
		// arrive at the merge block through a variable both arms write. Without
		// this the two arms ran and their results were discarded, so `if (c) { 100 }
		// else { 200 }` — whose whole purpose is to produce one of them — evaluated
		// to nothing at all.
		mergeValue := l.newTemp()

		// Then block
		l.currentBlock = thenBB
		l.lastStatement = nil
		l.pushScope()
		if s.Consequence != nil {
			for _, st := range s.Consequence.Statements {
				l.lowerStatement(st)
			}
		}
		l.popScope()
		if l.lastStatement != nil {
			l.emit(AssignInst{Dest: mergeValue, Src: l.lastStatement})
		}
		thenReaches := l.currentBlock.Terminator == nil
		if thenReaches {
			l.terminate(JumpTerminator{Target: mergeBB.ID})
		}

		// Else block
		l.currentBlock = elseBB
		l.lastStatement = nil
		l.pushScope()
		if s.Alternative != nil {
			for _, st := range s.Alternative.Statements {
				l.lowerStatement(st)
			}
		}
		l.popScope()
		if l.lastStatement != nil {
			l.emit(AssignInst{Dest: mergeValue, Src: l.lastStatement})
		}
		elseReaches := l.currentBlock.Terminator == nil
		if elseReaches {
			l.terminate(JumpTerminator{Target: mergeBB.ID})
		}

		// Continue in merge block
		l.currentBlock = mergeBB

		// The merge value is what the `if` produced — but only if some arm
		// actually reaches the merge block.
		//
		// An `if` whose arms both `return` never gets here: the merge block is
		// unreachable, and nothing ever writes the merge temporary. Reporting it
		// anyway made the function's return value a temp defined in no block, and
		// the module was rejected with "use of undefined value '%t2'" on a
		// `ret i64 %t2` that looks like any other return.
		//
		// It is not only a module problem. An `if` where both arms exit has no
		// value — there is no execution that reaches past it — so reporting one is
		// claiming something the program cannot produce.
		reachesMerge := thenReaches || elseReaches
		if reachesMerge {
			l.lastStatement = mergeValue
		} else {
			l.lastStatement = nil
		}

	case *hir.WhileStmt:
		// Hoisted bindings run once, before the loop header. They are emitted
		// into whatever block is current, which is the one immediately preceding
		// the loop — the only position where "once per loop entry" is the same
		// statement as "once, here".
		for _, hoisted := range s.PreLoop {
			l.lowerStatement(hoisted)
		}

		condBB := l.newBlock("bb_while_cond")
		bodyBB := l.newBlock("bb_while_body")
		exitBB := l.newBlock("bb_while_exit")

		l.terminate(JumpTerminator{Target: condBB.ID})

		// Condition block
		l.currentBlock = condBB
		condOp := l.lowerExpression(s.Condition)
		l.terminate(BranchTerminator{
			Cond:        condOp,
			TrueTarget:  bodyBB.ID,
			FalseTarget: exitBB.ID,
		})

		// Body block
		l.currentBlock = bodyBB
		l.pushScope()
		if s.Body != nil {
			for _, st := range s.Body.Statements {
				l.lowerStatement(st)
			}
		}
		l.popScope()
		if l.currentBlock.Terminator == nil {
			l.terminate(JumpTerminator{Target: condBB.ID})
		}

		// Continue in exit block
		l.currentBlock = exitBB
	}
}

// lowerAssignment lowers `target = value` for every shape the target can take.
//
// The three shapes are a variable, a subscript and a member. They were one shape —
// a name — and the other two were dropped: an `AssignStmt` whose left was an index
// or a member reached this function with an empty name, was stored into a slot
// called "", and the write silently went nowhere. `a[1] = 9` left `a` alone and
// the next read answered 2.
func (l *Lowerer) lowerAssignment(s *hir.AssignStmt) {
	value := l.lowerExpression(s.Value)

	switch target := s.Target.(type) {
	case *hir.VarRef:
		// An assignment writes the currently bound version, so a shadowed
		// name updates the shadow and an outer name updates the outer
		// binding — never a stale slot. Unmapped names keep their spelling
		// (parameters and globals the backend resolves itself).
		if v, ok := l.lookup(target.Name); ok {
			l.emit(StoreVarInst{Name: v, Src: value})
		} else {
			l.emit(StoreVarInst{Name: target.Name, Src: value})
		}

	case *hir.IndexExpr:
		container := l.lowerExpression(target.Left)
		key := l.lowerExpression(target.Index)
		dest := l.newTemp()
		l.emit(SetIndexInst{Dest: dest, Container: container, Key: key, Value: value})
		l.lastStatement = dest

	case *hir.DotExpr:
		// The parser has already folded `m.name = v` into a subscript whose key is
		// the string `"name"`, so a member assignment and a subscript assignment
		// are the same instruction by the time they get here. Handling `DotExpr`
		// anyway costs nothing and keeps MIR honest if that ever changes.
		container := l.lowerExpression(target.Left)
		dest := l.newTemp()
		l.emit(SetIndexInst{
			Dest:      dest,
			Container: container,
			Key:       ConstOperand{Value: target.Field},
			Value:     value,
		})
		l.lastStatement = dest

	default:
		// Nothing else is assignable in Nilang, and the checker has already said
		// so. Lowering it to a store into a fabricated name would hide that; a
		// comment records the gap instead.
		l.emit(StoreVarInst{Name: "?", Src: value})
		l.lastStatement = nil
	}
}

func (l *Lowerer) lowerComponentFunction(name string, parameters []string, body *hir.BlockStmt) string {
	outerFunction := l.currentFn
	outerBlock := l.currentBlock
	outerDepth := len(l.scopes)
	// A function body's last expression is the *function's* value, not the value of
	// whatever statement declared the function. Saving and restoring it is what
	// keeps `fn f() { g(); }` from making the enclosing program's answer be
	// whatever `g` produced.
	outerStatement := l.lastStatement

	function := &Function{Name: name, Params: append([]string(nil), parameters...), Blocks: []*BasicBlock{}}
	l.currentFn = function
	l.currentBlock = l.newBlock("bb_" + name)
	// Parameters bind raw in the function's own frame; body lets that shadow
	// them (or outer names) version from there.
	l.pushScope()
	for _, p := range parameters {
		l.bind(p)
	}
	// A block's value is its last statement's value, whatever kind that statement
	// is. The evaluator says so — `evalBlockStatement` returns whatever the last
	// statement produced — and so does the VM, which leaves it on the stack.
	//
	// A function body was returning nil for any body that ended in an expression
	// rather than a `return`, because the loop lowered the statements and then
	// returned nothing. `fn pick(c: Bool) -> Int { if c { 1 } else { 2 } }` lowered
	// to two empty blocks and an entry that returned nil: correct for the
	// interpreter, wrong for every compiled program, and invisible in the MIR
	// because the blocks looked plausible.
	//
	// Reset per statement, so that a trailing `let` yields nothing rather than
	// whatever it was written after — the same rule the program's top level uses,
	// and for the same reason.
	var bodyValue Operand
	if body != nil {
		for _, statement := range body.Statements {
			l.lastStatement = nil
			l.lowerStatement(statement)
			bodyValue = l.lastStatement
		}
	}
	if l.currentBlock.Terminator == nil {
		l.terminate(ReturnTerminator{Value: bodyValue})
	}
	l.currentProg.Functions[name] = function
	l.currentFn = outerFunction
	l.currentBlock = outerBlock
	l.lastStatement = outerStatement
	l.scopes = l.scopes[:outerDepth]
	return name
}

// lastDestination is the temp the most recently emitted instruction wrote to.
//
// It exists because a construct that ends in an expression has to *name* that
// expression's value, and the lowering has no other way to ask. Reading the last
// block's instruction list would be the same answer with more coupling to how
// instructions are stored.
func (l *Lowerer) lastDestination() Operand {
	block := l.currentBlock
	if block == nil || len(block.Instructions) == 0 {
		return ConstOperand{Value: nil}
	}
	last := block.Instructions[len(block.Instructions)-1]
	switch i := last.(type) {
	case AssignInst:
		return i.Dest
	case BinaryOpInst:
		return i.Dest
	case UnaryOpInst:
		return i.Dest
	case CallInst:
		return i.Dest
	case SelectInst:
		return i.Dest
	case LoadVarInst:
		return i.Dest
	}
	return ConstOperand{Value: nil}
}

func (l *Lowerer) lowerExpression(expr hir.Expression) Operand {
	if expr == nil {
		return ConstOperand{Value: nil}
	}

	switch e := expr.(type) {
	case *hir.IntLit:
		return ConstOperand{Value: e.Val}

	case *hir.FloatLit:
		return ConstOperand{Value: e.Val}

	case *hir.StringLit:
		return ConstOperand{Value: e.Val}

	case *hir.BoolLit:
		return ConstOperand{Value: e.Val}

	case *hir.NullLit:
		return ConstOperand{Value: nil}

	case *hir.VarRef:
		// A name that is a *function* is a value referring to that function, not a
		// slot to read.
		//
		// It used to load unconditionally, so `let apply = twice;` became
		// `_t = load twice` and the emitter failed with `loading "twice", which has
		// no slot` — a name that is not a variable being read as one. The fix has to
		// be here rather than in the emitter: the emitter sees a `VarOperand` and a
		// load and cannot tell which the program meant, because by then they are
		// different instructions that happen to carry the same name.
		//
		// Returning the `VarOperand` directly is what `FnLit` already does for a
		// declaration, so a function read and a function declared arrive in the same
		// shape and there is one place that knows what to do with it.
		if fn, isFunction := l.currentProg.Functions[e.Name]; isFunction && fn != nil {
			return VarOperand{Name: e.Name}
		}

		// A shadowed name reads its current version; anything else keeps its
		// spelling for the backend to resolve (parameters, globals).
		if v, ok := l.lookup(e.Name); ok {
			t := l.newTemp()
			l.emit(LoadVarInst{Dest: t, Name: v})
			return t
		}

		t := l.newTemp()
		l.emit(LoadVarInst{Dest: t, Name: e.Name})
		return t

	case *hir.BinaryExpr:
		leftOp := l.lowerExpression(e.Left)
		rightOp := l.lowerExpression(e.Right)
		dest := l.newTemp()
		l.emit(BinaryOpInst{
			Dest:  dest,
			Op:    e.Op,
			Left:  leftOp,
			Right: rightOp,
		})
		return dest

	case *hir.BlockExpr:
		// A block's value is its last expression. Lowering it in statement
		// position and returning the last statement's destination is what makes
		// `let x = v; body` behave like the two statements it actually is, rather
		// than becoming a construct of its own that only its producer understands.
		if len(e.Statements) == 0 {
			l.emit(AssignInst{Dest: l.newTemp(), Src: ConstOperand{Value: nil}})
			return l.lastDestination()
		}
		l.pushScope()
		for _, st := range e.Statements {
			if _, isExpr := st.(*hir.ExprStmt); isExpr {
				continue
			}
			l.lowerStatement(st)
		}
		// The value expression reads block bindings, so it lowers before the
		// scope closes; otherwise a shadowed name would resolve outside.
		for i := len(e.Statements) - 1; i >= 0; i-- {
			if st, isExpr := e.Statements[i].(*hir.ExprStmt); isExpr {
				dest := l.lowerExpression(st.Expr)
				l.popScope()
				l.emit(AssignInst{Dest: l.newTemp(), Src: dest})
				return l.lastDestination()
			}
		}
		l.popScope()
		l.emit(AssignInst{Dest: l.newTemp(), Src: ConstOperand{Value: nil}})
		return l.lastDestination()

	case *hir.IfExpr:
		cond := l.lowerExpression(e.Condition)
		whenTrue := l.lowerExpression(e.WhenTrue)
		whenFalse := l.lowerExpression(e.WhenFalse)
		dest := l.newTemp()
		l.emit(SelectInst{
			Dest:  dest,
			Cond:  cond,
			True:  whenTrue,
			False: whenFalse,
		})
		return dest

	case *hir.UnaryExpr:
		rightOp := l.lowerExpression(e.Right)
		dest := l.newTemp()
		l.emit(UnaryOpInst{
			Dest:  dest,
			Op:    e.Op,
			Right: rightOp,
		})
		return dest

	case *hir.CallExpr:
		calleeName := ""
		if v, ok := e.Callee.(*hir.VarRef); ok {
			calleeName = v.Name
		} else if e.Callee != nil {
			calleeName = e.Callee.String()
		}
		var args []Operand
		for _, a := range e.Args {
			args = append(args, l.lowerExpression(a))
		}
		dest := l.newTemp()
		l.emit(CallInst{
			Dest:   dest,
			Callee: calleeName,
			Args:   args,
		})
		return dest

	case *hir.FnLit:
		// A named function keeps its name. It used to become `fn_anon_N`, so
		// every named function in the program reached MIR under a different name
		// than the one a call site used — and the call site, having no function
		// under *its* name, failed with "undefined function add in MIR call".
		//
		// The name has to be unique across the program, because MIR's `Functions`
		// is keyed by it. Two functions sharing a name would silently overwrite
		// each other, so a collision gets a counter suffix — but a collision is
		// rare enough that the common case should stay readable in the MIR dump.
		fnName := e.Name
		if fnName == "" {
			fnName = fmt.Sprintf("fn_anon_%d", l.tempCounter)
			l.tempCounter++
		} else if _, taken := l.currentProg.Functions[fnName]; taken {
			fnName = fmt.Sprintf("%s$%d", e.Name, l.tempCounter)
			l.tempCounter++
		}

		fn := &Function{
			Name:   fnName,
			Params: e.Params,
			Blocks: []*BasicBlock{},
			IsPure: e.IsPure,
		}
		if l.currentProg != nil {
			l.currentProg.Functions[fnName] = fn
		}

		// A foreign function is a signature and nothing else. It gets no entry
		// block and no terminator, because there is nothing to execute — a body
		// here would be emitted as a definition and the linker would find a
		// duplicate symbol for a function that already exists elsewhere.
		//
		// It returns a variable operand rather than a temp, so a declaration
		// reads the same in MIR as it does in the source: `extern "C" fn sin(x)
		// -> Float;` is a fact about the program, not a computation.
		if e.ExternAbi != "" {
			fn.Foreign = true
			fn.Abi = e.ExternAbi
			fn.ReturnType = e.ExternReturn
			fn.ParamTypes = e.ExternParams
			fn.Blocks = nil
			return VarOperand{Name: fnName}
		}

		prevFn := l.currentFn
		prevBB := l.currentBlock
		prevDepth := len(l.scopes)

		l.currentFn = fn
		l.currentBlock = l.newBlock("bb_entry")
		l.pushScope()
		for _, p := range fn.Params {
			l.bind(p)
		}

		// The same rule as a component function's body: a block's value is its
		// last statement's value. This is the path a top-level `fn` takes — the
		// other one handles component methods — so a function whose body ended in
		// an expression returned nil here and answered correctly there, which is
		// two copies of one rule and therefore two chances to get it wrong.
		//
		// They did, which is how `fn pick(c) { if c { 1 } else { 2 } }` came to
		// return nil from a compiled program while the interpreter returned 1.
		outerStatement := l.lastStatement
		var bodyValue Operand
		if e.Body != nil {
			for _, st := range e.Body.Statements {
				l.lastStatement = nil
				l.lowerStatement(st)
				bodyValue = l.lastStatement
			}
		}
		if l.currentBlock != nil && l.currentBlock.Terminator == nil {
			l.terminate(ReturnTerminator{Value: bodyValue})
		}

		l.currentFn = prevFn
		l.currentBlock = prevBB
		l.lastStatement = outerStatement
		l.scopes = l.scopes[:prevDepth]

		return VarOperand{Name: fnName}

	case *hir.ListLit:
		var elems []Operand
		for _, el := range e.Elements {
			elems = append(elems, l.lowerExpression(el))
		}
		dest := l.newTemp()
		l.emit(AggregateInst{Dest: dest, Elems: elems})
		return dest

	case *hir.IndexExpr:
		leftOp := l.lowerExpression(e.Left)
		idxOp := l.lowerExpression(e.Index)
		dest := l.newTemp()
		l.emit(BinaryOpInst{
			Dest:  dest,
			Op:    "[]",
			Left:  leftOp,
			Right: idxOp,
		})
		return dest

	case *hir.DotExpr:
		leftOp := l.lowerExpression(e.Left)
		dest := l.newTemp()
		l.emit(BinaryOpInst{
			Dest:  dest,
			Op:    ".",
			Left:  leftOp,
			Right: ConstOperand{Value: e.Field},
		})
		return dest

	case *hir.MapLit:
		var args []Operand
		for i := range e.Keys {
			args = append(args, l.lowerExpression(e.Keys[i]))
			if i < len(e.Values) {
				args = append(args, l.lowerExpression(e.Values[i]))
			}
		}
		dest := l.newTemp()
		l.emit(AggregateInst{Dest: dest, Hash: true, Elems: args})
		return dest

	default:
		return ConstOperand{Value: nil}
	}
}
