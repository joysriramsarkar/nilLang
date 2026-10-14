// Package lower desugars Nilang's higher level loop forms into the subset that
// the execution engines actually implement.
//
// # Why a lowering pass rather than per-engine implementations
//
// `for` and `break`/`continue` have no representation in the stack VM's
// instruction set. Each engine could implement them separately, but then the two
// would be free to disagree, and cross-engine conformance would have to be
// maintained by hand for every construct.
//
// Lowering once, before any backend sees the program, makes agreement structural
// rather than tested: both engines run the same `while` loop, because there is
// only one.
//
// # What is lowered
//
//	for k, v in xs { body }   becomes  let __nil_coll = xs; let __nil_i = 0;
//	                             while (__nil_i < len(__nil_coll)) {
//	                               let k = __nil_i; let v = __nil_coll[__nil_i];
//	                               { body }
//	                               __nil_i = __nil_i + 1;
//	                             }
//
// A range becomes a cursor walking between two bounds rather than an array, so
// `for i in 0..1000000000` costs nothing.
//
// `break` and `continue` are *not* lowered away. They survive as AST nodes and
// each backend compiles them into its own control flow, because the only way to
// lower them would be a flag plus conditional jumps, which is strictly more code
// for identical behaviour.
//
// # What is not lowered
//
// The original AST is preserved for tooling. A formatter, an LSP, or a
// documentation generator must show the developer what they wrote, so
// frontend.Result carries both the original Program and the Lowered one.
package lower

import (
	"fmt"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/token"
)

// HiddenLocalPrefix marks the names the lowering introduces.
//
// The prefix is deliberately unlikely to collide with a developer's identifier.
// A collision is still handled: l.unique renames, rather than shadowing, so a
// program that happens to use the same name keeps working.
const HiddenLocalPrefix = "__nil_"

// Program lowers an entire program.
func Program(program *ast.Program) (*ast.Program, error) {
	if program == nil {
		return nil, fmt.Errorf("cannot lower a nil program")
	}

	l := &lowerer{
		reserved: make(map[string]bool),
		variants: collectVariants(program),
	}
	statements, err := l.statements(program.Statements)
	if err != nil {
		return nil, err
	}
	return &ast.Program{Statements: statements}, nil
}

// lowerer carries state across one lowering run.
type lowerer struct {
	// reserved records the hidden names already introduced, so a nested loop
	// does not reuse one.
	reserved map[string]bool

	// loopDepth counts enclosing loops, which is how `break` outside a loop is
	// detected at compile time rather than producing a silent no-op.
	loopDepth int

	// variants maps a variant name to how many arguments it takes, gathered
	// from every enum declaration in the program before lowering starts.
	//
	// Collecting them up front rather than as the declarations are reached is
	// what lets a variant be used above the declaration that introduces it.
	variants map[string]int

	// resolutions maps a method call to the generated function it compiles to.
	//
	// It comes from the checker rather than from this pass because this pass does
	// no type inference, and "which implementation does this call use" is
	// answered entirely by the receiver's static type. See lower/traits.go for
	// why the answer is recorded alongside the tree rather than written into it.
	resolutions map[ast.Expression]string

	// impls indexes the program's trait implementations by type and method.
	impls map[string]map[string]*ast.TraitMethod
}

// isVariant reports whether name is an enum variant of the given arity.
//
// Arity is part of the answer rather than an afterthought: `Circle(7)` is a
// variant, and a function that happens to take one argument is a call. A
// variant takes at most one payload, so a two-argument call is never one.
func (l *lowerer) isVariant(name string, arity int) bool {
	want, ok := l.variants[name]
	return ok && want == arity
}

// firstArgument returns the single argument of a one-argument call, or nil.
func (l *lowerer) firstArgument(args []ast.Expression) ast.Expression {
	if len(args) != 1 {
		return nil
	}
	return args[0]
}

