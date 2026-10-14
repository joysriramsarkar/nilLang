package lower

import (
	"fmt"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
)

// Traits lower to ordinary functions and a rewritten call site.
//
// # Why there is no method table
//
// `impl` does not introduce a dispatch table, a vtable, or a per-object type
// tag. Each method becomes a top-level function under a mangled name, and a call
// is rewritten to call it. That has three consequences, and all three are why it
// was chosen:
//
//   - Neither engine needs to change. They already call functions, so they cannot
//     disagree about what a method does.
//   - There is no run-time cost and no per-object bookkeeping.
//   - The receiver's *static* type picks the implementation, which is the whole
//     point of a trait. Resolution on the dynamic type would be a different
//     language; no resolution at all would turn a diagnosable mistake into a
//     crash.
//
// # Where the type comes from
//
// Rewriting `d.greet()` needs to know that `d` is a `Dog`, and the lowering pass
// does no type inference. The checker knows, so it records the resolution
// alongside its diagnostics — keyed on the node, not written back into the tree.
// Writing it back would corrupt the source program that a formatter reads, and
// the formatter and the formatter's tests are not worth that.
//
// A call with no recorded resolution is left exactly as written, so a method call
// on a value that has no impl reaches the backend and is reported there rather
// than being silently turned into a call to a function that does not exist.

// MethodResolver reports the generated name a method call should compile to, or
// "" when the call is not a trait method the checker resolved.
//
// It is an interface so that the lowering package does not depend on the
// typechecker's types, and so a caller with no resolutions at all can pass nil.
type MethodResolver interface {
	// ResolvedMethodName returns the function a method call compiles to.
	ResolvedMethodName(node ast.Expression) (string, bool)
}

// ProgramWithMethods lowers a program that has trait implementations in it.
//
// resolutions is the checker's record of which method calls resolved to which
// generated function; nil means the program has no trait methods.
func ProgramWithMethods(program *ast.Program, resolutions map[ast.Expression]string) (*ast.Program, error) {
	if program == nil {
		return nil, fmt.Errorf("cannot lower a nil program")
	}

	impls := collectImpls(program)
	if len(impls) == 0 && len(resolutions) == 0 {
		// Nothing trait-shaped in the program. Going through the trait path
		// anyway would rebuild every function in it for no reason.
		return Program(program)
	}

	l := &lowerer{
		reserved:    make(map[string]bool),
		resolutions: resolutions,
		impls:       impls,
		loopDepth:   0,
	}

	stmts := make([]ast.Statement, 0, len(program.Statements))
	for _, stmt := range program.Statements {
		switch node := ast.Unwrap(stmt).(type) {
		case *ast.ImplDecl:
			// The methods are hoisted below; the block itself contributes
			// nothing else, so emitting it here would leave an empty statement
			// behind.
			for _, method := range node.Methods {
				hoisted, err := l.implMethod(node.Type.Value, method)
				if err != nil {
					return nil, err
				}
				stmts = append(stmts, hoisted)
			}
			continue

		case *ast.TraitDecl:
			// A trait is a declaration with no run-time effect, exactly as a
			// struct is. Its constraints were checked; nothing survives to run.
			continue

		default:
			lowered, err := l.statement(stmt)
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, lowered)
		}
	}

	return &ast.Program{Statements: stmts}, nil
}

// implMethod turns one impl method into a top-level function literal.
//
// The receiver stays an ordinary first parameter: it is not removed and passed
// some other way. That keeps the generated function callable, checkable, and
// debuggable like any other, which is worth more than saving one argument.
func (l *lowerer) implMethod(typeName string, method *ast.TraitMethod) (ast.Statement, error) {
	if method.Body == nil {
		return nil, &Error{Line: method.Token.Line, Msg: fmt.Sprintf("method %q has no body", method.Name)}
	}

	params := make([]*ast.Parameter, 0, len(method.Parameters))
	for _, p := range method.Parameters {
		params = append(params, &ast.Parameter{
			Token:          p.Token,
			Name:           p.Name,
			TypeAnnotation: p.TypeAnnotation,
		})
	}

	body, err := l.block(method.Body)
	if err != nil {
		return nil, err
	}

	lit := &ast.FunctionLiteral{
		Token:      method.Token,
		Parameters: params,
		ReturnType: method.ReturnType,
		Body:       body,
		Name:       ast.ImplMethodName(typeName, method.Name),
	}

	return &ast.ExpressionStatement{
		Token:      method.Token,
		Expression: lit,
	}, nil
}

// rewriteMethodCall turns a resolved method call into a plain call.
//
// The receiver becomes the first argument. Nothing else changes: the arguments
// keep their order and their values, so a call `d.greet(greeting, 2)` becomes
// `__impl(Dog, greet)(d, greeting, 2)` with no other adjustment.
func (l *lowerer) rewriteMethodCall(call *ast.CallExpression) (*ast.CallExpression, bool) {
	if call == nil {
		return nil, false
	}
	dot, ok := call.Function.(*ast.DotExpression)
	if !ok {
		return nil, false
	}

	name, ok := l.resolve(dot)
	if !ok {
		return nil, false
	}

	args := make([]ast.Expression, 0, len(call.Arguments)+1)
	args = append(args, l.expressionOrOriginal(dot.Left))
	args = append(args, call.Arguments...)

	return &ast.CallExpression{
		Token:     call.Token,
		Function:  &ast.Identifier{Token: dot.Token, Value: name},
		Arguments: args,
	}, true
}

// expressionOrOriginal lowers an expression, falling back to the original when
// lowering fails.
//
// A method receiver is ordinary source, so a lowering failure in it is a real
// bug. But silently dropping a receiver would turn that bug into a call with the
// wrong arity, and the wrong arity is much harder to trace back than the original
// node. Keeping the original means the backend reports what it cannot compile.
func (l *lowerer) expressionOrOriginal(expr ast.Expression) ast.Expression {
	if expr == nil {
		return nil
	}
	lowered, err := l.expression(expr)
	if err != nil {
		return expr
	}
	return lowered
}

// resolve asks the checker's record which generated function a method call
// compiles to.
func (l *lowerer) resolve(node ast.Expression) (string, bool) {
	if l.resolutions == nil || node == nil {
		return "", false
	}
	name, ok := l.resolutions[node]
	if !ok || name == "" {
		return "", false
	}
	return name, true
}

// collectImpls indexes the program's implementations by type and method name.
//
// It exists so the lowering pass can answer "does this type implement that at
// all" without re-walking the program at every call site, and so the answer is
// the same for every call.
func collectImpls(program *ast.Program) map[string]map[string]*ast.TraitMethod {
	impls := make(map[string]map[string]*ast.TraitMethod)
	if program == nil {
		return impls
	}

	for _, stmt := range program.Statements {
		decl, ok := ast.Unwrap(stmt).(*ast.ImplDecl)
		if !ok {
			continue
		}
		methods, ok := impls[decl.Type.Value]
		if !ok {
			methods = make(map[string]*ast.TraitMethod, len(decl.Methods))
			impls[decl.Type.Value] = methods
		}
		for _, method := range decl.Methods {
			methods[method.Name] = method
		}
	}
	return impls
}
