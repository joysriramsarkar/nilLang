package evaluator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	compbuiltin "github.com/joysriramsarkar/nilLang/compiler/builtin"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/runtime/concurrency"
)

var (
	NULL  = object.NULL
	TRUE  = object.TRUE
	FALSE = object.FALSE
)

// CapabilityChecker gates effectful builtin calls at the call site (L6.4).
//
// When nil every builtin runs, which is the historical behaviour the
// conformance suite relies on. `nil run` installs the manifest's grant check
// here, so a denied operation fails with CategoryCapabilityDenied — the same
// category the VM reports for the same program. Tests set and reset it
// around the denied cases they assert on.
//
// The holder is atomic, not a plain variable: task bodies evaluate on their
// own goroutines and can outlive whatever installed the checker (a test's
// deferred restore runs while a straggler from an earlier case still reads
// it). A plain variable races that read against the next write. Install with
// SetCapabilityChecker and read with GetCapabilityChecker; direct assignment
// cannot be made race-free.
var capabilityChecker atomic.Pointer[capabilityCheckFunc]

// capabilityCheckFunc is the function type the atomic holder points at. A
// named type because atomic.Pointer needs an addressable value to store.
type capabilityCheckFunc func(operation string) error

// SetCapabilityChecker installs the process-wide capability gate, replacing
// any previous one. A nil checker restores the historical behaviour where
// every builtin runs.
func SetCapabilityChecker(checker func(operation string) error) {
	if checker == nil {
		capabilityChecker.Store(nil)
		return
	}
	fn := capabilityCheckFunc(checker)
	capabilityChecker.Store(&fn)
}

// GetCapabilityChecker returns the installed gate, or nil when every builtin
// runs.
func GetCapabilityChecker() func(operation string) error {
	if p := capabilityChecker.Load(); p != nil {
		return *p
	}
	return nil
}

// newEvalTaskScope returns the scope tasks spawned by this evaluation chain
// belong to, creating one on first use.
//
// # Why the scope is not a package global
//
// A single process-wide root makes every task in the process a sibling of every
// other one. The runtime cancels a faulting task's siblings, so with a global
// root one task's failure cancelled the tasks of every other evaluation in the
// process — including futures an earlier evaluation was still holding.
//
// The symptom was a conformance clause that held or failed depending on
// scheduling: CONCURRENCY.md §4.1 fault isolation passed in isolation and failed
// about three times in four when run alongside the rest of the table. Nothing
// about the program changed; only which goroutine won did.
//
// The scope therefore lives on the environment, alongside CallDepth and
// PureDepth, for the same reason those moved off a counter: per-evaluation
// state that a shared counter turns into a shared budget. A REPL session and a
// module body, which genuinely share bindings with their parent, inherit the
// same scope rather than getting one of their own.
func newEvalTaskScope(env *object.Environment) *concurrency.Task {
	if existing, ok := env.ScopeValue().(*concurrency.Task); ok && existing != nil {
		return existing
	}

	scope := concurrency.NewTaskScope(nil)
	env.SetScope(scope)
	return scope
}

// builtinMu guards Builtins so that concurrent reads (BuiltinName, VM call
// dispatch) and concurrent writes (tests registering temporary builtins) do
// not race. Builtins itself stays a plain map because many call sites index
// it by name; the lock is the synchronisation point for every access.
var (
	builtinMu        sync.RWMutex
	builtinNameCache sync.Map // *object.Builtin -> string
)

// BuiltinName resolves a builtin value to the name it is registered under,
// for the capability and effect gates. The second result is false for values
// that came from anywhere but the builtin table, such as native-module
// closures, which neither gate applies to.
func BuiltinName(fn *object.Builtin) (string, bool) {
	if name, ok := builtinNameCache.Load(fn); ok {
		return name.(string), true
	}
	builtinMu.RLock()
	defer builtinMu.RUnlock()
	for name, builtin := range Builtins {
		if builtin == fn {
			builtinNameCache.Store(fn, name)
			return name, true
		}
	}
	return "", false
}

// SetBuiltin registers a builtin under name, replacing any existing entry.
// Callers that previously wrote to evaluator.Builtins directly should use
// this instead so that concurrent BuiltinName readers do not race.
func SetBuiltin(name string, builtin *object.Builtin) {
	builtinMu.Lock()
	defer builtinMu.Unlock()
	Builtins[name] = builtin
	builtinNameCache.Store(builtin, name)
}

// DeleteBuiltin removes a builtin registered by name. It is the counterpart
// of SetBuiltin for tests that install temporary builtins and must restore
// the previous state.
func DeleteBuiltin(name string) {
	builtinMu.Lock()
	defer builtinMu.Unlock()
	delete(Builtins, name)
}

