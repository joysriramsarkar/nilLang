package typecheck

import (
	"fmt"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/types"
)

// This file adds the declarative type constructs — `struct`, `enum`, `type`
// alias, and `match` — to the checker.
//
// # Where each guarantee lives
//
// Struct identity, field names, field types, and enum variants are all checked
// here rather than at run time. That is the point of the design: a `User` with a
// misspelled field is a compile-time error, and the run-time representation (a
// hash) never has to defend against it.
//
// Exhaustiveness is checked here too, but only when the subject's type is known.
// A `match` over `Any` has nothing to be exhaustive against, so its non-exhaustive
// case is left to the `unreachable` call the lowering emits. That is why the
// diagnostic is E0507 and not a run-time failure: by the time code runs, the
// possibilities have already been ruled out.
//
// # Aliases
//
// An alias introduces a name for an existing type; it does not introduce a type.
// Resolution therefore happens in `resolveType`, which every annotation goes
// through, and an alias chain is followed to its end. A cycle is reported rather
// than followed, because a cyclic alias has no meaning and looping would hang.

// declaredType is a struct or enum a program declared.
type declaredType struct {
	name    string
	structT *types.StructType
	enumT   *types.EnumType
	line    int
	column  int
}

// variant is one enum variant, kept alongside its enum so a call can be checked
// without a second lookup.
type variant struct {
	enum       string
	payload    []types.Type
	hasPayload bool
	line       int
	column     int
}

