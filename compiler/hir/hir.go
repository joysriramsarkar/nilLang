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

// AssignStmt is `target = value`.
//
// # Why the target is an expression
//
// Assignment is not only to a variable. `a[1] = 9` and `s.field = 9` are
// assignments too, and this node used to carry a `Name string` and nothing else,
// which meant the HIR lowerer had nowhere to put them: it read the name, found
// none — the left of those assignments is an index or a field, not a name — and
// recorded an assignment to a variable called "".
//
// Nothing failed. The program compiled, ran, and the write went nowhere: `a[1] = 9`
// left `a` alone, and the next read answered the old value. A construct that is
// silently dropped is worse than one that is refused, because the program keeps
// running and the answer looks plausible.
//
// So the target is an expression and `VarName` asks the narrow question the places
// that only understand variables need to ask.
type AssignStmt struct {
	Target Expression
	Value  Expression
}

// VarName is the variable this assigns to, and whether it assigns to one.
//
// A caller that can only handle a variable — hoisting a loop invariant, naming a
// thing a loop writes — uses this to decline the rest rather than reading a
// `Name` that is no longer there and concluding the assignment writes nothing.
func (s *AssignStmt) VarName() (string, bool) {
	ref, ok := s.Target.(*VarRef)
	if !ok {
		return "", false
	}
	return ref.Name, true
}

