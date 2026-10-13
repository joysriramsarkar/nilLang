package typecheck

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/capability"
	"github.com/joysriramsarkar/nilLang/compiler/diagnostics"
	"github.com/joysriramsarkar/nilLang/compiler/types"
)

type Scope struct {
	parent    *Scope
	variables map[string]types.Type
	constants map[string]bool
	functions map[string]*types.FunctionType
	structs   map[string]*types.StructType
	entities  map[string]*types.EntityType
}

func NewScope(parent *Scope) *Scope {
	return &Scope{
		parent:    parent,
		variables: make(map[string]types.Type),
		constants: make(map[string]bool),
		functions: make(map[string]*types.FunctionType),
		structs:   make(map[string]*types.StructType),
		entities:  make(map[string]*types.EntityType),
	}
}

func (s *Scope) SetVar(name string, t types.Type) {
	s.variables[name] = t
}

func (s *Scope) SetConst(name string, t types.Type) {
	s.variables[name] = t
	s.constants[name] = true
}

func (s *Scope) IsConst(name string) bool {
	if _, ok := s.variables[name]; ok {
		return s.constants[name]
	}
	if s.parent != nil {
		return s.parent.IsConst(name)
	}
	return false
}

func (s *Scope) GetVar(name string) (types.Type, bool) {
	if t, ok := s.variables[name]; ok {
		return t, true
	}
	if s.parent != nil {
		return s.parent.GetVar(name)
	}
	return nil, false
}

func (s *Scope) SetFunc(name string, fn *types.FunctionType) {
	s.functions[name] = fn
}

func (s *Scope) GetFunc(name string) (*types.FunctionType, bool) {
	if fn, ok := s.functions[name]; ok {
		return fn, true
	}
	if s.parent != nil {
		return s.parent.GetFunc(name)
	}
	return nil, false
}

func (s *Scope) SetEntity(name string, ent *types.EntityType) {
	s.entities[name] = ent
}

func (s *Scope) GetEntity(name string) (*types.EntityType, bool) {
	if ent, ok := s.entities[name]; ok {
		return ent, true
	}
	if s.parent != nil {
		return s.parent.GetEntity(name)
	}
	return nil, false
}

type Checker struct {
	currentScope *Scope
	capabilities map[string]bool

	// inPureFn records that the body being checked belongs to a function
	// declared pure fn, which is what makes an effectful call inside it an
	// error rather than an ordinary one.
	inPureFn bool

	// inUnsafeFn records the same for unsafe fn. It suppresses the capability
	// requirement for operations performed inside, because the function has
	// declared that it takes responsibility for them.
	inUnsafeFn bool

	Diagnostics []*diagnostics.Diagnostic

	// types holds every struct and enum the program declares, keyed by name.
	//
	// They live on the checker rather than in a scope because a type is not a
	// value: it has no run-time slot, and scoping it would make a declaration
	// inside a function shadow a file-level one for no reason.
	types map[string]*declaredType

	// variants maps an enum variant name to its enum. Variant names share one
	// namespace across all enums, which is what lets `match` say `Ok(v)`
	// without naming the enum.
	variants map[string]*variant

	// aliases maps an alias name to the type it names.
	aliases map[string]string

	// traits maps a trait name to its declared methods.
	traits map[string]*traitDecl

	// impls maps a type name to the implementation it has.
	//
	// Keyed by type rather than by trait, because a type has at most one impl.
	// Two implementations of the same trait would make "which one runs"
	// unanswerable except by declaration order, which makes the answer depend on
	// where a line was written.
	impls map[string]*implDecl

	// resolutions maps a method call to the generated function it compiles to.
	//
	// Recorded rather than written into the tree, because the lowering pass is
	// the only consumer and a formatter reads the same program. See traits.go.
	resolutions map[ast.Expression]string

	// typeVars holds the type parameters in force while a generic function's body
	// is checked.
	//
	// It is restored rather than cleared when a body ends, because generics nest
	// through ordinary calls: a generic function may call another, and the inner
	// one's parameters must not outlive it.
	typeVars typeVarSet
}

func NewChecker() *Checker {
	c := &Checker{
		currentScope: NewScope(nil),
		capabilities: make(map[string]bool),
		Diagnostics:  []*diagnostics.Diagnostic{},
	}
	c.initBuiltins()
	return c
}

// EnableCapability records that the project has been granted a capability.
//
// The name is normalised on the way in because Nilang has two vocabularies for
// the same capability: a lowercase operation-oriented one (ilesystem) and a
// capitalised resource-oriented one (Filesystem) from the host side. Normalising
// only here would leave the two failing to match, which is a bug that looks like
// a missing grant.
func (c *Checker) EnableCapability(cap string) {
	c.capabilities[capability.NormalizeName(cap)] = true
}

func (c *Checker) HasCapability(cap string) bool {
	return c.capabilities[capability.NormalizeName(cap)]
}

func (c *Checker) registerBuiltin(name string, params []types.Type, minArgs, maxArgs int, ret types.Type, effects ...string) {
	fn := &types.FunctionType{
		Params:     params,
		ReturnType: ret,
		Effects:    effects,
		MinArgs:    minArgs,
		MaxArgs:    maxArgs,
	}
	c.currentScope.SetFunc(name, fn)
	c.currentScope.SetVar(name, fn)
}

