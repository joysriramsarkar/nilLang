package typecheck

import (
	"fmt"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/types"
)

// Traits: registration, agreement checking, and method-call resolution.
//
// # What is checked
//
//   - A trait declares methods; an impl for a type must provide all of them, with
//     the same arity, and may not add a method the trait does not declare.
//   - A type has at most one impl. Two would make "which implementation runs"
//     unanswerable, and answering it by declaration order would make it depend on
//     where a line was written.
//   - A method call resolves only when the receiver's static type has an impl
//     providing that method. Otherwise it is a diagnostic, not a call that fails
//     later.
//
// # What is recorded
//
// Resolved calls are recorded on the checker rather than written back into the
// tree. The lowering pass needs to know which generated function each call
// compiles to, and the checker is the only place that knows; writing the answer
// into the node would corrupt the source program a formatter reads.
//
// Keying on the node itself is safe because `CheckProgram` does not mutate the
// program. A lowerer and a formatter see disjoint concerns over the same tree.

const (
	// CodeTraitMethodMissing is reported when an impl does not provide a method
	// its trait declares.
	CodeTraitMethodMissing = "E0601"

	// CodeImplMethodNotInTrait is reported when an impl provides a method the
	// trait does not declare. It is the mirror of E0601: an impl that can do
	// more than the trait promises is not an implementation of that trait.
	CodeImplMethodNotInTrait = "E0602"

	// CodeNoImplForMethod is reported when a call names a method that the
	// receiver's type does not provide.
	CodeNoImplForMethod = "E0603"
)

// traitDecl is a registered trait.
type traitDecl struct {
	name    string
	methods map[string]*ast.TraitMethod
	order   []string
	line    int
	column  int
}

// implDecl is a registered implementation, keyed by the type it is for.
type implDecl struct {
	trait    string
	typeName string
	methods  map[string]*ast.TraitMethod
	line     int
	column   int
}

// registerTraits walks the program for `trait` and `impl` declarations before
// any statement is checked, so a call above the impl block still resolves.
func (c *Checker) registerTraits(prog *ast.Program) {
	if c.traits == nil {
		c.traits = make(map[string]*traitDecl)
	}
	if c.impls == nil {
		c.impls = make(map[string]*implDecl)
	}
	if prog == nil {
		return
	}

	for _, stmt := range prog.Statements {
		switch decl := ast.Unwrap(stmt).(type) {
		case *ast.TraitDecl:
			if decl.Name == nil {
				continue
			}
			name := decl.Name.Value
			if existing, ok := c.traits[name]; ok {
				c.report(CodeDuplicateType,
					fmt.Sprintf("trait %q is already declared at line %d", name, existing.line),
					decl.Token.Line, decl.Token.Column)
				continue
			}

			t := &traitDecl{
				name:    name,
				methods: make(map[string]*ast.TraitMethod, len(decl.Methods)),
				line:    decl.Token.Line,
				column:  decl.Token.Column,
			}
			for _, method := range decl.Methods {
				if _, duplicate := t.methods[method.Name]; duplicate {
					c.report(CodeTraitMethodMissing,
						fmt.Sprintf("trait %q declares method %q twice", name, method.Name),
						method.Token.Line, method.Token.Column)
					continue
				}
				t.methods[method.Name] = method
				t.order = append(t.order, method.Name)
			}
			c.traits[name] = t

		case *ast.ImplDecl:
			if decl.Type == nil {
				continue
			}
			typeName := decl.Type.Value
			if existing, ok := c.impls[typeName]; ok {
				c.report(CodeImplMethodNotInTrait,
					fmt.Sprintf("type %q already implements %q at line %d",
						typeName, existing.trait, existing.line),
					decl.Token.Line, decl.Token.Column)
				continue
			}

			impl := &implDecl{
				trait:    decl.Trait.Value,
				typeName: typeName,
				methods:  make(map[string]*ast.TraitMethod, len(decl.Methods)),
				line:     decl.Token.Line,
				column:   decl.Token.Column,
			}
			for _, method := range decl.Methods {
				impl.methods[method.Name] = method
			}
			c.impls[typeName] = impl
		}
	}
}

