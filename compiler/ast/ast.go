package ast

import (
	"bytes"
	"sort"
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
	Token    token.Token // token.LET or token.CONST
	Name     *Identifier
	Type     string
	Value    Expression
	Constant bool
}

func (ls *LetStatement) statementNode()       {}
func (ls *LetStatement) TokenLiteral() string { return ls.Token.Literal }
func (ls *LetStatement) String() string {
	var out bytes.Buffer
	keyword := "let"
	if ls.Constant {
		keyword = "const"
	}
	out.WriteString(keyword + " ")
	out.WriteString(ls.Name.String())
	if ls.Type != "" {
		out.WriteString(": ")
		out.WriteString(ls.Type)
	}
	if ls.Value != nil {
		out.WriteString(" = ")
		out.WriteString(ls.Value.String())
	}
	out.WriteString(";")
	return out.String()
}

type AssignStatement struct {
	Token    token.Token // token.IDENT
	Name     *Identifier
	Operator string
	Value    Expression
}

func (as *AssignStatement) statementNode()       {}
func (as *AssignStatement) TokenLiteral() string { return as.Token.Literal }
func (as *AssignStatement) String() string {
	var out bytes.Buffer
	out.WriteString(as.Name.String())
	operator := as.Operator
	if operator == "" {
		operator = "="
	}
	out.WriteString(" " + operator + " ")
	if as.Value != nil {
		out.WriteString(as.Value.String())
	}
	out.WriteString(";")
	return out.String()
}

type IndexAssignStatement struct {
	Token token.Token // token.LBRACKET or token.ASSIGN
	Left  Expression
	Index Expression
	Value Expression
}

