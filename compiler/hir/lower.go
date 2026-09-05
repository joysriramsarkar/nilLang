package hir

import (
	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/types"
)

type Lowerer struct {
	symbols map[string]types.Type
}

func NewLowerer() *Lowerer {
	return &Lowerer{
		symbols: make(map[string]types.Type),
	}
}

func (l *Lowerer) LowerProgram(prog *ast.Program) *Program {
	hp := &Program{
		Statements: []Statement{},
	}
	for _, s := range prog.Statements {
		if hs := l.lowerStatement(s); hs != nil {
			hp.Statements = append(hp.Statements, hs)
		}
	}
	return hp
}

func (l *Lowerer) lowerStatement(stmt ast.Statement) Statement {
	switch s := stmt.(type) {
	case *ast.LetStatement:
		val := l.lowerExpression(s.Value)
		var valT types.Type = types.Any
		if val != nil {
			valT = val.Type()
		}
		name := ""
		if s.Name != nil {
			name = s.Name.Value
			l.symbols[name] = valT
		}
		return &LetStmt{
			Name:     name,
			VarType:  valT,
			Value:    val,
			Constant: false,
		}

	case *ast.AssignStatement:
		val := l.lowerExpression(s.Value)
		name := ""
		if s.Name != nil {
			name = s.Name.Value
		}
		return &AssignStmt{
			Name:  name,
			Value: val,
		}

	case *ast.ReturnStatement:
		return &ReturnStmt{
			Value: l.lowerExpression(s.ReturnValue),
		}

	case *ast.ExpressionStatement:
		if ifExpr, ok := s.Expression.(*ast.IfExpression); ok {
			return &IfStmt{
				Condition:   l.lowerExpression(ifExpr.Condition),
				Consequence: l.lowerBlock(ifExpr.Consequence),
				Alternative: l.lowerBlock(ifExpr.Alternative),
			}
		}
		return &ExprStmt{
			Expr: l.lowerExpression(s.Expression),
		}

	case *ast.WhileStatement:
		return &WhileStmt{
			Condition: l.lowerExpression(s.Condition),
			Body:      l.lowerBlock(s.Body),
		}

	case *ast.BlockStatement:
		return l.lowerBlock(s)

	default:
		return nil
	}
}

func (l *Lowerer) lowerBlock(block *ast.BlockStatement) *BlockStmt {
	if block == nil {
		return &BlockStmt{}
	}
	hb := &BlockStmt{Statements: []Statement{}}
	for _, s := range block.Statements {
		if hs := l.lowerStatement(s); hs != nil {
			hb.Statements = append(hb.Statements, hs)
		}
	}
	return hb
}

