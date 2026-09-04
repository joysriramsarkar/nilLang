package ast

import (
	"bytes"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/token"
)

type Node interface {
	TokenLiteral() string
	String() string
}

type Statement interface {
	Node
	statementNode()
}

type Expression interface {
	Node
	expressionNode()
}

type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}

func (p *Program) String() string {
	var out bytes.Buffer
	for _, s := range p.Statements {
		out.WriteString(s.String())
	}
	return out.String()
}

// ─── STATEMENTS ─────────────────────────────────────────────────────────────

type LetStatement struct {
	Token token.Token // token.LET
	Name  *Identifier
	Value Expression
}

func (ls *LetStatement) statementNode()       {}
func (ls *LetStatement) TokenLiteral() string { return ls.Token.Literal }
func (ls *LetStatement) String() string {
	var out bytes.Buffer
	out.WriteString(ls.TokenLiteral() + " ")
	out.WriteString(ls.Name.String())
	out.WriteString(" = ")
	if ls.Value != nil {
		out.WriteString(ls.Value.String())
	}
	out.WriteString(";")
	return out.String()
}

type AssignStatement struct {
	Token token.Token // token.IDENT
	Name  *Identifier
	Value Expression
}

func (as *AssignStatement) statementNode()       {}
func (as *AssignStatement) TokenLiteral() string { return as.Token.Literal }
func (as *AssignStatement) String() string {
	var out bytes.Buffer
	out.WriteString(as.Name.String())
	out.WriteString(" = ")
	if as.Value != nil {
		out.WriteString(as.Value.String())
	}
	out.WriteString(";")
	return out.String()
}

type ReturnStatement struct {
	Token       token.Token // token.RETURN
	ReturnValue Expression
}

func (rs *ReturnStatement) statementNode()       {}
func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }
func (rs *ReturnStatement) String() string {
	var out bytes.Buffer
	out.WriteString(rs.TokenLiteral() + " ")
	if rs.ReturnValue != nil {
		out.WriteString(rs.ReturnValue.String())
	}
	out.WriteString(";")
	return out.String()
}

type ExpressionStatement struct {
	Token      token.Token
	Expression Expression
}

func (es *ExpressionStatement) statementNode()       {}
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExpressionStatement) String() string {
	if es.Expression != nil {
		return es.Expression.String()
	}
	return ""
}

type BlockStatement struct {
	Token      token.Token // token.LBRACE
	Statements []Statement
}

func (bs *BlockStatement) statementNode()       {}
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStatement) String() string {
	var out bytes.Buffer
	for _, s := range bs.Statements {
		out.WriteString(s.String())
	}
	return out.String()
}

type WhileStatement struct {
	Token     token.Token // token.WHILE
	Condition Expression
	Body      *BlockStatement
}

func (ws *WhileStatement) statementNode()       {}
func (ws *WhileStatement) TokenLiteral() string { return ws.Token.Literal }
func (ws *WhileStatement) String() string {
	var out bytes.Buffer
	out.WriteString("while (")
	out.WriteString(ws.Condition.String())
	out.WriteString(") { ")
	out.WriteString(ws.Body.String())
	out.WriteString(" }")
	return out.String()
}

// ─── EXPRESSIONS ────────────────────────────────────────────────────────────

type Identifier struct {
	Token token.Token // token.IDENT
	Value string
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }
func (i *Identifier) String() string       { return i.Value }

type IntegerLiteral struct {
	Token token.Token
	Value int64
}

func (il *IntegerLiteral) expressionNode()      {}
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }
func (il *IntegerLiteral) String() string       { return il.Token.Literal }

type FloatLiteral struct {
	Token token.Token
	Value float64
}

func (fl *FloatLiteral) expressionNode()      {}
func (fl *FloatLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FloatLiteral) String() string       { return fl.Token.Literal }

type Boolean struct {
	Token token.Token
	Value bool
}

func (b *Boolean) expressionNode()      {}
func (b *Boolean) TokenLiteral() string { return b.Token.Literal }
func (b *Boolean) String() string       { return b.Token.Literal }

type NullLiteral struct {
	Token token.Token
}

func (nl *NullLiteral) expressionNode()      {}
func (nl *NullLiteral) TokenLiteral() string { return nl.Token.Literal }
func (nl *NullLiteral) String() string       { return nl.Token.Literal }

type StringLiteral struct {
	Token token.Token
	Value string
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLiteral) String() string       { return `"` + sl.Value + `"` }

type TemplatePart struct {
	IsExpression bool
	Literal      string
	Expression   Expression
}

type StringTemplate struct {
	Token token.Token
	Parts []TemplatePart
}

func (st *StringTemplate) expressionNode()      {}
func (st *StringTemplate) TokenLiteral() string { return st.Token.Literal }
func (st *StringTemplate) String() string {
	var out bytes.Buffer
	out.WriteString("\"")
	for _, p := range st.Parts {
		if p.IsExpression {
			out.WriteString("\\(")
			if p.Expression != nil {
				out.WriteString(p.Expression.String())
			}
			out.WriteString(")")
		} else {
			out.WriteString(p.Literal)
		}
	}
	out.WriteString("\"")
	return out.String()
}

type PrefixExpression struct {
	Token    token.Token // !, -
	Operator string
	Right    Expression
}

func (pe *PrefixExpression) expressionNode()      {}
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PrefixExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(pe.Operator)
	out.WriteString(pe.Right.String())
	out.WriteString(")")
	return out.String()
}

type InfixExpression struct {
	Token    token.Token
	Left     Expression
	Operator string
	Right    Expression
}