func (c *Checker) initBuiltins() {
	// Variadic IO & System
	c.registerBuiltin("print", []types.Type{}, 0, -1, types.Void, "io")
	c.registerBuiltin("println", []types.Type{}, 0, -1, types.Void, "io")
	c.registerBuiltin("puts", []types.Type{}, 0, -1, types.Void, "io")
	c.registerBuiltin("native", []types.Type{types.String}, 1, -1, types.Any, "pure")
	c.registerBuiltin("exec", []types.Type{types.String}, 1, -1, types.Int, "io")

	// Optional Arguments Builtins
	c.registerBuiltin("assert", []types.Type{types.Bool, types.String}, 1, 2, types.Void, "pure")
	c.registerBuiltin("emit", []types.Type{types.Any, types.Any}, 1, 2, types.Any, "ui")
	c.registerBuiltin("trim", []types.Type{types.Any, types.String}, 1, 2, types.String, "pure")
	c.registerBuiltin("split", []types.Type{types.Any, types.String}, 1, 2, &types.GenericType{Base: "List", Parameters: []types.Type{types.String}}, "pure")
	c.registerBuiltin("substr", []types.Type{types.String, types.Int, types.Int}, 2, 3, types.String, "pure")
	c.registerBuiltin("input", []types.Type{types.String}, 0, 1, types.String, "io")
	c.registerBuiltin("listDir", []types.Type{types.String}, 0, 1, &types.GenericType{Base: "List", Parameters: []types.Type{types.String}}, "io")
	c.registerBuiltin("exit", []types.Type{types.Int}, 0, 1, types.Void, "io")

	// 0-argument Builtins
	c.registerBuiltin("time", []types.Type{}, 0, 0, types.Int, "pure")
	c.registerBuiltin("clear", []types.Type{}, 0, 0, types.Void, "io")
	c.registerBuiltin("cwd", []types.Type{}, 0, 0, types.String, "io")
	c.registerBuiltin("None", []types.Type{}, 0, 0, &types.OptionalType{Base: types.Any}, "pure")

	// 1-argument Builtins
	c.registerBuiltin("len", []types.Type{types.Any}, 1, 1, types.Int, "pure")
	c.registerBuiltin("type", []types.Type{types.Any}, 1, 1, types.String, "pure")
	c.registerBuiltin("str", []types.Type{types.Any}, 1, 1, types.String, "pure")
	c.registerBuiltin("int", []types.Type{types.Any}, 1, 1, types.Int, "pure")
	c.registerBuiltin("first", []types.Type{types.Any}, 1, 1, types.Any, "pure")
	c.registerBuiltin("last", []types.Type{types.Any}, 1, 1, types.Any, "pure")
	c.registerBuiltin("rest", []types.Type{types.Any}, 1, 1, types.Any, "pure")
	c.registerBuiltin("lower", []types.Type{types.Any}, 1, 1, types.String, "pure")
	c.registerBuiltin("upper", []types.Type{types.Any}, 1, 1, types.String, "pure")
	c.registerBuiltin("readFile", []types.Type{types.String}, 1, 1, types.String, "io")
	c.registerBuiltin("fileExists", []types.Type{types.String}, 1, 1, types.Bool, "io")
	c.registerBuiltin("isDir", []types.Type{types.String}, 1, 1, types.Bool, "io")
	c.registerBuiltin("makeDir", []types.Type{types.String}, 1, 1, types.Bool, "io")
	c.registerBuiltin("removeFile", []types.Type{types.String}, 1, 1, types.Bool, "io")
	c.registerBuiltin("chdir", []types.Type{types.String}, 1, 1, types.Bool, "io")
	c.registerBuiltin("Channel", []types.Type{types.Int}, 1, 1, types.Any, "concurrency")
	c.registerBuiltin("receive", []types.Type{types.Any}, 1, 1, types.Any, "concurrency")
	c.registerBuiltin("Ok", []types.Type{types.Any}, 1, 1, &types.GenericType{Base: "Result", Parameters: []types.Type{types.Any}}, "pure")
	c.registerBuiltin("Err", []types.Type{types.Any}, 1, 1, &types.GenericType{Base: "Result", Parameters: []types.Type{types.Any}}, "pure")
	c.registerBuiltin("Some", []types.Type{types.Any}, 1, 1, &types.OptionalType{Base: types.Any}, "pure")
	c.registerBuiltin("unwrap", []types.Type{types.Any}, 1, 1, types.Any, "pure")
	c.registerBuiltin("isOk", []types.Type{types.Any}, 1, 1, types.Bool, "pure")
	c.registerBuiltin("isErr", []types.Type{types.Any}, 1, 1, types.Bool, "pure")
	c.registerBuiltin("errValue", []types.Type{types.Any}, 1, 1, types.Any, "pure")
	c.registerBuiltin("unreachable", []types.Type{types.String}, 0, 1, types.Any, "pure")
	c.registerBuiltin("isSome", []types.Type{types.Any}, 1, 1, types.Bool, "pure")
	// `??` and `?.` lower to a call to this, so the checker has to know the
	// name exists — a lowering that emitted a call the checker had never heard of
	// would fail on every program using either operator.
	c.registerBuiltin("isNull", []types.Type{types.Any}, 1, 1, types.Bool, "pure")
	c.registerBuiltin("isNone", []types.Type{types.Any}, 1, 1, types.Bool, "pure")
	c.registerBuiltin("tensorShape", []types.Type{types.Any}, 1, 1, types.Any, "pure")
	c.registerBuiltin("tensorDtype", []types.Type{types.Any}, 1, 1, types.String, "pure")
	c.registerBuiltin("tensorSum", []types.Type{types.Any}, 1, 1, types.Any, "pure")
	// U2 reactivity (Nilang builtins: portable value subset typechecks in both
	// engines; closure-invoking forms evaluate in the evaluator engine).
	c.registerBuiltin("signal", []types.Type{types.Any}, 1, 1, types.Any, "pure")
	c.registerBuiltin("sget", []types.Type{types.Any}, 1, 1, types.Any, "pure")
	c.registerBuiltin("sver", []types.Type{types.Any}, 1, 1, types.Int, "pure")
	c.registerBuiltin("computed", []types.Type{types.Any}, 1, 1, types.Any, "pure")
	c.registerBuiltin("cget", []types.Type{types.Any}, 1, 1, types.Any, "pure")
	c.registerBuiltin("effect", []types.Type{types.Any, types.Any}, 1, 2, types.Any, "pure")
	c.registerBuiltin("dispose", []types.Type{types.Any}, 1, 1, types.Void, "pure")
	c.registerBuiltin("batch", []types.Type{types.Any}, 1, 1, types.Void, "pure")
	c.registerBuiltin("batch_begin", []types.Type{}, 0, 0, types.Void, "pure")
	c.registerBuiltin("batch_end", []types.Type{}, 0, 0, types.Void, "pure")
	c.registerBuiltin("reactiveArray", []types.Type{types.Any}, 0, -1, types.Any, "pure")
	c.registerBuiltin("ra_len", []types.Type{types.Any}, 1, 1, types.Int, "pure")
	c.registerBuiltin("ra_get", []types.Type{types.Any, types.Any}, 2, 2, types.Any, "pure")
	c.registerBuiltin("store", []types.Type{types.Any}, 1, 1, types.Any, "pure")

	// 2-argument Builtins
	c.registerBuiltin("push", []types.Type{types.Any, types.Any}, 2, 2, types.Any, "pure")
	c.registerBuiltin("append", []types.Type{types.Any, types.Any}, 2, 2, types.Any, "pure")
	c.registerBuiltin("join", []types.Type{types.Any, types.String}, 2, 2, types.String, "pure")
	c.registerBuiltin("contains", []types.Type{types.Any, types.Any}, 2, 2, types.Bool, "pure")
	c.registerBuiltin("hasPrefix", []types.Type{types.Any, types.Any}, 2, 2, types.Bool, "pure")
	c.registerBuiltin("hasSuffix", []types.Type{types.Any, types.Any}, 2, 2, types.Bool, "pure")
	c.registerBuiltin("writeFile", []types.Type{types.String, types.String}, 2, 2, types.Bool, "io")
	c.registerBuiltin("send", []types.Type{types.Any, types.Any}, 2, 2, types.Void, "concurrency")
	// withTimeout runs a zero-argument function under a millisecond deadline
	// (CONCURRENCY.md §3.2). The second parameter is Any because a function
	// value has no first-order parameter type here; the evaluator and the VM
	// both refuse a non-function at the call.
	c.registerBuiltin("withTimeout", []types.Type{types.Int, types.Any}, 2, 2, types.Any, "concurrency")
	c.registerBuiltin("unwrapOr", []types.Type{types.Any, types.Any}, 2, 2, types.Any, "pure")
	c.registerBuiltin("tensor", []types.Type{types.Any, types.Any}, 2, 3, types.Any, "pure")
	c.registerBuiltin("tensorGet", []types.Type{types.Any, types.Any}, 2, 2, types.Any, "pure")
	c.registerBuiltin("tensorAdd", []types.Type{types.Any, types.Any}, 2, 2, types.Any, "pure")
	c.registerBuiltin("tensorMul", []types.Type{types.Any, types.Any}, 2, 2, types.Any, "pure")
	c.registerBuiltin("tensorCast", []types.Type{types.Any, types.String}, 2, 2, types.Any, "pure")
	c.registerBuiltin("tensorDot", []types.Type{types.Any, types.Any}, 2, 2, types.Any, "pure")
	c.registerBuiltin("tensorMatmul", []types.Type{types.Any, types.Any}, 2, 2, types.Any, "pure")
	c.registerBuiltin("sset", []types.Type{types.Any, types.Any}, 2, 2, types.Any, "pure")
	c.registerBuiltin("ra_push", []types.Type{types.Any, types.Any}, 2, 2, types.Int, "pure")
	c.registerBuiltin("ra_set", []types.Type{types.Any, types.Any, types.Any}, 3, 3, types.Any, "pure")
	c.registerBuiltin("store_get", []types.Type{types.Any, types.Any}, 2, 2, types.Any, "pure")
	c.registerBuiltin("store_set", []types.Type{types.Any, types.Any, types.Any}, 3, 3, types.Any, "pure")

	// 3-argument Builtins
	c.registerBuiltin("set", []types.Type{types.Any, types.Any, types.Any}, 3, 3, types.Any, "pure")
	c.registerBuiltin("replace", []types.Type{types.Any, types.Any, types.Any}, 3, 3, types.String, "pure")
	c.registerBuiltin("tensorSlice", []types.Type{types.Any, types.Any, types.Any}, 3, 3, types.Any, "pure")
}