// checkBuiltinCall enforces the capability sandbox (L6.4) and the pure
// context (L6.3) on a builtin call. It returns nil when the call may run.
//
// Purity comes from the caller's environment, not a counter: two tasks have
// two call stacks, and a shared counter would let one task's purity forbid
// another's effects.
func checkBuiltinCall(fn *object.Builtin, env *object.Environment) object.Object {
	checker := GetCapabilityChecker()
	if checker == nil && envPureDepth(env) == 0 {
		return nil
	}
	name, ok := BuiltinName(fn)
	if !ok {
		return nil
	}
	if envPureDepth(env) > 0 && compbuiltin.IsEffectful(name) {
		return runtimeError(object.CategoryTypeError,
			"E0202: cannot call effectful builtin %q inside a pure function", name)
	}
	if checker != nil {
		if class, known := compbuiltin.Classify(name); known && class.Gated {
			if err := checker(name); err != nil {
				return object.RuntimeErrorWrapf(object.CategoryCapabilityDenied, err,
					"capability denied: %s requires [%s]: %s", name, class.Requires, err)
			}
		}
	}
	return nil
}

// taskFaultError converts a task body's failure into the future's error.
//
// Every failure inside a task — a returned error object, a Go error, a
// panic — becomes a TaskFaulted fault that names its cause
// (CONCURRENCY.md §4). Cancellation is the exception: a cancelled task
// reports TaskCancelled, because the parent did not fail, it was stopped.
// The cause keeps its own category underneath for inspection, but what
// `await` reports is the fault: the parent did not divide by zero, its
// child did.
func taskFaultError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, concurrency.ErrTaskCancelled) {
		return object.RuntimeErrorWrapf(object.CategoryTaskCancelled, err, "task cancelled")
	}
	if category, ok := object.CategoryOf(err); ok {
		if category == object.CategoryTaskFaulted || category == object.CategoryTaskCancelled {
			return err
		}
	}
	var runtimeErr *object.RuntimeError
	if errors.As(err, &runtimeErr) {
		return object.RuntimeErrorWrapf(object.CategoryTaskFaulted, err,
			"task failed: %s", runtimeErr.Message)
	}
	return object.RuntimeErrorWrapf(object.CategoryTaskFaulted, err,
		"task failed: %s", err.Error())
}

// taskBodyError converts an error object a task body evaluated to into the Go
// error the task tree completes with, preserving a categorised error as is.
func taskBodyError(obj object.Object) error {
	if runtimeErr, ok := obj.(*object.RuntimeError); ok {
		return runtimeErr
	}
	if errObj, ok := obj.(*object.Error); ok {
		return errors.New(errObj.Inspect())
	}
	return errors.New("task failed")
}

// faultAsObject converts a faulted future's error back into the object the
// awaiting program observes, preserving a typed fault rather than flattening
// it into prose the other engine would word differently.
func faultAsObject(err error) object.Object {
	var runtimeErr *object.RuntimeError
	if errors.As(err, &runtimeErr) {
		return runtimeErr
	}
	return newError("task failed: %s", err)
}

// Eval evaluates an AST node in an environment.
//
// Recursion is bounded in applyFunction rather than here, so that the bound
// counts call frames the way the VM's MaxFrames does. Counting AST nodes
// instead would make the two engines disagree about when a given program
// overflows.
func Eval(node ast.Node, env *object.Environment) object.Object {
	return evalNode(node, env)
}

