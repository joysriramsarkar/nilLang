package typecheck

import (
	"fmt"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/diagnostics"
	"github.com/joysriramsarkar/nilLang/compiler/types"
)

type Scope struct {
	parent    *Scope
	variables map[string]types.Type
	functions map[string]*types.FunctionType
	structs   map[string]*types.StructType
	entities  map[string]*types.EntityType
}

func NewScope(parent *Scope) *Scope {
	return &Scope{
		parent:    parent,
		variables: make(map[string]types.Type),
		functions: make(map[string]*types.FunctionType),
		structs:   make(map[string]*types.StructType),
		entities:  make(map[string]*types.EntityType),
	}
}

func (s *Scope) SetVar(name string, t types.Type) {
	s.variables[name] = t
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

func (c *Checker) initBuiltins() {
	// Standard builtins
	c.currentScope.SetFunc("puts", &types.FunctionType{
		Params:     []types.Type{types.Any},
		ReturnType: types.Void,
		Effects:    []string{"io"},
	})
	c.currentScope.SetFunc("println", &types.FunctionType{
		Params:     []types.Type{types.Any},
		ReturnType: types.Void,
		Effects:    []string{"io"},
	})
	c.currentScope.SetFunc("print", &types.FunctionType{
		Params:     []types.Type{types.Any},
		ReturnType: types.Void,
		Effects:    []string{"io"},
	})
	c.currentScope.SetFunc("len", &types.FunctionType{
		Params:     []types.Type{types.Any},
		ReturnType: types.Int,
		Effects:    []string{"pure"},
	})
	c.currentScope.SetFunc("str", &types.FunctionType{
		Params:     []types.Type{types.Any},
		ReturnType: types.String,
		Effects:    []string{"pure"},
	})
	c.currentScope.SetFunc("push", &types.FunctionType{
		Params:     []types.Type{types.Any, types.Any},
		ReturnType: types.Any,
		Effects:    []string{"pure"},
	})
	c.currentScope.SetFunc("first", &types.FunctionType{
		Params:     []types.Type{types.Any},
		ReturnType: types.Any,
		Effects:    []string{"pure"},
	})
	c.currentScope.SetFunc("last", &types.FunctionType{
		Params:     []types.Type{types.Any},
		ReturnType: types.Any,
		Effects:    []string{"pure"},
	})
	c.currentScope.SetFunc("rest", &types.FunctionType{
		Params:     []types.Type{types.Any},
		ReturnType: types.Any,
		Effects:    []string{"pure"},
	})
	c.currentScope.SetFunc("assert", &types.FunctionType{
		Params:     []types.Type{types.Bool, types.String},
		ReturnType: types.Void,
		Effects:    []string{"pure"},
	})
	c.currentScope.SetFunc("emit", &types.FunctionType{
		Params:     []types.Type{types.Any, types.Any},
		ReturnType: types.Any,
		Effects:    []string{"ui"},
	})
	for _, builtin := range []struct {
		name   string
		params []types.Type
	}{
		{"tensor", []types.Type{types.Any, types.Any}},
		{"tensorShape", []types.Type{types.Any}},
		{"tensorGet", []types.Type{types.Any, types.Any}},
		{"tensorAdd", []types.Type{types.Any, types.Any}},
		{"tensorMul", []types.Type{types.Any, types.Any}},
		{"tensorSlice", []types.Type{types.Any, types.Any, types.Any}},
		{"tensorDtype", []types.Type{types.Any}},
		{"tensorCast", []types.Type{types.Any, types.String}},
		{"tensorDot", []types.Type{types.Any, types.Any}},
		{"tensorMatmul", []types.Type{types.Any, types.Any}},
		{"tensorSum", []types.Type{types.Any}},
	} {
		c.currentScope.SetFunc(builtin.name, &types.FunctionType{
			Params:     builtin.params,
			ReturnType: types.Any,
			Effects:    []string{"pure"},
		})
	}
}

func (c *Checker) CheckProgram(prog *ast.Program) bool {
	for _, stmt := range prog.Statements {
		c.checkStatement(stmt)
	}
	return len(c.Diagnostics) == 0
}

func (c *Checker) checkStatement(stmt ast.Statement) {
	switch s := stmt.(type) {
	case *ast.AppStatement:
		if s.Body != nil {
			for _, nested := range s.Body.Statements {
				c.checkStatement(nested)
			}
		}

	case *ast.LetStatement:
		valType := c.inferExpression(s.Value)
		var declType types.Type
		if s.Name != nil {
			declType = valType
			c.currentScope.SetVar(s.Name.Value, declType)
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
		valType := c.inferExpression(s.Value)
		if s.Name != nil {
			varType, ok := c.currentScope.GetVar(s.Name.Value)
			if !ok {
				c.report("E0102", fmt.Sprintf("Undefined identifier %q in assignment", s.Name.Value), s.Token.Line, s.Token.Column)
				return
			}
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
		conseqType := types.Void
		if e.Consequence != nil {
			c.currentScope = NewScope(c.currentScope)
			for _, s := range e.Consequence.Statements {
				c.checkStatement(s)
			}
			c.currentScope = c.currentScope.parent
		}
		if e.Alternative != nil {
			c.currentScope = NewScope(c.currentScope)
			for _, s := range e.Alternative.Statements {
				c.checkStatement(s)
			}
			c.currentScope = c.currentScope.parent
		}
		return conseqType

	case *ast.FunctionLiteral:
		fnScope := NewScope(c.currentScope)
		var paramTypes []types.Type
		for _, p := range e.Parameters {
			paramTypes = append(paramTypes, types.Any)
			fnScope.SetVar(p.Value, types.Any)
		}

		oldScope := c.currentScope
		c.currentScope = fnScope
		if e.Body != nil {
			for _, s := range e.Body.Statements {
				c.checkStatement(s)
			}
		}
		c.currentScope = oldScope

		return &types.FunctionType{
			Params:     paramTypes,
			ReturnType: types.Any,
		}

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

		for _, arg := range e.Arguments {
			c.inferExpression(arg)
		}

		if fnSig != nil {
			// Check effects in pure function
			if c.inPureFn {
				for _, eff := range fnSig.Effects {
					if eff != "pure" {
						c.report("E0202", fmt.Sprintf("Cannot call side-effecting function (%s) inside pure context", eff), e.Token.Line, e.Token.Column)
					}
				}
			}
			return fnSig.ReturnType
		}

		return types.Any

	case *ast.ArrayLiteral:
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
