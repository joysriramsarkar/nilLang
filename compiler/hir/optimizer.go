package hir

// Optimizer optimizes HIR trees via constant folding and dead code elimination
type Optimizer struct{}

func NewOptimizer() *Optimizer {
	return &Optimizer{}
}

func (o *Optimizer) Optimize(p *Program) *Program {
	optProg := &Program{Statements: []Statement{}}
	for _, s := range p.Statements {
		optStmt := o.optimizeStatement(s)
		if optStmt != nil {
			optProg.Statements = append(optProg.Statements, optStmt)
		}
	}
	return optProg
}

func (o *Optimizer) optimizeStatement(stmt Statement) Statement {
	switch s := stmt.(type) {
	case *LetStmt:
		return &LetStmt{
			Name:     s.Name,
			VarType:  s.VarType,
			Value:    o.optimizeExpression(s.Value),
			Constant: s.Constant,
		}
	case *AssignStmt:
		return &AssignStmt{
			Name:  s.Name,
			Value: o.optimizeExpression(s.Value),
		}
	case *ReturnStmt:
		return &ReturnStmt{
			Value: o.optimizeExpression(s.Value),
		}
	case *ExprStmt:
		return &ExprStmt{
			Expr: o.optimizeExpression(s.Expr),
		}
	case *IfStmt:
		cond := o.optimizeExpression(s.Condition)
		// Dead code elimination: if condition is constant bool
		if b, ok := cond.(*BoolLit); ok {
			if b.Val {
				return o.optimizeBlock(s.Consequence)
			} else if s.Alternative != nil {
				return o.optimizeBlock(s.Alternative)
			}
			return nil
		}
		return &IfStmt{
			Condition:   cond,
			Consequence: o.optimizeBlock(s.Consequence),
			Alternative: o.optimizeBlock(s.Alternative),
		}
	case *WhileStmt:
		return &WhileStmt{
			Condition: o.optimizeExpression(s.Condition),
			Body:      o.optimizeBlock(s.Body),
		}
	case *BlockStmt:
		return o.optimizeBlock(s)
	default:
		return stmt
	}
}

func (o *Optimizer) optimizeBlock(b *BlockStmt) *BlockStmt {
	if b == nil {
		return nil
	}
	res := &BlockStmt{Statements: []Statement{}}
	for _, s := range b.Statements {
		opt := o.optimizeStatement(s)
		if opt != nil {
			res.Statements = append(res.Statements, opt)
		}
	}
	return res
}

func (o *Optimizer) optimizeExpression(expr Expression) Expression {
	if expr == nil {
		return nil
	}

	switch e := expr.(type) {
	case *BinaryExpr:
		left := o.optimizeExpression(e.Left)
		right := o.optimizeExpression(e.Right)

		// Constant folding on integers
		if li, isLeftInt := left.(*IntLit); isLeftInt {
			if ri, isRightInt := right.(*IntLit); isRightInt {
				switch e.Op {
				case "+":
					return &IntLit{Val: li.Val + ri.Val}
				case "-":
					return &IntLit{Val: li.Val - ri.Val}
				case "*":
					return &IntLit{Val: li.Val * ri.Val}
				case "/":
					if ri.Val != 0 {
						return &IntLit{Val: li.Val / ri.Val}
					}
				case "%":
					if ri.Val != 0 {
						return &IntLit{Val: li.Val % ri.Val}
					}
				case "==":
					return &BoolLit{Val: li.Val == ri.Val}
				case "!=":
					return &BoolLit{Val: li.Val != ri.Val}
				case "<":
					return &BoolLit{Val: li.Val < ri.Val}
				case "<=":
					return &BoolLit{Val: li.Val <= ri.Val}
				case ">":
					return &BoolLit{Val: li.Val > ri.Val}
				case ">=":
					return &BoolLit{Val: li.Val >= ri.Val}
				}
			}
		}

		// Constant folding on strings
		if ls, isLeftStr := left.(*StringLit); isLeftStr {
			if rs, isRightStr := right.(*StringLit); isRightStr {
				if e.Op == "+" {
					return &StringLit{Val: ls.Val + rs.Val}
				}
				if e.Op == "==" {
					return &BoolLit{Val: ls.Val == rs.Val}
				}
				if e.Op == "!=" {
					return &BoolLit{Val: ls.Val != rs.Val}
				}
			}
		}

		return &BinaryExpr{
			Left:     left,
			Op:       e.Op,
			Right:    right,
			ExprType: e.ExprType,
		}

	case *UnaryExpr:
		right := o.optimizeExpression(e.Right)
		if e.Op == "-" {
			if ri, ok := right.(*IntLit); ok {
				return &IntLit{Val: -ri.Val}
			}
			if rf, ok := right.(*FloatLit); ok {
				return &FloatLit{Val: -rf.Val}
			}
		}
		if e.Op == "!" {
			if rb, ok := right.(*BoolLit); ok {
				return &BoolLit{Val: !rb.Val}
			}
		}
		return &UnaryExpr{Op: e.Op, Right: right, ExprType: e.ExprType}

	case *CallExpr:
		callee := o.optimizeExpression(e.Callee)
		var args []Expression
		for _, a := range e.Args {
			args = append(args, o.optimizeExpression(a))
		}
		return &CallExpr{Callee: callee, Args: args, CallType: e.CallType}

	case *FnLit:
		return &FnLit{
			Params: e.Params,
			Body:   o.optimizeBlock(e.Body),
			FnType: e.FnType,
		}

	case *ListLit:
		var elems []Expression
		for _, el := range e.Elements {
			elems = append(elems, o.optimizeExpression(el))
		}
		return &ListLit{Elements: elems, ListType: e.ListType}

	case *IndexExpr:
		return &IndexExpr{
			Left:     o.optimizeExpression(e.Left),
			Index:    o.optimizeExpression(e.Index),
			ExprType: e.ExprType,
		}

	case *DotExpr:
		return &DotExpr{
			Left:     o.optimizeExpression(e.Left),
			Field:    e.Field,
			ExprType: e.ExprType,
		}

	case *MapLit:
		var keys []Expression
		var values []Expression
		for i := range e.Keys {
			keys = append(keys, o.optimizeExpression(e.Keys[i]))
			values = append(values, o.optimizeExpression(e.Values[i]))
		}
		return &MapLit{
			Keys:     keys,
			Values:   values,
			HashType: e.HashType,
		}

	default:
		return expr
	}
}