func evalNode(node ast.Node, env *object.Environment) object.Object {
	// A visibility marker changes what leaves the module, not what the
	// declaration does when the module runs, so the wrapper is transparent here.
	if wrapped, ok := node.(*ast.ExportStatement); ok && wrapped.Decl != nil {
		return Eval(wrapped.Decl, env)
	}

	switch node := node.(type) {
	case *ast.Program:
		return evalProgram(node, env)
	case *ast.BlockStatement:
		return evalBlockStatement(node, env)
	case *ast.AppStatement:
		// An app body is deliberately NOT a scope (see the checker): evaluate
		// its statements in this environment instead of enclosing a block one.
		if node.Body != nil {
			for _, s := range node.Body.Statements {
				result := Eval(s, env)
				if result != nil {
					switch result.Type() {
					case object.RETURN_VALUE_OBJ, object.ERROR_OBJ,
						object.BREAK_SIGNAL_OBJ, object.CONTINUE_SIGNAL_OBJ:
						return result
					}
				}
			}
		}
		return NULL
	case *ast.ExpressionStatement:
		return Eval(node.Expression, env)
	case *ast.ReturnStatement:
		val := Eval(node.ReturnValue, env)
		if isError(val) {
			return val
		}
		return &object.ReturnValue{Value: val}
	case *ast.LetStatement:
		if node.Value == nil {
			return newError("variable %s requires an initializer", node.Name.Value)
		}
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}
		if node.Constant {
			env.SetConst(node.Name.Value, val)
		} else {
			env.Set(node.Name.Value, val)
		}
	case *ast.AssignStatement:
		if env.IsConst(node.Name.Value) {
			return newError("cannot assign to constant: %s", node.Name.Value)
		}
		value := node.Value
		if node.Operator == "+=" || node.Operator == "-=" {
			value = &ast.InfixExpression{Token: node.Token, Left: node.Name, Operator: strings.TrimSuffix(node.Operator, "="), Right: node.Value}
		}
		val := Eval(value, env)
		if isError(val) {
			return val
		}
		if !env.Assign(node.Name.Value, val) {
			return newError("identifier not found: %s", node.Name.Value)
		}
		return val
	case *ast.IndexAssignStatement:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		index := Eval(node.Index, env)
		if isError(index) {
			return index
		}
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}
		return evalIndexAssign(left, index, val)
	case *ast.StateDeclaration:
		var val object.Object = NULL
		if node.Value != nil {
			val = Eval(node.Value, env)
			if isError(val) {
				return val
			}
		}
		if node.Name != nil {
			env.Set(node.Name.Value, val)
		}
	case *ast.WhileStatement:
		return evalWhileStatement(node, env)
	case *ast.LetExpression:
		return evalLetExpression(node, env)
	case *ast.StructDecl, *ast.EnumDecl, *ast.TypeAliasDecl:
		// Declarations have no run-time effect: a struct's name is a type and
		// its fields are checked statically. The type checker is what knows
		// about them; by the time code runs, there is nothing left to do.
		return NULL
	case *ast.MatchExpression:
		return newError("internal error: a `match` reached the evaluator without being lowered")

	case *ast.CoalesceExpression, *ast.SafeAccessExpression:
		// Refusing explicitly matters here more than anywhere else. Without a
		// case this function returns nil, the enclosing statement carries on with
		// nothing on the stack, and the program produces a *wrong answer* instead
		// of stopping — which is the one outcome a reader cannot diagnose.
		return newError("internal error: `??` or `?.` reached the evaluator without being lowered")
	case *ast.EnumVariantExpr:
		return newError("internal error: an enum variant reached the evaluator without being lowered")
	case *ast.StructLiteral:
		return newError("internal error: a struct literal reached the evaluator without being lowered")
	case *ast.ForStatement:
		return evalForStatement(node, env)
	case *ast.BreakStatement:
		return BREAK_SIGNAL
	case *ast.ContinueStatement:
		return CONTINUE_SIGNAL
	case *ast.EntityStatement:
		fields := make([]object.EntityFieldObj, len(node.Fields))
		for i, f := range node.Fields {
			fields[i] = object.EntityFieldObj{
				Name:         f.Name,
				Type:         f.Type,
				IsPrimary:    f.IsPrimary,
				IsRequired:   f.IsRequired,
				IsUnique:     f.IsUnique,
				TargetEntity: f.TargetEntity,
			}
		}
		var name string
		if node.Name != nil {
			name = node.Name.Value
		}
		entityObj := &object.Entity{
			Name:   name,
			Fields: fields,
		}
		if node.Name != nil {
			env.Set(node.Name.Value, entityObj)
		}
		return entityObj

	// Expressions
	case *ast.IntegerLiteral:
		return &object.Integer{Value: node.Value}
	case *ast.FloatLiteral:
		return &object.Float{Value: node.Value}
	case *ast.Boolean:
		return nativeBoolToBooleanObject(node.Value)
	case *ast.NullLiteral:
		return NULL
	case *ast.StringLiteral:
		return &object.String{Value: node.Value}
	case *ast.StringTemplate:
		return evalStringTemplate(node, env)
	case *ast.PrefixExpression:
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalPrefixExpression(node.Operator, right)
	case *ast.InfixExpression:
		if node.Operator == "&&" {
			left := Eval(node.Left, env)
			if isError(left) {
				return left
			}
			if !isTruthy(left) {
				return FALSE
			}
			right := Eval(node.Right, env)
			if isError(right) {
				return right
			}
			return nativeBoolToBooleanObject(isTruthy(right))
		}
		if node.Operator == "||" {
			left := Eval(node.Left, env)
			if isError(left) {
				return left
			}
			if isTruthy(left) {
				return TRUE
			}
			right := Eval(node.Right, env)
			if isError(right) {
				return right
			}
			return nativeBoolToBooleanObject(isTruthy(right))
		}
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalInfixExpression(node.Operator, left, right)
	case *ast.IfExpression:
		return evalIfExpression(node, env)
	case *ast.Identifier:
		return evalIdentifier(node, env)
	case *ast.FunctionLiteral:
		body := node.Body
		// Only the names are kept: the annotations were the compiler's business
		// and the evaluator has nothing to do with them once checked.
		fn := &object.Function{Parameters: node.ParameterNames(), Env: env, Body: body, IsPure: node.IsPure}
		if node.Name != "" {
			env.Set(node.Name, fn)
		}
		return fn
	case *ast.TaskExpression:
		future := object.NewFuture()
		// Resolve the scope before enclosing: the task body's environment
		// inherits the scope from this one, so a scope created after the
		// enclosure would not reach the body.
		scope := newEvalTaskScope(env)
		// Shared bindings, not a copy: the child environment encloses the
		// parent's, so the task reads the parent's variables and assigns
		// through to them (L6.2). The store lock in object.Environment is
		// what makes that sharing memory-safe across goroutines.
		taskEnv := object.NewEnclosedEnvironment(env)
		task := scope.Spawn(func(ctx context.Context) (object.Object, error) {
			result := Eval(node.Body, taskEnv)
			if returnValue, ok := result.(*object.ReturnValue); ok {
				result = returnValue.Value
			}
			if result == nil {
				result = NULL
			}
			// A body that evaluated to an error faults the task rather than
			// fulfilling it with the error as a value: awaiting the future
			// must observe the failure, and the tree must cancel the
			// siblings, both of which only happen through the error path.
			if isError(result) {
				return nil, taskBodyError(result)
			}
			return result, nil
		})
		// Bridge the task tree's future to the language-level future the
		// program awaits. The tree owns cancellation and fault routing;
		// this future owns the value the program observes.
		go func() {
			value, err := task.Await()
			if err != nil {
				future.Complete(nil, taskFaultError(err))
				return
			}
			future.Complete(value, nil)
		}()
		return future
	case *ast.AwaitExpression:
		value := Eval(node.Right, env)
		if isError(value) {
			return value
		}
		future, ok := value.(*object.Future)
		if !ok {
			return newError("cannot await non-future value: %s", value.Type())
		}
		result, err := future.Await()
		if err != nil {
			return faultAsObject(err)
		}
		return result
	case *ast.CallExpression:
		function := Eval(node.Function, env)
		if isError(function) {
			return function
		}
		args := evalExpressions(node.Arguments, env)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}
		return applyFunction(function, args, env)
	case *ast.ArrayLiteral:
		elements := evalExpressions(node.Elements, env)
		if len(elements) == 1 && isError(elements[0]) {
			return elements[0]
		}
		return &object.Array{Elements: elements}
	case *ast.IndexExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		index := Eval(node.Index, env)
		if isError(index) {
			return index
		}
		return evalIndexExpression(left, index)
	case *ast.DotExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		return evalDotExpression(left, node.Member.Value)
	case *ast.HashLiteral:
		return evalHashLiteral(node, env)
	case *ast.ImportStatement:
		return evalImportStatement(node, env)
	case *ast.ComponentLiteral:
		return evalComponentLiteral(node, env)
	}

	return nil
}