// collectVariants records every enum variant a program declares.
func collectVariants(program *ast.Program) map[string]int {
	variants := make(map[string]int)
	if program == nil {
		return variants
	}

	ast.Inspect(program, func(node ast.Node) bool {
		decl, ok := node.(*ast.EnumDecl)
		if !ok {
			return true
		}
		for _, variant := range decl.Variants {
			if variant.HasPayload() {
				variants[variant.Name.Value] = 1
			} else {
				variants[variant.Name.Value] = 0
			}
		}
		return true
	})

	return variants
}

// statements lowers a statement list.
func (l *lowerer) statements(list []ast.Statement) ([]ast.Statement, error) {
	out := make([]ast.Statement, 0, len(list))
	for _, stmt := range list {
		lowered, err := l.statement(stmt)
		if err != nil {
			return nil, err
		}
		if lowered != nil {
			out = append(out, lowered)
		}
	}
	return out, nil
}

// statement lowers one statement.
func (l *lowerer) statement(stmt ast.Statement) (ast.Statement, error) {
	// A visibility marker does not change what a declaration means, only what
	// leaves the module. The marker is still carried through so that a tool
	// reading the lowered program can see which names a file exports.
	if wrapped, ok := stmt.(*ast.ExportStatement); ok && wrapped.Decl != nil {
		lowered, err := l.statement(wrapped.Decl)
		if err != nil {
			return nil, err
		}
		return &ast.ExportStatement{
			Token:      wrapped.Token,
			Visibility: wrapped.Visibility,
			Decl:       lowered,
		}, nil
	}

	switch node := stmt.(type) {
	case *ast.BlockStatement:
		return l.block(node)

	case *ast.ForStatement:
		return l.forStatement(node)

	case *ast.WhileStatement:
		l.loopDepth++
		body, err := l.block(node.Body)
		l.loopDepth--
		if err != nil {
			return nil, err
		}
		return &ast.WhileStatement{Token: node.Token, Condition: node.Condition, Body: body}, nil

	case *ast.WhileLetStatement:
		l.loopDepth++
		lowered, err := l.whileLet(node)
		l.loopDepth--
		if err != nil {
			return nil, err
		}
		return lowered, nil

	case *ast.BreakStatement:
		if l.loopDepth == 0 {
			return nil, &Error{Line: node.Token.Line, Msg: "`break` used outside of a loop"}
		}
		return node, nil

	case *ast.ContinueStatement:
		if l.loopDepth == 0 {
			return nil, &Error{Line: node.Token.Line, Msg: "`continue` used outside of a loop"}
		}
		return node, nil

	case *ast.StructDecl:
		// A struct declaration has no run-time effect: the name is a type, and
		// the fields are checked statically. Backends explicitly accept and
		// ignore it.
		return node, nil

	case *ast.EnumDecl:
		return node, nil

	case *ast.TypeAliasDecl:
		return node, nil

	case *ast.LetStatement:
		// `const` is the same node with Constant set, so both flow through here.
		value, err := l.expression(node.Value)
		if err != nil {
			return nil, err
		}
		return &ast.LetStatement{
			Token:    node.Token,
			Name:     node.Name,
			Type:     node.Type,
			Value:    value,
			Constant: node.Constant,
		}, nil

	case *ast.AssignStatement:
		value, err := l.expression(node.Value)
		if err != nil {
			return nil, err
		}
		return &ast.AssignStatement{
			Token:    node.Token,
			Name:     node.Name,
			Operator: node.Operator,
			Value:    value,
		}, nil

	case *ast.IndexAssignStatement:
		index, err := l.expression(node.Index)
		if err != nil {
			return nil, err
		}
		value, err := l.expression(node.Value)
		if err != nil {
			return nil, err
		}
		return &ast.IndexAssignStatement{
			Token: node.Token,
			Left:  node.Left,
			Index: index,
			Value: value,
		}, nil

	case *ast.ExpressionStatement:
		value, err := l.expression(node.Expression)
		if err != nil {
			return nil, err
		}
		return &ast.ExpressionStatement{Token: node.Token, Expression: value}, nil

	case *ast.ReturnStatement:
		value, err := l.expression(node.ReturnValue)
		if err != nil {
			return nil, err
		}
		return &ast.ReturnStatement{Token: node.Token, ReturnValue: value}, nil

	case *ast.StateDeclaration:
		value, err := l.expression(node.Value)
		if err != nil {
			return nil, err
		}
		return &ast.StateDeclaration{
			Token: node.Token,
			Name:  node.Name,
			Type:  node.Type,
			Value: value,
		}, nil

	case *ast.AppStatement:
		body, err := l.block(node.Body)
		if err != nil {
			return nil, err
		}
		return &ast.AppStatement{Token: node.Token, Name: node.Name, Body: body}, nil

	default:
		// Style, entity, and import statements cannot contain a loop.
		return stmt, nil
	}
}

