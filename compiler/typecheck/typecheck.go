package typecheck

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
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
	inPureFn     bool
	Diagnostics  []*diagnostics.Diagnostic
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

func (c *Checker) EnableCapability(cap string) {
	c.capabilities[cap] = true
}

func (c *Checker) HasCapability(cap string) bool {
	return c.capabilities[cap]
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
	c.registerBuiltin("isSome", []types.Type{types.Any}, 1, 1, types.Bool, "pure")
	c.registerBuiltin("isNone", []types.Type{types.Any}, 1, 1, types.Bool, "pure")
	c.registerBuiltin("tensorShape", []types.Type{types.Any}, 1, 1, types.Any, "pure")
	c.registerBuiltin("tensorDtype", []types.Type{types.Any}, 1, 1, types.String, "pure")
	c.registerBuiltin("tensorSum", []types.Type{types.Any}, 1, 1, types.Any, "pure")

	// 2-argument Builtins
	c.registerBuiltin("push", []types.Type{types.Any, types.Any}, 2, 2, types.Any, "pure")
	c.registerBuiltin("append", []types.Type{types.Any, types.Any}, 2, 2, types.Any, "pure")
	c.registerBuiltin("join", []types.Type{types.Any, types.String}, 2, 2, types.String, "pure")
	c.registerBuiltin("contains", []types.Type{types.Any, types.Any}, 2, 2, types.Bool, "pure")
	c.registerBuiltin("hasPrefix", []types.Type{types.Any, types.Any}, 2, 2, types.Bool, "pure")
	c.registerBuiltin("hasSuffix", []types.Type{types.Any, types.Any}, 2, 2, types.Bool, "pure")
	c.registerBuiltin("writeFile", []types.Type{types.String, types.String}, 2, 2, types.Bool, "io")
	c.registerBuiltin("send", []types.Type{types.Any, types.Any}, 2, 2, types.Void, "concurrency")
	c.registerBuiltin("unwrapOr", []types.Type{types.Any, types.Any}, 2, 2, types.Any, "pure")
	c.registerBuiltin("tensor", []types.Type{types.Any, types.Any}, 2, 2, types.Any, "pure")
	c.registerBuiltin("tensorGet", []types.Type{types.Any, types.Any}, 2, 2, types.Any, "pure")
	c.registerBuiltin("tensorAdd", []types.Type{types.Any, types.Any}, 2, 2, types.Any, "pure")
	c.registerBuiltin("tensorMul", []types.Type{types.Any, types.Any}, 2, 2, types.Any, "pure")
	c.registerBuiltin("tensorCast", []types.Type{types.Any, types.String}, 2, 2, types.Any, "pure")
	c.registerBuiltin("tensorDot", []types.Type{types.Any, types.Any}, 2, 2, types.Any, "pure")
	c.registerBuiltin("tensorMatmul", []types.Type{types.Any, types.Any}, 2, 2, types.Any, "pure")

	// 3-argument Builtins
	c.registerBuiltin("set", []types.Type{types.Any, types.Any, types.Any}, 3, 3, types.Any, "pure")
	c.registerBuiltin("replace", []types.Type{types.Any, types.Any, types.Any}, 3, 3, types.String, "pure")
	c.registerBuiltin("tensorSlice", []types.Type{types.Any, types.Any, types.Any}, 3, 3, types.Any, "pure")
}

func (c *Checker) CheckProgram(prog *ast.Program) bool {
	for _, stmt := range prog.Statements {
		c.checkStatement(stmt)
	}
	return len(c.Diagnostics) == 0
}