func (l *Lowerer) lowerExpression(expr ast.Expression) Expression {
	if expr == nil {
		return nil
	}

	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		return &IntLit{Val: e.Value}

	case *ast.FloatLiteral:
		return &FloatLit{Val: e.Value}

	case *ast.StringLiteral:
		return &StringLit{Val: e.Value}

	case *ast.StringTemplate:
		if len(e.Parts) == 0 {
			return &StringLit{Val: ""}
		}
		var current Expression
		for _, part := range e.Parts {
			var partExpr Expression
			if part.IsExpression {
				if part.Expression != nil {
					partExpr = l.lowerExpression(part.Expression)
				}
				if partExpr == nil {
					partExpr = &StringLit{Val: ""}
				}
			} else {
				partExpr = &StringLit{Val: part.Literal}
			}

			if current == nil {
				current = partExpr
			} else {
				current = &BinaryExpr{
					Left:     current,
					Op:       "+",
					Right:    partExpr,
					ExprType: types.String,
				}
			}
		}
		if current == nil {
			return &StringLit{Val: ""}
		}
		return current

	case *ast.Boolean:
		return &BoolLit{Val: e.Value}

	case *ast.NullLiteral:
		return &NullLit{}

	case *ast.Identifier:
		t, ok := l.symbols[e.Value]
		if !ok {
			t = types.Any
		}
		return &VarRef{Name: e.Value, VarType: t}

	case *ast.PrefixExpression:
		right := l.lowerExpression(e.Right)
		var t types.Type = types.Any
		if right != nil {
			t = right.Type()
		}
		return &UnaryExpr{Op: e.Operator, Right: right, ExprType: t}

	case *ast.InfixExpression:
		left := l.lowerExpression(e.Left)
		right := l.lowerExpression(e.Right)
		var t types.Type = types.Any
		if e.Operator == "==" || e.Operator == "!=" || e.Operator == "<" || e.Operator == ">" || e.Operator == "<=" || e.Operator == ">=" || e.Operator == "&&" || e.Operator == "||" {
			t = types.Bool
		} else if left != nil && right != nil {
			if left.Type().Equals(types.String) || right.Type().Equals(types.String) {
				t = types.String
			} else if left.Type().Equals(types.Float) || right.Type().Equals(types.Float) {
				t = types.Float
			} else if left.Type().Equals(types.Int) && right.Type().Equals(types.Int) {
				t = types.Int
			}
		}
		return &BinaryExpr{Left: left, Op: e.Operator, Right: right, ExprType: t}

	case *ast.IfExpression:
		cond := l.lowerExpression(e.Condition)
		conseq := l.lowerBlock(e.Consequence)
		var alt *BlockStmt
		if e.Alternative != nil {
			alt = l.lowerBlock(e.Alternative)
		}
		_ = alt

		var rightExpr Expression = &NullLit{}
		if conseq != nil && len(conseq.Statements) > 0 {
			switch st := conseq.Statements[0].(type) {
			case *ExprStmt:
				rightExpr = st.Expr
			case *ReturnStmt:
				if st.Value != nil {
					rightExpr = st.Value
				}
			}
		}

		return &BinaryExpr{
			Left:     cond,
			Op:       "?",
			Right:    rightExpr,
			ExprType: types.Any,
		}

	case *ast.FunctionLiteral:
		var params []string
		var paramTypes []types.Type
		for _, p := range e.Parameters {
			params = append(params, p.Value)
			paramTypes = append(paramTypes, types.Any)
			l.symbols[p.Value] = types.Any
		}
		body := l.lowerBlock(e.Body)
		return &FnLit{
			Params: params,
			Body:   body,
			FnType: &types.FunctionType{Params: paramTypes, ReturnType: types.Any},
		}

	case *ast.CallExpression:
		callee := l.lowerExpression(e.Function)
		var args []Expression
		for _, a := range e.Arguments {
			args = append(args, l.lowerExpression(a))
		}
		return &CallExpr{Callee: callee, Args: args, CallType: types.Any}

	case *ast.ArrayLiteral:
		var elems []Expression
		for _, el := range e.Elements {
			elems = append(elems, l.lowerExpression(el))
		}
		return &ListLit{Elements: elems, ListType: &types.GenericType{Base: "List", Parameters: []types.Type{types.Any}}}

	case *ast.IndexExpression:
		left := l.lowerExpression(e.Left)
		idx := l.lowerExpression(e.Index)
		return &IndexExpr{
			Left:     left,
			Index:    idx,
			ExprType: types.Any,
		}

	case *ast.DotExpression:
		left := l.lowerExpression(e.Left)
		field := ""
		if e.Member != nil {
			field = e.Member.Value
		}
		return &DotExpr{
			Left:     left,
			Field:    field,
			ExprType: types.Any,
		}

	case *ast.HashLiteral:
		var keys []Expression
		var values []Expression
		for k, v := range e.Pairs {
			keys = append(keys, l.lowerExpression(k))
			values = append(values, l.lowerExpression(v))
		}
		return &MapLit{
			Keys:   keys,
			Values: values,
		}

	default:
		return nil
	}
}
