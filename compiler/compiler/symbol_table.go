package compiler

type SymbolScope string

const (
	LocalScope    SymbolScope = "LOCAL"
	GlobalScope   SymbolScope = "GLOBAL"
	BuiltinScope  SymbolScope = "BUILTIN"
	FreeScope     SymbolScope = "FREE"
	FunctionScope SymbolScope = "FUNCTION"
)

type Symbol struct {
	Name     string
	Scope    SymbolScope
	Index    int
	Constant bool
	// LoopFresh marks a top-level loop-body binding. Each pass declares a
	// fresh binding (SEMANTICS.md §4.2 rule 2), so a closure created inside
	// the body must capture that pass's cell rather than reading the shared
	// slot live. Resolve free-converts marked globals; the VM installs a
	// fresh cell per pass (OpSetGlobalFresh) and shares the current one at
	// creation (OpBoxGlobal). Plain top-level bindings are unmarked: they
	// keep direct shared-global access, which is what makes top-level
	// mutation through closures work.
	LoopFresh bool
	// Module is the import generation that defined the binding.
	//
	// Every `import` of a file module inlines that module's top-level
	// declarations into one shared global scope. Without a generation stamp,
	// two modules declaring the same name (for example both `std/crypto/hash`
	// and `std/crypto/hmac` declaring `sha256`) silently share one slot: the
	// second module overwrites the first, the module-alias snapshot misses or
	// misbinds the name, and internal calls compiled against the slot resolve
	// to whichever module wrote it last. The evaluator never had this problem
	// because each module evaluates in its own enclosed environment.
	//
	// The zero value is the main program's generation, so programs that import
	// nothing behave exactly as before.
	Module int
}

type SymbolTable struct {
	Outer *SymbolTable

	store          map[string]Symbol
	numDefinitions int

	FreeSymbols []Symbol

	// ModuleGen is the current import generation for this scope.
	//
	// The compiler bumps it while inlining a module so that module's
	// declarations take fresh global slots even when an earlier module used
	// the same names. Enclosed (function-local) scopes inherit the value at
	// creation; locals never collide across modules anyway.
	ModuleGen int

	// Block scopes are tracked, not chained: a `{ ... }` block reuses this
	// table's instruction stream and frame, so a fresh table would restart
	// slot numbering at zero and collide with the enclosing locals. Instead
	// EnterBlock/LeaveBlock snapshot and restore the name bindings while
	// slot indices keep growing monotonically (see Define).
	savedStack []savedBinding
	blockMarks []int
}

// savedBinding records one name overwritten (or introduced) inside a block so
// LeaveBlock can restore the enclosing binding.
type savedBinding struct {
	name string
	old  Symbol
	had  bool
}

// EnterBlock opens a `{ ... }` scope in the current table. Every Define until
// the matching LeaveBlock shadows instead of reusing, even for a name the
// enclosing scope already binds.
func (s *SymbolTable) EnterBlock() {
	s.blockMarks = append(s.blockMarks, len(s.savedStack))
}

// LeaveBlock closes the innermost open block, forgetting its bindings and
// revealing the shadowed ones. Slot indices are NOT rewound: code already
// emitted references them, and the frame is sized for the high-water mark.
func (s *SymbolTable) LeaveBlock() {
	if len(s.blockMarks) == 0 {
		return
	}
	mark := s.blockMarks[len(s.blockMarks)-1]
	s.blockMarks = s.blockMarks[:len(s.blockMarks)-1]
	for i := len(s.savedStack) - 1; i >= mark; i-- {
		sb := s.savedStack[i]
		if sb.had {
			s.store[sb.name] = sb.old
		} else {
			delete(s.store, sb.name)
		}
	}
	s.savedStack = s.savedStack[:mark]
}

// inBlock reports whether a `{ ... }` scope is open in this table.
func (s *SymbolTable) inBlock() bool {
	return len(s.blockMarks) > 0
}

// recordBlockDefine remembers the binding a block-scoped Define is about to
// shadow (or the absence of one) so LeaveBlock can undo it.
func (s *SymbolTable) recordBlockDefine(name string) {
	if existing, ok := s.store[name]; ok {
		s.savedStack = append(s.savedStack, savedBinding{name: name, old: existing, had: true})
	} else {
		s.savedStack = append(s.savedStack, savedBinding{name: name, had: false})
	}
}

func NewSymbolTable() *SymbolTable {
	s := make(map[string]Symbol)
	free := []Symbol{}
	return &SymbolTable{store: s, FreeSymbols: free}
}

