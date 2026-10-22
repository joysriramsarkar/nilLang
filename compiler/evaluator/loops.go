package evaluator

import (
	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"math"
)

// Loop control signals.
//
// `break` and `continue` unwind through the evaluator's ordinary value channel,
// the same way `return` already does: a statement evaluates to a marker object
// and each enclosing loop checks for it. That keeps the evaluator free of Go
// exceptions for what is, in the target semantics, ordinary control flow.
//
// The signals are unexported and have no bytecode counterpart, because the VM
// compiles a loop into real branches instead.
var (
	BREAK_SIGNAL    = object.NewLoopSignal(object.BREAK_SIGNAL_OBJ)
	CONTINUE_SIGNAL = object.NewLoopSignal(object.CONTINUE_SIGNAL_OBJ)
)

// evalForStatement evaluates `for key, value in iterable { body }`.
//
// The iterable is materialised once, before the loop starts, so that mutating
// the underlying collection inside the body cannot change the iteration. A
// `for` loop that silently skipped or repeated elements because the body
// appended to the array being walked would be a very hard bug to see.
func evalForStatement(fs *ast.ForStatement, env *object.Environment) object.Object {
	iterable := Eval(fs.Iterable, env)
	if isError(iterable) {
		return iterable
	}

	var items []loopItem

	// A range is expanded lazily rather than materialised, so
	// or i in 0..1000000000 does not allocate a billion element array.
	if rangeExpr, ok := fs.Iterable.(*ast.RangeExpression); ok {
		return evalRangeFor(fs, rangeExpr, env)
	}

	switch collection := iterable.(type) {
	case *object.Array:
		for index, element := range collection.Elements {
			items = append(items, loopItem{
				Key:   &object.Integer{Value: int64(index)},
				Value: element,
			})
		}

	case *object.String:
		// Iterating a string yields its characters, counted in runes so that
		// Bengali text advances by grapheme-relevant unit rather than by byte.
		for index, r := range []rune(collection.Value) {
			items = append(items, loopItem{
				Key:   &object.Integer{Value: int64(index)},
				Value: &object.String{Value: string(r)},
			})
		}

	case *object.Hash:
		// Hash iteration follows insertion order, not the map's own order, so a
		// `for k, v in hash` is reproducible run to run.
		for _, pair := range collection.OrderedPairs() {
			key, ok := pair.Key.(*object.String)
			if !ok {
				key = &object.String{Value: pair.Key.Inspect()}
			}
			items = append(items, loopItem{Key: key, Value: pair.Value})
		}

	default:
		if object.IsNull(iterable) {
			// Iterating null yields nothing, matching the "no elements" idea
			// rather than failing. An optional that is absent is an empty walk.
			return NULL
		}
		return runtimeError(object.CategoryTypeError,
			"cannot iterate over %s", describeIterableType(iterable))
	}

	return runFor(fs, items, env)
}

// loopItem is one iteration's bound key and value.
type loopItem struct {
	Key   object.Object
	Value object.Object
}

// evalRangeFor expands an integer range into iteration items.
//
// A range is not materialised into an array: a `for i in 0..1000000000` would
// allocate a billion element array. The items are produced lazily by runRange.
func evalRangeFor(fs *ast.ForStatement, re *ast.RangeExpression, env *object.Environment) object.Object {
	start := Eval(re.Start, env)
	if isError(start) {
		return start
	}
	end := Eval(re.End, env)
	if isError(end) {
		return end
	}

	startInt, ok := start.(*object.Integer)
	if !ok {
		return runtimeError(object.CategoryTypeError,
			"range start must be an integer, got %s", start.Type())
	}
	endInt, ok := end.(*object.Integer)
	if !ok {
		return runtimeError(object.CategoryTypeError,
			"range end must be an integer, got %s", end.Type())
	}

	return runRange(fs, startInt.Value, endInt.Value, re.Inclusive, env)
}

// runFor walks precomputed items, binding the loop variables in a fresh scope
// for each iteration.
func runFor(fs *ast.ForStatement, items []loopItem, env *object.Environment) object.Object {
	var result object.Object = NULL

	for _, item := range items {
		iterationEnv := object.NewEnclosedEnvironment(env)
		if fs.Key != nil {
			iterationEnv.Set(fs.Key.Value, item.Key)
		}
		if fs.Value != nil {
			iterationEnv.Set(fs.Value.Value, item.Value)
		}

		evaluated := Eval(fs.Body, iterationEnv)
		if evaluated != nil {
			switch evaluated.Type() {
			case object.BREAK_SIGNAL_OBJ:
				return NULL
			case object.CONTINUE_SIGNAL_OBJ:
				continue
			case object.RETURN_VALUE_OBJ, object.ERROR_OBJ:
				return evaluated
			}
			result = evaluated
		}
	}

	return result
}

// runRange walks an integer range without materialising it.
//
// The direction is inferred from the bounds: a descending range counts down, so
// `for i in 10..0` is a natural countdown rather than an error.
func runRange(fs *ast.ForStatement, start, end int64, inclusive bool, env *object.Environment) object.Object {
	var result object.Object = NULL
	descending := end < start
	if descending {
		if start-end < 0 || (inclusive && start-end == math.MaxInt64) {
			return runtimeError(object.CategoryTypeError, "range is too large to iterate")
		}
	} else if end-start < 0 || (inclusive && end-start == math.MaxInt64) {
		return runtimeError(object.CategoryTypeError, "range is too large to iterate")
	}

	for value := start; ; {
		if (!descending && (value > end || (!inclusive && value == end))) ||
			(descending && (value < end || (!inclusive && value == end))) {
			break
		}

		iterationEnv := object.NewEnclosedEnvironment(env)
		if fs.Key != nil {
			iterationEnv.Set(fs.Key.Value, &object.Integer{Value: value})
		}
		if fs.Value != nil && fs.Value.Value != fs.Key.Value {
			iterationEnv.Set(fs.Value.Value, &object.Integer{Value: value})
		}

		evaluated := Eval(fs.Body, iterationEnv)
		if evaluated != nil {
			switch evaluated.Type() {
			case object.BREAK_SIGNAL_OBJ:
				return NULL
			case object.CONTINUE_SIGNAL_OBJ:
				continue
			case object.RETURN_VALUE_OBJ, object.ERROR_OBJ:
				return evaluated
			}
			result = evaluated
		}
		if value == end {
			break
		}
		if descending {
			value--
		} else {
			value++
		}
	}

	return result
}

// isLoopSignal reports whether obj is a loop control signal.
func isLoopSignal(obj object.Object) bool {
	if obj == nil {
		return false
	}
	t := obj.Type()
	return t == object.BREAK_SIGNAL_OBJ || t == object.CONTINUE_SIGNAL_OBJ
}

// describeIterableType names a type in a message a developer can act on.
func describeIterableType(obj object.Object) string {
	if obj == nil {
		return "null"
	}
	name := obj.Type()
	if name == object.HASH_OBJ {
		return "hash (iterate with `for key, value in ...`)"
	}
	return string(name)
}