// block lowers a nested block.
func (l *lowerer) block(node *ast.BlockStatement) (*ast.BlockStatement, error) {
	if node == nil {
		return &ast.BlockStatement{Statements: []ast.Statement{}}, nil
	}
	statements, err := l.statements(node.Statements)
	if err != nil {
		return nil, err
	}
	return &ast.BlockStatement{Token: node.Token, Statements: statements}, nil
}

// expression lowers an expression tree and rejects a range that is not part of
// a `for` header.
//
// A range has no runtime representation of its own: the lowering reads its
// bounds and builds a cursor. Anywhere else there is nothing to build, so
// accepting it would produce a program whose meaning silently depends on where
// it appeared.
func (l *lowerer) expression(expr ast.Expression) (ast.Expression, error) {
	if rangeExpr, ok := expr.(*ast.RangeExpression); ok {
		return nil, &Error{Line: rangeExpr.Token.Line, Msg: "a range expression is only valid in a `for` loop header"}
	}

	switch node := expr.(type) {
	case nil:
		return nil, nil

	case *ast.LetExpression:
		value, err := l.expression(node.Value)
		if err != nil {
			return nil, err
		}
		body, err := l.expression(node.Body)
		if err != nil {
			return nil, err
		}
		return &ast.LetExpression{
			Token: node.Token,
			Name:  node.Name,
			Type:  node.Type,
			Value: value,
			Body:  body,
		}, nil

	case *ast.FunctionLiteral:
		// A function body is a fresh loop scope: a `break` inside it is an error
		// even when the function is declared inside a loop, because a function
		// boundary is a jump the loop cannot see across.
		savedDepth := l.loopDepth
		l.loopDepth = 0

		body, err := l.block(node.Body)
		l.loopDepth = savedDepth
		if err != nil {
			return nil, err
		}

		lowered := &ast.FunctionLiteral{
			Token:      node.Token,
			Name:       node.Name,
			Parameters: node.Parameters,
			Body:       body,
			// Modifiers and signature survive lowering: the backends read
			// IsPure for the runtime E0202 check, and dropping Requires or
			// the return annotation would silently change what the function
			// promises. Lowering rewrites control flow, not declarations.
			IsPure:     node.IsPure,
			IsUnsafe:   node.IsUnsafe,
			Requires:   node.Requires,
			ReturnType: node.ReturnType,
			TypeParams: node.TypeParams,
		}
		return lowered, nil

	case *ast.CallExpression:
		// `a?.b(args)` is lowered as one construct, not a callee followed by a
		// call. Lowering the callee would produce `null` for the empty case and
		// then call it, which is the opposite of what the operator is for.
		if access, ok := node.Function.(*ast.SafeAccessExpression); ok {
			return l.safeAccessCall(node, access)
		}

		// A `?.` further down the callee — `a?.b().c()` — guards the call too.
		if guarded, ok, err := l.guardChain(node.Token, node, findSafeAccess(node.Function)); ok {
			return guarded, err
		}

		// A resolved trait method becomes a plain call to its generated function,
		// with the receiver moved to the front. Checked before the callee is
		// lowered, because the whole point is to replace the callee.
		if rewritten, ok := l.rewriteMethodCall(node); ok {
			return rewritten, nil
		}

		fn, err := l.expression(node.Function)
		if err != nil {
			return nil, err
		}
		args := make([]ast.Expression, 0, len(node.Arguments))
		for _, arg := range node.Arguments {
			lowered, err := l.expression(arg)
			if err != nil {
				return nil, err
			}
			args = append(args, lowered)
		}

		// `Circle(7)` is written like a call, because there is nothing else it
		// could be. Turning it into a value is the lowering's job, and knowing
		// that `Circle` names a variant rather than a function needs the enum
		// declarations, which this pass collected before it started.
		if ident, ok := fn.(*ast.Identifier); ok && l.isVariant(ident.Value, len(args)) {
			return l.enumVariant(&ast.EnumVariantExpr{
				Token:   node.Token,
				Name:    ident,
				Payload: l.firstArgument(args),
			}), nil
		}

		return &ast.CallExpression{Token: node.Token, Function: fn, Arguments: args}, nil

	case *ast.PrefixExpression:
		right, err := l.expression(node.Right)
		if err != nil {
			return nil, err
		}
		return &ast.PrefixExpression{Token: node.Token, Operator: node.Operator, Right: right}, nil

	case *ast.MatchExpression:
		return l.matchExpression(node)

	case *ast.CoalesceExpression:
		return l.coalesce(node)

	case *ast.SafeAccessExpression:
		return l.safeAccess(node)

	case *ast.IfExpression:
		// A ternary, or an `if` this pass itself produced. Either way its parts
		// may still contain constructs that need lowering, and they reach the
		// backends untouched without this: a ternary whose condition was `a ?? b`
		// reached the compiler holding a `CoalesceExpression`, which no backend
		// knows how to execute.
		condition, err := l.expression(node.Condition)
		if err != nil {
			return nil, err
		}
		consequence, err := l.block(node.Consequence)
		if err != nil {
			return nil, err
		}
		alternative, err := l.block(node.Alternative)
		if err != nil {
			return nil, err
		}
		return &ast.IfExpression{
			Token:       node.Token,
			Condition:   condition,
			Consequence: consequence,
			Alternative: alternative,
		}, nil

	case *ast.DotExpression:
		// A `?.` anywhere in the receiver spine guards the whole access, not just
		// the step it appears on.
		if guarded, ok, err := l.guardChain(node.Token, node, findSafeAccess(node.Left)); ok {
			return guarded, err
		}

		// The receiver is an ordinary expression and may hold anything, so it is
		// lowered like one.
		left, err := l.expression(node.Left)
		if err != nil {
			return nil, err
		}
		return &ast.DotExpression{Token: node.Token, Left: left, Member: node.Member}, nil

	case *ast.EnumVariantExpr:
		return l.enumVariant(node), nil

	case *ast.StructLiteral:
		return l.structLiteral(node)

	case *ast.Identifier:
		// A payload-free variant is used as a bare name: `let s = Square;`.
		// Nothing else in the grammar could mean that, and a call form would
		// require writing `Square()` for a value that carries nothing.
		if l.isVariant(node.Value, 0) {
			return l.enumVariant(&ast.EnumVariantExpr{Token: node.Token, Name: node}), nil
		}
		return node, nil

	case *ast.StringLiteral, *ast.IntegerLiteral, *ast.FloatLiteral, *ast.Boolean:
		return node, nil

	case *ast.InfixExpression:
		left, err := l.expression(node.Left)
		if err != nil {
			return nil, err
		}
		right, err := l.expression(node.Right)
		if err != nil {
			return nil, err
		}
		return &ast.InfixExpression{
			Token:    node.Token,
			Left:     left,
			Operator: node.Operator,
			Right:    right,
		}, nil

	case *ast.ArrayLiteral:
		elements := make([]ast.Expression, 0, len(node.Elements))
		for _, elem := range node.Elements {
			lowered, err := l.expression(elem)
			if err != nil {
				return nil, err
			}
			elements = append(elements, lowered)
		}
		return &ast.ArrayLiteral{Token: node.Token, Elements: elements}, nil

	case *ast.IndexExpression:
		index, err := l.expression(node.Index)
		if err != nil {
			return nil, err
		}
		return &ast.IndexExpression{Token: node.Token, Left: node.Left, Index: index}, nil

	case *ast.HashLiteral:
		pairs := make(map[ast.Expression]ast.Expression, len(node.Pairs))
		ordered := make([]ast.HashPairExpr, 0, len(node.OrderedPairs))
		for _, entry := range node.OrderedPairs {
			key, err := l.expression(entry.Key)
			if err != nil {
				return nil, err
			}
			value, err := l.expression(entry.Value)
			if err != nil {
				return nil, err
			}
			ordered = append(ordered, ast.HashPairExpr{Key: key, Value: value})
			pairs[key] = value
		}
		return &ast.HashLiteral{Token: node.Token, Pairs: pairs, OrderedPairs: ordered}, nil

	case *ast.StringTemplate:
		// The interpolated expressions are real expressions and must be lowered
		// like any other. They used to be returned unchanged, which meant a
		// `for` loop, an enum variant, or a `match` written inside an
		// interpolation reached a backend with no lowering at all, and failed
		// there as a call to a name that is not a function.
		parts := make([]ast.TemplatePart, 0, len(node.Parts))
		for _, part := range node.Parts {
			if !part.IsExpression {
				parts = append(parts, part)
				continue
			}
			lowered, err := l.expression(part.Expression)
			if err != nil {
				return nil, err
			}
			parts = append(parts, ast.TemplatePart{IsExpression: true, Expression: lowered})
		}
		return &ast.StringTemplate{Token: node.Token, Parts: parts}, nil

	default:
		return expr, nil
	}
}