// registerDeclarations walks a program once before checking it, so that a type
// may be used before the line that declares it.
//
// A single forward pass in source order would make `let u = User { ... }` above
// `struct User` an error, which is not a rule any developer expects from a
// declaration.
func (c *Checker) registerDeclarations(prog *ast.Program) {
	if c.types == nil {
		c.types = make(map[string]*declaredType)
	}
	if c.variants == nil {
		c.variants = make(map[string]*variant)
	}
	if c.aliases == nil {
		c.aliases = make(map[string]string)
	}

	ast.Inspect(prog, func(node ast.Node) bool {
		switch decl := node.(type) {
		case *ast.StructDecl:
			if decl.Name == nil {
				return true
			}
			name := decl.Name.Value
			if existing, ok := c.types[name]; ok {
				c.report(CodeDuplicateType,
					fmt.Sprintf("type %q is already declared at line %d", name, existing.line),
					decl.Token.Line, decl.Token.Column)
				return true
			}

			structT := &types.StructType{Name: name}
			for _, field := range decl.Fields {
				fieldType, err := c.resolveType(field.TypeAnnotation)
				if err != nil {
					c.report(CodeUnknownType,
						fmt.Sprintf("unknown type %q for field %q of struct %q",
							field.TypeAnnotation, field.Name.Value, name),
						decl.Token.Line, decl.Token.Column)
					continue
				}
				structT.Fields = append(structT.Fields, types.StructField{
					Name: field.Name.Value,
					Type: fieldType,
				})
			}

			c.types[name] = &declaredType{
				name:    name,
				structT: structT,
				line:    decl.Token.Line,
				column:  decl.Token.Column,
			}
			// The name is also a value-level constructor for a struct literal,
			// which is how `User { ... }` resolves.
			c.currentScope.SetVar(name, structT)

		case *ast.EnumDecl:
			if decl.Name == nil {
				return true
			}
			name := decl.Name.Value
			if existing, ok := c.types[name]; ok {
				c.report(CodeDuplicateType,
					fmt.Sprintf("type %q is already declared at line %d", name, existing.line),
					decl.Token.Line, decl.Token.Column)
				return true
			}

			enumT := &types.EnumType{Name: name}
			for _, v := range decl.Variants {
				payload := []types.Type{}
				if v.HasPayload() {
					payloadType, err := c.resolveType(v.PayloadType)
					if err != nil {
						c.report(CodeUnknownType,
							fmt.Sprintf("unknown type %q for the payload of %s.%s",
								v.PayloadType, name, v.Name.Value),
							decl.Token.Line, decl.Token.Column)
						continue
					}
					payload = []types.Type{payloadType}
				}

				enumT.Variants = append(enumT.Variants, types.EnumVariant{
					Name:    v.Name.Value,
					Payload: payload,
				})

				if _, clash := c.variants[v.Name.Value]; clash {
					c.report(CodeDuplicateType,
						fmt.Sprintf("enum variant %q is declared more than once; variant names share one namespace",
							v.Name.Value),
						v.Token.Line, v.Token.Column)
					continue
				}
				c.variants[v.Name.Value] = &variant{
					enum:       name,
					payload:    payload,
					hasPayload: len(payload) > 0,
					line:       v.Token.Line,
					column:     v.Token.Column,
				}

				// A payload-free variant is also usable as a bare value —
				// `let s = Square;` — so the name must resolve as a variable
				// whose type is the enum. Without this, `inferExpression` reports
				// an undefined identifier for a name the program did declare.
				if len(payload) == 0 {
					c.currentScope.SetVar(v.Name.Value, enumT)
				}
			}

			c.types[name] = &declaredType{
				name:   name,
				enumT:  enumT,
				line:   decl.Token.Line,
				column: decl.Token.Column,
			}

		case *ast.TypeAliasDecl:
			if decl.Name == nil {
				return true
			}
			c.aliases[decl.Name.Value] = decl.Target
		}
		return true
	})

	// Pre-register top-level named function declarations so a function can call
	// another top-level function declared later in the same file.
	for _, stmt := range prog.Statements {
		if exp, ok := stmt.(*ast.ExportStatement); ok && exp.Decl != nil {
			stmt = exp.Decl
		}
		exprStmt, ok := stmt.(*ast.ExpressionStatement)
		if !ok {
			continue
		}
		fn, ok := exprStmt.Expression.(*ast.FunctionLiteral)
		if !ok || fn.Name == "" {
			continue
		}
		if _, exists := c.currentScope.GetFunc(fn.Name); exists {
			continue
		}
		previousTypeVars := c.withTypeParams(fn.TypeParams)
		paramTypes := make([]types.Type, 0, len(fn.Parameters))
		for _, p := range fn.Parameters {
			if p == nil || p.Name == nil {
				continue
			}
			pt := types.Type(types.Any)
			if p.TypeAnnotation != "" {
				if resolved, err := c.resolveType(p.TypeAnnotation); err == nil && c.isKnownTypeName(p.TypeAnnotation) {
					pt = resolved
				}
			}
			paramTypes = append(paramTypes, pt)
		}
		returnType := types.Type(types.Any)
		if fn.ReturnType != "" {
			if resolved, err := c.resolveType(fn.ReturnType); err == nil && c.isKnownTypeName(fn.ReturnType) {
				returnType = resolved
			}
		}
		c.typeVars = previousTypeVars
		fnType := &types.FunctionType{
			Params:     paramTypes,
			ReturnType: returnType,
			Effects:    functionEffects(fn),
			MinArgs:    len(paramTypes),
			MaxArgs:    len(paramTypes),
			Requires:   fn.Requires,
			IsPure:     fn.IsPure,
			IsUnsafe:   fn.IsUnsafe,
		}
		c.currentScope.SetFunc(fn.Name, fnType)
		c.currentScope.SetVar(fn.Name, fnType)
	}
}