func (c *Checker) checkStatement(stmt ast.Statement) {
	switch s := stmt.(type) {
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
			declarationType, err = types.Parse(s.Type)
			if err != nil {
				c.report("E0101", fmt.Sprintf("Invalid type %q for variable %q: %v", s.Type, s.Name.Value, err), s.Token.Line, s.Token.Column)
				return
			}
			if !valType.AssignableTo(declarationType) {
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
			declarationType, err = types.Parse(s.Type)
			if err != nil {
				c.report("E0101", fmt.Sprintf("Invalid state type %q: %v", s.Type, err), s.Token.Line, s.Token.Column)
				declarationType = types.Any
			} else if s.Value != nil && !valueType.AssignableTo(declarationType) {
				c.report("E0101", fmt.Sprintf("Cannot initialize state %q of type %s with %s", s.Name.Value, declarationType, valueType), s.Token.Line, s.Token.Column)
			}
		}
		if s.Name != nil {
			c.currentScope.SetVar(s.Name.Value, declarationType)
		}

	case *ast.ComponentLiteral:
		if s.Name == nil {
			c.report("E0301", "Component declaration must have a name", s.Token.Line, s.Token.Column)
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
			if !valType.AssignableTo(varType) {
				c.report("E0101", fmt.Sprintf("Cannot assign %s to variable of type %s", valType, varType), s.Token.Line, s.Token.Column)
			}
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
	}
}

func (c *Checker) checkEntityStatement(s *ast.EntityStatement) {
	if s.Name == nil || s.Name.Value == "" {
		c.report("E0201", "Entity declaration must have a valid identifier name", s.Token.Line, s.Token.Column)
		return
	}

	entName := s.Name.Value
	if _, exists := c.currentScope.GetEntity(entName); exists {
		c.report("E0202", fmt.Sprintf("Duplicate entity declaration: %q already exists in scope", entName), s.Token.Line, s.Token.Column)
		return
	}

	fieldDefs := make([]types.EntityFieldDef, 0, len(s.Fields))
	fieldNames := make(map[string]bool)
	primaryKeys := 0

	for _, f := range s.Fields {
		if fieldNames[f.Name] {
			c.report("E0203", fmt.Sprintf("Duplicate field %q in entity %q", f.Name, entName), s.Token.Line, s.Token.Column)
			continue
		}
		fieldNames[f.Name] = true

		if f.IsPrimary {
			primaryKeys++
		}

		ft, err := types.Parse(f.Type)
		if err != nil {
			c.report("E0204", fmt.Sprintf("Invalid type %q for field %q in entity %q: %v", f.Type, f.Name, entName, err), s.Token.Line, s.Token.Column)
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
		c.report("E0205", fmt.Sprintf("Entity %q has %d primary keys; at most one primary key allowed", entName, primaryKeys), s.Token.Line, s.Token.Column)
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
		left := c.inferExpression(e.Left)
		right := c.inferExpression(e.Right)

		switch e.Operator {
		case "+":
			if left.Equals(types.String) || right.Equals(types.String) {
				return types.String
			}
			if left.Equals(types.Int) && right.Equals(types.Int) {
				return types.Int
			}
			if left.Equals(types.Float) || right.Equals(types.Float) {
				return types.Float
			}
			return types.Any

		case "-", "*", "/", "%":
			if left.Equals(types.Float) || right.Equals(types.Float) {
				return types.Float
			}
			return types.Int

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

		var expectedParams []types.Type
		if e.Name != "" {
			if existing, ok := c.currentScope.GetFunc(e.Name); ok && len(existing.Params) == len(e.Parameters) {
				expectedParams = existing.Params
			}
		}

		for i, p := range e.Parameters {
			pt := types.Type(types.Any)
			if i < len(expectedParams) && expectedParams[i] != nil {
				pt = expectedParams[i]
			}
			paramTypes = append(paramTypes, pt)
			fnScope.SetVar(p.Value, pt)
		}

		fnType := &types.FunctionType{
			Params:     paramTypes,
			ReturnType: types.Any,
			MinArgs:    len(paramTypes),
			MaxArgs:    len(paramTypes),
		}
		if e.Name != "" {
			c.currentScope.SetFunc(e.Name, fnType)
			c.currentScope.SetVar(e.Name, fnType)
			fnScope.SetFunc(e.Name, fnType)
			fnScope.SetVar(e.Name, fnType)
		}

		oldScope := c.currentScope
		c.currentScope = fnScope
		if e.Body != nil {
			for _, s := range e.Body.Statements {
				c.checkStatement(s)
			}
		}
		c.currentScope = oldScope

		return fnType

	case *ast.CallExpression:
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

			// Check effects in pure function
			if c.inPureFn {
				for _, eff := range fnSig.Effects {
					if eff != "pure" {
						c.report("E0202", fmt.Sprintf("Cannot call side-effecting function (%s) inside pure context", eff), e.Token.Line, e.Token.Column)
					}
				}
			}
		}

		for i, arg := range e.Arguments {
			argType := c.inferExpression(arg)
			if fnSig != nil && i < len(fnSig.Params) {
				paramType := fnSig.Params[i]
				if paramType != nil && !paramType.Equals(types.Any) {
					if !argType.AssignableTo(paramType) {
						line, col := exprPosition(arg)
						if line == 0 {
							line, col = e.Token.Line, e.Token.Column
						}
						c.report("E0101", fmt.Sprintf("cannot pass argument %d of type %s to parameter of type %s", i+1, argType, paramType), line, col)
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
		return &types.GenericType{Base: "Hash", Parameters: []types.Type{types.Any, types.Any}}

	case *ast.IndexExpression:
		return types.Any

	case *ast.DotExpression:
		c.inferExpression(e.Left)
		return types.Any

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