// forStatement lowers a `for` loop into a block containing a `while` loop.
func (l *lowerer) forStatement(node *ast.ForStatement) (ast.Statement, error) {
	if node.Body == nil {
		return nil, &Error{Line: node.Token.Line, Msg: "`for` loop has no body"}
	}
	if node.Value == nil {
		return nil, &Error{Line: node.Token.Line, Msg: "`for` loop must bind a variable"}
	}

	if rangeExpr, ok := node.Iterable.(*ast.RangeExpression); ok {
		return l.rangeForStatement(node, rangeExpr)
	}

	tok := node.Token
	collection := l.identifier(tok, l.unique("coll"))
	cursor := l.identifier(tok, l.unique("i"))

	setup := []ast.Statement{
		&ast.LetStatement{Token: tok, Name: collection, Value: node.Iterable},
		&ast.LetStatement{
			Token: tok,
			Name:  cursor,
			Value: &ast.IntegerLiteral{Token: tok, Value: 0},
		},
	}

	// while (__nil_i < len(__nil_coll))
	condition := l.lessThan(tok, cursor, l.len(tok, collection))

	// The parser sets Key and Value to the same identifier for the
	// single-variable form, so identity of the two names is what distinguishes
	// `for x in xs` from `for i, x in xs`. In the first case the variable is the
	// element; in the second the first is the index and the second the element.
	singleVariable := node.Key == nil || node.Value == nil ||
		node.Key.Value == node.Value.Value

	var bindings []ast.Statement
	if !singleVariable && node.Key != nil {
		bindings = append(bindings, &ast.LetStatement{Token: tok, Name: node.Key, Value: cursor})
	}
	if node.Value != nil {
		bindings = append(bindings, &ast.LetStatement{
			Token: tok,
			Name:  node.Value,
			Value: l.index(tok, collection, cursor),
		})
	}

	increment := l.assignAdd(tok, cursor, one(tok))

	return l.assembleFor(tok, setup, condition, bindings, increment, node)
}