func evalProgram(program *ast.Program, env *object.Environment) object.Object {
	var result object.Object

	for _, statement := range program.Statements {
		result = Eval(statement, env)

		switch result := result.(type) {
		case *object.ReturnValue:
			return result.Value
		case *object.Error:
			return result
		case *object.RuntimeError:
			// A categorised failure halts the program exactly like a legacy
			// error: the VM unwinds its whole Run on the same fault, and a
			// program that continued past it in one engine but not the other
			// would disagree with itself about what ran (ERROR_MODEL.md §4).
			return result
		}
	}

	return result
}

// evalBlockStatement evaluates a block, propagating control flow signals.
//
// A reak or continue must stop the block immediately, not merely be its
// last value: in { if (x) { break; } work(); } the work() after the
// reak must not run. Checking only the final value would let it.
func evalBlockStatement(block *ast.BlockStatement, env *object.Environment) object.Object {
	// A block is a scope: `let` inside shadows instead of overwriting the
	// enclosing binding, matching the type checker (which rejects reads of
	// block locals from outside). Assignments still walk outward via Assign.
	// The program root never reaches here (evalProgram loops directly), so
	// top-level bindings stay global.
	scoped := object.NewEnclosedEnvironment(env)
	var result object.Object

	for _, statement := range block.Statements {
		result = Eval(statement, scoped)

		if result != nil {
			switch result.Type() {
			case object.RETURN_VALUE_OBJ, object.ERROR_OBJ,
				object.BREAK_SIGNAL_OBJ, object.CONTINUE_SIGNAL_OBJ:
				return result
			}
		}
	}

	return result
}

// evalLetExpression evaluates `let name = value; body`.
//
// The binding is scoped to the body, matching the block form of `let`. Scoping
// matters rather than being cosmetic: the lowering of `match` introduces these,
// and a leaked name would collide with the next `match`'s hidden subject.
//
// The result is the body's value, which is what makes this an expression rather
// than a statement.
func evalLetExpression(le *ast.LetExpression, env *object.Environment) object.Object {
	if le.Name == nil || le.Value == nil || le.Body == nil {
		return newError("a `let` expression needs a name, a value, and a body")
	}

	value := Eval(le.Value, env)
	if isError(value) {
		return value
	}

	scoped := object.NewEnclosedEnvironment(env)
	scoped.Set(le.Name.Value, value)

	return Eval(le.Body, scoped)
}

func evalWhileStatement(ws *ast.WhileStatement, env *object.Environment) object.Object {
	var result object.Object = NULL

	for {
		condition := Eval(ws.Condition, env)
		if isError(condition) {
			return condition
		}

		if !isTruthy(condition) {
			break
		}

		bodyResult := Eval(ws.Body, env)
		if bodyResult != nil {
			switch bodyResult.Type() {
			case object.BREAK_SIGNAL_OBJ:
				// The loop is consumed by the signal, so the loop's value is
				// whatever the body last produced, not the signal itself.
				return result
			case object.CONTINUE_SIGNAL_OBJ:
				// Skip the rest of this pass and re-test the condition. The
				// cursor increment lives inside the lowered body, so a `for`
				// loop's `continue` still advances it.
				continue
			case object.RETURN_VALUE_OBJ, object.ERROR_OBJ:
				return bodyResult
			}
			result = bodyResult
		}
	}

	return result
}

