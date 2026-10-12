package hir

import (
	"fmt"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/types"
)

type Lowerer struct {
	symbols map[string]types.Type

	// exported records the visibility each declaration was written with.
	exported map[string]ast.Visibility

	// unhandled names every AST node this lowering had no translation for.
	//
	// Why it is collected rather than returned: a missing case used to produce
	// `nil` — the same value a *legitimately* empty translation produces, so the
	// two were indistinguishable downstream. A caller that skipped a nil would skip
	// a bug and skip a no-op with the same face, and the failure would surface much
	// later as a nil dereference in a backend that had no way to know what had gone
	// missing.

	// Collecting the names turns "something went wrong" into "this node has no
	// translation", which is the difference between a bug report and a shrug. It
	// is a list rather than an error return because lowering a program must be able
	// to continue past a node it cannot represent — the alternative is that one
	// unknown node hides every other error in the file.
	unhandled map[string]int
}

// UnhandledNodes reports which AST node types had no HIR translation, and how many
// of each were dropped.
//
// It is exported because a backend cannot fix what it cannot see. The lowering
// itself runs long before anyone is told what it lost, so a caller that wants to
// warn, or to fail loudly rather than proceed, has to be able to ask.
func (l *Lowerer) UnhandledNodes() map[string]int {
	if l.unhandled == nil {
		return nil
	}
	out := make(map[string]int, len(l.unhandled))
	for name, count := range l.unhandled {
		out[name] = count
	}
	return out
}

// noteUnhandled records an AST node type that has no translation.
func (l *Lowerer) noteUnhandled(node any) {
	if l.unhandled == nil {
		l.unhandled = make(map[string]int)
	}
	l.unhandled[fmt.Sprintf("%T", node)]++
}

func NewLowerer() *Lowerer {
	return &Lowerer{
		symbols:   make(map[string]types.Type),
		exported:  make(map[string]ast.Visibility),
		unhandled: make(map[string]int),
	}
}

// structTypeOf is the type a struct declaration introduces.
//
// It is the same `types.StructType` a struct literal produces, so a value and the
// name that describes it compare equal. A distinct "declared struct" type would
// make `let p: P = P { x: 1 }` fail on every use — a declaration that cannot be
// used to annotate anything is not a declaration.
func structTypeOf(decl *TypeDeclStmt) types.Type {
	st := &types.StructType{Name: decl.Name}
	for _, f := range decl.Fields {
		fieldType := f.Type
		if fieldType == nil {
			fieldType = types.Any
		}
		st.Fields = append(st.Fields, types.StructField{Name: f.Name, Type: fieldType})
	}
	return st
}