// checkImplAgrees verifies an impl against the trait it names.
//
// It runs over every registration rather than at the `impl` statement so that a
// trait declared *after* the impl is still checked against it — the same forward
// reference rule the rest of the type system already follows.
func (c *Checker) checkImplAgrees() {
	names := make([]string, 0, len(c.impls))
	for name := range c.impls {
		names = append(names, name)
	}
	sortStrings(names)

	for _, typeName := range names {
		impl := c.impls[typeName]

		trait, ok := c.traits[impl.trait]
		if !ok {
			c.report(CodeNoImplForMethod,
				fmt.Sprintf("type %q implements %q, which is not a declared trait", typeName, impl.trait),
				impl.line, impl.column)
			continue
		}

		// Every declared method must be provided, checked in declaration order so
		// that the first diagnostic is the same one every run reports.
		for _, methodName := range trait.order {
			declared := trait.methods[methodName]
			implMethod, provided := impl.methods[methodName]
			if !provided {
				c.report(CodeTraitMethodMissing,
					fmt.Sprintf("impl for %q does not provide method %q, which trait %q declares",
						typeName, methodName, trait.name),
					impl.line, impl.column)
				continue
			}
			if got, want := len(implMethod.Parameters), len(declared.Parameters); got != want {
				c.report(CodeTraitMethodMissing,
					fmt.Sprintf("method %q of %q takes %d parameters, but trait %q declares %d",
						methodName, typeName, got, trait.name, want),
					implMethod.Token.Line, implMethod.Token.Column)
			}
		}

		// And nothing beyond them, checked in declaration order for the same
		// reason.
		for _, method := range orderedMethods(impl.methods) {
			if _, declared := trait.methods[method.Name]; declared {
				continue
			}
			c.report(CodeImplMethodNotInTrait,
				fmt.Sprintf("impl for %q provides method %q, which trait %q does not declare",
					typeName, method.Name, trait.name),
				method.Token.Line, method.Token.Column)
		}
	}
}

// resolveMethodCall records how a method call compiles, if it is a trait method.
//
// `node` is the whole callee — a DotExpression or a SafeAccessExpression — because
// that is the node the lowering pass looks up. `left` and `member` are its parts,
// passed separately so one implementation serves both forms.
//
// receiverType is the call's receiver type as the checker inferred it. It is
// only consulted for a named type: an Int has no impl, and looking one up would
// find nothing anyway.
func (c *Checker) resolveMethodCall(node ast.Expression, left ast.Expression, member *ast.Identifier, receiverType types.Type) (string, bool) {
	if node == nil || member == nil {
		return "", false
	}
	memberName := member.Value

	typeName, ok := namedTypeName(receiverType)
	if !ok {
		return "", false
	}

	impl, ok := c.impls[typeName]
	if !ok {
		c.report(CodeNoImplForMethod,
			fmt.Sprintf("type %q has no `impl` block, so it cannot be called as .%s()", typeName, memberName),
			member.Token.Line, member.Token.Column)
		return "", false
	}

	method, ok := impl.methods[memberName]
	if !ok {
		c.report(CodeNoImplForMethod,
			fmt.Sprintf("type %q does not provide method %q", typeName, memberName),
			member.Token.Line, member.Token.Column)
		return "", false
	}
	if c.traits[impl.trait] != nil {
		if _, declared := c.traits[impl.trait].methods[memberName]; !declared {
			c.report(CodeImplMethodNotInTrait,
				fmt.Sprintf("method %q is implemented for %q but is not part of trait %q",
					memberName, typeName, impl.trait),
				member.Token.Line, member.Token.Column)
			return "", false
		}
	}

	if c.resolutions == nil {
		c.resolutions = make(map[ast.Expression]string)
	}
	name := ast.ImplMethodName(typeName, memberName)
	c.resolutions[node] = name

	return name, len(method.Parameters) > 0
}

// MethodResolutions returns the recorded method-call resolutions.
//
// It is read by the lowering pass, which is the only consumer: the trees do not
// get rewritten, so a formatter reading the same program sees the source.
func (c *Checker) MethodResolutions() map[ast.Expression]string {
	if c.resolutions == nil {
		return map[ast.Expression]string{}
	}
	return c.resolutions
}