func evalStringTemplate(st *ast.StringTemplate, env *object.Environment) object.Object {
	var sb strings.Builder
	for _, part := range st.Parts {
		if part.IsExpression {
			val := Eval(part.Expression, env)
			if isError(val) {
				return val
			}
			sb.WriteString(val.Inspect())
		} else {
			sb.WriteString(part.Literal)
		}
	}
	return &object.String{Value: sb.String()}
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

func evalPrefixExpression(operator string, right object.Object) object.Object {
	switch operator {
	case "!":
		return evalBangOperatorExpression(right)
	case "-":
		return evalMinusPrefixOperatorExpression(right)
	default:
		return runtimeError(object.CategoryTypeError, "unknown operator: %s%s", operator, right.Type())
	}
}

// evalBangOperatorExpression implements `!x`.
//
// Like isTruthy, this matches on type and value rather than pointer identity, so
// `!isOk(x)` negates a boolean that came from a builtin instead of quietly
// reporting false.
func evalBangOperatorExpression(right object.Object) object.Object {
	switch value := right.(type) {
	case *object.Boolean:
		if value.Value {
			return FALSE
		}
		return TRUE
	case *object.Null:
		return TRUE
	case *object.Integer:
		if value.Value == 0 {
			return TRUE
		}
		return FALSE
	default:
		return FALSE
	}
}

func evalMinusPrefixOperatorExpression(right object.Object) object.Object {
	switch right := right.(type) {
	case *object.Integer:
		return &object.Integer{Value: -right.Value}
	case *object.Float:
		return &object.Float{Value: -right.Value}
	default:
		return runtimeError(object.CategoryTypeError, "unknown operator: -%s", right.Type())
	}
}

func evalInfixExpression(operator string, left, right object.Object) object.Object {
	switch {
	case left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ:
		return evalIntegerInfixExpression(operator, left, right)
	case left.Type() == object.FLOAT_OBJ || right.Type() == object.FLOAT_OBJ:
		return evalFloatInfixExpression(operator, left, right)
	case left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ:
		return evalStringInfixExpression(operator, left, right)
	case left.Type() == object.STRING_OBJ || right.Type() == object.STRING_OBJ:
		if operator == "+" {
			return &object.String{Value: left.Inspect() + right.Inspect()}
		}
		if operator == "==" {
			return nativeBoolToBooleanObject(left.Inspect() == right.Inspect())
		}
		if operator == "!=" {
			return nativeBoolToBooleanObject(left.Inspect() != right.Inspect())
		}
		return runtimeError(object.CategoryTypeError, "unknown operator: %s %s %s", left.Type(), operator, right.Type())
	case operator == "==":
		return nativeBoolToBooleanObject(left == right)
	case operator == "!=":
		return nativeBoolToBooleanObject(left != right)
	case operator == "&&":
		return nativeBoolToBooleanObject(isTruthy(left) && isTruthy(right))
	case operator == "||":
		return nativeBoolToBooleanObject(isTruthy(left) || isTruthy(right))
	case left.Type() != right.Type():
		return runtimeError(object.CategoryTypeError, "type mismatch: %s %s %s", left.Type(), operator, right.Type())
	default:
		return runtimeError(object.CategoryTypeError, "unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalIntegerInfixExpression(operator string, left, right object.Object) object.Object {
	leftVal := left.(*object.Integer).Value
	rightVal := right.(*object.Integer).Value

	switch operator {
	case "+":
		return &object.Integer{Value: leftVal + rightVal}
	case "-":
		return &object.Integer{Value: leftVal - rightVal}
	case "*":
		return &object.Integer{Value: leftVal * rightVal}
	case "/":
		if rightVal == 0 {
			return runtimeError(object.CategoryDivisionByZero, "division by zero")
		}
		return &object.Integer{Value: leftVal / rightVal}
	case "%":
		if rightVal == 0 {
			return runtimeError(object.CategoryDivisionByZero, "modulo by zero")
		}
		return &object.Integer{Value: leftVal % rightVal}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	case "&&":
		return nativeBoolToBooleanObject(leftVal != 0 && rightVal != 0)
	case "||":
		return nativeBoolToBooleanObject(leftVal != 0 || rightVal != 0)
	default:
		return runtimeError(object.CategoryTypeError, "unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalFloatInfixExpression(operator string, left, right object.Object) object.Object {
	var leftVal, rightVal float64

	if left.Type() == object.INTEGER_OBJ {
		leftVal = float64(left.(*object.Integer).Value)
	} else {
		leftVal = left.(*object.Float).Value
	}

	if right.Type() == object.INTEGER_OBJ {
		rightVal = float64(right.(*object.Integer).Value)
	} else {
		rightVal = right.(*object.Float).Value
	}

	switch operator {
	case "+":
		return &object.Float{Value: leftVal + rightVal}
	case "-":
		return &object.Float{Value: leftVal - rightVal}
	case "*":
		return &object.Float{Value: leftVal * rightVal}
	case "/":
		return &object.Float{Value: leftVal / rightVal}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return runtimeError(object.CategoryTypeError, "unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalStringInfixExpression(operator string, left, right object.Object) object.Object {
	leftVal := left.(*object.String).Value
	rightVal := right.(*object.String).Value

	switch operator {
	case "+":
		return &object.String{Value: leftVal + rightVal}
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return runtimeError(object.CategoryTypeError, "unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalIfExpression(ie *ast.IfExpression, env *object.Environment) object.Object {
	condition := Eval(ie.Condition, env)
	if isError(condition) {
		return condition
	}

	if isTruthy(condition) {
		return Eval(ie.Consequence, env)
	} else if ie.Alternative != nil {
		return Eval(ie.Alternative, env)
	} else {
		return NULL
	}
}

func evalIdentifier(node *ast.Identifier, env *object.Environment) object.Object {
	if val, ok := env.Get(node.Value); ok {
		return val
	}

	if node.Value == "ui" {
		return makeUINamespace()
	}

	if builtin, ok := Builtins[node.Value]; ok {
		return builtin
	}

	return newError("identifier not found: %s", node.Value)
}

func makeUINamespace() *object.Hash {
	return MakeHashObj(map[string]object.Object{
		"NewPage": &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				title := "Page"
				if len(args) > 0 {
					title = args[0].Inspect()
				}
				return MakeHashObj(map[string]object.Object{
					"type":    &object.String{Value: "Page"},
					"title":   &object.String{Value: title},
					"content": &object.Array{Elements: []object.Object{}},
				})
			},
		},
		"NewNavigation": &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				brand := "App"
				if len(args) > 0 {
					brand = args[0].Inspect()
				}
				return MakeHashObj(map[string]object.Object{
					"type":  &object.String{Value: "Navigation"},
					"brand": &object.String{Value: brand},
					"items": &object.Array{Elements: []object.Object{}},
				})
			},
		},
		"NewDashboard": &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				title := "Dashboard"
				if len(args) > 0 {
					title = args[0].Inspect()
				}
				return MakeHashObj(map[string]object.Object{
					"type":    &object.String{Value: "Dashboard"},
					"title":   &object.String{Value: title},
					"metrics": &object.Array{Elements: []object.Object{}},
				})
			},
		},
		"NewTable": &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				headers := &object.Array{Elements: args}
				return MakeHashObj(map[string]object.Object{
					"type":    &object.String{Value: "Table"},
					"headers": headers,
					"rows":    &object.Array{Elements: []object.Object{}},
				})
			},
		},
		"NewForm": &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				title := "Form"
				if len(args) > 0 {
					title = args[0].Inspect()
				}
				return MakeHashObj(map[string]object.Object{
					"type":   &object.String{Value: "Form"},
					"title":  &object.String{Value: title},
					"fields": &object.Array{Elements: []object.Object{}},
				})
			},
		},
	})
}