func (c *Checker) CheckProgram(prog *ast.Program) bool {
	// Declarations are registered before any statement is checked, so a type may
	// be used above the line that declares it. Traits and impls go in the same
	// pass for the same reason: a method call above the `impl` block must
	// resolve.
	c.registerDeclarations(prog)
	c.registerTraits(prog)
	c.checkImplAgrees()

	for _, stmt := range prog.Statements {
		c.checkStatement(stmt)
	}
	return !c.HasErrors()
}

// HasErrors reports whether any diagnostic is an error.
//
// The count of diagnostics is not the answer, because warnings travel in the same
// list. Counting them as failures would make a warning stop the program, which is
// the opposite of what a warning is for and leaves a developer with no way to say
// something they have been told about and accept.
func (c *Checker) HasErrors() bool {
	for _, d := range c.Diagnostics {
		if d.Severity == diagnostics.SeverityError {
			return true
		}
	}
	return false
}

// Warnings returns the diagnostics that are not errors, in source order.
//
// They are separate from HasErrors rather than folded into it because a caller
// printing diagnostics wants both, and a caller deciding whether to proceed wants
// only one of them.
func (c *Checker) Warnings() []*diagnostics.Diagnostic {
	var out []*diagnostics.Diagnostic
	for _, d := range c.Diagnostics {
		if d.Severity != diagnostics.SeverityError {
			out = append(out, d)
		}
	}
	return out
}

