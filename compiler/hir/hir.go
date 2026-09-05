package hir

import (
	"fmt"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/types"
)

type Node interface {
	String() string
}

type Statement interface {
	Node
	stmtNode()
}

type Expression interface {
	Node
	exprNode()
	Type() types.Type
}

type Program struct {
	Statements []Statement
}

func (p *Program) String() string {
	var b strings.Builder
	for _, s := range p.Statements {
		b.WriteString(s.String())
		b.WriteByte('\n')
	}
	return b.String()
}

// ─── STATEMENTS ─────────────────────────────────────────────────────────────

type LetStmt struct {
	Name     string
	VarType  types.Type
	Value    Expression
	Constant bool
}

func (s *LetStmt) stmtNode() {}
func (s *LetStmt) String() string {
	kw := "let"
	if s.Constant {
		kw = "const"
	}
	val := ""
	if s.Value != nil {
		val = " = " + s.Value.String()
	}
	return fmt.Sprintf("%s %s: %s%s;", kw, s.Name, s.VarType.String(), val)
}

type AssignStmt struct {
	Name  string
	Value Expression
}

func (s *AssignStmt) stmtNode() {}
func (s *AssignStmt) String() string {
	return fmt.Sprintf("%s = %s;", s.Name, s.Value.String())
}

type ReturnStmt struct {
	Value Expression
}

func (s *ReturnStmt) stmtNode() {}
func (s *ReturnStmt) String() string {
	if s.Value == nil {
		return "return;"
	}
	return fmt.Sprintf("return %s;", s.Value.String())
}

type ExprStmt struct {
	Expr Expression
}

func (s *ExprStmt) stmtNode() {}
func (s *ExprStmt) String() string {
	return s.Expr.String() + ";"
}

type BlockStmt struct {
	Statements []Statement
}

func (s *BlockStmt) stmtNode() {}
func (s *BlockStmt) String() string {
	var b strings.Builder
	b.WriteString("{\n")
	for _, st := range s.Statements {
		b.WriteString("  ")
		b.WriteString(st.String())
		b.WriteByte('\n')
	}
	b.WriteString("}")
	return b.String()
}

type IfStmt struct {
	Condition   Expression
	Consequence *BlockStmt
	Alternative *BlockStmt
}

func (s *IfStmt) stmtNode() {}
func (s *IfStmt) String() string {
	res := fmt.Sprintf("if (%s) %s", s.Condition.String(), s.Consequence.String())
	if s.Alternative != nil {
		res += fmt.Sprintf(" else %s", s.Alternative.String())
	}
	return res
}

type WhileStmt struct {
	Condition Expression
	Body      *BlockStmt
}

func (s *WhileStmt) stmtNode() {}
func (s *WhileStmt) String() string {
	return fmt.Sprintf("while (%s) %s", s.Condition.String(), s.Body.String())
}

// ─── EXPRESSIONS ────────────────────────────────────────────────────────────

type IntLit struct {
	Val int64
}

func (e *IntLit) exprNode()        {}
func (e *IntLit) Type() types.Type { return types.Int }
func (e *IntLit) String() string   { return fmt.Sprintf("%d", e.Val) }

type FloatLit struct {
	Val float64
}

func (e *FloatLit) exprNode()        {}
func (e *FloatLit) Type() types.Type { return types.Float }
func (e *FloatLit) String() string   { return fmt.Sprintf("%f", e.Val) }

type StringLit struct {
	Val string
}

func (e *StringLit) exprNode()        {}
func (e *StringLit) Type() types.Type { return types.String }
func (e *StringLit) String() string   { return fmt.Sprintf("%q", e.Val) }

type BoolLit struct {
	Val bool
}

func (e *BoolLit) exprNode()        {}
func (e *BoolLit) Type() types.Type { return types.Bool }
func (e *BoolLit) String() string   { return fmt.Sprintf("%t", e.Val) }

type NullLit struct{}

func (e *NullLit) exprNode()        {}
func (e *NullLit) Type() types.Type { return types.Null }
func (e *NullLit) String() string   { return "null" }

type VarRef struct {
	Name    string
	VarType types.Type
}

func (e *VarRef) exprNode()        {}
func (e *VarRef) Type() types.Type { return e.VarType }
func (e *VarRef) String() string   { return e.Name }

type BinaryExpr struct {
	Left     Expression
	Op       string
	Right    Expression
	ExprType types.Type
}

