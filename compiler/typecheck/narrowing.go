package typecheck

import (
	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/types"
)

// Narrowing: `Ok`, `Err`, `unwrap`, and `unwrapOr`.
//
// # What it buys
//
// Registered on their own, these four builtins are `Any` in and `Any` out. That
// is not a small imprecision: it means `unwrap(r)` has no type, so every value it
// produces is unchecked from there on, and a program can read an error payload
// as if it were a success value without the checker noticing.
//
// Each of the four already *computes* the answer — the payload type of a
// `Result<T, E>`, the value type of a `?T` — and just does not say it. Nothing
// here adds a rule; it reports what the value already carries.
//
// # Why `Ok(x)` infers its own type
//
// `Ok(5)` is `Result<Int, Any>`, not `Result<Any, Any>`. Building the constructor
// from its argument is what makes a `let` binding downstream checkable: a value
// stored as `Result<Int, Any>` rejects `Err("boom")` where one stored as `Any`
// would not.
//
// The error side stays `Any` because a type checker that has not read every `Err`
// in a program cannot know the narrowest error type, and guessing one would
// reject correct programs.

// CodeNotNarrowable is reported when a narrowing builtin is applied to a value
// that carries nothing to narrow.
const CodeNotNarrowable = "E0112"

// narrowingMessage explains why a narrowing call cannot narrow, naming the type
// so the reader does not have to go and look up what `Any` meant in context.
func narrowingMessage(name string, argument types.Type) string {
	switch name {
	case "unwrap":
		return "unwrap expects a Result or an optional, got " + argument.String()
	case "unwrapOr":
		return "unwrapOr expects a Result or an optional, got " + argument.String()
	}
	return name + " cannot narrow " + argument.String()
}

// narrowingResult returns the specific type a narrowing builtin produces, or nil
// when the call is not one and the registered signature stands.
func (c *Checker) narrowingResult(name string, argTypes []types.Type, registered types.Type) types.Type {
	switch name {
	case "unwrap":
		if len(argTypes) == 0 {
			return registered
		}
		if payload := payloadOf(argTypes[0]); payload != nil {
			return payload
		}
		return registered

	case "unwrapOr":
		if len(argTypes) == 0 {
			return registered
		}
		if payload := payloadOf(argTypes[0]); payload != nil && !payload.Equals(types.Any) {
			return payload
		}
		// Nothing to unwrap to, so the fallback is the answer. That is what makes
		// `x?.name ?? "-"` and `unwrapOr(x, "-")` agree about their result.
		if len(argTypes) > 1 && argTypes[1] != nil {
			return argTypes[1]
		}
		return registered

	case "Some", "None":
		// `Some(5)` is `?Int`. Without this it is `?Any`, and a binding declared
		// `?Int` rejects it — which is what made `let r: ?Int = Some(5)` fail
		// while `unwrap(r)` worked fine.
		var payload types.Type = types.Any
		if len(argTypes) > 0 && argTypes[0] != nil {
			payload = argTypes[0]
		}
		return &types.OptionalType{Base: payload}

	case "Ok", "Err":
		var payload types.Type = types.Any
		if len(argTypes) > 0 && argTypes[0] != nil {
			payload = argTypes[0]
		}
		return &types.GenericType{Base: "Result", Parameters: []types.Type{payload, types.Any}}
	}

	return registered
}

// payloadOf is the value a Result or an Optional carries, or `Any` when the
// payload is not known.
//
// A `Result<T, E>` carries `T` on the success side and a `?T` carries `T` — the
// same answer in both cases, because in both the *unwrapped* value is the one a
// caller wants. That is why `Some` and `Ok` take exactly one payload type between
// them in the type system at all.
//
// It always answers `Any` for a type that is a Result or an Optional. `None` is
// `?Any`: nothing to unwrap *to*, but still an optional, and refusing `unwrap`
// on it would reject `unwrap(None)` — which is precisely the call whose run-time
// failure a test wants to observe.
func payloadOf(t types.Type) types.Type {
	switch v := t.(type) {
	case *types.ResultType:
		if v.Ok != nil {
			return v.Ok
		}
		return types.Any
	case *types.OptionalType:
		if v.Base != nil {
			return v.Base
		}
		return types.Any
	case *types.GenericType:
		if len(v.Parameters) == 0 || v.Parameters[0] == nil {
			return types.Any
		}
		switch v.Base {
		case "Optional", "Result":
			return v.Parameters[0]
		}
	}
	return nil
}

// isNarrowable reports whether a value is a Result or an Optional at all.
//
// It is a separate question from knowing the payload, and conflating the two is
// what made `unwrap(None)` a compile error: `None` carries nothing to name, which
// is not the same as being something other than an optional.
func isNarrowable(t types.Type) bool {
	return payloadOf(t) != nil
}

// isUnknown reports whether a value's static type is unknown (`Any`).
//
// `Any` is what every unannotated function returns — including every standard
// library helper — so refusing `unwrap` on it made `unwrap(money.add(a, b))`
// a compile error and the language's own documented patterns unusable under
// `nil run`. `Any` may hold a Result at run time, and the run-time unwrap
// already fails loudly on anything else, so the static gate stands aside and
// lets the dynamic one decide. Only a type that is *definitely* not a Result
// or an Optional (an `Int` literal, a `String`) is still rejected.
func isUnknown(t types.Type) bool {
	if t == nil {
		return true
	}
	return t.Kind() == types.KindAny
}

// applyNarrowing rewrites a builtin call's result type, and reports when the call
// is a narrowing one applied to something that cannot be narrowed.
//
// The second half is what stops `unwrap(5)` from quietly producing `Any` for the
// rest of the program. A builtin that cannot fail is being asked to, which is a
// mistake worth naming — the same mistake as calling `length` on an integer.
func (c *Checker) applyNarrowing(e *ast.CallExpression, fnSig *types.FunctionType) *types.FunctionType {
	ident, ok := e.Function.(*ast.Identifier)
	if !ok || fnSig == nil {
		return fnSig
	}

	switch ident.Value {
	case "unwrap", "unwrapOr", "Ok", "Err", "Some", "None":
	default:
		return fnSig
	}

	argTypes := make([]types.Type, 0, len(e.Arguments))
	for _, arg := range e.Arguments {
		argTypes = append(argTypes, c.inferExpression(arg))
	}

	// The arguments were already inferred once by the caller; inferring them again
	// here is cheap and keeps this function independent of where it is called from.
	if ident.Value == "unwrap" || ident.Value == "unwrapOr" {
		if len(argTypes) == 0 {
			return fnSig
		}
		if !isNarrowable(argTypes[0]) && !isUnknown(argTypes[0]) {
			c.report(CodeNotNarrowable,
				narrowingMessage(ident.Value, argTypes[0]),
				e.Token.Line, e.Token.Column)
			return fnSig
		}
	}

	adjusted := *fnSig
	adjusted.ReturnType = c.narrowingResult(ident.Value, argTypes, fnSig.ReturnType)
	return &adjusted
}