func (s *AssignStmt) stmtNode() {}
func (s *AssignStmt) String() string {
	return fmt.Sprintf("%s = %s;", s.Target.String(), s.Value.String())
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

// IfExpr is an `if` used where a value is expected.
//
// # Why it exists
//
// The lowering used to answer this with a `BinaryExpr` whose operator was `"?"`
// and whose right operand was the *consequence only*. The alternative block was
// lowered — built, walked, allocated — and then dropped with `_ = alt`.
//
// A node that names one arm and not the other is not a ternary, it is a bug with
// a ternary's syntax. Naming both is what makes the IR checkable: a reader can see
// that both arms exist, and a printer cannot accidentally omit one.
type IfExpr struct {
	Condition Expression
	WhenTrue  Expression
	WhenFalse Expression
	ExprType  types.Type
}

func (e *IfExpr) exprNode() {}

func (e *IfExpr) Type() types.Type {
	if e.ExprType != nil {
		return e.ExprType
	}
	return types.Any
}

func (e *IfExpr) String() string {
	alternative := "null"
	if e.WhenFalse != nil {
		alternative = e.WhenFalse.String()
	}
	return fmt.Sprintf("%s ? %s : %s", e.Condition.String(), e.WhenTrue.String(), alternative)
}

// blockValue is the value a block produces: its last statement, if it has one.
//
// A ternary's arm is a block, and a block's value is what it last computed. Looking
// at the *first* statement would be a guess that happens to work for a one-line
// arm and silently misreads a longer one.
func blockValue(b *BlockStmt) Expression {
	if b == nil || len(b.Statements) == 0 {
		return &NullLit{}
	}
	switch st := b.Statements[len(b.Statements)-1].(type) {
	case *ExprStmt:
		return st.Expr
	case *ReturnStmt:
		if st.Value != nil {
			return st.Value
		}
	}
	return &NullLit{}
}

// BlockExpr is a block used where a value is expected.
//
// # Why a block needs to be an expression
//
// `let x = v; body` is a declaration followed by an expression, and it arrives
// from the lowering pass as a single node. Without a way to say that in an
// expression position there are two choices, both bad: a dedicated node with its
// own body field, which MIR must then learn, or dropping the body, which silently
// discards whatever the construct was for.
//
// The second is what a `?.` or a `??` looks like when it is mishandled: the guard
// survives and the guarded access does not.
//
// A block in a statement position is already handled by `BlockStmt`; this is the
// same thing with a value at the end.
type BlockExpr struct {
	Statements []Statement
}

func (b *BlockExpr) exprNode() {}

// Type is the type of the block's last expression, which is the block's value.
func (b *BlockExpr) Type() types.Type {
	if len(b.Statements) == 0 {
		return types.Any
	}
	if last, ok := b.Statements[len(b.Statements)-1].(*ExprStmt); ok {
		return last.Expr.Type()
	}
	return types.Any
}

func (b *BlockExpr) String() string {
	var out strings.Builder
	out.WriteString("{\n")
	for _, s := range b.Statements {
		out.WriteString("\t" + s.String() + "\n")
	}
	out.WriteString("}")
	return out.String()
}

// TypeDeclKind names which kind of type a `TypeDeclStmt` declares.
//
// It is a string rather than an enum because the node is *information*, not control
// flow: no backend branches on it. Naming the kind is enough for a diagnostic, a
// pretty-printer, or a later pass that wants to know a name is an enum.
type TypeDeclKind string

const (
	KindStruct     TypeDeclKind = "struct"
	KindEnum       TypeDeclKind = "enum"
	KindTypeAlias  TypeDeclKind = "type"
	KindTrait      TypeDeclKind = "trait"
	KindVisibility TypeDeclKind = "visibility"
)

// TypeDeclStmt is a declaration with no run-time effect: a struct, an enum, a type
// alias, or a trait.
//
// # Why it exists when the run-time needs nothing
//
// A declaration like `struct P { x: Int }` produces no value and emits no
// instruction. The tempting move is to drop it — and that is precisely what the
// `default` arm of this lowering did, because dropping and having-nothing-to-emit
// both return `nil`.
//
// The difference matters downstream. A dropped node cannot be reported, cannot be
// pretty-printed, and cannot be answered for: "does this program declare a struct
// called `P`?" becomes unanswerable, which is the question a language server asks
// first.
//
// So the declaration is kept as data and emits nothing. Backends ignore it, which
// is what they should do, and a pass that needs it has something to read.
type TypeDeclStmt struct {
	Kind     TypeDeclKind
	Name     string
	Fields   []TypeField
	Variants []TypeVariant
	// Target is the aliased type, for `type Meters = Int`.
	Target types.Type
}

// TypeField is one field of a declared struct or entity.
type TypeField struct {
	Name       string
	TypeString string
	Type       types.Type
}

// TypeVariant is one variant of a declared enum.
type TypeVariant struct {
	Name       string
	Payload    string
	HasPayload bool
}

func (s *TypeDeclStmt) stmtNode() {}

func (s *TypeDeclStmt) String() string {
	if s.Kind == KindTypeAlias {
		return "type " + s.Name + " = " + s.Target.String()
	}
	var out strings.Builder
	out.WriteString(string(s.Kind) + " " + s.Name + " {")
	for i, f := range s.Fields {
		if i > 0 {
			out.WriteString(",")
		}
		out.WriteString(" " + f.Name + ": " + f.TypeString)
	}
	for i, v := range s.Variants {
		if i > 0 || len(s.Fields) > 0 {
			out.WriteString(",")
		}
		out.WriteString(" " + v.Name)
	}
	out.WriteString(" }")
	return out.String()
}

// LoopControlStmt is `break` or `continue`.
//
// # Why it is one node with a kind
//
// Both mean "leave the innermost loop", and they differ only in where they land:
// `break` at the loop's exit, `continue` at its condition. Splitting them into two
// nodes would mean every piece of loop machinery — MIR's lowering, any later
// analysis, a pretty-printer — handling both, with one branch between them that
// says which is which. One node with a kind keeps that branch in one place.
//
// They are statements rather than expressions because neither produces a value. A
// `break` in expression position would need a type, and the only honest one is
// "no value", which the language does not have.
type LoopControlStmt struct {
	Kind  string // "break" or "continue"
	Label string // reserved for labelled loops; empty today
}

func (s *LoopControlStmt) stmtNode() {}

func (s *LoopControlStmt) String() string {
	if s.Label != "" {
		return s.Kind + " " + s.Label
	}
	return s.Kind
}

// TaskExpr starts an asynchronous computation and evaluates to a future.
//
// `await` on that future blocks until it finishes. They are separate nodes
// because the language separates them: a task can be started and awaited much
// later, and awaiting the same future twice returns the same result — which a
// single node that both starts and awaits could not express.
type TaskExpr struct {
	Body     *BlockStmt
	ExprType types.Type
}

func (e *TaskExpr) exprNode() {}

func (e *TaskExpr) Type() types.Type { return e.ExprType }

func (e *TaskExpr) String() string { return "task " + e.Body.String() }

// AwaitExpr blocks until the future it is given has finished, and yields its
// value.
type AwaitExpr struct {
	Future   Expression
	ExprType types.Type
}

func (e *AwaitExpr) exprNode() {}

func (e *AwaitExpr) Type() types.Type { return e.ExprType }

func (e *AwaitExpr) String() string { return "await " + e.Future.String() }

type WhileStmt struct {
	Condition Expression
	Body      *BlockStmt

	// PreLoop holds bindings the optimiser lifted out of the body because the
	// loop cannot change them.
	//
	// They live on the loop rather than in the enclosing block because the loop
	// can be nested anywhere, and a pass that reached into the enclosing block
	// would have to be told where the loop was. A backend lowers them into the
	// block immediately before the loop header, which is the only place they mean
	// anything.
	PreLoop []Statement
}

type ComponentEventDecl struct {
	Name   string
	Params []string
	Body   *BlockStmt
}

type ComponentDeclStmt struct {
	Name     string
	States   []*LetStmt
	Render   *BlockStmt
	Build    *BlockStmt
	Handlers []ComponentEventDecl
}

func (s *ComponentDeclStmt) stmtNode() {}
func (s *ComponentDeclStmt) String() string {
	return fmt.Sprintf("component %s { states=%d handlers=%d }", s.Name, len(s.States), len(s.Handlers))
}

func (s *WhileStmt) stmtNode() {}
func (s *WhileStmt) String() string {
	// Hoisted bindings are printed with the loop, not inside it, because that is
	// where they now live. A `while` that silently omits them would read as though
	// the values had been computed inside the body — and a reader checking whether
	// a hoist is sound would be reading the wrong scope.
	if len(s.PreLoop) > 0 {
		var hoisted strings.Builder
		for _, stmt := range s.PreLoop {
			hoisted.WriteString("\t" + stmt.String() + "\n")
		}
		return hoisted.String() + fmt.Sprintf("while (%s) %s", s.Condition.String(), s.Body.String())
	}
	return fmt.Sprintf("while (%s) %s", s.Condition.String(), s.Body.String())
}

// ─── ENTITY DECLARATION (web-implications.md Section 26) ────────────────────

type EntityFieldDecl struct {
	Name         string
	Type         types.Type
	IsPrimary    bool
	IsRequired   bool
	IsUnique     bool
	TargetEntity string
}

type EntityDeclStmt struct {
	Name       string
	EntityType *types.EntityType
	Fields     []EntityFieldDecl
}

func (s *EntityDeclStmt) stmtNode() {}
func (s *EntityDeclStmt) String() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("entity %s {\n", s.Name))
	for _, f := range s.Fields {
		b.WriteString(fmt.Sprintf("  %s: %s;\n", f.Name, f.Type.String()))
	}
	b.WriteString("}")
	return b.String()
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
	// Name is the function's name, empty for an anonymous function literal.
	//
	// It was never read, so every named function reached HIR as `fn()` and two
	// functions were indistinguishable in the IR. That is worse than missing
	// information in a diagnostic: a reader looking at HIR cannot tell which
	// function a body belongs to, and neither can a pass that wants to rename
	// one.
	Name   string
	Params []string
	Body   *BlockStmt
	FnType types.Type

	// IsPure carries a `pure fn` declaration through the IR. The backends
	// read it for the runtime E0202 check; a pass that rebuilds the node
	// without copying it silently deletes the declaration.
	IsPure bool

	// ExternAbi names the ABI this function is declared in, and is empty for a
	// function this program defines.
	//
	// # Why a field rather than a separate node
	//
	// A foreign function has the same signature as a Nilang one and differs in
	// exactly two ways: it has no body, and it is not emitted. Both are facts
	// *about* a function, and a separate `ExternDecl` node would mean every pass
	// that walks functions — the optimiser, the hoister, the walker — needs a case
	// for it that does exactly what the `FnLit` case does minus two fields. The
	// one place that must behave differently is code generation, and that one
	// place reads this field.
	ExternAbi string

	// ExternReturn is the written return type of an extern declaration, which the
	// C signature needs and a Nilang function type does not carry.
	//
	// It is separate from `FnType` because `FnType` describes a Nilang function
	// value: a type usable in a variable annotation, a parameter, a return. An
	// extern declaration is not that — it is a name the linker resolves — and
	// flattening the two would put a C signature into every place a function type
	// is written.
	ExternReturn string

	// ExternParams is each parameter's written annotation, in order.
	//
	// # Why the text rather than the resolved type
	//
	// Because what a backend needs is how the value *crosses*, and that is decided
	// by what the programmer wrote: `Int` crosses as `int64_t` and `String` as a
	// pointer and a length. Resolving them first would collapse `Int` and `Any`
	// into the same Nilang type and leave nothing to say how either crosses.
	ExternParams []string
}

func (e *FnLit) exprNode()        {}
func (e *FnLit) Type() types.Type { return e.FnType }
func (e *FnLit) String() string {
	// An extern declaration prints as what it is. Printing it as `fn f(...) {}`
	// would say the program defines a function it does not define, and would not
	// re-parse.
	if e.ExternAbi != "" {
		signature := fmt.Sprintf("extern %q fn %s(%s)", e.ExternAbi, e.Name, strings.Join(e.Params, ", "))
		if e.ExternReturn != "" {
			signature += " -> " + e.ExternReturn
		}
		return signature + ";"
	}

	body := "{}"
	if e.Body != nil {
		body = e.Body.String()
	}
	if e.Name != "" {
		return fmt.Sprintf("fn %s(%s) %s", e.Name, strings.Join(e.Params, ", "), body)
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