func (ie *InfixExpression) expressionNode()      {}
func (ie *InfixExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *InfixExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(ie.Left.String())
	out.WriteString(" " + ie.Operator + " ")
	out.WriteString(ie.Right.String())
	out.WriteString(")")
	return out.String()
}

type IfExpression struct {
	Token       token.Token // token.IF
	Condition   Expression
	Consequence *BlockStatement
	Alternative *BlockStatement
}

func (ie *IfExpression) expressionNode()      {}
func (ie *IfExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IfExpression) String() string {
	var out bytes.Buffer
	out.WriteString("if ")
	out.WriteString(ie.Condition.String())
	out.WriteString(" ")
	out.WriteString(ie.Consequence.String())
	if ie.Alternative != nil {
		out.WriteString(" else ")
		out.WriteString(ie.Alternative.String())
	}
	return out.String()
}

type FunctionLiteral struct {
	Token      token.Token // token.FN
	Parameters []*Identifier
	Body       *BlockStatement
	Name       string
}

func (fl *FunctionLiteral) expressionNode()      {}
func (fl *FunctionLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FunctionLiteral) String() string {
	var out bytes.Buffer
	params := []string{}
	for _, p := range fl.Parameters {
		params = append(params, p.String())
	}
	out.WriteString(fl.TokenLiteral())
	if fl.Name != "" {
		out.WriteString(" " + fl.Name)
	}
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") ")
	out.WriteString(fl.Body.String())
	return out.String()
}

type CallExpression struct {
	Token     token.Token // token.LPAREN
	Function  Expression  // Identifier or FunctionLiteral
	Arguments []Expression
}

func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CallExpression) String() string {
	var out bytes.Buffer
	args := []string{}
	for _, a := range ce.Arguments {
		args = append(args, a.String())
	}
	out.WriteString(ce.Function.String())
	out.WriteString("(")
	out.WriteString(strings.Join(args, ", "))
	out.WriteString(")")
	return out.String()
}

type ArrayLiteral struct {
	Token    token.Token // token.LBRACKET
	Elements []Expression
}

func (al *ArrayLiteral) expressionNode()      {}
func (al *ArrayLiteral) TokenLiteral() string { return al.Token.Literal }
func (al *ArrayLiteral) String() string {
	var out bytes.Buffer
	elems := []string{}
	for _, el := range al.Elements {
		elems = append(elems, el.String())
	}
	out.WriteString("[")
	out.WriteString(strings.Join(elems, ", "))
	out.WriteString("]")
	return out.String()
}

type IndexExpression struct {
	Token token.Token // token.LBRACKET
	Left  Expression
	Index Expression
}

func (ie *IndexExpression) expressionNode()      {}
func (ie *IndexExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IndexExpression) String() string {
	var out bytes.Buffer
	out.WriteString("(")
	out.WriteString(ie.Left.String())
	out.WriteString("[")
	out.WriteString(ie.Index.String())
	out.WriteString("])")
	return out.String()
}

type DotExpression struct {
	Token  token.Token // token.DOT
	Left   Expression
	Member *Identifier
}

func (de *DotExpression) expressionNode()      {}
func (de *DotExpression) TokenLiteral() string { return de.Token.Literal }
func (de *DotExpression) String() string {
	return de.Left.String() + "." + de.Member.String()
}

type HashLiteral struct {
	Token token.Token // token.LBRACE
	Pairs map[Expression]Expression
}

func (hl *HashLiteral) expressionNode()      {}
func (hl *HashLiteral) TokenLiteral() string { return hl.Token.Literal }
func (hl *HashLiteral) String() string {
	var out bytes.Buffer
	pairs := []string{}
	for k, v := range hl.Pairs {
		pairs = append(pairs, k.String()+":"+v.String())
	}
	out.WriteString("{")
	out.WriteString(strings.Join(pairs, ", "))
	out.WriteString("}")
	return out.String()
}

// ─── DECLARATIVE COMPONENT AST ──────────────────────────────────────────────

type PropertyDeclaration struct {
	Token token.Token
	Name  *Identifier
	Type  string
	Value Expression
}

type StateDeclaration struct {
	Token token.Token
	Name  *Identifier
	Type  string
	Value Expression
}

type RenderMethod struct {
	Token token.Token
	Body  *BlockStatement
}

type MethodDeclaration struct {
	Token      token.Token
	Name       *Identifier
	Parameters []*Identifier
	Body       *BlockStatement
}

type StyleStatement struct {
	Token      token.Token
	Name       *Identifier
	Properties map[string]string
}

func (ss *StyleStatement) statementNode()       {}
func (ss *StyleStatement) TokenLiteral() string { return ss.Token.Literal }
func (ss *StyleStatement) String() string {
	var out bytes.Buffer
	out.WriteString("style ")
	if ss.Name != nil {
		out.WriteString(ss.Name.String() + " ")
	}
	out.WriteString("{ ... }")
	return out.String()
}

type ComponentLiteral struct {
	Token      token.Token // token.COMPONENT
	Name       *Identifier
	Properties []*PropertyDeclaration
	States     []*StateDeclaration
	Render     *RenderMethod
	Methods    []*MethodDeclaration
	Body       *BlockStatement
}

func (cl *ComponentLiteral) expressionNode()      {}
func (cl *ComponentLiteral) statementNode()       {}
func (cl *ComponentLiteral) TokenLiteral() string { return cl.Token.Literal }
func (cl *ComponentLiteral) String() string {
	var out bytes.Buffer
	out.WriteString("component ")
	if cl.Name != nil {
		out.WriteString(cl.Name.String() + " ")
	}
	out.WriteString("{ ... }")
	return out.String()
}