// resolveType parses a written type and substitutes any aliases it names.
func (c *Checker) resolveType(annotation string) (types.Type, error) {
	if annotation == "" {
		return types.Any, nil
	}

	// Follow the alias chain, refusing a cycle.
	seen := make(map[string]bool)
	current := annotation
	for i := 0; i < 32; i++ {
		target, ok := c.aliases[current]
		if !ok {
			break
		}
		if seen[current] {
			return nil, fmt.Errorf("type alias %q refers to itself", current)
		}
		seen[current] = true
		current = target
	}

	parsed, err := types.Parse(current)
	if err != nil {
		return nil, err
	}

	// `User` written where a struct is declared parses as a bare name; the
	// checker substitutes the real type so field checks can run.
	if named := c.structByName(parsed.String()); named != nil {
		return named, nil
	}
	if named := c.enumByName(parsed.String()); named != nil {
		return named, nil
	}

	// The same substitution has to happen *inside* an optional, or `?Address`
	// stays a wrapper around a placeholder name with no fields. That made a
	// `?Address` field reject an `Address` value — the one thing the type exists
	// to allow — because the placeholder and the declared struct are not equal.
	//
	// It is also what `?.` needs to exist at all: an optional field is the
	// ordinary reason a receiver can be null.
	if opt, isOptional := parsed.(*types.OptionalType); isOptional {
		base, err := c.resolveType(opt.Base.String())
		if err != nil {
			return nil, err
		}
		return &types.OptionalType{Base: base}, nil
	}

	// A type parameter resolves to a variable, before it reaches the declared-type
	// lookups above. `T` inside `fn id<T>(x: T) -> T` is deliberately not fixed,
	// and resolving it to a placeholder struct would make it equal to no type at
	// all.
	if c.isTypeParam(current) {
		return &types.TypeVar{Name: current}, nil
	}

	// And inside a union. `Int | String` parses as a UnionType whose members are
	// placeholders, and a placeholder compares equal to nothing — so before this
	// the annotation parsed, was never checked against anything, and accepted
	// every value. `let x: Int | String = true;` compiled clean.
	if union, isUnion := parsed.(*types.UnionType); isUnion {
		members := make([]types.Type, 0, len(union.Types))
		for _, member := range union.Types {
			resolved, err := c.resolveType(member.String())
			if err != nil {
				return nil, err
			}
			members = append(members, resolved)
		}
		return &types.UnionType{Types: members}, nil
	}

	// A generic's parameters are resolved the same way, or `Array<T>` keeps a
	// placeholder for its `T` and the variable is invisible to everything
	// downstream — inference, substitution, and the "is this signature generic"
	// question all of which look *inside* the parameter list.
	if generic, isGeneric := parsed.(*types.GenericType); isGeneric {
		params := make([]types.Type, 0, len(generic.Parameters))
		for _, param := range generic.Parameters {
			resolved, err := c.resolveType(param.String())
			if err != nil {
				return nil, err
			}
			params = append(params, resolved)
		}
		return &types.GenericType{Base: generic.Base, Parameters: params}, nil
	}

	return parsed, nil
}

// structByName returns the declared struct with this name, or nil.
func (c *Checker) structByName(name string) *types.StructType {
	decl, ok := c.types[name]
	if !ok || decl.structT == nil {
		return nil
	}
	return decl.structT
}

// enumByName returns the declared enum with this name, or nil.
func (c *Checker) enumByName(name string) *types.EnumType {
	decl, ok := c.types[name]
	if !ok || decl.enumT == nil {
		return nil
	}
	return decl.enumT
}

// isKnownTypeName reports whether a written type names something the program
// can actually mean.
//
// This matters because types.Parse is deliberately permissive: an unrecognised
// name becomes a struct type with no fields rather than an error, so that a type
// from a module the checker has not read still parses. The cost is that a typo
// reads as a perfectly good empty struct, and `let x: NoSuchType = 1` reports a
// type mismatch about `NoSuchType` rather than saying the name is unknown.
//
// It is applied to declarations, where a typo is both most likely and most
// confusing, rather than to every annotation, where rejecting a type the checker
// has not seen would be a false positive.
func (c *Checker) isKnownTypeName(annotation string) bool {
	name := strings.TrimSpace(annotation)
	if name == "" {
		return true
	}

	// A union's members are each checked on their own. `Parse` succeeds for every
	// name, so without this `Int | Nope` resolved to a union containing a
	// placeholder and accepted every value — the annotation was written down and
	// then ignored.
	//
	// Optionals and generics are not second-guessed: `?T` is checked through
	// `resolveType`, which substitutes the declared struct or enum, and a generic
	// base is a module's business.
	if strings.HasPrefix(name, "|") || strings.Contains(name, "|") {
		for _, member := range strings.Split(name, "|") {
			if !c.isKnownTypeName(strings.TrimSpace(member)) {
				return false
			}
		}
		return true
	}

	if strings.ContainsAny(name, "<(?") {
		return true
	}

	// An alias may name another alias; the chain is followed at resolution time,
	// and its end is what has to name something real.
	if _, isAlias := c.aliases[name]; isAlias {
		return true
	}

	// Parse succeeds for every name, so "parses" says nothing. What matters is
	// whether the name reaches a type that is not a bare placeholder.
	parsed, err := types.Parse(name)
	if err != nil {
		return false
	}

	// A type parameter is deliberately not a declared type, so it is checked
	// before the placeholder test rather than after it. Reporting E0502 for `T`
	// inside `fn id<T>(x: T)` because `T` is not a struct would reject every
	// generic function.
	if _, placeholder := parsed.(*types.StructType); placeholder && c.isTypeParam(name) {
		return true
	}

	if _, placeholder := parsed.(*types.StructType); placeholder {
		return c.structByName(name) != nil || c.enumByName(name) != nil
	}
	return true
}

