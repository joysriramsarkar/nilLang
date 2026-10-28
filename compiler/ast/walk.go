package ast

// Inspect walks an AST in source order, calling f for every Node. If f returns
// false, that node's children are not visited.
//
// Source order matters for callers that collect information in sequence, such
// as the capability scanner and any future "first occurrence" diagnostic. The
// walk visits children left to right for every node kind, which mirrors
// evaluation order as specified in SEMANTICS.md §1.
//
// Types that are not Nodes in their own right (RenderMethod, EventHandler,
// EntityField) are traversed in place rather than as separate cases.
func Inspect(node Node, f func(Node) bool) {
	if node == nil {
		return
	}
	if !f(node) {
		return
	}

	switch n := node.(type) {
	// ── Statements ────────────────────────────────────────────────────────
	case *Program:
		for _, stmt := range n.Statements {
			Inspect(stmt, f)
		}

	case *ExpressionStatement:
		Inspect(n.Expression, f)

	case *ExportStatement:
		// The wrapper is visited and then what it wraps, so an analysis looking
		// for declarations finds them and one looking for visibility finds that.
		if n.Decl != nil {
			Inspect(n.Decl, f)
		}

	case *WhileLetStatement:
		if n.Value != nil {
			Inspect(n.Value, f)
		}
		inspectMatchPattern(n.Pattern, f)
		InspectBlockBody(n.Body, f)

	// The declarative type nodes are listed so that an analysis walking the tree
	// finds them. Without these cases a visitor silently skips every `struct`,
	// `enum`, `match`, and `for` in the program — which is how the lowering pass
	// ended up not seeing an enum's variants and leaving `Circle(3)` as a call to
	// a name that is not a function.
	case *StructDecl:
		// A struct's fields are written types, not expressions, so there is
		// nothing inside one for an expression walker to visit.
	case *EnumDecl:
		// Likewise for variants and their payload types.
	case *TypeAliasDecl:
		// The target is a written type.
	case *StructLiteral:
		Inspect(n.Name, f)
		for _, field := range n.Fields {
			if field.Value != nil {
				Inspect(field.Value, f)
			}
		}
	case *EnumVariantExpr:
		Inspect(n.Name, f)
		if n.Payload != nil {
			Inspect(n.Payload, f)
		}
	// The optional-access nodes are listed for the same reason. A visitor that
	// does not know them skips their whole subtrees, so an analysis looking for a
	// call inside `a ?? f()` never sees `f`.
	case *CoalesceExpression:
		if n.Left != nil {
			Inspect(n.Left, f)
		}
		if n.Right != nil {
			Inspect(n.Right, f)
		}

	case *SafeAccessExpression:
		if n.Left != nil {
			Inspect(n.Left, f)
		}
		if n.Member != nil {
			Inspect(n.Member, f)
		}

	case *MatchExpression:
		if n.Subject != nil {
			Inspect(n.Subject, f)
		}
		for _, arm := range n.Arms {
			inspectMatchPattern(arm.Pattern, f)
			if arm.Guard != nil {
				Inspect(arm.Guard, f)
			}
			if arm.Body != nil {
				Inspect(arm.Body, f)
			}
		}
	case *LetExpression:
		if n.Name != nil {
			Inspect(n.Name, f)
		}
		if n.Value != nil {
			Inspect(n.Value, f)
		}
		if n.Body != nil {
			Inspect(n.Body, f)
		}
	case *RangeExpression:
		if n.Start != nil {
			Inspect(n.Start, f)
		}
		if n.End != nil {
			Inspect(n.End, f)
		}
	case *ForStatement:
		if n.Key != nil {
			Inspect(n.Key, f)
		}
		if n.Value != nil {
			Inspect(n.Value, f)
		}
		if n.Iterable != nil {
			Inspect(n.Iterable, f)
		}
		InspectBlockBody(n.Body, f)

	case *LetStatement:
		if n.Name != nil {
			Inspect(n.Name, f)
		}
		Inspect(n.Value, f)

	case *AssignStatement:
		Inspect(n.Name, f)
		Inspect(n.Value, f)

	case *IndexAssignStatement:
		Inspect(n.Left, f)
		Inspect(n.Index, f)
		Inspect(n.Value, f)

	case *ReturnStatement:
		Inspect(n.ReturnValue, f)

	case *WhileStatement:
		Inspect(n.Condition, f)
		InspectBlockBody(n.Body, f)

	case *BlockStatement:
		for _, stmt := range n.Statements {
			Inspect(stmt, f)
		}

	case *AppStatement:
		if n.Name != nil {
			Inspect(n.Name, f)
		}
		InspectBlockBody(n.Body, f)

	case *StateDeclaration:
		if n.Name != nil {
			Inspect(n.Name, f)
		}
		Inspect(n.Value, f)

	case *StyleStatement:
		if n.Name != nil {
			Inspect(n.Name, f)
		}

	case *EntityStatement:
		// Fields carry no expressions: a field type is a type name, not an
		// expression that could contain a call.

	case *ImportStatement:
		if n.Path != nil {
			Inspect(n.Path, f)
		}
		if n.Alias != nil {
			Inspect(n.Alias, f)
		}
		for _, name := range n.Names {
			Inspect(name, f)
		}

	case *ComponentLiteral:
		if n.Name != nil {
			Inspect(n.Name, f)
		}
		for _, prop := range n.Properties {
			if prop.Name != nil {
				Inspect(prop.Name, f)
			}
			Inspect(prop.Value, f)
		}
		for _, state := range n.States {
			if state.Name != nil {
				Inspect(state.Name, f)
			}
			Inspect(state.Value, f)
		}
		InspectRenderMethod(n.Render, f)
		InspectRenderMethod(n.Build, f)
		for _, handler := range n.Handlers {
			for _, param := range handler.Parameters {
				Inspect(param, f)
			}
			InspectBlockBody(handler.Body, f)
		}
		for _, method := range n.Methods {
			if method.Name != nil {
				Inspect(method.Name, f)
			}
			for _, param := range method.Parameters {
				Inspect(param, f)
			}
			InspectBlockBody(method.Body, f)
		}
		InspectBlockBody(n.Body, f)

	case *TraitDecl:
		if n.Name != nil {
			Inspect(n.Name, f)
		}
		for _, method := range n.Methods {
			inspectTraitMethod(method, f)
		}
		// The raw Body is not walked: the parser lifts every statement in it
		// into Methods (and rejects anything else), so walking both would
		// visit each signature twice. A trait method has no body, so nothing
		// expression-bearing is lost.

	case *ImplDecl:
		if n.Trait != nil {
			Inspect(n.Trait, f)
		}
		if n.Type != nil {
			Inspect(n.Type, f)
		}
		for _, method := range n.Methods {
			inspectTraitMethod(method, f)
		}

	case *ExternStatement:
		// The declaration is a signature with no body, but the Fn is an
		// ordinary FunctionLiteral, so walking it visits the name and the
		// parameter names the same way a definition's are visited.
		if n.Fn != nil {
			Inspect(n.Fn, f)
		}

	// ── Expressions ───────────────────────────────────────────────────────
	case *IntegerLiteral, *FloatLiteral, *StringLiteral, *Boolean, *NullLiteral,
		*Identifier:
		// Leaf nodes.

	// Await and task are not leaves: each wraps work of its own — the awaited
	// expression, the task's body — and an analysis that stops at the wrapper
	// never sees the calls inside either. They were once listed with the
	// leaves above, which is how calls under `await` went uncollected.
	case *AwaitExpression:
		if n.Right != nil {
			Inspect(n.Right, f)
		}

	case *TaskExpression:
		InspectBlockBody(n.Body, f)

	case *PrefixExpression:
		Inspect(n.Right, f)

	case *InfixExpression:
		Inspect(n.Left, f)
		Inspect(n.Right, f)

	case *IfExpression:
		Inspect(n.Condition, f)
		InspectBlockBody(n.Consequence, f)
		InspectBlockBody(n.Alternative, f)

	case *FunctionLiteral:
		// The walk visits each parameter's name, which is the node that can
		// appear in an expression. A Parameter's type annotation is not walked:
		// it is written text resolved statically, never an expression, so there
		// is nothing in it for an expression walker to find.
		for _, param := range n.Parameters {
			if param != nil && param.Name != nil {
				Inspect(param.Name, f)
			}
		}
		InspectBlockBody(n.Body, f)

	case *CallExpression:
		Inspect(n.Function, f)
		for _, arg := range n.Arguments {
			Inspect(arg, f)
		}

	case *ArrayLiteral:
		for _, elem := range n.Elements {
			Inspect(elem, f)
		}

	case *IndexExpression:
		Inspect(n.Left, f)
		Inspect(n.Index, f)

	case *DotExpression:
		Inspect(n.Left, f)
		if n.Member != nil {
			Inspect(n.Member, f)
		}

	case *HashLiteral:
		for _, entry := range n.OrderedPairs {
			Inspect(entry.Key, f)
			Inspect(entry.Value, f)
		}
		// Nodes built without an order slice still need their contents visited.
		for key, value := range n.Pairs {
			Inspect(key, f)
			Inspect(value, f)
		}

	case *StringTemplate:
		for _, part := range n.Parts {
			if part.IsExpression {
				Inspect(part.Expression, f)
			}
		}
	}
}