func (l *Lowerer) LowerProgram(prog *ast.Program) *Program {
	hp := &Program{
		Statements: []Statement{},
	}
	for _, s := range prog.Statements {
		if app, ok := s.(*ast.AppStatement); ok {
			if app.Body != nil {
				for _, nested := range app.Body.Statements {
					if hs := l.lowerStatement(nested); hs != nil {
						hp.Statements = append(hp.Statements, hs)
					}
				}
			}
			continue
		}
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
			Constant: s.Constant,
		}

	case *ast.StateDeclaration:
		value := l.lowerExpression(s.Value)
		var valueType types.Type = types.Any
		if s.Type != "" {
			if parsed, err := types.Parse(s.Type); err == nil {
				valueType = parsed
			}
		} else if value != nil {
			valueType = value.Type()
		}
		name := ""
		if s.Name != nil {
			name = s.Name.Value
			l.symbols[name] = valueType
		}
		return &LetStmt{Name: name, VarType: valueType, Value: value}

	case *ast.ComponentLiteral:
		component := &ComponentDeclStmt{}
		if s.Name != nil {
			component.Name = s.Name.Value
			l.symbols[component.Name] = types.Any
		}
		for _, state := range s.States {
			if lowered, ok := l.lowerStatement(state).(*LetStmt); ok {
				component.States = append(component.States, lowered)
			}
		}
		if s.Render != nil {
			component.Render = l.lowerBlock(s.Render.Body)
		}
		if s.Build != nil {
			component.Build = l.lowerBlock(s.Build.Body)
		}
		for _, handler := range s.Handlers {
			name := ""
			if handler.Event != nil {
				name = handler.Event.Value
			}
			parameters := make([]string, 0, len(handler.Parameters))
			for _, parameter := range handler.Parameters {
				parameters = append(parameters, parameter.Value)
			}
			component.Handlers = append(component.Handlers, ComponentEventDecl{Name: name, Params: parameters, Body: l.lowerBlock(handler.Body)})
		}
		return component

	case *ast.AssignStatement:
		value := s.Value
		if s.Operator == "+=" || s.Operator == "-=" {
			value = &ast.InfixExpression{Token: s.Token, Left: s.Name, Operator: strings.TrimSuffix(s.Operator, "="), Right: s.Value}
		}
		val := l.lowerExpression(value)
		// The target is the expression on the left of the `=`, lowered like any
		// other. It is a variable far more often than not, but `a[1] = 9` and
		// `s.field = 9` reach here too, and reading a name off an index
		// expression used to produce an assignment to a variable called "".
		target := l.lowerExpression(s.Name)
		return &AssignStmt{
			Target: target,
			Value:  val,
		}

	case *ast.ExternStatement:
		// Wrapped in an expression statement, which is how an ordinary named
		// function declaration reaches HIR too. The two are then identical apart
		// from two fields, and every pass that walks functions handles both without
		// knowing either exists as a special case.
		//
		// The parameter annotations travel as written rather than resolved. A
		// backend needs to know how each value *crosses* the boundary, and that is
		// decided by the declared type; resolving it first would leave `Int` and
		// `Any` indistinguishable at exactly the point where they cross differently.
		params := make([]string, 0, len(s.Fn.Parameters))
		for _, p := range s.Fn.Parameters {
			if p != nil {
				params = append(params, p.TypeAnnotation)
			}
		}

		fn := l.lowerExpression(s.Fn)
		lit, ok := fn.(*FnLit)
		if !ok {
			return nil
		}
		lit.ExternAbi = s.Abi
		lit.ExternReturn = s.Fn.ReturnType
		lit.ExternParams = params
		return &ExprStmt{Expr: lit}

	case *ast.IndexAssignStatement:
		// `a[i] = v` and `a.b = v` reach HIR through here, and there was no case
		// for them at all — `lowerStatement` fell through its switch and returned
		// nothing, so the write was dropped between the parse tree and the IR.
		//
		// Nothing failed, because dropping a statement is what `nil` in that
		// switch means. `a[1] = 9` compiled, ran, and left `a` alone; the next
		// read answered 2. The interpreter and the VM walk the AST directly, which
		// is why both were right and only the compiled back end was wrong — a
		// disagreement that pointed at the back end and was actually three stages
		// earlier.
		//
		// The parser has already normalised both spellings into one node, with a
		// member arriving as a string-literal index, so both become one subscript.
		return &AssignStmt{
			Target: &IndexExpr{
				Left:  l.lowerExpression(s.Left),
				Index: l.lowerExpression(s.Index),
			},
			Value: l.lowerExpression(s.Value),
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

	case *ast.EntityStatement:
		name := ""
		if s.Name != nil {
			name = s.Name.Value
		}
		fields := make([]EntityFieldDecl, 0, len(s.Fields))
		typeFields := make([]types.EntityFieldDef, 0, len(s.Fields))
		for _, f := range s.Fields {
			ft, err := types.Parse(f.Type)
			if err != nil {
				ft = types.Any
			}
			fields = append(fields, EntityFieldDecl{
				Name:         f.Name,
				Type:         ft,
				IsPrimary:    f.IsPrimary,
				IsRequired:   f.IsRequired,
				IsUnique:     f.IsUnique,
				TargetEntity: f.TargetEntity,
			})
			typeFields = append(typeFields, types.EntityFieldDef{
				Name:         f.Name,
				Type:         ft,
				IsPrimary:    f.IsPrimary,
				IsRequired:   f.IsRequired,
				IsUnique:     f.IsUnique,
				TargetEntity: f.TargetEntity,
			})
		}
		entType := &types.EntityType{
			Name:   name,
			Fields: typeFields,
		}
		l.symbols[name] = entType
		return &EntityDeclStmt{
			Name:       name,
			EntityType: entType,
			Fields:     fields,
		}

	case *ast.BreakStatement:
		return &LoopControlStmt{Kind: "break"}

	case *ast.ContinueStatement:
		return &LoopControlStmt{Kind: "continue"}

	case *ast.StructDecl:
		// A declaration, kept as data and emitting nothing.
		//
		// The run-time needs no struct — its fields are checked statically and its
		// values are ordinary maps. What the run-time does not need is not the same
		// as what is worth discarding: a dropped node cannot be reported or
		// printed, and "what does this program declare?" becomes unanswerable.
		decl := &TypeDeclStmt{Kind: KindStruct}
		if s.Name != nil {
			decl.Name = s.Name.Value
		}
		for _, f := range s.Fields {
			field := TypeField{}
			if f.Name != nil {
				field.Name = f.Name.Value
			}
			field.TypeString = f.TypeAnnotation
			if parsed, err := types.Parse(f.TypeAnnotation); err == nil {
				field.Type = parsed
				field.TypeString = parsed.String()
			}
			decl.Fields = append(decl.Fields, field)
		}
		l.symbols[decl.Name] = structTypeOf(decl)
		return decl

	case *ast.EnumDecl:
		decl := &TypeDeclStmt{Kind: KindEnum}
		if s.Name != nil {
			decl.Name = s.Name.Value
		}
		for _, v := range s.Variants {
			variant := TypeVariant{Payload: v.PayloadType}
			if v.Name != nil {
				variant.Name = v.Name.Value
			}
			variant.HasPayload = v.PayloadType != ""
			decl.Variants = append(decl.Variants, variant)

			// A variant is also a *value*: `Red` is an expression, not a type.
			// Registering it here is what makes a bare `Red` resolve in expression
			// position rather than reporting an undefined identifier.
			if variant.Name != "" {
				l.symbols[variant.Name] = types.Any
			}
		}
		l.symbols[decl.Name] = types.Any
		return decl

	case *ast.TypeAliasDecl:
		decl := &TypeDeclStmt{Kind: KindTypeAlias, Target: types.Any}
		if s.Name != nil {
			decl.Name = s.Name.Value
		}
		if parsed, err := types.Parse(s.Target); err == nil {
			decl.Target = parsed
		}
		// The alias resolves to its target, not to itself. A name bound to a
		// distinct "alias" type would make every use of it fail assignability,
		// which is the opposite of what an alias is for.
		l.symbols[decl.Name] = decl.Target
		return decl

	case *ast.TraitDecl:
		decl := &TypeDeclStmt{Kind: KindTrait}
		if s.Name != nil {
			decl.Name = s.Name.Value
		}
		for _, m := range s.Methods {
			if m == nil || m.Name == "" {
				continue
			}
			decl.Fields = append(decl.Fields, TypeField{Name: m.Name, TypeString: "fn"})
		}
		l.symbols[decl.Name] = types.Any
		return decl

	case *ast.ExportStatement:
		// `export` and `private` wrap a declaration rather than replacing one, so
		// the declaration is lowered as itself and the visibility is recorded
		// beside it.
		//
		// The visibility has already been enforced — an import of a non-exported
		// name is an error before this runs. What is recorded here is for a later
		// pass that has to answer "what does this module offer?", which a
		// visibility the lowering throws away cannot answer.
		if s.Decl == nil {
			return nil
		}
		inner := l.lowerStatement(s.Decl)
		if inner != nil {
			l.exported[s.Decl.String()] = s.Visibility
		}
		return inner
	default:
		l.noteUnhandled(s)
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

	case *ast.TaskExpression:
		// A task is a *body*, not an expression inside one, so it keeps a block
		// rather than being spliced into the enclosing statement list. Splicing
		// would make the task's statements indistinguishable from the enclosing
		// ones, and a `break` written inside the task would then look like it
		// belonged to the loop outside it.
		return &TaskExpr{
			Body:     l.lowerBlock(e.Body),
			ExprType: types.Any,
		}

	case *ast.AwaitExpression:
		return &AwaitExpr{
			Future:   l.lowerExpression(e.Right),
			ExprType: types.Any,
		}

	case *ast.Boolean:
		return &BoolLit{Val: e.Value}

	case *ast.LetExpression:
		// `let x = v; body` — a braced block, and the single most important node
		// the lowering pass produces. `?.`, `??` and `match` all desugar into it,
		// because binding the receiver once is what stops a chain from reading the
		// same value twice.
		//
		// It lowers to a block rather than to a node of its own because it *is* a
		// block: a declaration followed by an expression. A node carrying its own
		// body would need MIR to understand it, and every construct that desugars
		// into a `let` would have to teach MIR about that field rather than reuse
		// the block machinery every other statement already goes through.
		if e.Name == nil {
			return l.lowerExpression(e.Body)
		}

		value := l.lowerExpression(e.Value)
		var declared types.Type = types.Any
		if e.Type != "" {
			if parsed, err := types.Parse(e.Type); err == nil {
				declared = parsed
			}
		} else if value != nil {
			declared = value.Type()
		}
		l.symbols[e.Name.Value] = declared

		return &BlockExpr{
			Statements: []Statement{
				&LetStmt{Name: e.Name.Value, VarType: declared, Value: value},
				&ExprStmt{Expr: l.lowerExpression(e.Body)},
			},
		}

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

		// Both arms are lowered, and both are named. The alternative used to be
		// built and then discarded, which left an operator with one operand and
		// made `cond ? a : b` evaluate to `a` no matter what `cond` said.
		conseq := l.lowerBlock(e.Consequence)
		var alt *BlockStmt
		if e.Alternative != nil {
			alt = l.lowerBlock(e.Alternative)
		}

		whenTrue := blockValue(conseq)
		var whenFalse Expression = &NullLit{}
		if alt != nil {
			whenFalse = blockValue(alt)
		}

		return &IfExpr{
			Condition: cond,
			WhenTrue:  whenTrue,
			WhenFalse: whenFalse,
			ExprType:  types.Any,
		}

	case *ast.FunctionLiteral:
		var params []string
		var paramTypes []types.Type
		for _, p := range e.Parameters {
			if p == nil || p.Name == nil {
				continue
			}
			name := p.Name.Value
			params = append(params, name)
			paramTypes = append(paramTypes, types.Any)
			l.symbols[name] = types.Any
		}
		body := l.lowerBlock(e.Body)
		return &FnLit{
			Name:   e.Name,
			Params: params,
			Body:   body,
			FnType: &types.FunctionType{Params: paramTypes, ReturnType: types.Any},
			IsPure: e.IsPure,
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
		// Lower in source order so that MIR, and therefore every backend built
		// on it, observes the same evaluation order as the evaluator.
		keys := make([]Expression, 0, e.EntryCount())
		values := make([]Expression, 0, e.EntryCount())
		if len(e.OrderedPairs) > 0 {
			for _, entry := range e.OrderedPairs {
				keys = append(keys, l.lowerExpression(entry.Key))
				values = append(values, l.lowerExpression(entry.Value))
			}
		} else {
			for k, v := range e.Pairs {
				keys = append(keys, l.lowerExpression(k))
				values = append(values, l.lowerExpression(v))
			}
		}
		return &MapLit{
			Keys:   keys,
			Values: values,
		}

	default:
		l.noteUnhandled(e)
		return nil
	}
}
