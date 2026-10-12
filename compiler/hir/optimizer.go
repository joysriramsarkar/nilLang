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
			Target: o.optimizeExpression(s.Target),
			Value:  o.optimizeExpression(s.Value),
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
		// A loop invariant binding is computed on every iteration of a loop that
		// changes nothing about it. `let n = xs.length; while ... { ... n ... }`
		// recomputes `xs.length` each time round, which is a method call, an index
		// and a comparison, for a value the program wrote down once.
		//
		// It is the most valuable hoist in the language because a loop body that
		// reads a collection almost always reads its length first, and a UI
		// component's render body almost always does.
		hoisted, body := hoistLoopInvariants(s.Body)
		return &WhileStmt{
			Condition: o.optimizeExpression(s.Condition),
			PreLoop:   hoistDeclarations(hoisted),
			Body:      o.optimizeBlock(body),
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
	case *IfExpr:
		// A select's arms are constant-folded, but the condition is left alone.
		//
		// Folding the condition would turn a two-way choice into a single value
		// and delete the other arm — which is a *different* optimisation, one that
		// has to know the arm is pure. An arm may be an arbitrary expression, and
		// "it does not have side effects" is a claim this pass cannot make.
		return &IfExpr{
			Condition: o.optimizeExpression(e.Condition),
			WhenTrue:  o.optimizeExpression(e.WhenTrue),
			WhenFalse: o.optimizeExpression(e.WhenFalse),
			ExprType:  e.ExprType,
		}

	case *BlockExpr:
		// Both arms of a `?.` or a `??` sit inside one of these, so a pass that
		// walks expressions but not blocks walks straight past the whole construct.
		stmts := make([]Statement, 0, len(e.Statements))
		for _, s := range e.Statements {
			if opt := o.optimizeStatement(s); opt != nil {
				stmts = append(stmts, opt)
			}
		}
		return &BlockExpr{Statements: stmts}

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
			// The name is copied because this case *rebuilds* the node rather
			// than returning it. Omitting it made every named function print as
			// `fn()` after optimisation while the pre-optimisation HIR showed the
			// name — a difference that appears only when the optimiser runs, and so
			// looks like the optimiser's fault rather than an omission.
			Name:   e.Name,
			Params: e.Params,
			Body:   o.optimizeBlock(e.Body),
			FnType: e.FnType,
			// Purity travels for the same reason: a field not written here
			// is a field the optimiser deletes.
			IsPure: e.IsPure,
			// The same reason the name is copied: this case rebuilds the node, so
			// a field not written here is a field the optimiser deletes. Dropping
			// the ABI turned every extern declaration into an empty function
			// definition, which the linker then rejected as a duplicate symbol for
			// a function that already existed.
			ExternAbi:    e.ExternAbi,
			ExternReturn: e.ExternReturn,
			ExternParams: e.ExternParams,
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
