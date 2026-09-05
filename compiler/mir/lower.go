package mir

import (
	"fmt"

	"github.com/joysriramsarkar/nilLang/compiler/hir"
)

type Lowerer struct {
	tempCounter  int
	blockCounter int
	currentProg  *Program
	currentFn    *Function
	currentBlock *BasicBlock
}

func NewLowerer() *Lowerer {
	return &Lowerer{}
}

func (l *Lowerer) newTemp() Operand {
	id := l.tempCounter
	l.tempCounter++
	return TempOperand{ID: id}
}

func (l *Lowerer) newBlock(name string) *BasicBlock {
	id := fmt.Sprintf("%s_%d", name, l.blockCounter)
	l.blockCounter++
	bb := &BasicBlock{
		ID:           id,
		Instructions: []Instruction{},
	}
	if l.currentFn != nil {
		l.currentFn.Blocks = append(l.currentFn.Blocks, bb)
	}
	return bb
}

func (l *Lowerer) emit(inst Instruction) {
	if l.currentBlock != nil {
		l.currentBlock.Instructions = append(l.currentBlock.Instructions, inst)
	}
}

func (l *Lowerer) terminate(term Terminator) {
	if l.currentBlock != nil && l.currentBlock.Terminator == nil {
		l.currentBlock.Terminator = term
	}
}

func (l *Lowerer) LowerHIR(p *hir.Program) *Program {
	prog := &Program{
		Functions: make(map[string]*Function),
	}
	l.currentProg = prog

	mainFn := &Function{
		Name:   "main",
		Params: []string{},
		Blocks: []*BasicBlock{},
	}
	l.currentFn = mainFn
	l.currentBlock = l.newBlock("bb_entry")

	for _, stmt := range p.Statements {
		l.lowerStatement(stmt)
	}

	// Ensure final block has a terminator
	if l.currentBlock != nil && l.currentBlock.Terminator == nil {
		l.terminate(ReturnTerminator{})
	}

	prog.Main = mainFn
	return prog
}

func (l *Lowerer) lowerStatement(stmt hir.Statement) {
	switch s := stmt.(type) {
	case *hir.LetStmt:
		valOp := l.lowerExpression(s.Value)
		l.emit(StoreVarInst{Name: s.Name, Src: valOp})

	case *hir.AssignStmt:
		valOp := l.lowerExpression(s.Value)
		l.emit(StoreVarInst{Name: s.Name, Src: valOp})

	case *hir.ReturnStmt:
		valOp := l.lowerExpression(s.Value)
		l.terminate(ReturnTerminator{Value: valOp})

	case *hir.ExprStmt:
		l.lowerExpression(s.Expr)

	case *hir.IfStmt:
		condOp := l.lowerExpression(s.Condition)
		thenBB := l.newBlock("bb_then")
		elseBB := l.newBlock("bb_else")
		mergeBB := l.newBlock("bb_merge")

		l.terminate(BranchTerminator{
			Cond:        condOp,
			TrueTarget:  thenBB.ID,
			FalseTarget: elseBB.ID,
		})

		// Then block
		l.currentBlock = thenBB
		if s.Consequence != nil {
			for _, st := range s.Consequence.Statements {
				l.lowerStatement(st)
			}
		}
		if l.currentBlock.Terminator == nil {
			l.terminate(JumpTerminator{Target: mergeBB.ID})
		}

		// Else block
		l.currentBlock = elseBB
		if s.Alternative != nil {
			for _, st := range s.Alternative.Statements {
				l.lowerStatement(st)
			}
		}
		if l.currentBlock.Terminator == nil {
			l.terminate(JumpTerminator{Target: mergeBB.ID})
		}

		// Continue in merge block
		l.currentBlock = mergeBB

	case *hir.WhileStmt:
		condBB := l.newBlock("bb_while_cond")
		bodyBB := l.newBlock("bb_while_body")
		exitBB := l.newBlock("bb_while_exit")

		l.terminate(JumpTerminator{Target: condBB.ID})

		// Condition block
		l.currentBlock = condBB
		condOp := l.lowerExpression(s.Condition)
		l.terminate(BranchTerminator{
			Cond:        condOp,
			TrueTarget:  bodyBB.ID,
			FalseTarget: exitBB.ID,
		})

		// Body block
		l.currentBlock = bodyBB
		if s.Body != nil {
			for _, st := range s.Body.Statements {
				l.lowerStatement(st)
			}
		}
		if l.currentBlock.Terminator == nil {
			l.terminate(JumpTerminator{Target: condBB.ID})
		}

		// Continue in exit block
		l.currentBlock = exitBB
	}
}