func (ias *IndexAssignStatement) statementNode()       {}
func (ias *IndexAssignStatement) TokenLiteral() string { return ias.Token.Literal }
func (ias *IndexAssignStatement) String() string {
	var out bytes.Buffer
	out.WriteString(ias.Left.String())
	out.WriteString("[")
	out.WriteString(ias.Index.String())
	out.WriteString("] = ")
	if ias.Value != nil {
		out.WriteString(ias.Value.String())
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

type AppStatement struct {
	Token token.Token // contextual "app" identifier
	Name  *Identifier
	Body  *BlockStatement
}

func (as *AppStatement) statementNode()       {}
func (as *AppStatement) TokenLiteral() string { return as.Token.Literal }
func (as *AppStatement) String() string {
	var out bytes.Buffer
	out.WriteString("app")
	if as.Name != nil {
		out.WriteString(" " + as.Name.String())
	}
	out.WriteString(" { ")
	if as.Body != nil {
		out.WriteString(as.Body.String())
	}
	out.WriteString(" }")
	return out.String()
}

type WhileStatement struct {
	Token     token.Token // token.WHILE
	Condition Expression
	Body      *BlockStatement
}

// ForStatement is `for ... in ... { }` in any of its three forms:
//
//	for i in 0..10       iterate an integer range, i from 0 up to but not including 10
//	for x in collection  bind each element in turn
//	for k, v in pairs    bind index and element together
//
// The range's upper bound is exclusive, matching the counting a developer
// expects from `0..10` and keeping the loop body free of an off-by-one guard.
type ForStatement struct {
	Token token.Token // token.FOR

	// Key is the index variable, which is always bound. For the single-variable
	// forms it holds the same name as Value.
	Key *Identifier

	// Value is the element variable.
	Value *Identifier

	// Iterable is what is being iterated: a RangeExpression, an array
	// expression, a string, or a hash.
	Iterable Expression

	Body *BlockStatement
}

// RangeExpression is `start..end` or `start..=end`.
//
// The inclusive form is spelled with `=`, mirroring Python's `..=`, because
// "does the last element appear" is a question a loop asks constantly and an
// exclusive-only range forces an off-by-one guard into every such body.
type RangeExpression struct {
	Token     token.Token
	Start     Expression
	End       Expression
	Inclusive bool
}

// BreakStatement exits the innermost enclosing loop.
type BreakStatement struct {
	Token token.Token
}

// ContinueStatement starts the next iteration of the innermost enclosing loop.
type ContinueStatement struct {
	Token token.Token
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

func (fs *ForStatement) statementNode()       {}
func (fs *ForStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *ForStatement) String() string {
	var out bytes.Buffer
	out.WriteString("for ")
	if fs.Key != nil && fs.Value != nil && fs.Key.Value != fs.Value.Value {
		out.WriteString(fs.Key.Value)
		out.WriteString(", ")
	}
	if fs.Value != nil {
		out.WriteString(fs.Value.Value)
	}
	out.WriteString(" in ")
	if fs.Iterable != nil {
		out.WriteString(fs.Iterable.String())
	}
	out.WriteString(" { ")
	if fs.Body != nil {
		out.WriteString(fs.Body.String())
	}
	out.WriteString(" }")
	return out.String()
}

// LetExpression is `let name = value; body` in expression position.
//
// It exists because lowering `match` has to evaluate the subject exactly once
// and then refer to it from every arm's test. Without it, each test would
// re-evaluate the subject, which is observably wrong whenever the subject has
// a side effect or costs something to compute.
//
// The binding is scoped to the body: the name is not visible after the
// expression, and the body's value is the expression's value.
type LetExpression struct {
	Token token.Token
	Name  *Identifier
	Type  string
	Value Expression
	Body  Expression
}

func (le *LetExpression) expressionNode()      {}
func (le *LetExpression) TokenLiteral() string { return le.Token.Literal }
func (le *LetExpression) String() string {
	return "let " + le.Name.Value + " = " + le.Value.String() + "; " + le.Body.String()
}

func (re *RangeExpression) expressionNode()      {}
func (re *RangeExpression) TokenLiteral() string { return re.Token.Literal }
func (re *RangeExpression) String() string {
	var out bytes.Buffer
	if re.Start != nil {
		out.WriteString(re.Start.String())
	}
	if re.Inclusive {
		out.WriteString("..=")
	} else {
		out.WriteString("..")
	}
	if re.End != nil {
		out.WriteString(re.End.String())
	}
	return out.String()
}

func (bs *BreakStatement) statementNode()       {}
func (bs *BreakStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BreakStatement) String() string       { return "break;" }

func (cs *ContinueStatement) statementNode()       {}
func (cs *ContinueStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *ContinueStatement) String() string       { return "continue;" }

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

type TaskExpression struct {
	Token token.Token // token.TASK
	Body  *BlockStatement
}

func (te *TaskExpression) expressionNode()      {}
func (te *TaskExpression) TokenLiteral() string { return te.Token.Literal }
func (te *TaskExpression) String() string {
	return "task { " + te.Body.String() + " }"
}

type AwaitExpression struct {
	Token token.Token // token.AWAIT
	Right Expression
}

func (ae *AwaitExpression) expressionNode()      {}
func (ae *AwaitExpression) TokenLiteral() string { return ae.Token.Literal }
func (ae *AwaitExpression) String() string {
	return "await " + ae.Right.String()
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

// Parameter is one formal parameter: a name and an optional written type.
//
// The type is optional rather than required because `fn f(x) { }` was already
// legal and remains so; a parameter with no annotation is `Any`, which is what
// every parameter meant before annotations existed.
//
// Keeping the annotation as text rather than a parsed type is deliberate. It is
// resolved against the file's scope by the type checker, in the same way a struct
// field's type is, so a parameter may name a type declared after it.
type Parameter struct {
	Token token.Token
	Name  *Identifier

	// TypeAnnotation is the written type, empty when none was given.
	TypeAnnotation string
}

func (p *Parameter) String() string {
	if p.TypeAnnotation == "" {
		return p.Name.Value
	}
	return p.Name.Value + ": " + p.TypeAnnotation
}

// ParameterNames returns just the names, for the consumers that bind them without
// caring about the annotations — the evaluator's frame setup, for one.
func (fl *FunctionLiteral) ParameterNames() []*Identifier {
	names := make([]*Identifier, 0, len(fl.Parameters))
	for _, p := range fl.Parameters {
		names = append(names, p.Name)
	}
	return names
}

type FunctionLiteral struct {
	Token token.Token // token.FN

	// IsPure declares the function to have no effects. A side-effecting call
	// inside one is E0202, which is what makes the declaration worth writing.
	//
	// It is a claim, not a fact the compiler establishes. Nothing forces a pure
	// function's body to be pure; the checker rejects the calls it can see, and a
	// call it cannot see is why the modifier is written `pure fn` rather than
	// being inferred.
	IsPure bool

	// IsUnsafe declares that the function takes responsibility for
	// capability-gated operations it performs.
	//
	// Inside such a function a gated builtin does not require its capability to
	// be granted to the project: the function has declared that it copes with
	// the absence. The project must grant `unsafe` itself, so the exemption is a
	// decision someone recorded rather than one any function could make.
	IsUnsafe bool

	// Requires names the capabilities this function needs.
	//
	// It is checked at every call site rather than at run time, because a
	// function whose capability is missing cannot do its job: the failure is
	// known before the program runs.
	Requires []string

	// Parameters are the formal parameters, each with an optional type.
	Parameters []*Parameter

	// ReturnType is the written return type, empty when none was given.
	//
	// An absent annotation means `Any`, not "nothing": a function with no
	// annotation has always been callable and assignable anywhere, and saying it
	// returns nothing would reject every existing call.
	ReturnType string

	Body *BlockStatement
	Name string

	// TypeParams are the type parameters of a generic function: the `T, U` in
	// `fn map<T, U>(...)`.
	//
	// They are names, not types, because nothing is decided until a call site
	// binds them. That is the difference from a placeholder struct the parser
	// produces for an unknown name: a type variable is deliberately unresolved.
	TypeParams []string
}

// Modifiers renders the prefixes before `fn`, in the order they are written.
func (fl *FunctionLiteral) Modifiers() string {
	out := ""
	if fl.IsPure {
		out += "pure "
	}
	if fl.IsUnsafe {
		out += "unsafe "
	}
	return out
}

func (fl *FunctionLiteral) expressionNode()      {}
func (fl *FunctionLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FunctionLiteral) String() string {
	var out bytes.Buffer
	params := []string{}
	for _, p := range fl.Parameters {
		params = append(params, p.String())
	}
	out.WriteString(fl.Modifiers())
	out.WriteString(fl.TokenLiteral())
	if fl.Name != "" {
		out.WriteString(" " + fl.Name)
	}
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(")")
	if fl.ReturnType != "" {
		out.WriteString(" -> " + fl.ReturnType)
	}
	if len(fl.Requires) > 0 {
		out.WriteString(" requires " + strings.Join(fl.Requires, ", "))
	}
	// A signature has no body: a `trait` holds them, and so does an `extern`
	// declaration. Printing one dereferenced a nil Body, so printing a trait's own
	// method list — or an extern declaration — panicked rather than producing text.
	//
	// The `;` is what the source says. A signature that printed as if it had an
	// empty body would re-parse as a definition.
	if fl.Body == nil {
		out.WriteString(";")
		return out.String()
	}
	out.WriteString(" ")
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

	// Pairs is the key/value set of the literal.
	//
	// Deprecated for iteration: a Go map has no order, and SEMANTICS.md §1
	// requires left-to-right lexical evaluation. Use OrderedPairs.
	Pairs map[Expression]Expression

	// OrderedPairs preserves source order. The parser appends to it as it
	// reads each `key: value` pair, so evaluation, bytecode emission, and
	// pretty printing all observe the order the developer wrote.
	//
	// OrderedPairs is authoritative when non-empty. Pairs is retained so that
	// existing callers that look up by key keep working.
	OrderedPairs []HashPairExpr
}

// HashPairExpr is one `key: value` entry of a hash literal.
type HashPairExpr struct {
	Key   Expression
	Value Expression
}

// EntryCount reports how many pairs the literal holds, preferring the ordered
// slice so that callers do not have to know which field is populated.
func (hl *HashLiteral) EntryCount() int {
	if len(hl.OrderedPairs) > 0 {
		return len(hl.OrderedPairs)
	}
	return len(hl.Pairs)
}

// SetPair records a key/value pair in both the ordered slice and the map.
//
// It lives here so that the parser, the formatter, and any future builder all
// keep the two views consistent; a pair present in one and missing from the
// other is a determinism bug.
func (hl *HashLiteral) SetPair(key, value Expression) {
	if hl.Pairs == nil {
		hl.Pairs = make(map[Expression]Expression)
	}
	hl.Pairs[key] = value
	hl.OrderedPairs = append(hl.OrderedPairs, HashPairExpr{Key: key, Value: value})
}

func (hl *HashLiteral) expressionNode()      {}
func (hl *HashLiteral) TokenLiteral() string { return hl.Token.Literal }
func (hl *HashLiteral) String() string {
	// Print in source order when available. Falling back to a sorted map keeps
	// the output stable for hash nodes built by hand without an order slice.
	pairs := make([]string, 0, hl.EntryCount())
	if len(hl.OrderedPairs) > 0 {
		for _, p := range hl.OrderedPairs {
			pairs = append(pairs, p.Key.String()+":"+p.Value.String())
		}
	} else {
		rendered := make([]string, 0, len(hl.Pairs))
		for k, v := range hl.Pairs {
			rendered = append(rendered, k.String()+":"+v.String())
		}
		sort.Strings(rendered)
		pairs = rendered
	}
	return "{" + strings.Join(pairs, ", ") + "}"
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

func (sd *StateDeclaration) statementNode()       {}
func (sd *StateDeclaration) TokenLiteral() string { return sd.Token.Literal }
func (sd *StateDeclaration) String() string {
	var out bytes.Buffer
	out.WriteString(sd.TokenLiteral() + " ")
	if sd.Name != nil {
		out.WriteString(sd.Name.String())
	}
	if sd.Type != "" {
		out.WriteString(": " + sd.Type)
	}
	if sd.Value != nil {
		out.WriteString(" = " + sd.Value.String())
	}
	out.WriteString(";")
	return out.String()
}

type RenderMethod struct {
	Token token.Token
	Body  *BlockStatement
}

type EventHandler struct {
	Token      token.Token
	Event      *Identifier
	Parameters []*Identifier
	Body       *BlockStatement
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
	Build      *RenderMethod
	Handlers   []*EventHandler
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

// ─── IMPORT STATEMENT ───────────────────────────────────────────────────────

type ImportStatement struct {
	Token token.Token // token.IMPORT
	Path  *StringLiteral
	Alias *Identifier
	Names []*Identifier
}

// ExternStatement is `extern "C" fn name(params) -> Ret;` — a declaration that a
// function exists, not a definition of one.
//
// # Why the signature and not a body
//
// The declaration *is* the whole content. A C function is compiled elsewhere and
// linked in; there is nothing to put between the braces, and writing `{}` would
// claim a definition the linker would then find a duplicate of.
//
// The `Fn` it carries has no body for the same reason. Everything else about it —
// name, parameters, return annotation — is ordinary, and is parsed by the same
// code that parses a definition, so the two cannot drift apart in what they accept.
type ExternStatement struct {
	Token token.Token // token.EXTERN

	// Abi is the calling convention, written as a string. Only "C" exists, and it
	// is written out rather than implied so that a second ABI later is a different
	// declaration rather than a misspelling.
	Abi string

	// Fn is the signature. Body is always nil.
	Fn *FunctionLiteral
}

func (es *ExternStatement) statementNode()       {}
func (es *ExternStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExternStatement) String() string {
	var out bytes.Buffer
	out.WriteString("extern \"" + es.Abi + "\" ")
	if es.Fn != nil {
		// The signature prints its own `;` when it has no body, so appending one
		// here would give two.
		return out.String() + es.Fn.String()
	}
	out.WriteString(";")
	return out.String()
}

func (is *ImportStatement) statementNode()       {}
func (is *ImportStatement) TokenLiteral() string { return is.Token.Literal }
func (is *ImportStatement) String() string {
	var out bytes.Buffer
	out.WriteString(is.TokenLiteral() + " ")
	if len(is.Names) > 0 {
		out.WriteString("{ ")
		names := []string{}
		for _, n := range is.Names {
			names = append(names, n.String())
		}
		out.WriteString(strings.Join(names, ", "))
		out.WriteString(" } from ")
	}
	if is.Path != nil {
		out.WriteString(is.Path.String())
	}
	if is.Alias != nil {
		out.WriteString(" as " + is.Alias.String())
	}
	out.WriteString(";")
	return out.String()
}

// ─── ENTITY STATEMENT (web-implications.md Section 26) ──────────────────────

// EntityField represents a single field inside an entity declaration
type EntityField struct {
	Name         string
	Type         string
	IsPrimary    bool
	IsRequired   bool
	IsUnique     bool
	TargetEntity string
}

// EntityStatement represents "entity Product { ... }"
type EntityStatement struct {
	Token  token.Token // token.ENTITY
	Name   *Identifier
	Fields []EntityField
}

func (es *EntityStatement) statementNode()       {}
func (es *EntityStatement) TokenLiteral() string { return es.Token.Literal }
func (es *EntityStatement) String() string {
	var out bytes.Buffer
	out.WriteString("entity ")
	if es.Name != nil {
		out.WriteString(es.Name.String() + " ")
	}
	out.WriteString("{\n")
	for _, f := range es.Fields {
		out.WriteString("    " + f.Name + ": " + f.Type + "\n")
	}
	out.WriteString("}")
	return out.String()
}