// isTruthy reports whether a value counts as true.
//
// The comparison is by type and value, never by pointer identity, so a boolean
// that arrived from a host boundary or a builtin is treated the same as one this
// package created. See the note on the VM's copy of this function for the bug
// that motivated the rule.
func isTruthy(obj object.Object) bool {
	switch value := obj.(type) {
	case nil:
		return false
	case *object.Boolean:
		return value.Value
	case *object.Null:
		return false
	case *object.Integer:
		return value.Value != 0
	default:
		return true
	}
}

func newError(format string, a ...interface{}) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, a...)}
}

// runtimeError builds a categorised error for the tree-walking engine.
//
// ERROR_MODEL.md §4 requires both engines to report the same category for the
// same fault. The evaluator unwinds by returning Objects rather than Go errors,
// so the category travels on the returned object. runtimeErrorf is the
// counterpart of the VM's vmError, and the two must be changed together.
func runtimeError(category object.ErrorCategory, format string, a ...interface{}) *object.RuntimeError {
	return object.RuntimeErrorf(category, format, a...)
}

// evalCallDepthLimit bounds evaluator call recursion.
//
// The tree-walking engine recurses in Go, so unbounded user recursion would
// exhaust the goroutine stack and crash the process rather than report an
// error. The VM already bounds this with MaxFrames (1024); matching that number
// here is what makes the two engines agree on which programs overflow.
const evalCallDepthLimit = 1024

// envCallDepth, envPureDepth and envDepthLimit read one evaluation chain's
// counters, tolerating a nil environment (built-in calls made outside any
// program start at depth zero with the default limit).
func envCallDepth(env *object.Environment) int {
	if env == nil {
		return 0
	}
	return env.CallDepth
}

func envPureDepth(env *object.Environment) int {
	if env == nil {
		return 0
	}
	return env.PureDepth
}

func envDepthLimit(env *object.Environment) int {
	if env != nil && env.DepthLimit > 0 {
		return env.DepthLimit
	}
	return evalCallDepthLimit
}

// dotenv returns the environment a function value carries for depth and
// purity purposes: a closure's defining environment, or nil for builtins,
// which have none.
//
// It is used where a Nilang function is invoked without a dynamic caller
// environment in scope — component dispatch, reactive callbacks, timeouts.
// The lexical chain undercounts the dynamic one, which errs toward allowing
// the call; the overflow limit still bounds every chain that grows through
// ordinary calls, which is the one that can exhaust a goroutine stack.
func dotenv(fn object.Object) *object.Environment {
	if function, ok := fn.(*object.Function); ok {
		return function.Env
	}
	return nil
}

func isError(obj object.Object) bool {
	if obj != nil {
		return obj.Type() == object.ERROR_OBJ
	}
	return false
}

func evalExpressions(exps []ast.Expression, env *object.Environment) []object.Object {
	var result []object.Object

	for _, e := range exps {
		evaluated := Eval(e, env)
		if isError(evaluated) {
			return []object.Object{evaluated}
		}
		result = append(result, evaluated)
	}

	return result
}