func (c *Checker) checkStatement(stmt ast.Statement) {
	// A visibility marker does not change what a declaration means, only what
	// leaves the module, so the wrapper is transparent to every check here.
	if wrapped, ok := stmt.(*ast.ExportStatement); ok && wrapped.Decl != nil {
		c.checkStatement(wrapped.Decl)
		return
	}

	switch s := stmt.(type) {
	case *ast.TraitDecl:
		// Everything a trait has to be right about — no duplicate methods, no
		// method with a body — was settled at registration.
	case *ast.ImplDecl:
		// Agreement with the trait was checked in checkImplAgrees, over every
		// registration rather than here, so that a trait declared below its impl
		// is still checked against it.
		c.checkImplBodies(s)
	case *ast.ForStatement:
		c.checkForStatement(s)
		return

	case *ast.WhileLetStatement:
		c.checkWhileLet(s)
		return

	case *ast.StructDecl:
		c.checkStructDecl(s)
		return

	case *ast.EnumDecl:
		c.checkEnumDecl(s)
		return

	case *ast.TypeAliasDecl:
		c.checkTypeAliasDecl(s)
		return

	case *ast.ExternStatement:
		// An extern declaration introduces a name, and the name has to type-check at
		// every call site like any other. Registering it with the annotations as
		// written is the whole check: a call that passes the wrong types is then
		// caught here rather than by the FFI at run time, which is the difference
		// between a diagnostic and a corrupted call.
		c.checkExternDecl(s)
		return

	case *ast.ImportStatement:
		if s.Alias != nil {
			c.currentScope.SetVar(s.Alias.Value, types.Any)
		} else if len(s.Names) > 0 {
			for _, ident := range s.Names {
				c.currentScope.SetVar(ident.Value, types.Any)
			}
		} else if s.Path != nil {
			baseName := filepath.Base(s.Path.Value)
			baseName = strings.TrimSuffix(baseName, filepath.Ext(baseName))
			c.currentScope.SetVar(baseName, types.Any)
		}

	case *ast.AppStatement:
		if s.Body != nil {
			// An app body is deliberately NOT a scope: its bindings are
			// program-level (tests pin `app H { let ax = 9; } ax;` and
			// `app Hello { state count } count;`). The evaluator and the VM
			// treat it the same way — see below.
			for _, nested := range s.Body.Statements {
				c.checkStatement(nested)
			}
		}

	case *ast.LetStatement:
		if s.Value == nil {
			c.report("E0103", fmt.Sprintf("Variable %q requires an initializer until definite assignment is supported", s.Name.Value), s.Token.Line, s.Token.Column)
			return
		}
		if fnLit, ok := s.Value.(*ast.FunctionLiteral); ok && s.Name != nil {
			if fnLit.Name == "" {
				fnLit.Name = s.Name.Value
			}
			var placeholderParams []types.Type
			var expectedRet types.Type = types.Any
			if s.Type != "" {
				if pt, err := types.Parse(s.Type); err == nil {
					if ft, ok := pt.(*types.FunctionType); ok {
						placeholderParams = ft.Params
						expectedRet = ft.ReturnType
					}
				}
			}
			if placeholderParams == nil {
				for range fnLit.Parameters {
					placeholderParams = append(placeholderParams, types.Any)
				}
			}
			placeholder := &types.FunctionType{
				Params:     placeholderParams,
				ReturnType: expectedRet,
				MinArgs:    len(placeholderParams),
				MaxArgs:    len(placeholderParams),
			}
			c.currentScope.SetFunc(s.Name.Value, placeholder)
			c.currentScope.SetVar(s.Name.Value, placeholder)
		}
		valType := c.inferExpression(s.Value)
		declarationType := types.Type(valType)
		if s.Type != "" {
			var err error
			declarationType, err = c.resolveType(s.Type)
			if err != nil {
				c.report("E0101", fmt.Sprintf("Invalid type %q for variable %q: %v", s.Type, s.Name.Value, err), s.Token.Line, s.Token.Column)
				return
			}
			// A name that resolves to nothing is reported as such, rather than
			// left to the assignability check below.
			//
			// `types.Parse` is deliberately permissive: an unrecognised name becomes
			// an empty struct rather than an error, so a type from a module the
			// checker has not read still parses. The cost is that `let x: Nope = 1`
			// used to read as a type *mismatch* about `Nope` when the real problem
			// was the name — and inside a union it cost nothing at all, because the
			// other member already accepted the value.
			if !c.isKnownTypeName(s.Type) {
				c.report(CodeUnknownType,
					fmt.Sprintf("unknown type %q for variable %q", s.Type, s.Name.Value),
					s.Token.Line, s.Token.Column)
				return
			}
			if !c.checkAssignable(s.Value, valType, declarationType, s.Token.Line, s.Token.Column,
				fmt.Sprintf("Cannot initialize variable %q of type %s with %s", s.Name.Value, declarationType, valType)) {
				c.report("E0101", fmt.Sprintf("Cannot initialize variable %q of type %s with %s", s.Name.Value, declarationType, valType), s.Token.Line, s.Token.Column)
			}
		}
		if s.Name != nil {
			if s.Constant {
				c.currentScope.SetConst(s.Name.Value, declarationType)
			} else {
				c.currentScope.SetVar(s.Name.Value, declarationType)
			}
			if ft, ok := declarationType.(*types.FunctionType); ok {
				c.currentScope.SetFunc(s.Name.Value, ft)
			} else if ft, ok := valType.(*types.FunctionType); ok {
				c.currentScope.SetFunc(s.Name.Value, ft)
			}
		}

	case *ast.StateDeclaration:
		var valueType types.Type = types.Null
		if s.Value != nil {
			valueType = c.inferExpression(s.Value)
		}
		declarationType := types.Type(valueType)
		if s.Type != "" {
			var err error
			declarationType, err = c.resolveType(s.Type)
			if err != nil {
				c.report("E0101", fmt.Sprintf("Invalid state type %q: %v", s.Type, err), s.Token.Line, s.Token.Column)
				declarationType = types.Any
			} else if s.Value != nil && !c.checkAssignable(s.Value, valueType, declarationType, s.Token.Line, s.Token.Column,
				fmt.Sprintf("Cannot initialize state %q of type %s with %s", s.Name.Value, declarationType, valueType)) {
			}
		}
		if s.Name != nil {
			c.currentScope.SetVar(s.Name.Value, declarationType)
		}

	case *ast.ComponentLiteral:
		if s.Name == nil {
			c.report("E0406", "Component declaration must have a name", s.Token.Line, s.Token.Column)
			return
		}
		c.currentScope.SetVar(s.Name.Value, types.Any)
		outerScope := c.currentScope
		c.currentScope = NewScope(outerScope)
		for _, state := range s.States {
			c.checkStatement(state)
		}
		if s.Render != nil && s.Render.Body != nil {
			for _, nested := range s.Render.Body.Statements {
				c.checkStatement(nested)
			}
		}
		if s.Build != nil && s.Build.Body != nil {
			for _, nested := range s.Build.Body.Statements {
				c.checkStatement(nested)
			}
		}
		for _, handler := range s.Handlers {
			componentScope := c.currentScope
			c.currentScope = NewScope(componentScope)
			for _, parameter := range handler.Parameters {
				c.currentScope.SetVar(parameter.Value, types.Any)
			}
			if handler.Body != nil {
				for _, nested := range handler.Body.Statements {
					c.checkStatement(nested)
				}
			}
			c.currentScope = componentScope
		}
		c.currentScope = outerScope

	case *ast.AssignStatement:
		if s.Name != nil {
			varType, ok := c.currentScope.GetVar(s.Name.Value)
			if !ok {
				c.report("E0102", fmt.Sprintf("Undefined identifier %q in assignment", s.Name.Value), s.Token.Line, s.Token.Column)
				return
			}
			if c.currentScope.IsConst(s.Name.Value) {
				c.report("E0104", fmt.Sprintf("Cannot assign to constant %q", s.Name.Value), s.Token.Line, s.Token.Column)
				return
			}
			value := s.Value
			if s.Operator == "+=" || s.Operator == "-=" {
				value = &ast.InfixExpression{Token: s.Token, Left: s.Name, Operator: strings.TrimSuffix(s.Operator, "="), Right: s.Value}
			}
			valType := c.inferExpression(value)
			// Through `checkAssignable` so that a literal narrows here as it does
			// everywhere else. This site had its own comparison, which is how four
			// sites came to give four answers.
			c.checkAssignable(value, valType, varType, s.Token.Line, s.Token.Column,
				fmt.Sprintf("Cannot assign %s to variable of type %s", valType, varType))
		}

	case *ast.ReturnStatement:
		if s.ReturnValue != nil {
			c.inferExpression(s.ReturnValue)
		}

	case *ast.ExpressionStatement:
		c.inferExpression(s.Expression)

	case *ast.WhileStatement:
		cond := c.inferExpression(s.Condition)
		if !cond.Equals(types.Bool) && !cond.Equals(types.Any) {
			c.report("E0101", fmt.Sprintf("While condition must evaluate to Bool, got %s", cond), s.Token.Line, s.Token.Column)
		}
		if s.Body != nil {
			c.currentScope = NewScope(c.currentScope)
			for _, nested := range s.Body.Statements {
				c.checkStatement(nested)
			}
			c.currentScope = c.currentScope.parent
		}

	case *ast.BlockStatement:
		c.currentScope = NewScope(c.currentScope)
		for _, nested := range s.Statements {
			c.checkStatement(nested)
		}
		c.currentScope = c.currentScope.parent

	case *ast.EntityStatement:
		c.checkEntityStatement(s)

	case *ast.IndexAssignStatement:
		if s.Left != nil {
			_ = c.inferExpression(s.Left)
		}
		if s.Index != nil {
			_ = c.inferExpression(s.Index)
		}
		if s.Value != nil {
			_ = c.inferExpression(s.Value)
		}

	case *ast.StyleStatement:
		// Style statements are declarative blocks; no inner expressions

	default:
		// Unchecked or decorative statement
	}
}