// checkStructDecl, checkEnumDecl, and checkTypeAliasDecl consume the
// declarations that registerDeclarations already walked.
//
// They exist to recurse into a declaration's own annotations, which nothing else
// visits, and to give the checker a place to hang a future rule about where a
// declaration may appear. Reporting nothing here is not the same as reporting
// nothing in registerDeclarations: this is the pass that would say "a struct may
// not be declared inside a function".
func (c *Checker) checkStructDecl(decl *ast.StructDecl) {
	if decl.Name == nil {
		return
	}
	for _, field := range decl.Fields {
		if !c.isKnownTypeName(field.TypeAnnotation) {
			c.report(CodeUnknownType,
				fmt.Sprintf("unknown type %q for field %q of struct %q",
					field.TypeAnnotation, field.Name.Value, decl.Name.Value),
				field.Token.Line, field.Token.Column)
		}
	}
}

func (c *Checker) checkEnumDecl(decl *ast.EnumDecl) {
	if decl.Name == nil {
		return
	}
	for _, v := range decl.Variants {
		if !v.HasPayload() {
			continue
		}
		if !c.isKnownTypeName(v.PayloadType) {
			c.report(CodeUnknownType,
				fmt.Sprintf("unknown type %q for the payload of %s.%s",
					v.PayloadType, decl.Name.Value, v.Name.Value),
				v.Token.Line, v.Token.Column)
		}
	}
}

func (c *Checker) checkTypeAliasDecl(decl *ast.TypeAliasDecl) {
	if decl.Name == nil {
		return
	}
	if !c.isKnownTypeName(decl.Target) {
		c.report(CodeUnknownType,
			fmt.Sprintf("unknown type %q for alias %q", decl.Target, decl.Name.Value),
			decl.Token.Line, decl.Token.Column)
	}
}

// inferStructLiteral checks the fields of a struct literal against its
// declaration.
func (c *Checker) inferStructLiteral(literal *ast.StructLiteral) types.Type {
	if literal.Name == nil {
		return types.Any
	}

	decl := c.structByName(literal.Name.Value)
	if decl == nil {
		// A name that is not a struct could still be a function being called,
		// which is not this node's problem. Staying silent lets the surrounding
		// call check report something more useful.
		if _, isEnum := c.types[literal.Name.Value]; !isEnum {
			c.report(CodeUnknownType,
				fmt.Sprintf("unknown struct type %q", literal.Name.Value),
				literal.Token.Line, literal.Token.Column)
		}
		return types.Any
	}

	fields := make(map[string]types.StructField, len(decl.Fields))
	for _, field := range decl.Fields {
		fields[field.Name] = field
	}

	seen := make(map[string]bool, len(literal.Fields))
	for _, entry := range literal.Fields {
		name := entry.Name.Value
		field, known := fields[name]
		if !known {
			c.report(CodeUnknownField,
				fmt.Sprintf("struct %q has no field %q", decl.Name, name),
				entry.Token.Line, entry.Token.Column)
			continue
		}
		if seen[name] {
			c.report(CodeDuplicateField,
				fmt.Sprintf("field %q is given twice in a %s literal", name, decl.Name),
				entry.Token.Line, entry.Token.Column)
			continue
		}
		seen[name] = true

		valueType := c.inferExpression(entry.Value)
		if valueType != nil && field.Type != nil && !types.Any.Equals(field.Type) &&
			!valueType.AssignableTo(field.Type) {
			c.report("E0101",
				fmt.Sprintf("field %q of %s expects %s, got %s", name, decl.Name, field.Type, valueType),
				entry.Token.Line, entry.Token.Column)
		}
	}

	// Every field must be given. A struct value with a field missing has no
	// correct reading: defaulting it would invent data the developer did not
	// write, and leaving it unset would fail later, further from the mistake.
	missing := make([]string, 0, len(decl.Fields))
	for _, field := range decl.Fields {
		if !seen[field.Name] {
			missing = append(missing, field.Name)
		}
	}
	if len(missing) > 0 {
		c.report(CodeMissingField,
			fmt.Sprintf("%s literal is missing %s: %s",
				decl.Name, plural(len(missing), "field"), strings.Join(missing, ", ")),
			literal.Token.Line, literal.Token.Column)
	}

	return decl
}