func applyFunction(fn object.Object, args []object.Object, env *object.Environment) object.Object {
	if fn == nil {
		return newError("attempted to call an invalid expression")
	}
	switch fn := fn.(type) {
	case *object.Function:
		// A non-pure function called while a pure one is on the stack
		// contradicts the pure declaration: the call may perform an effect
		// the declaration promised it would not (runtime E0202, L6.3).
		if envPureDepth(env) > 0 && !fn.IsPure {
			return runtimeError(object.CategoryTypeError,
				"E0202: cannot call side-effecting function inside a pure function")
		}
		// The frame extends the caller's environment, so depth counts the
		// dynamic chain on this goroutine — concurrent tasks count their own
		// (a shared counter was a data race), and unbounded user recursion
		// reports FrameOverflow instead of exhausting the goroutine stack.
		// This mirrors the VM's MaxFrames, which is what lets both engines
		// agree on which programs overflow.
		extendedEnv := extendFunctionEnv(fn, args)
		extendedEnv.CallDepth = envCallDepth(env) + 1
		extendedEnv.PureDepth = envPureDepth(env)
		extendedEnv.DepthLimit = envDepthLimit(env)
		if fn.IsPure {
			extendedEnv.PureDepth++
		}
		limit := extendedEnv.DepthLimit
		if extendedEnv.CallDepth > limit {
			return runtimeError(object.CategoryFrameOverflow,
				"call frame depth exceeded %d", limit)
		}

		evaluated := Eval(fn.Body, extendedEnv)
		return unwrapReturnValue(evaluated)
	case *object.Builtin:
		if gated := checkBuiltinCall(fn, env); gated != nil {
			return gated
		}
		return fn.Fn(args...)
	default:
		return runtimeError(object.CategoryTypeError, "not a function: %s", fn.Type())
	}
}

func extendFunctionEnv(fn *object.Function, args []object.Object) *object.Environment {
	env := object.NewEnclosedEnvironment(fn.Env)

	for paramIdx, param := range fn.Parameters {
		if paramIdx < len(args) {
			env.Set(param.Value, args[paramIdx])
		}
	}

	return env
}

func unwrapReturnValue(obj object.Object) object.Object {
	if returnValue, ok := obj.(*object.ReturnValue); ok {
		return returnValue.Value
	}
	return obj
}

func evalIndexExpression(left, index object.Object) object.Object {
	if left == nil || index == nil {
		return newError("index operator received an invalid expression")
	}
	switch {
	case left.Type() == object.ARRAY_OBJ && index.Type() == object.INTEGER_OBJ:
		return evalArrayIndexExpression(left, index)
	case left.Type() == object.STRING_OBJ && index.Type() == object.INTEGER_OBJ:
		return evalStringIndexExpression(left, index)
	case left.Type() == object.HASH_OBJ:
		return evalHashIndexExpression(left, index)
	default:
		return runtimeError(object.CategoryTypeError, "index operator not supported: %s[%s]", left.Type(), index.Type())
	}
}

func evalStringIndexExpression(str, index object.Object) object.Object {
	strObj := str.(*object.String)
	idx := index.(*object.Integer).Value
	runes := []rune(strObj.Value)
	if idx < 0 || idx >= int64(len(runes)) {
		return NULL
	}
	return &object.String{Value: string(runes[idx])}
}

func evalArrayIndexExpression(array, index object.Object) object.Object {
	arrayObject := array.(*object.Array)
	idx := index.(*object.Integer).Value
	max := int64(len(arrayObject.Elements) - 1)

	if idx < 0 || idx > max {
		return NULL
	}

	return arrayObject.Elements[idx]
}

func evalHashLiteral(node *ast.HashLiteral, env *object.Environment) object.Object {
	pairs := object.NewHash(node.EntryCount())

	// Evaluate in source order. SEMANTICS.md §1 requires left-to-right lexical
	// evaluation, and when a key expression has side effects that order is
	// observable. node.OrderedPairs is populated by the parser; the map
	// fallback keeps hash nodes built by hand working.
	if len(node.OrderedPairs) > 0 {
		for _, entry := range node.OrderedPairs {
			key := Eval(entry.Key, env)
			if isError(key) {
				return key
			}

			if _, ok := key.(object.Hashable); !ok {
				return runtimeError(object.CategoryTypeError, "unusable as hash key: %s", key.Type())
			}

			value := Eval(entry.Value, env)
			if isError(value) {
				return value
			}

			pairs.SetPair(key, value)
		}
		return pairs
	}

	for keyNode, valueNode := range node.Pairs {
		key := Eval(keyNode, env)
		if isError(key) {
			return key
		}

		if _, ok := key.(object.Hashable); !ok {
			return runtimeError(object.CategoryTypeError, "unusable as hash key: %s", key.Type())
		}

		value := Eval(valueNode, env)
		if isError(value) {
			return value
		}

		pairs.SetPair(key, value)
	}

	return pairs
}

func evalHashIndexExpression(hash, index object.Object) object.Object {
	hashObject := hash.(*object.Hash)

	key, ok := index.(object.Hashable)
	if !ok {
		return runtimeError(object.CategoryTypeError, "unusable as hash key: %s", index.Type())
	}

	pair, ok := hashObject.Pairs[key.HashKey()]
	if !ok {
		return NULL
	}

	return pair.Value
}

