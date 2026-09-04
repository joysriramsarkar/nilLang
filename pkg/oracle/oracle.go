package oracle

import (
	"fmt"
	"strings"
)

// Effect represents an operational effect of a function or expression
type Effect string

const (
	EffectPure    Effect = "PURE"
	EffectIO      Effect = "IO"
	EffectMutates Effect = "MUTATES"
	EffectAsync   Effect = "ASYNC"
	EffectSyscall Effect = "SYSCALL"
)

// SymbolKind represents the classification of a symbol
type SymbolKind string

const (
	KindVariable SymbolKind = "VARIABLE"
	KindFunction SymbolKind = "FUNCTION"
	KindType     SymbolKind = "TYPE"
	KindModule   SymbolKind = "MODULE"
	KindBuiltin  SymbolKind = "BUILTIN"
)

// Symbol represents metadata about a language identifier
type Symbol struct {
	Name       string     `json:"name"`
	Kind       SymbolKind `json:"kind"`
	TypeSig    string     `json:"type_signature"`
	Doc        string     `json:"doc"`
	Effects    []Effect   `json:"effects"`
	Parameters []string   `json:"parameters,omitempty"`
	ReturnType string     `json:"return_type,omitempty"`
}

// Contract represents an invariant or precondition
type Contract struct {
	TargetSymbol string `json:"target_symbol"`
	Condition    string `json:"condition"`
	Description  string `json:"description"`
}

// LanguageOracle provides language-level truth and verification
type LanguageOracle struct {
	symbols   map[string]Symbol
	contracts []Contract
}

// NewLanguageOracle creates a language oracle initialized with standard NilLang truth
func NewLanguageOracle() *LanguageOracle {
	o := &LanguageOracle{
		symbols:   make(map[string]Symbol),
		contracts: []Contract{},
	}
	o.initBuiltinSymbols()
	return o
}

func (o *LanguageOracle) initBuiltinSymbols() {
	// Standard types
	types := []string{"Int", "Float", "String", "Bool", "List", "Hash", "Null", "Function"}
	for _, t := range types {
		o.RegisterSymbol(Symbol{
			Name:       t,
			Kind:       KindType,
			TypeSig:    "type",
			Doc:        fmt.Sprintf("Primitive type %s", t),
			Effects:    []Effect{EffectPure},
			ReturnType: t,
		})
	}

	// Builtin functions
	o.RegisterSymbol(Symbol{
		Name:       "puts",
		Kind:       KindBuiltin,
		TypeSig:    "fn(...Any) -> Null",
		Doc:        "Prints expressions to standard output with a newline",
		Effects:    []Effect{EffectIO},
		Parameters: []string{"...args: Any"},
		ReturnType: "Null",
	})

	o.RegisterSymbol(Symbol{
		Name:       "len",
		Kind:       KindBuiltin,
		TypeSig:    "fn(List | String | Hash) -> Int",
		Doc:        "Returns the length of a string, array, or hash",
		Effects:    []Effect{EffectPure},
		Parameters: []string{"collection: List | String | Hash"},
		ReturnType: "Int",
	})

	o.RegisterSymbol(Symbol{
		Name:       "push",
		Kind:       KindBuiltin,
		TypeSig:    "fn(List, Any) -> List",
		Doc:        "Appends an element to an array returning a new array",
		Effects:    []Effect{EffectPure},
		Parameters: []string{"array: List", "element: Any"},
		ReturnType: "List",
	})

	o.RegisterSymbol(Symbol{
		Name:       "first",
		Kind:       KindBuiltin,
		TypeSig:    "fn(List) -> Any",
		Doc:        "Returns first element of a list",
		Effects:    []Effect{EffectPure},
		Parameters: []string{"array: List"},
		ReturnType: "Any",
	})

	o.RegisterSymbol(Symbol{
		Name:       "last",
		Kind:       KindBuiltin,
		TypeSig:    "fn(List) -> Any",
		Doc:        "Returns last element of a list",
		Effects:    []Effect{EffectPure},
		Parameters: []string{"array: List"},
		ReturnType: "Any",
	})

	o.RegisterSymbol(Symbol{
		Name:       "rest",
		Kind:       KindBuiltin,
		TypeSig:    "fn(List) -> List",
		Doc:        "Returns elements of a list except the first",
		Effects:    []Effect{EffectPure},
		Parameters: []string{"array: List"},
		ReturnType: "List",
	})
}

// RegisterSymbol registers a symbol with the oracle
func (o *LanguageOracle) RegisterSymbol(s Symbol) {
	o.symbols[s.Name] = s
}

// LookupSymbol finds a symbol
func (o *LanguageOracle) LookupSymbol(name string) (Symbol, bool) {
	s, ok := o.symbols[name]
	return s, ok
}

// AddContract adds a code contract
func (o *LanguageOracle) AddContract(c Contract) {
	o.contracts = append(o.contracts, c)
}

// ExplainError gives AI and human-readable diagnostic analysis of a compiler error
func (o *LanguageOracle) ExplainError(errText string) string {
	lower := strings.ToLower(errText)
	switch {
	case strings.Contains(lower, "type mismatch"):
		return "Type Mismatch: NilLang strictly checks types at evaluation/compilation. Ensure operand types match the operator or function signature."
	case strings.Contains(lower, "identifier not found"):
		return "Unresolved Symbol: The identifier has not been bound in the local or enclosing scope. Check variable naming or import statements."
	case strings.Contains(lower, "wrong number of arguments"):
		return "Arity Error: The number of arguments passed in call expression does not match function parameter count."
	case strings.Contains(lower, "not a function"):
		return "Invocation Error: Attempted to call an expression that does not evaluate to a callable function."
	default:
		return fmt.Sprintf("Compiler diagnostic: %s", errText)
	}
}

// VerifyCall checks if a function call matches known signatures
func (o *LanguageOracle) VerifyCall(fnName string, argCount int) error {
	sym, ok := o.LookupSymbol(fnName)
	if !ok {
		return fmt.Errorf("unknown symbol: %s", fnName)
	}

	if sym.Kind != KindFunction && sym.Kind != KindBuiltin {
		return fmt.Errorf("symbol %s is not callable (is %s)", fnName, sym.Kind)
	}

	// For variadic functions like puts
	if len(sym.Parameters) > 0 && strings.HasPrefix(sym.Parameters[0], "...") {
		return nil
	}

	if len(sym.Parameters) != argCount {
		return fmt.Errorf("function %s expects %d argument(s), got %d", fnName, len(sym.Parameters), argCount)
	}

	return nil
}