// inferEnumVariantCall checks a call that names an enum variant.
//
// The parser cannot tell `Circle(7)` from a call to a function, because both are
// an identifier followed by a parenthesised list. The distinction needs the enum
// declarations, which is what this looks them up in. Reporting nil means "not a
// variant", and the ordinary call check takes over.
func (c *Checker) inferEnumVariantCall(call *ast.CallExpression) types.Type {
	ident, ok := call.Function.(*ast.Identifier)
	if !ok {
		return nil
	}

	v, ok := c.variants[ident.Value]
	if !ok {
		return nil
	}

	want := 0
	if v.hasPayload {
		want = 1
	}
	if len(call.Arguments) != want {
		c.report(CodeWrongArity,
			fmt.Sprintf("variant %s.%s takes %s but got %d argument(s)",
				v.enum, ident.Value, plural(want, "argument"), len(call.Arguments)),
			call.Token.Line, call.Token.Column)
	}

	if enumT := c.enumByName(v.enum); enumT != nil {
		return enumT
	}
	return types.Any
}

// checkMatch checks a match expression's patterns against its subject.
//
// Two things are worth catching here and neither is worth catching at run time:
// an arm that can never be reached because an earlier arm already covered its
// value, and a match that does not cover every case of a known type.
func (c *Checker) checkMatch(match *ast.MatchExpression) {
	subjectType := c.inferExpression(match.Subject)

	// Each arm's bindings are in scope for that arm's guard and body, and
	// nowhere else. A binding is introduced per arm, so an earlier arm's name
	// must not leak into a later one that never binds it.
	for _, arm := range match.Arms {
		c.checkPattern(arm.Pattern, subjectType, match.Token.Line, match.Token.Column)

		c.currentScope = NewScope(c.currentScope)
		for _, name := range patternBindings(arm.Pattern) {
			c.currentScope.SetVar(name, types.Any)
		}

		if arm.Guard != nil {
			c.inferExpression(arm.Guard)
		}
		c.inferExpression(arm.Body)

		c.currentScope = c.currentScope.parent
	}

	c.checkExhaustive(match, subjectType)
}

// patternBindings collects the names a pattern introduces, in the order they are
// introduced, so that `Ok(v) if v > 5` checks `v` as defined.
func patternBindings(pattern ast.MatchPattern) []string {
	switch {
	case pattern.Binding != nil:
		return []string{pattern.Binding.Value}

	case pattern.EnumVariant != nil:
		if v := pattern.EnumVariant.PayloadBinding; v != nil && !pattern.EnumVariant.DiscardPayload {
			return []string{v.Value}
		}
		return nil

	case pattern.StructPattern != nil:
		var names []string
		for _, field := range pattern.StructPattern.Fields {
			names = append(names, patternBindings(field.Pattern)...)
		}
		return names

	case len(pattern.Alternatives) > 0:
		var names []string
		for _, alternative := range pattern.Alternatives {
			names = append(names, patternBindings(alternative)...)
		}
		return names

	default:
		return nil
	}
}