// rangeForStatement lowers `for v in start..end` and its inclusive form.
//
// The direction is inferred from the bounds, so `for i in 10..0` counts down
// rather than failing: a descending countdown is what a developer means, and
// making them write a `while` with a negated step would be worse.
func (l *lowerer) rangeForStatement(node *ast.ForStatement, re *ast.RangeExpression) (ast.Statement, error) {
	tok := node.Token

	start := l.identifier(tok, l.unique("start"))
	end := l.identifier(tok, l.unique("end"))
	step := l.identifier(tok, l.unique("step"))
	cursor := l.identifier(tok, l.unique("i"))

	setup := []ast.Statement{
		&ast.LetStatement{Token: tok, Name: start, Value: re.Start},
		&ast.LetStatement{Token: tok, Name: end, Value: re.End},
		// The step follows the sign of the span.
		&ast.LetStatement{
			Token: tok,
			Name:  step,
			Value: l.ifElse(tok,
				l.greaterOrEqual(tok, end, start),
				one(tok),
				&ast.IntegerLiteral{Token: tok, Value: -1}),
		},
		&ast.LetStatement{Token: tok, Name: cursor, Value: start},
	}

	var ascending, descending ast.Expression
	if re.Inclusive {
		ascending = l.lessOrEqual(tok, cursor, end)
		descending = l.greaterOrEqual(tok, cursor, end)
	} else {
		ascending = l.lessThan(tok, cursor, end)
		descending = l.greaterThan(tok, cursor, end)
	}

	// while (__nil_step == 1 ? ascending : descending)
	condition := l.ifElse(tok, l.equal(tok, step, one(tok)), ascending, descending)

	// A range has no index, so only the value is bound.
	var bindings []ast.Statement
	if node.Value != nil {
		bindings = append(bindings, &ast.LetStatement{Token: tok, Name: node.Value, Value: cursor})
	}

	increment := l.assignAdd(tok, cursor, step)

	return l.assembleFor(tok, setup, condition, bindings, increment, node)
}