func (e *BinaryExpr) exprNode() {}
func (e *BinaryExpr) Type() types.Type {
	if e.ExprType != nil {
		return e.ExprType
	}
	return types.Any
}
func (e *BinaryExpr) String() string {
	left := "<nil>"
	if e.Left != nil {
		left = e.Left.String()
	}
	right := "<nil>"
	if e.Right != nil {
		right = e.Right.String()
	}
	return fmt.Sprintf("(%s %s %s)", left, e.Op, right)
}

type UnaryExpr struct {
	Op       string
	Right    Expression
	ExprType types.Type
}

func (e *UnaryExpr) exprNode() {}
func (e *UnaryExpr) Type() types.Type {
	if e.ExprType != nil {
		return e.ExprType
	}
	return types.Any
}
func (e *UnaryExpr) String() string {
	right := "<nil>"
	if e.Right != nil {
		right = e.Right.String()
	}
	return fmt.Sprintf("(%s%s)", e.Op, right)
}

type CallExpr struct {
	Callee   Expression
	Args     []Expression
	CallType types.Type
}

func (e *CallExpr) exprNode() {}
func (e *CallExpr) Type() types.Type {
	if e.CallType != nil {
		return e.CallType
	}
	return types.Any
}
func (e *CallExpr) String() string {
	var a []string
	for _, arg := range e.Args {
		if arg != nil {
			a = append(a, arg.String())
		} else {
			a = append(a, "<nil>")
		}
	}
	callee := "<nil>"
	if e.Callee != nil {
		callee = e.Callee.String()
	}
	return fmt.Sprintf("%s(%s)", callee, strings.Join(a, ", "))
}

type FnLit struct {
	Params []string
	Body   *BlockStmt
	FnType types.Type
}

func (e *FnLit) exprNode()        {}
func (e *FnLit) Type() types.Type { return e.FnType }
func (e *FnLit) String() string {
	body := "{}"
	if e.Body != nil {
		body = e.Body.String()
	}
	return fmt.Sprintf("fn(%s) %s", strings.Join(e.Params, ", "), body)
}

type ListLit struct {
	Elements []Expression
	ListType types.Type
}

func (e *ListLit) exprNode()        {}
func (e *ListLit) Type() types.Type { return e.ListType }
func (e *ListLit) String() string {
	var el []string
	for _, elem := range e.Elements {
		if elem != nil {
			el = append(el, elem.String())
		} else {
			el = append(el, "<nil>")
		}
	}
	return fmt.Sprintf("[%s]", strings.Join(el, ", "))
}

type IndexExpr struct {
	Left     Expression
	Index    Expression
	ExprType types.Type
}

func (e *IndexExpr) exprNode() {}
func (e *IndexExpr) Type() types.Type {
	if e.ExprType != nil {
		return e.ExprType
	}
	return types.Any
}
func (e *IndexExpr) String() string {
	left := "<nil>"
	if e.Left != nil {
		left = e.Left.String()
	}
	idx := "<nil>"
	if e.Index != nil {
		idx = e.Index.String()
	}
	return fmt.Sprintf("%s[%s]", left, idx)
}

type DotExpr struct {
	Left     Expression
	Field    string
	ExprType types.Type
}

func (e *DotExpr) exprNode() {}
func (e *DotExpr) Type() types.Type {
	if e.ExprType != nil {
		return e.ExprType
	}
	return types.Any
}
func (e *DotExpr) String() string {
	left := "<nil>"
	if e.Left != nil {
		left = e.Left.String()
	}
	return fmt.Sprintf("%s.%s", left, e.Field)
}

type MapLit struct {
	Keys     []Expression
	Values   []Expression
	HashType types.Type
}

func (e *MapLit) exprNode() {}
func (e *MapLit) Type() types.Type {
	if e.HashType != nil {
		return e.HashType
	}
	return &types.GenericType{Base: "Hash", Parameters: []types.Type{types.Any, types.Any}}
}
func (e *MapLit) String() string {
	var pairs []string
	for i := range e.Keys {
		k := "<nil>"
		if e.Keys[i] != nil {
			k = e.Keys[i].String()
		}
		v := "<nil>"
		if i < len(e.Values) && e.Values[i] != nil {
			v = e.Values[i].String()
		}
		pairs = append(pairs, fmt.Sprintf("%s: %s", k, v))
	}
	return fmt.Sprintf("{%s}", strings.Join(pairs, ", "))
}