// checkPattern checks one arm's pattern against the subject's type.
func (c *Checker) checkPattern(pattern ast.MatchPattern, subjectType types.Type, line, col int) {
	switch {
	case pattern.Wildcard:
		return

	case pattern.Binding != nil:
		// A bare name is a binding, unless it names a declared variant; then it
		// is that variant. See the lowering for why the exception exists — the
		// two must agree or the same program would check one way and run another.
		if _, isVariant := c.variants[pattern.Binding.Value]; isVariant {
			c.checkEnumPattern(&ast.MatchEnumPattern{Variant: pattern.Binding}, line, col)
		}
		return

	case pattern.Literal != nil:
		c.inferExpression(pattern.Literal)
		return

	case pattern.EnumVariant != nil:
		c.checkEnumPattern(pattern.EnumVariant, line, col)
		return

	case pattern.StructPattern != nil:
		c.checkStructPattern(pattern.StructPattern, line, col)
		return

	case len(pattern.Alternatives) > 0:
		for _, alternative := range pattern.Alternatives {
			c.checkPattern(alternative, subjectType, line, col)
		}
		return
	}
}

// checkEnumPattern checks that a variant pattern names a variant that exists.
func (c *Checker) checkEnumPattern(variantPattern *ast.MatchEnumPattern, line, col int) {
	name := variantPattern.Variant.Value

	v, known := c.variants[name]
	if !known {
		// `Ok`, `Err`, `Some`, and `None` are built-in cases, not declared
		// variants; they are matched by their own names.
		switch name {
		case "Ok", "Err", "Some", "None":
			return
		}
		c.report(CodeUnknownVariant,
			fmt.Sprintf("unknown enum variant %q", name), line, col)
		return
	}

	if !variantPattern.DiscardPayload && variantPattern.PayloadBinding == nil && v.hasPayload {
		c.report(CodeWrongArity,
			fmt.Sprintf("variant %s.%s carries a payload; match it with %s(value) or %s(_)",
				v.enum, name, name, name),
			line, col)
	}
	if variantPattern.PayloadBinding != nil && !v.hasPayload {
		c.report(CodeWrongArity,
			fmt.Sprintf("variant %s.%s carries no payload, so %s cannot bind one",
				v.enum, name, name),
			line, col)
	}
}

// checkStructPattern checks that every named field exists on the struct.
func (c *Checker) checkStructPattern(pattern *ast.MatchStructPattern, line, col int) {
	decl := c.structByName(pattern.Name.Value)
	if decl == nil {
		c.report(CodeUnknownType,
			fmt.Sprintf("unknown struct type %q in a pattern", pattern.Name.Value),
			line, col)
		return
	}

	known := make(map[string]bool, len(decl.Fields))
	for _, field := range decl.Fields {
		known[field.Name] = true
	}

	for _, field := range pattern.Fields {
		if !known[field.Name.Value] {
			c.report(CodeUnknownField,
				fmt.Sprintf("struct %q has no field %q", decl.Name, field.Name.Value),
				field.Name.Token.Line, field.Name.Token.Column)
			continue
		}
		c.checkPattern(field.Pattern, nil, line, col)
	}
}