// assembleFor builds the lowered block, once both iterable shapes agree on the
// parts they share.
func (l *lowerer) assembleFor(
	tok token.Token,
	setup []ast.Statement,
	condition ast.Expression,
	bindings []ast.Statement,
	increment ast.Statement,
	node *ast.ForStatement,
) (ast.Statement, error) {
	l.loopDepth++
	body, err := l.block(node.Body)
	l.loopDepth--
	if err != nil {
		return nil, err
	}

	// The bindings are re-evaluated on every pass, so they belong inside the
	// loop rather than in the setup.
	//
	// The increment goes *before* the body, and that placement is load bearing.
	// A `continue` inside the body unwinds by stopping the enclosing block, so
	// anything after it in that block would never run. Advancing the cursor first
	// means the only thing a `continue` skips is the rest of the body, which is
	// exactly its meaning. Putting the increment last, where it reads more
	// naturally, would make `continue` an infinite loop.
	loopBody := make([]ast.Statement, 0, len(bindings)+2)
	loopBody = append(loopBody, bindings...)
	loopBody = append(loopBody, increment)
	loopBody = append(loopBody, body)

	loop := &ast.WhileStatement{
		Token:     tok,
		Condition: condition,
		Body:      &ast.BlockStatement{Token: tok, Statements: loopBody},
	}

	return &ast.BlockStatement{
		Token:      tok,
		Statements: append(setup, loop),
	}, nil
}