func NewEnclosedSymbolTable(outer *SymbolTable) *SymbolTable {
	s := NewSymbolTable()
	s.Outer = outer
	if outer != nil {
		s.ModuleGen = outer.ModuleGen
	}
	return s
}

// Define introduces a binding in this scope.
//
// An existing *builtin* of the same name is shadowed rather than reused. That is
// the difference between `let first = ""` working and it silently referring to
// the `first` builtin: builtins live in the same store, so returning the existing
// symbol made a shadowing declaration write to a local slot while every read went
// to the builtin, and the program failed at run time with an unrelated type
// error.
//
// A redefinition of a *non-builtin* name still returns the existing symbol, which
// is what keeps a variable declared twice in the same block pointing at one
// binding — with one exception: at global scope, a name from an *earlier import
// generation* takes a fresh slot. Inlined modules share one global scope, so
// reusing the slot would let the second module clobber the first and would drop
// the name from the importing module's alias snapshot (whose collection only
// keeps slots defined since the import began).
func (s *SymbolTable) Define(name string) Symbol {
	if existing, ok := s.store[name]; ok && existing.Scope != BuiltinScope {
		if !s.inBlock() && (s.Outer != nil || existing.Module == s.ModuleGen) {
			return existing
		}
	}
	if s.inBlock() {
		// Inside a block a declaration always shadows: recording first keeps
		// LeaveBlock able to reveal the outer binding (or builtin) again.
		s.recordBlockDefine(name)
	}
	symbol := Symbol{Name: name, Index: s.numDefinitions, Module: s.ModuleGen}
	if s.Outer == nil {
		symbol.Scope = GlobalScope
	} else {
		symbol.Scope = LocalScope
	}

	s.store[name] = symbol
	s.numDefinitions++
	return symbol
}

func (s *SymbolTable) DefineConst(name string) Symbol {
	if existing, ok := s.store[name]; ok && existing.Scope != BuiltinScope {
		if !s.inBlock() && (s.Outer != nil || existing.Module == s.ModuleGen) {
			existing.Constant = true
			s.store[name] = existing
			return existing
		}
	}
	if s.inBlock() {
		s.recordBlockDefine(name)
		// Create directly rather than via Define: Define would record the
		// same binding a second time (harmless but noisy).
		symbol := Symbol{Name: name, Index: s.numDefinitions, Module: s.ModuleGen, Constant: true}
		if s.Outer == nil {
			symbol.Scope = GlobalScope
		} else {
			symbol.Scope = LocalScope
		}
		s.store[name] = symbol
		s.numDefinitions++
		return symbol
	}
	symbol := s.Define(name)
	symbol.Constant = true
	s.store[name] = symbol
	return symbol
}

// Names returns every symbol in this scope, keyed by name.
//
// It exists so a backend can report a variable by the name a developer wrote.
// Reaching into `store` is possible within the package, but a symbol table whose
// internals every backend has to know the shape of is one that changes whenever a
// symbol gains a field.
func (s *SymbolTable) Names() map[string]Symbol {
	out := make(map[string]Symbol, len(s.store))
	for name, sym := range s.store {
		out[name] = sym
	}
	return out
}

func (s *SymbolTable) NumDefinitions() int {
	return s.numDefinitions
}

func (s *SymbolTable) Resolve(name string) (Symbol, bool) {
	obj, ok := s.store[name]
	if !ok && s.Outer != nil {
		obj, ok = s.Outer.Resolve(name)
		if !ok {
			return obj, ok
		}

		// Loop-fresh globals are the exception to direct global access: a
		// closure must capture the current pass's cell (see LoopFresh).
		if obj.Scope == GlobalScope && !obj.LoopFresh {
			return obj, ok
		}
		if obj.Scope == BuiltinScope {
			return obj, ok
		}

		free := s.defineFree(obj)
		return free, true
	}
	return obj, ok
}

func (s *SymbolTable) DefineBuiltin(index int, name string) Symbol {
	symbol := Symbol{Name: name, Index: index, Scope: BuiltinScope}
	s.store[name] = symbol
	return symbol
}

func (s *SymbolTable) DefineFunctionName(name string) Symbol {
	symbol := Symbol{Name: name, Index: 0, Scope: FunctionScope}
	s.store[name] = symbol
	return symbol
}

func (s *SymbolTable) defineFree(original Symbol) Symbol {
	s.FreeSymbols = append(s.FreeSymbols, original)

	symbol := Symbol{Name: original.Name, Index: len(s.FreeSymbols) - 1, Constant: original.Constant}
	symbol.Scope = FreeScope

	s.store[original.Name] = symbol
	return symbol
}