// inspectMatchPattern walks one arm's pattern.
//
// A pattern's sub-expressions are the literals inside it — a string in
// match x { "a" => ... } — so those are visited. A binding is a name being
// introduced rather than a read, so it is not.
func inspectMatchPattern(pattern MatchPattern, f func(Node) bool) {
	if pattern.Literal != nil {
		Inspect(pattern.Literal, f)
	}
	if pattern.EnumVariant != nil && pattern.EnumVariant.Variant != nil {
		Inspect(pattern.EnumVariant.Variant, f)
	}
	if pattern.StructPattern != nil {
		if pattern.StructPattern.Name != nil {
			Inspect(pattern.StructPattern.Name, f)
		}
		for _, field := range pattern.StructPattern.Fields {
			inspectMatchPattern(field.Pattern, f)
		}
	}
	for _, alternative := range pattern.Alternatives {
		inspectMatchPattern(alternative, f)
	}
}

// inspectTraitMethod walks one trait or impl method in place.
//
// A TraitMethod is not a Node in its own right, so — like RenderMethod and
// EventHandler — it is traversed where it sits. Parameter names are visited
// as they are for a FunctionLiteral; the body is walked for an impl method
// and is nil for a trait signature.
func inspectTraitMethod(method *TraitMethod, f func(Node) bool) {
	if method == nil {
		return
	}
	for _, param := range method.Parameters {
		if param != nil && param.Name != nil {
			Inspect(param.Name, f)
		}
	}
	InspectBlockBody(method.Body, f)
}

// InspectBlockBody visits a block, tolerating a nil body.
func InspectBlockBody(body *BlockStatement, f func(Node) bool) {
	if body == nil {
		return
	}
	for _, stmt := range body.Statements {
		Inspect(stmt, f)
	}
}

// InspectRenderMethod visits a render or build block, tolerating nil.
func InspectRenderMethod(method *RenderMethod, f func(Node) bool) {
	if method == nil {
		return
	}
	InspectBlockBody(method.Body, f)
}