// checkExhaustive reports a match that does not cover every case of a type whose
// cases are known.
//
// Exhaustiveness is only decidable for a closed set: an enum, a struct whose
// fields are all constrained, a bool, or one of the language's own Result and
// Optional. A match over anything else is left alone, because demanding a
// wildcard arm there would be noise rather than safety.
func (c *Checker) checkExhaustive(match *ast.MatchExpression, subjectType types.Type) {
	covered := c.coveredCases(match)

	switch {
	case c.hasWildcardArm(match):
		return

	case subjectType != nil && types.Bool.Equals(subjectType):
		if !covered["true"] || !covered["false"] {
			c.report(CodeNonExhaustive,
				"match on a boolean must cover both `true` and `false`", match.Token.Line, match.Token.Column)
		}
		return

	case isResultType(subjectType):
		if !covered["Ok"] || !covered["Err"] {
			c.report(CodeNonExhaustive,
				"match on a Result must cover both `Ok` and `Err`", match.Token.Line, match.Token.Column)
		}
		return

	case subjectType != nil && subjectType.Kind() == types.KindOptional:
		if !covered["Some"] || !covered["None"] {
			c.report(CodeNonExhaustive,
				"match on an Optional must cover both `Some` and `None`", match.Token.Line, match.Token.Column)
		}
		return

	case subjectType != nil && subjectType.Kind() == types.KindEnum:
		enumT, _ := subjectType.(*types.EnumType)
		if enumT == nil {
			return
		}
		var missing []string
		for _, v := range enumT.Variants {
			if !covered[v.Name] {
				missing = append(missing, v.Name)
			}
		}
		if len(missing) > 0 {
			c.report(CodeNonExhaustive,
				fmt.Sprintf("match on %s does not cover %s: %s",
					enumT.Name, plural(len(missing), "variant"), strings.Join(missing, ", ")),
				match.Token.Line, match.Token.Column)
		}
	}
}

// isResultType reports whether a type is a Result.
//
// `Ok` and `Err` are declared as returning `Result<T, E>`, which parses to a
// generic named Result rather than to the dedicated ResultType. Both spellings
// have to be recognised or a match on `Ok(x)` would silently stop being checked
// for exhaustiveness.
// inferHashLiteral types a hash from its entries.
//
// It used to report `Hash<Any, Any>` for every literal, which is honest about
// nothing and made a declared `Map<String, Int>` unreadable: a function returning
// one always failed its own return-type check. Inferring from the entries costs a
// walk of something the checker already walks, and it is what lets a hash be
// passed where a declared map is expected.
//
// The value type is the entries' common type, or `Any` when they disagree. An
// empty hash is `Hash<Any, Any>`: there is nothing to infer from, and claiming a
// narrower type would reject `let empty: Map<String, Int> = {}` for no reason.
func (c *Checker) inferHashLiteral(literal *ast.HashLiteral) types.Type {
	pairs := literal.OrderedPairs
	if len(pairs) == 0 {
		return &types.GenericType{Base: "Hash", Parameters: []types.Type{types.Any, types.Any}}
	}

	keyType := types.Type(types.Any)
	valueType := types.Type(types.Any)
	for i, entry := range pairs {
		entryKey := c.inferExpression(entry.Key)
		entryValue := c.inferExpression(entry.Value)

		if i == 0 {
			keyType, valueType = entryKey, entryValue
			continue
		}
		if !keyType.Equals(entryKey) {
			keyType = types.Any
		}
		if !valueType.Equals(entryValue) {
			valueType = types.Any
		}
	}

	return &types.GenericType{Base: "Hash", Parameters: []types.Type{keyType, valueType}}
}

// describeFunction names a function in a diagnostic, falling back to "the
// function" for an anonymous literal.
func describeFunction(name string) string {
	if name == "" {
		return "the function"
	}
	return "function " + name
}

// isResultType reports whether a type is a Result.
//
// `Ok` and `Err` are declared as returning `Result<T, E>`, which parses to a
// generic named Result rather than to the dedicated ResultType. Both spellings
// have to be recognised or a match on `Ok(x)` would silently stop being checked
// for exhaustiveness.
func isResultType(t types.Type) bool {
	if t == nil {
		return false
	}
	if t.Kind() == types.KindResult {
		return true
	}
	generic, ok := t.(*types.GenericType)
	return ok && generic.Base == "Result"
}