// namedTypeName returns the declared name of a type, or "" if it has none.
//
// A composite or a primitive has no impl block, so there is nothing to look up
// and the caller should not pretend otherwise.
func namedTypeName(t types.Type) (string, bool) {
	if t == nil {
		return "", false
	}
	switch named := t.(type) {
	case *types.StructType:
		return named.Name, named.Name != ""
	case *types.EnumType:
		return named.Name, named.Name != ""
	}
	return "", false
}

// methodResultType is what a resolved method call evaluates to.
//
// An impl's method carries its own return annotation, and that is the answer —
// the trait's annotation is what the method *should* return, not a second
// declaration of what it does. When neither side is annotated the call is
// `Any`, matching every other unannotated function in the language.
func (c *Checker) methodResultType(member *ast.Identifier, receiver types.Type) types.Type {
	typeName, ok := namedTypeName(receiver)
	if !ok {
		return types.Any
	}
	impl, ok := c.impls[typeName]
	if !ok {
		return types.Any
	}
	method, ok := impl.methods[member.Value]
	if !ok || method.ReturnType == "" {
		return types.Any
	}

	resolved, err := c.resolveType(method.ReturnType)
	if err != nil {
		c.report(CodeUnknownType,
			fmt.Sprintf("unknown return type %q for method %q of %q",
				method.ReturnType, method.Name, typeName),
			method.Token.Line, method.Token.Column)
		return types.Any
	}
	return resolved
}

// orderedMethods returns an impl's methods in declaration order.
//
// A Go map has no order, and a diagnostic that names a different method on each
// run is a diagnostic nobody can act on.
func orderedMethods(methods map[string]*ast.TraitMethod) []*ast.TraitMethod {
	names := make([]string, 0, len(methods))
	for name := range methods {
		names = append(names, name)
	}
	sortStrings(names)

	out := make([]*ast.TraitMethod, 0, len(names))
	for _, name := range names {
		out = append(out, methods[name])
	}
	return out
}

// checkImplBodies type-checks every impl method's body.
//
// The body is checked by turning the method into an ordinary function literal —
// with `self` annotated as the implemented type — and running the same path a
// declared function takes. That is what makes the body ordinary: its parameters,
// its return type, and the types of everything it touches are checked by the code
// that already checks a function, so a method cannot end up with weaker checking
// than a free function just because it was written inside an impl block.
//
// It matters in particular for method calls *inside* a method. A body that is
// never checked is a body whose `self.other()` never resolves, which reaches the
// backend as a dot access on a plain hash and fails there — far from the line
// that wrote it.
func (c *Checker) checkImplBodies(decl *ast.ImplDecl) {
	if decl == nil {
		return
	}

	for _, method := range decl.Methods {
		if method.Body == nil {
			// Already reported by the parser: a method with no body in an impl.
			continue
		}

		params := make([]*ast.Parameter, 0, len(method.Parameters)+1)
		if len(method.Parameters) == 0 || method.Parameters[0] == nil ||
			method.Parameters[0].Name == nil || method.Parameters[0].Name.Value != "self" {
			c.report(CodeNoImplForMethod,
				fmt.Sprintf("method %q of %q does not take a receiver", method.Name, decl.Type.Value),
				method.Token.Line, method.Token.Column)
			continue
		}
		params = append(params, &ast.Parameter{
			Token:          method.Parameters[0].Token,
			Name:           method.Parameters[0].Name,
			TypeAnnotation: decl.Type.Value,
		})
		params = append(params, method.Parameters[1:]...)

		lit := &ast.FunctionLiteral{
			Token:      method.Token,
			Name:       ast.ImplMethodName(decl.Type.Value, method.Name),
			Parameters: params,
			ReturnType: method.ReturnType,
			Body:       method.Body,
		}
		c.inferExpression(lit)
	}
}

// checkMethodBodies checks the methods declared on a type, by name.
//
// It exists so the checker can run a method's body on demand — used by `nil
// check`, which walks declarations rather than calls — without changing how a
// body is checked when it is reached normally.
func (c *Checker) checkMethodBodies(prog *ast.Program) {
	if prog == nil {
		return
	}
	for _, stmt := range prog.Statements {
		if decl, ok := ast.Unwrap(stmt).(*ast.ImplDecl); ok {
			c.checkImplBodies(decl)
		}
	}
}

// sortStrings sorts in place, wrapped so the intent reads at the call site.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// describeTypeName renders a receiver type for a diagnostic.
func describeTypeName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "this value"
	}
	return name
}