func evalIndexAssign(left, index, val object.Object) object.Object {
	switch l := left.(type) {
	case *object.Hash:
		if _, ok := index.(object.Hashable); !ok {
			return runtimeError(object.CategoryTypeError, "unusable as hash key: %s", index.Type())
		}
		// Through `SetPair`, never by writing `Pairs` directly.
		//
		// A hash holds two views of itself — a lookup table and the order the keys
		// were written in — and writing to one of them leaves the other stale. The
		// symptom is not a wrong lookup: it is a value that prints in a different
		// order than it was written, on one engine and not the others.
		//
		// SEMANTICS.md §1 records this for the VM; the evaluator had the same write
		// and it was found again by a map built with twelve assignments, which is the
		// only way a map is ever grown.
		l.SetPair(index, val)
		return val
	case *object.Array:
		idxObj, ok := index.(*object.Integer)
		if !ok {
			return newError("array index must be an integer, got %s", index.Type())
		}
		idx := idxObj.Value
		if idx < 0 || idx >= int64(len(l.Elements)) {
			return runtimeError(object.CategoryIndexOutOfBounds, "index %d out of bounds", idx)
		}
		l.Elements[idx] = val
		return val
	default:
		return runtimeError(object.CategoryTypeError, "index-assignment not supported on: %s", left.Type())
	}
}

func evalDotExpression(left object.Object, member string) object.Object {
	switch obj := left.(type) {
	case *object.Channel:
		switch strings.ToLower(member) {
		case "send":
			return &object.Builtin{Fn: func(args ...object.Object) object.Object {
				if len(args) != 1 {
					return newError("wrong number of arguments to `channel.send`. got=%d, want=1", len(args))
				}
				obj.Values <- args[0]
				return NULL
			}}
		case "receive":
			return &object.Builtin{Fn: func(args ...object.Object) object.Object {
				if len(args) != 0 {
					return newError("wrong number of arguments to `channel.receive`. got=%d, want=0", len(args))
				}
				return <-obj.Values
			}}
		default:
			return newError("unknown method %s on channel", member)
		}
	case *object.Hash:
		strKey := &object.String{Value: member}
		if pair, ok := obj.Pairs[strKey.HashKey()]; ok {
			return pair.Value
		}

		for _, pair := range obj.Pairs {
			if strObj, ok := pair.Key.(*object.String); ok {
				if strings.EqualFold(strObj.Value, member) {
					return pair.Value
				}
			}
		}

		return getHashMethod(obj, member)

	default:
		return runtimeError(object.CategoryTypeError, "property access .%s not supported on type %s", member, left.Type())
	}
}

func getHashMethod(hash *object.Hash, method string) object.Object {
	normMethod := strings.ToLower(method)
	switch normMethod {
	case "setnav":
		return &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				if len(args) > 0 {
					setHashKey(hash, "navigation", args[0])
				}
				return hash
			},
		}
	case "add":
		return &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				if len(args) > 0 {
					appendHashList(hash, "content", args[0])
				}
				return hash
			},
		}
	case "setfooter":
		return &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				if len(args) > 0 {
					setHashKey(hash, "footer", args[0])
				}
				return hash
			},
		}
	case "additem":
		return &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				if len(args) >= 2 {
					item := MakeHashObj(map[string]object.Object{
						"label": args[0],
						"path":  args[1],
					})
					appendHashList(hash, "items", item)
				}
				return hash
			},
		}
	case "addmetric":
		return &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				if len(args) >= 3 {
					metric := MakeHashObj(map[string]object.Object{
						"label": args[0],
						"value": args[1],
						"delta": args[2],
					})
					appendHashList(hash, "metrics", metric)
				}
				return hash
			},
		}
	case "addrow":
		return &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				row := &object.Array{Elements: args}
				appendHashList(hash, "rows", row)
				return hash
			},
		}
	case "addfield":
		return &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				if len(args) >= 3 {
					fld := MakeHashObj(map[string]object.Object{
						"label":       args[0],
						"name":        args[1],
						"placeholder": args[2],
					})
					appendHashList(hash, "fields", fld)
				}
				return hash
			},
		}
	default:
		return newError("unknown method %s on object", method)
	}
}

func setHashKey(hash *object.Hash, key string, val object.Object) {
	k := &object.String{Value: key}
	hash.SetPair(k, val)
}

func appendHashList(hash *object.Hash, listKey string, item object.Object) {
	k := &object.String{Value: listKey}
	hKey := k.HashKey()

	var arr *object.Array
	if pair, ok := hash.Pairs[hKey]; ok {
		if a, isArr := pair.Value.(*object.Array); isArr {
			arr = a
		}
	}

	if arr == nil {
		arr = &object.Array{Elements: []object.Object{}}
		hash.SetPair(k, arr)
	}

	arr.Elements = append(arr.Elements, item)
}

func MakeHashObj(m map[string]object.Object) *object.Hash {
	pairs := object.NewHash(len(m))
	for k, v := range m {
		pairs.SetPair(&object.String{Value: k}, v)
	}
	return pairs
}
