package oracle

import (
	"fmt"
	"strings"
	"sync"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/diagnostics"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/compiler/typecheck"
	"github.com/joysriramsarkar/nilLang/compiler/types"
)

// State classifies the maturity level of a symbol or component
type State string

const (
	StateKnown        State = "KNOWN"
	StateExperimental State = "EXPERIMENTAL"
	StateVerified     State = "VERIFIED"
	StateStable       State = "STABLE"
)

// Symbol represents reflective metadata about an identifier
type Symbol struct {
	Name         string     `json:"name"`
	Kind         string     `json:"kind"` // "TYPE", "FUNCTION", "VARIABLE", "BUILTIN"
	Type         types.Type `json:"type"`
	TypeSig      string     `json:"type_signature"`
	Doc          string     `json:"doc"`
	State        State      `json:"state"`
	Effects      []string   `json:"effects,omitempty"`
	Capabilities []string   `json:"capabilities,omitempty"`
	Parameters   []string   `json:"parameters,omitempty"`
	ReturnType   string     `json:"return_type,omitempty"`
}

type CheckResult struct {
	Valid        bool     `json:"valid"`
	InferredType string   `json:"inferred_type,omitempty"`
	Diagnostics  []string `json:"diagnostics,omitempty"`
}

// CompilerOracle provides ground-truth reflection to AI models and developers
type CompilerOracle struct {
	mu           sync.RWMutex
	symbols      map[string]Symbol
	typeRegistry map[string]types.Type
	contracts    []Contract
}

func NewCompilerOracle() *CompilerOracle {
	o := &CompilerOracle{
		symbols:      make(map[string]Symbol),
		typeRegistry: make(map[string]types.Type),
		contracts:    []Contract{},
	}
	o.initBuiltinSymbols()
	return o
}

func (o *CompilerOracle) initBuiltinSymbols() {
	// Register standard types
	stdTypes := []types.Type{
		types.Int, types.Float, types.String, types.Bool,
		types.Byte, types.Null, types.Void, types.Any,
	}
	for _, t := range stdTypes {
		o.typeRegistry[t.String()] = t
		o.RegisterSymbol(Symbol{
			Name:    t.String(),
			Kind:    "TYPE",
			Type:    t,
			TypeSig: "type",
			Doc:     fmt.Sprintf("Primitive type %s", t.String()),
			State:   StateStable,
		})
	}

	// Builtin functions
	builtins := []struct {
		name       string
		sig        string
		doc        string
		effects    []string
		params     []string
		returnType string
	}{
		{"puts", "fn(...Any) -> Void [io]", "Prints expressions to standard output with newline", []string{"io"}, []string{"...args: Any"}, "Void"},
		{"println", "fn(...Any) -> Void [io]", "Prints expressions to standard output with newline", []string{"io"}, []string{"...args: Any"}, "Void"},
		{"print", "fn(...Any) -> Void [io]", "Prints expressions without newline", []string{"io"}, []string{"...args: Any"}, "Void"},
		{"len", "fn(List | String | Hash) -> Int [pure]", "Returns length of collection or string", []string{"pure"}, []string{"collection: List | String | Hash"}, "Int"},
		{"push", "fn(List, Any) -> List [pure]", "Appends element to list and returns new list", []string{"pure"}, []string{"array: List", "item: Any"}, "List"},
		{"first", "fn(List) -> Any [pure]", "Returns first element of list", []string{"pure"}, []string{"array: List"}, "Any"},
		{"last", "fn(List) -> Any [pure]", "Returns last element of list", []string{"pure"}, []string{"array: List"}, "Any"},
		{"rest", "fn(List) -> List [pure]", "Returns elements of list after the first", []string{"pure"}, []string{"array: List"}, "List"},
		{"assert", "fn(Bool, String) -> Void [pure]", "Asserts condition or raises panic", []string{"pure"}, []string{"condition: Bool", "msg: String"}, "Void"},
		{"input", "fn(String) -> String [io]", "Reads line of user input from stdin", []string{"io"}, []string{"prompt: String"}, "String"},
		{"time", "fn() -> Int [pure]", "Returns current Unix timestamp in milliseconds", []string{"pure"}, []string{}, "Int"},
	}

	for _, b := range builtins {
		parsedT, _ := types.Parse(b.sig)
		o.RegisterSymbol(Symbol{
			Name:       b.name,
			Kind:       "BUILTIN",
			Type:       parsedT,
			TypeSig:    b.sig,
			Doc:        b.doc,
			State:      StateStable,
			Effects:    b.effects,
			Parameters: b.params,
			ReturnType: b.returnType,
		})
	}
}

func (o *CompilerOracle) RegisterSymbol(s Symbol) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if s.State == "" {
		s.State = StateKnown
	}
	o.symbols[s.Name] = s
}

func (o *CompilerOracle) ListTypes() []string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	res := make([]string, 0, len(o.typeRegistry))
	for name := range o.typeRegistry {
		res = append(res, name)
	}
	return res
}

func (o *CompilerOracle) ListFunctions() []Symbol {
	o.mu.RLock()
	defer o.mu.RUnlock()
	var res []Symbol
	for _, s := range o.symbols {
		if s.Kind == "FUNCTION" || s.Kind == "BUILTIN" {
			res = append(res, s)
		}
	}
	return res
}

func (o *CompilerOracle) FindSymbol(name string) (*Symbol, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	s, ok := o.symbols[name]
	if !ok {
		return nil, false
	}
	return &s, true
}

func (o *CompilerOracle) InspectType(typeName string) (types.Type, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	if t, ok := o.typeRegistry[typeName]; ok {
		return t, nil
	}
	return types.Parse(typeName)
}

func (o *CompilerOracle) CheckExpression(exprSource string) (*CheckResult, error) {
	l := lexer.New(exprSource)
	p := parser.New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		return &CheckResult{
			Valid:       false,
			Diagnostics: p.Errors(),
		}, nil
	}

	checker := typecheck.NewChecker()
	ok := checker.CheckProgram(prog)

	var diagStrings []string
	for _, d := range checker.Diagnostics {
		diagStrings = append(diagStrings, d.String())
	}

	inferred := "Void"
	if len(prog.Statements) > 0 {
		if es, isExpr := prog.Statements[0].(*ast.ExpressionStatement); isExpr {
			// Lower and type inspect
			_ = es
			inferred = "Expression"
		}
	}

	return &CheckResult{
		Valid:        ok,
		InferredType: inferred,
		Diagnostics:  diagStrings,
	}, nil
}

func (o *CompilerOracle) CheckCapability(capName string, allowedCaps []string) bool {
	norm := strings.ToLower(strings.TrimSpace(capName))
	for _, a := range allowedCaps {
		if strings.ToLower(strings.TrimSpace(a)) == norm {
			return true
		}
	}
	return false
}

func (o *CompilerOracle) ExplainError(errCode string, details string) string {
	base := diagnostics.ExplainCode(errCode)
	if details != "" {
		return fmt.Sprintf("%s Context: %s", base, details)
	}
	return base
}

func (o *CompilerOracle) GenerateSignature(symbolName string) string {
	sym, ok := o.FindSymbol(symbolName)
	if !ok {
		return fmt.Sprintf("// Symbol %s not found in Language Oracle", symbolName)
	}
	return fmt.Sprintf("%s: %s (State: %s)", sym.Name, sym.TypeSig, sym.State)
}
