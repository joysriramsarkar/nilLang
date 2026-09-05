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
}

func NewScope(parent *Scope) *Scope {
	return &Scope{
		parent:    parent,
		variables: make(map[string]types.Type),
		functions: make(map[string]*types.FunctionType),
		structs:   make(map[string]*types.StructType),
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
}

func (c *Checker) CheckProgram(prog *ast.Program) bool {
	for _, stmt := range prog.Statements {
		c.checkStatement(stmt)
	}
	return len(c.Diagnostics) == 0
}

func (c *Checker) checkStatement(stmt ast.Statement) {
	switch s := stmt.(type) {
	case *ast.LetStatement:
		valType := c.inferExpression(s.Value)
		var declType types.Type
		if s.Name != nil {
			declType = valType
			c.currentScope.SetVar(s.Name.Value, declType)
		}

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
	}
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