// checkFunctionBody checks a function's statements and reports the type of the
// value it produces.
//
// The type comes from the trailing expression rather than from a `return`, which
// is how a function's value is spelled in this language. A body whose last
// statement is a declaration produces nothing, which is `Void` rather than a
// mismatch: `fn log() -> Int { let x = 1; }` is a function that returns nothing,
// and the error says so.
func (c *Checker) checkFunctionBody(fn *ast.FunctionLiteral, declared types.Type) types.Type {
	if fn.Body == nil {
		return types.Void
	}

	for i, stmt := range fn.Body.Statements {
		if i < len(fn.Body.Statements)-1 {
			c.checkStatement(stmt)
			continue
		}

		// The last statement decides what the body produces.
		switch last := stmt.(type) {
		case *ast.ExpressionStatement:
			return c.inferExpression(last.Expression)
		case *ast.ReturnStatement:
			return c.inferExpression(last.ReturnValue)
		default:
			c.checkStatement(stmt)
			return types.Void
		}
	}

	return types.Void
}

// coveredCases collects the names a match's arms handle.
//
// A wildcard or a binding covers everything, which is reported separately; here it
// simply means the rest of the check has nothing left to say.
func (c *Checker) coveredCases(match *ast.MatchExpression) map[string]bool {
	covered := make(map[string]bool)

	var walk func(pattern ast.MatchPattern)
	walk = func(pattern ast.MatchPattern) {
		switch {
		case pattern.Wildcard:
			return
		case pattern.Binding != nil:
			// A bare name that is a declared variant covers that case rather
			// than matching everything, which is what checkPattern reads it as.
			if _, isVariant := c.variants[pattern.Binding.Value]; isVariant {
				covered[pattern.Binding.Value] = true
			}
			return
		case pattern.EnumVariant != nil:
			covered[pattern.EnumVariant.Variant.Value] = true
		case pattern.StructPattern != nil:
			covered[pattern.StructPattern.Name.Value] = true
		case len(pattern.Alternatives) > 0:
			for _, alternative := range pattern.Alternatives {
				walk(alternative)
			}
		case pattern.Literal != nil:
			if b, ok := pattern.Literal.(*ast.Boolean); ok {
				if b.Value {
					covered["true"] = true
				} else {
					covered["false"] = true
				}
			}
		}
	}

	for _, arm := range match.Arms {
		walk(arm.Pattern)
	}
	return covered
}

// hasWildcardArm reports whether some arm matches anything.
//
// A bare name is a catch-all only when it is not a declared variant. `Square =>`
// covers one case; `other =>` covers all of them, and treating the first as the
// second would make exhaustiveness checking silently do nothing for any enum
// whose variant names are also plausible variable names.
func (c *Checker) hasWildcardArm(match *ast.MatchExpression) bool {
	found := false
	var walk func(pattern ast.MatchPattern)
	walk = func(pattern ast.MatchPattern) {
		switch {
		case pattern.Wildcard:
			found = true
		case pattern.Binding != nil:
			if _, isVariant := c.variants[pattern.Binding.Value]; !isVariant {
				found = true
			}
		case len(pattern.Alternatives) > 0:
			for _, alternative := range pattern.Alternatives {
				walk(alternative)
			}
		}
	}
	for _, arm := range match.Arms {
		walk(arm.Pattern)
	}
	return found
}

// plural renders "1 field" and "2 fields" correctly, so a diagnostic does not
// have to choose between being wrong about the noun and being clunky.
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// Diagnostic codes for the declarative type constructs.
//
// They live beside the checks that raise them rather than in a central table,
// because a code a reader cannot find next to the rule it belongs to is a code
// nobody maintains.
const (
	// CodeDuplicateType covers a type or variant declared twice.
	CodeDuplicateType = "E0501"

	// CodeUnknownType covers a type name that nothing declares.
	CodeUnknownType = "E0502"

	// CodeUnknownField covers a field name a struct does not have.
	CodeUnknownField = "E0503"

	// CodeDuplicateField covers a field given or matched twice.
	CodeDuplicateField = "E0504"

	// CodeUnknownVariant covers an enum variant that nothing declares.
	CodeUnknownVariant = "E0505"

	// CodeMissingField covers a struct literal that leaves a field unset.
	CodeMissingField = "E0506"

	// CodeNonExhaustive covers a match that does not cover every case.
	CodeNonExhaustive = "E0507"

	// CodeWrongArity covers a variant used with the wrong number of arguments.
	CodeWrongArity = "E0510"
)