// structLiteral lowers a struct literal to a hash keyed by field name.
//
// Field access already compiles to an index, so representing a struct as a hash
// means `u.name` costs nothing extra and needs no new opcode. The trade-off is
// that `u["name"]` works too and a plain hash with the same keys satisfies the
// same struct patterns; struct identity is enforced by the type checker, which is
// where it can be enforced without ambiguity. See the package comment in adt.go.
func (l *lowerer) structLiteral(node *ast.StructLiteral) (ast.Expression, error) {
	tok := node.Token

	pairs := make([]ast.HashPairExpr, 0, len(node.Fields))
	byKey := make(map[ast.Expression]ast.Expression, len(node.Fields))

	for _, field := range node.Fields {
		if field.Value == nil {
			return nil, &Error{Line: tok.Line, Msg: fmt.Sprintf("field %q has no value", field.Name.Value)}
		}
		value, err := l.expression(field.Value)
		if err != nil {
			return nil, err
		}
		key := &ast.StringLiteral{Token: tok, Value: field.Name.Value}
		pairs = append(pairs, ast.HashPairExpr{Key: key, Value: value})
		byKey[key] = value
	}

	return &ast.HashLiteral{Token: tok, Pairs: byKey, OrderedPairs: pairs}, nil
}

// unique reserves a hidden name, avoiding collisions with earlier loops.
func (l *lowerer) unique(suffix string) string {
	name := HiddenLocalPrefix + suffix
	if !l.reserved[name] {
		l.reserved[name] = true
		return name
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s%s_%d", HiddenLocalPrefix, suffix, i)
		if !l.reserved[candidate] {
			l.reserved[candidate] = true
			return candidate
		}
	}
}

func (l *lowerer) identifier(tok token.Token, name string) *ast.Identifier {
	return &ast.Identifier{Token: tok, Value: name}
}

func one(tok token.Token) ast.Expression {
	return &ast.IntegerLiteral{Token: tok, Value: 1}
}

func (l *lowerer) len(tok token.Token, target ast.Expression) ast.Expression {
	return &ast.CallExpression{
		Token:     tok,
		Function:  &ast.Identifier{Token: tok, Value: "len"},
		Arguments: []ast.Expression{target},
	}
}

func (l *lowerer) index(tok token.Token, target, index ast.Expression) ast.Expression {
	return &ast.IndexExpression{Token: tok, Left: target, Index: index}
}

func (l *lowerer) lessThan(tok token.Token, left, right ast.Expression) ast.Expression {
	return &ast.InfixExpression{Token: tok, Left: left, Operator: "<", Right: right}
}

func (l *lowerer) greaterThan(tok token.Token, left, right ast.Expression) ast.Expression {
	return &ast.InfixExpression{Token: tok, Left: left, Operator: ">", Right: right}
}

func (l *lowerer) lessOrEqual(tok token.Token, left, right ast.Expression) ast.Expression {
	return &ast.InfixExpression{Token: tok, Left: left, Operator: "<=", Right: right}
}

func (l *lowerer) greaterOrEqual(tok token.Token, left, right ast.Expression) ast.Expression {
	return &ast.InfixExpression{Token: tok, Left: left, Operator: ">=", Right: right}
}

func (l *lowerer) equal(tok token.Token, left, right ast.Expression) ast.Expression {
	return &ast.InfixExpression{Token: tok, Left: left, Operator: "==", Right: right}
}

// ifElse builds `cond ? a : b`.
func (l *lowerer) ifElse(tok token.Token, cond, whenTrue, whenFalse ast.Expression) ast.Expression {
	return &ast.IfExpression{
		Token:     tok,
		Condition: cond,
		Consequence: &ast.BlockStatement{
			Token:      tok,
			Statements: []ast.Statement{&ast.ExpressionStatement{Token: tok, Expression: whenTrue}},
		},
		Alternative: &ast.BlockStatement{
			Token:      tok,
			Statements: []ast.Statement{&ast.ExpressionStatement{Token: tok, Expression: whenFalse}},
		},
	}
}

// assignAdd builds `name = name + amount`.
func (l *lowerer) assignAdd(tok token.Token, name *ast.Identifier, amount ast.Expression) ast.Statement {
	return &ast.AssignStatement{
		Token:    tok,
		Name:     name,
		Operator: "+=",
		Value:    amount,
	}
}