func (c *Checker) checkEntityStatement(s *ast.EntityStatement) {
	if s == nil {
		return
	}
	if s.Name == nil || s.Name.Value == "" {
		c.report("E0401", "Entity declaration must have a valid identifier name", s.Token.Line, s.Token.Column)
		return
	}

	entName := s.Name.Value
	if _, exists := c.currentScope.GetEntity(entName); exists {
		c.report("E0402", fmt.Sprintf("Duplicate entity declaration: %q already exists in scope", entName), s.Token.Line, s.Token.Column)
		return
	}

	fieldDefs := make([]types.EntityFieldDef, 0, len(s.Fields))
	fieldNames := make(map[string]bool)
	primaryKeys := 0

	for _, f := range s.Fields {
		if fieldNames[f.Name] {
			c.report("E0403", fmt.Sprintf("Duplicate field %q in entity %q", f.Name, entName), s.Token.Line, s.Token.Column)
			continue
		}
		fieldNames[f.Name] = true

		if f.IsPrimary {
			primaryKeys++
		}

		ft, err := c.resolveType(f.Type)
		if err != nil {
			c.report("E0404", fmt.Sprintf("Invalid type %q for field %q in entity %q: %v", f.Type, f.Name, entName, err), s.Token.Line, s.Token.Column)
			ft = types.Any
		}

		fieldDefs = append(fieldDefs, types.EntityFieldDef{
			Name:         f.Name,
			Type:         ft,
			IsPrimary:    f.IsPrimary,
			IsRequired:   f.IsRequired,
			IsUnique:     f.IsUnique,
			TargetEntity: f.TargetEntity,
		})
	}

	if primaryKeys > 1 {
		c.report("E0405", fmt.Sprintf("Entity %q has %d primary keys; at most one primary key allowed", entName, primaryKeys), s.Token.Line, s.Token.Column)
	}

	entType := &types.EntityType{
		Name:   entName,
		Fields: fieldDefs,
	}

	c.currentScope.SetEntity(entName, entType)
}