func (l *Lowerer) lowerExpression(expr hir.Expression) Operand {
	if expr == nil {
		return ConstOperand{Value: nil}
	}

	switch e := expr.(type) {
	case *hir.IntLit:
		return ConstOperand{Value: e.Val}

	case *hir.FloatLit:
		return ConstOperand{Value: e.Val}

	case *hir.StringLit:
		return ConstOperand{Value: e.Val}

	case *hir.BoolLit:
		return ConstOperand{Value: e.Val}

	case *hir.NullLit:
		return ConstOperand{Value: nil}

	case *hir.VarRef:
		t := l.newTemp()
		l.emit(LoadVarInst{Dest: t, Name: e.Name})
		return t

	case *hir.BinaryExpr:
		leftOp := l.lowerExpression(e.Left)
		rightOp := l.lowerExpression(e.Right)
		dest := l.newTemp()
		l.emit(BinaryOpInst{
			Dest:  dest,
			Op:    e.Op,
			Left:  leftOp,
			Right: rightOp,
		})
		return dest

	case *hir.UnaryExpr:
		rightOp := l.lowerExpression(e.Right)
		dest := l.newTemp()
		l.emit(UnaryOpInst{
			Dest:  dest,
			Op:    e.Op,
			Right: rightOp,
		})
		return dest

	case *hir.CallExpr:
		calleeName := ""
		if v, ok := e.Callee.(*hir.VarRef); ok {
			calleeName = v.Name
		} else if e.Callee != nil {
			calleeName = e.Callee.String()
		}
		var args []Operand
		for _, a := range e.Args {
			args = append(args, l.lowerExpression(a))
		}
		dest := l.newTemp()
		l.emit(CallInst{
			Dest:   dest,
			Callee: calleeName,
			Args:   args,
		})
		return dest

	case *hir.FnLit:
		fnName := fmt.Sprintf("fn_anon_%d", l.tempCounter)
		l.tempCounter++
		fn := &Function{
			Name:   fnName,
			Params: e.Params,
			Blocks: []*BasicBlock{},
		}
		if l.currentProg != nil {
			l.currentProg.Functions[fnName] = fn
		}

		prevFn := l.currentFn
		prevBB := l.currentBlock

		l.currentFn = fn
		l.currentBlock = l.newBlock("bb_entry")
		if e.Body != nil {
			for _, st := range e.Body.Statements {
				l.lowerStatement(st)
			}
		}
		if l.currentBlock != nil && l.currentBlock.Terminator == nil {
			l.terminate(ReturnTerminator{})
		}

		l.currentFn = prevFn
		l.currentBlock = prevBB

		return VarOperand{Name: fnName}

	case *hir.ListLit:
		var elems []Operand
		for _, el := range e.Elements {
			elems = append(elems, l.lowerExpression(el))
		}
		dest := l.newTemp()
		l.emit(CallInst{
			Dest:   dest,
			Callee: "array",
			Args:   elems,
		})
		return dest

	case *hir.IndexExpr:
		leftOp := l.lowerExpression(e.Left)
		idxOp := l.lowerExpression(e.Index)
		dest := l.newTemp()
		l.emit(BinaryOpInst{
			Dest:  dest,
			Op:    "[]",
			Left:  leftOp,
			Right: idxOp,
		})
		return dest

	case *hir.DotExpr:
		leftOp := l.lowerExpression(e.Left)
		dest := l.newTemp()
		l.emit(BinaryOpInst{
			Dest:  dest,
			Op:    ".",
			Left:  leftOp,
			Right: ConstOperand{Value: e.Field},
		})
		return dest

	case *hir.MapLit:
		var args []Operand
		for i := range e.Keys {
			args = append(args, l.lowerExpression(e.Keys[i]))
			if i < len(e.Values) {
				args = append(args, l.lowerExpression(e.Values[i]))
			}
		}
		dest := l.newTemp()
		l.emit(CallInst{
			Dest:   dest,
			Callee: "map",
			Args:   args,
		})
		return dest

	default:
		return ConstOperand{Value: nil}
	}
}