func (c *Checker) inferExpression(expr ast.Expression) types.Type {
	if expr == nil {
		return types.Void
	}

	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return types.Int

	case *ast.FloatLiteral:
		return types.Float

	case *ast.StringLiteral:
		return types.String

	case *ast.CoalesceExpression:
		// `a ?? b` is whichever of the two is present, so it is the left type only
		// when the left is a type a value can actually *be*.
		//
		// `Null` is not one: an expression whose declared type is `Null` evaluates
		// to `b` whenever `a` is absent, so the answer is `b`'s type. Reporting `Null`
		// instead would make `a ?? b ? c : d` fail on its condition, and it would
		// name the wrong half of the expression as the problem.
		left := c.inferExpression(e.Left)
		right := c.inferExpression(e.Right)

		if left != nil && !left.Equals(types.Any) && !left.Equals(types.Null) {
			return left
		}
		if right != nil {
			return right
		}
		return types.Any

	case *ast.StringTemplate:
		for _, p := range e.Parts {
			if p.IsExpression && p.Expression != nil {
				c.inferExpression(p.Expression)
			}
		}
		return types.String

	case *ast.Boolean:
		return types.Bool

	case *ast.NullLiteral:
		return types.Null

	case *ast.Identifier:
		if t, ok := c.currentScope.GetVar(e.Value); ok {
			return t
		}
		if fn, ok := c.currentScope.GetFunc(e.Value); ok {
			return fn
		}
		c.report("E0102", fmt.Sprintf("Undefined identifier %q", e.Value), e.Token.Line, e.Token.Column)
		return types.Any

	case *ast.PrefixExpression:
		right := c.inferExpression(e.Right)
		if e.Operator == "!" {
			if !right.Equals(types.Bool) && !right.Equals(types.Any) {
				c.report("E0101", fmt.Sprintf("Operator ! expected Bool, got %s", right), e.Token.Line, e.Token.Column)
			}
			return types.Bool
		}
		if e.Operator == "-" {
			if !right.Equals(types.Int) && !right.Equals(types.Float) && !right.Equals(types.Any) {
				c.report("E0101", fmt.Sprintf("Operator - expected Int or Float, got %s", right), e.Token.Line, e.Token.Column)
			}
			return right
		}
		return right

	case *ast.InfixExpression:
		switch e.Operator {
		case "+", "-", "*", "/", "%":
			// Widths carry through: `u8 + u8` is `u8`, and anything else falls
			// back to `Int`, which is wider than every width in the language.
			return c.numericArithmetic(e)

		case "==", "!=":
			return types.Bool

		case "<", "<=", ">", ">=":
			return types.Bool

		case "&&", "||":
			return types.Bool
		}
		return types.Any

	case *ast.IfExpression:
		cond := c.inferExpression(e.Condition)
		if !cond.Equals(types.Bool) && !cond.Equals(types.Any) {
			c.report("E0101", fmt.Sprintf("If condition must evaluate to Bool, got %s", cond), e.Token.Line, e.Token.Column)
		}
		var conseqType types.Type = types.Void
		if e.Consequence != nil {
			c.currentScope = NewScope(c.currentScope)
			for i, s := range e.Consequence.Statements {
				if i == len(e.Consequence.Statements)-1 {
					if exprStmt, ok := s.(*ast.ExpressionStatement); ok {
						conseqType = c.inferExpression(exprStmt.Expression)
						continue
					} else if retStmt, ok := s.(*ast.ReturnStatement); ok && retStmt.ReturnValue != nil {
						conseqType = c.inferExpression(retStmt.ReturnValue)
						continue
					}
				}
				c.checkStatement(s)
			}
			c.currentScope = c.currentScope.parent
		}
		var altType types.Type = types.Void
		if e.Alternative != nil {
			c.currentScope = NewScope(c.currentScope)
			for i, s := range e.Alternative.Statements {
				if i == len(e.Alternative.Statements)-1 {
					if exprStmt, ok := s.(*ast.ExpressionStatement); ok {
						altType = c.inferExpression(exprStmt.Expression)
						continue
					} else if retStmt, ok := s.(*ast.ReturnStatement); ok && retStmt.ReturnValue != nil {
						altType = c.inferExpression(retStmt.ReturnValue)
						continue
					}
				}
				c.checkStatement(s)
			}
			c.currentScope = c.currentScope.parent
		}
		if e.Alternative == nil {
			return conseqType
		}
		if conseqType.Equals(altType) {
			return conseqType
		}
		if conseqType.AssignableTo(altType) {
			return altType
		}
		if altType.AssignableTo(conseqType) {
			return conseqType
		}
		return types.NewUnion(conseqType, altType)

	case *ast.FunctionLiteral:
		fnScope := NewScope(c.currentScope)
		var paramTypes []types.Type

		// The type parameters are in force before any annotation is resolved, not
		// just for the body: a parameter's own annotation is where `T` is written
		// most often, and resolving `x: T` before pushing `T` would report the
		// parameter's type as unknown.
		previousTypeVars := c.withTypeParams(e.TypeParams)
		defer func() { c.typeVars = previousTypeVars }()

		// A function declared earlier, or bound through a `let` with a function
		// type, carries parameter types the literal does not repeat. Reusing them
		// is what makes a recursive call checked against its own signature rather
		// than against `Any`.
		var expectedParams []types.Type
		if e.Name != "" {
			if existing, ok := c.currentScope.GetFunc(e.Name); ok && len(existing.Params) == len(e.Parameters) {
				expectedParams = existing.Params
			}
		}

		for i, p := range e.Parameters {
			if p == nil || p.Name == nil {
				continue
			}

			// A written annotation is the source of truth. Without one, the
			// parameter is `Any` — or whatever the enclosing declaration already
			// said about it, which is the recursive case.
			pt := types.Type(types.Any)
			switch {
			case p.TypeAnnotation != "":
				resolved, err := c.resolveType(p.TypeAnnotation)
				switch {
				case err != nil:
					c.report(CodeUnknownType,
						fmt.Sprintf("unknown type %q for parameter %q",
							p.TypeAnnotation, p.Name.Value),
						p.Token.Line, p.Token.Column)
				case !c.isKnownTypeName(p.TypeAnnotation):
					// Parse succeeds for any name, so a typo arrives as an empty
					// struct type rather than an error. Reporting it here is what
					// turns "cannot initialise x of type NoSuchType" into "no such
					// type", which points at the actual mistake.
					c.report(CodeUnknownType,
						fmt.Sprintf("unknown type %q for parameter %q",
							p.TypeAnnotation, p.Name.Value),
						p.Token.Line, p.Token.Column)
				default:
					pt = resolved
				}
			case i < len(expectedParams) && expectedParams[i] != nil:
				pt = expectedParams[i]
			}

			paramTypes = append(paramTypes, pt)
			fnScope.SetVar(p.Name.Value, pt)
		}

		returnType := types.Type(types.Any)
		if e.ReturnType != "" {
			resolved, err := c.resolveType(e.ReturnType)
			switch {
			case err != nil:
				c.report(CodeUnknownType,
					fmt.Sprintf("unknown return type %q for %s",
						e.ReturnType, describeFunction(e.Name)),
					e.Token.Line, e.Token.Column)
			case !c.isKnownTypeName(e.ReturnType):
				c.report(CodeUnknownType,
					fmt.Sprintf("unknown return type %q for %s",
						e.ReturnType, describeFunction(e.Name)),
					e.Token.Line, e.Token.Column)
			default:
				returnType = resolved
			}
		}

		c.checkFunctionRequires(e)
		c.checkModifierUsage(e)

		fnType := &types.FunctionType{
			Params:     paramTypes,
			ReturnType: returnType,
			Effects:    functionEffects(e),
			MinArgs:    len(paramTypes),
			MaxArgs:    len(paramTypes),
			Requires:   e.Requires,
			IsPure:     e.IsPure,
			IsUnsafe:   e.IsUnsafe,
		}
		if e.Name != "" {
			c.currentScope.SetFunc(e.Name, fnType)
			c.currentScope.SetVar(e.Name, fnType)
			fnScope.SetFunc(e.Name, fnType)
			fnScope.SetVar(e.Name, fnType)
		}

		oldScope := c.currentScope
		c.currentScope = fnScope

		// The modifiers apply to this body only, so they are saved and restored
		// rather than left set: a nested closure is checked under its own
		// modifiers, and one with neither is not under its caller's.
		wasPure, wasUnsafe := c.inPureFn, c.inUnsafeFn
		c.inPureFn, c.inUnsafeFn = e.IsPure, e.IsUnsafe

		bodyType := c.checkFunctionBody(e, returnType)

		c.inPureFn, c.inUnsafeFn = wasPure, wasUnsafe
		c.currentScope = oldScope

		if bodyType != nil && e.ReturnType != "" &&
			!types.Any.Equals(returnType) && !types.Void.Equals(returnType) {
			if !bodyType.AssignableTo(returnType) {
				c.report("E0102",
					fmt.Sprintf("%s returns %s but its body produces %s",
						describeFunction(e.Name), returnType, bodyType),
					e.Token.Line, e.Token.Column)
			}
		}

		return fnType

	case *ast.StructLiteral:
		return c.inferStructLiteral(e)

	case *ast.MatchExpression:
		c.checkMatch(e)
		return types.Any

	case *ast.CallExpression:
		// `Circle(7)` is written like a call. If the name is a declared
		// variant it is a value, and checking it as a call would report an
		// undefined function that does not exist.
		if variantType := c.inferEnumVariantCall(e); variantType != nil {
			return variantType
		}

		// A method call resolves against the receiver's static type. It is
		// checked before the callee is inferred, because a DotExpression is not
		// something the rest of this function knows how to type, and because the
		// resolution is what the lowering pass needs from this call.
		var memberLeft ast.Expression
		var member *ast.Identifier

		switch callee := e.Function.(type) {
		case *ast.DotExpression:
			memberLeft, member = callee.Left, callee.Member
		case *ast.SafeAccessExpression:
			// `d?.greet()` names a method just as `d.greet()` does. Only what
			// happens when the receiver is null differs, and that is a run-time
			// question the lowering answers — not a reason to skip the check.
			memberLeft, member = callee.Left, callee.Member
		}

		if member != nil {
			receiver := c.inferExpression(memberLeft)
			c.resolveMethodCall(e.Function, memberLeft, member, receiver)

			for _, arg := range e.Arguments {
				c.inferExpression(arg)
			}
			return c.methodResultType(member, receiver)
		}

		fnType := c.inferExpression(e.Function)
		var fnSig *types.FunctionType

		if ft, ok := fnType.(*types.FunctionType); ok {
			fnSig = ft
		} else if ident, ok := e.Function.(*ast.Identifier); ok {
			if s, found := c.currentScope.GetFunc(ident.Value); found {
				fnSig = s
			}
		}

		if fnType != nil && !fnType.Equals(types.Any) && fnSig == nil {
			c.report("E0106", fmt.Sprintf("cannot call expression of non-function type %s", fnType), e.Token.Line, e.Token.Column)
		}

		if fnSig != nil {
			argCount := len(e.Arguments)
			if !fnSig.CheckArity(argCount) {
				min := fnSig.MinExpectedArgs()
				max := fnSig.MaxExpectedArgs()
				if max == -1 {
					c.report("E0105", fmt.Sprintf("wrong number of arguments to function. got=%d, want at least %d", argCount, min), e.Token.Line, e.Token.Column)
				} else if min == max {
					c.report("E0105", fmt.Sprintf("wrong number of arguments to function. got=%d, want=%d", argCount, min), e.Token.Line, e.Token.Column)
				} else {
					c.report("E0105", fmt.Sprintf("wrong number of arguments to function. got=%d, want=%d..%d", argCount, min, max), e.Token.Line, e.Token.Column)
				}
			}

			// An effectful call inside a pure fn contradicts the declaration,
			// which is the whole point of writing it.
			if c.inPureFn {
				for _, eff := range fnSig.Effects {
					if eff == "pure" {
						continue
					}
					c.report("E0202",
						fmt.Sprintf("cannot call side-effecting function (%s) inside pure context", eff),
						e.Token.Line, e.Token.Column)
				}
			}

			c.checkRequiresClause(fnSig, e)
		}

		// Argument types are collected before any of them are checked against the
		// signature, because a generic function's signature is not usable until the
		// arguments have said what the variables are. `first([1, 2])` against
		// `Array<T>` only means anything once `T` is `Int`.
		argTypes := make([]types.Type, len(e.Arguments))
		genericSignature := fnSig != nil && hasTypeVarIn(fnSig.Params)

		for i, arg := range e.Arguments {
			argType := c.inferExpression(arg)
			argTypes[i] = argType

			// A generic signature is checked after it is instantiated, not here:
			// against the bare `T` every argument would either match or not for a
			// reason that says nothing about the call.
			if genericSignature || fnSig == nil || i >= len(fnSig.Params) {
				continue
			}
			if paramType := fnSig.Params[i]; paramType != nil && !paramType.Equals(types.Any) {
				if !c.checkAssignable(arg, argType, paramType, argLine(arg, e), exprPositionOr(arg, e),
					fmt.Sprintf("cannot pass argument %d of type %s to parameter of type %s", i+1, argType, paramType)) {
				}
			}
		}

		if fnSig != nil {
			// A narrowing builtin reports the type it actually produces rather
			// than the `Any` it is registered with. This runs after instantiation
			// so that a generic whose argument is a Result narrows too.
			fnSig = c.applyNarrowing(e, fnSig)

			// A generic signature is instantiated with what the arguments said, and
			// both the remaining checks and the call's own type come from the
			// instantiated form. Without this the argument check above would
			// compare `Int` against the bare `T` and reject every correct call.
			if instantiated := c.inferCall(fnSig, argTypes); instantiated != nil {
				fnSig = instantiated
				for i, arg := range e.Arguments {
					if i >= len(fnSig.Params) || fnSig.Params[i] == nil {
						continue
					}
					if !argTypes[i].AssignableTo(fnSig.Params[i]) {
						line, col := exprPosition(arg)
						if line == 0 {
							line, col = e.Token.Line, e.Token.Column
						}
						c.report(CodeGenericMismatch,
							fmt.Sprintf("cannot pass argument %d of type %s to parameter of type %s",
								i+1, argTypes[i], fnSig.Params[i]),
							line, col)
					}
				}
			}
		}

		if fnSig != nil && fnSig.ReturnType != nil {
			return fnSig.ReturnType
		}

		return types.Any

	case *ast.ArrayLiteral:
		if len(e.Elements) == 0 {
			return &types.GenericType{Base: "List", Parameters: []types.Type{types.Any}}
		}
		firstElemType := c.inferExpression(e.Elements[0])
		homogeneous := true
		for _, el := range e.Elements[1:] {
			elType := c.inferExpression(el)
			if !elType.Equals(firstElemType) {
				homogeneous = false
			}
		}
		if homogeneous && !firstElemType.Equals(types.Void) {
			return &types.GenericType{Base: "List", Parameters: []types.Type{firstElemType}}
		}
		return &types.GenericType{Base: "List", Parameters: []types.Type{types.Any}}

	case *ast.HashLiteral:
		return c.inferHashLiteral(e)

	case *ast.IndexExpression:
		return types.Any

	case *ast.DotExpression:
		// A member chain resolves through the receiver's declared type. It used
		// to answer `Any` unconditionally, which left every field-typed
		// expression — including a trait method called on a field — working with
		// no information, and sent a wrong member to the backend instead of
		// reporting it here.
		return c.memberAccess(e)

	case *ast.SafeAccessExpression:
		// Checked the same as a plain access: `?.` changes what a null receiver
		// does, not what a member is called.
		return c.safeMemberAccess(e)

	default:
		return types.Any
	}
}

func exprPosition(e ast.Expression) (int, int) {
	if e == nil {
		return 0, 0
	}
	switch n := e.(type) {
	case *ast.Identifier:
		return n.Token.Line, n.Token.Column
	case *ast.IntegerLiteral:
		return n.Token.Line, n.Token.Column
	case *ast.FloatLiteral:
		return n.Token.Line, n.Token.Column
	case *ast.StringLiteral:
		return n.Token.Line, n.Token.Column
	case *ast.Boolean:
		return n.Token.Line, n.Token.Column
	case *ast.NullLiteral:
		return n.Token.Line, n.Token.Column
	case *ast.CallExpression:
		return n.Token.Line, n.Token.Column
	case *ast.PrefixExpression:
		return n.Token.Line, n.Token.Column
	case *ast.InfixExpression:
		return n.Token.Line, n.Token.Column
	case *ast.FunctionLiteral:
		return n.Token.Line, n.Token.Column
	case *ast.ArrayLiteral:
		return n.Token.Line, n.Token.Column
	case *ast.HashLiteral:
		return n.Token.Line, n.Token.Column
	case *ast.IndexExpression:
		return n.Token.Line, n.Token.Column
	case *ast.DotExpression:
		return n.Token.Line, n.Token.Column
	default:
		return 0, 0
	}
}

func (c *Checker) report(code, message string, line, col int) {
	d := &diagnostics.Diagnostic{
		Code:     code,
		Severity: diagnostics.SeverityError,
		Message:  message,
		Span: diagnostics.Span{
			StartLine: line,
			StartCol:  col,
		},
		AIExplanation: diagnostics.ExplainCode(code),
	}
	c.Diagnostics = append(c.Diagnostics, d)
}
