package typecheck

import (
	"github.com/joysriramsarkar/nilLang/compiler/types"
)

// Generic functions: type parameters, and what a call site works out from them.
//
// # What is implemented
//
// A function may declare type parameters (`fn map<T, U>(...)`). Inside its
// declaration every mention of `T` is a `types.TypeVar`, so the body is checked
// with a real type rather than skipped. At a call site the checker matches each
// parameter type against the argument type and binds the variables it can —
// `Array<T>` against `List<Int>` gives `T = Int` — then checks the argument
// against the *substituted* signature and reports the substituted return type.
//
// # What is not
//
// Inference is structural, not unification. It sees through a generic type, an
// optional, and a union; it does not reason through a function type's parameters
// (`fn(T) -> T` against a concrete function), and a variable it cannot pin down
// stays `Any` rather than becoming an error.
//
// That is a deliberate stopping point rather than a partial unification engine.
// It covers the shape generic functions are written for — "same type in, same type
// out" over a container — and a variable it cannot pin down is genuinely not
// knowable here: the caller may be in a module this checker has not read.
//
// # Why it is erased
//
// There is no monomorphisation. `map` is compiled once, and a call site passes
// ordinary values. That keeps both engines untouched — they already call
// functions, so they cannot disagree about what a generic one does — and it costs
// nothing at run time.
//
// The cost is that a call site which cannot pin a variable down is not told so.
// `let xs: Array<Int> = maybeGeneric([])` is accepted because `maybeGeneric`
// returns `Array<Any>`. Tightening that needs the return type to be inferred from
// the body, which is a much larger piece of work.

// CodeGenericMismatch is reported when a call cannot satisfy a generic
// signature's structure — an integer where `Array<T>` is expected, most often.
//
// It is separate from `E0101` because the two are reached by different reasoning:
// `E0101` compares two concrete types, while this compares an argument against a
// signature the checker has just instantiated. Sharing a code would hide which
// of the two a developer is looking at.
const CodeGenericMismatch = "E0204"

// typeVarSet is the type parameters in force while a generic function's body is
// checked.
//
// A set rather than a list because a nested function's parameters must not shadow
// by accident and must not leak: both are handled by replacing the set for the
// body's duration and putting the previous one back.
type typeVarSet map[string]bool

// withTypeParams pushes a function's type parameters for the duration of its
// body, and returns the set that was in force.
func (c *Checker) withTypeParams(params []string) typeVarSet {
	previous := c.typeVars

	next := make(typeVarSet, len(params))
	for name := range previous {
		next[name] = true
	}
	for _, name := range params {
		next[name] = true
	}
	c.typeVars = next

	return previous
}

// isTypeParam reports whether a name is a type parameter in force here.
func (c *Checker) isTypeParam(name string) bool {
	return c.typeVars[name]
}

// inferCall works out a generic function's type arguments from a call's
// arguments, and returns the instantiated signature.
//
// The order matters: binding first, then substituting. Checking the argument
// against the *unbound* signature would accept anything a variable appears in,
// which is precisely the mistake that makes a generic function unchecked.
func (c *Checker) inferCall(sig *types.FunctionType, argTypes []types.Type) *types.FunctionType {
	if sig == nil || !types.HasTypeVar(sig.ReturnType) {
		if sig == nil {
			return nil
		}
		if !hasTypeVarIn(sig.Params) {
			return sig
		}
	}

	bindings := make(map[string]types.Type)
	for i, paramType := range sig.Params {
		if i >= len(argTypes) || paramType == nil {
			continue
		}
		unify(paramType, argTypes[i], bindings)
	}

	if len(bindings) == 0 {
		return sig
	}

	instantiated := &types.FunctionType{
		Params:     substituteAll(sig.Params, bindings),
		ReturnType: types.SubstituteTypeVars(sig.ReturnType, bindings),
		Effects:    sig.Effects,
		MinArgs:    sig.MinArgs,
		MaxArgs:    sig.MaxArgs,
	}
	return instantiated
}

// hasTypeVarIn reports whether any of the given types mentions a type variable.
func hasTypeVarIn(list []types.Type) bool {
	for _, t := range list {
		if types.HasTypeVar(t) {
			return true
		}
	}
	return false
}

func substituteAll(list []types.Type, bindings map[string]types.Type) []types.Type {
	if len(list) == 0 || len(bindings) == 0 {
		return list
	}
	out := make([]types.Type, 0, len(list))
	for _, t := range list {
		out = append(out, types.SubstituteTypeVars(t, bindings))
	}
	return out
}

// unify matches a declared parameter type against an actual one, binding type
// variables as it goes.
//
// It is structural rather than a full unification algorithm: it looks through a
// generic type, an optional, and a union, and it does not descend into a function
// type's parameter list. Where it cannot decide it binds nothing, and the
// variable stays `Any` — which is honest, because a value the checker has not
// read can be anything.
func unify(pattern types.Type, actual types.Type, bindings map[string]types.Type) {
	if pattern == nil || actual == nil || actual.Equals(types.Any) {
		return
	}

	switch p := pattern.(type) {
	case *types.TypeVar:
		if existing, bound := bindings[p.Name]; bound {
			// Already pinned down. A second, different answer is not an error here:
			// the caller reports the mismatch against the instantiated signature,
			// which is the place that can name both types.
			if !existing.Equals(actual) {
				return
			}
			return
		}
		bindings[p.Name] = actual
		return

	case *types.GenericType:
		a, ok := actual.(*types.GenericType)
		if !ok || len(a.Parameters) != len(p.Parameters) {
			return
		}
		for i := range p.Parameters {
			unify(p.Parameters[i], a.Parameters[i], bindings)
		}
		return

	case *types.OptionalType:
		if inner, ok := unwrapOptional(actual); ok {
			unify(p.Base, inner, bindings)
		}
		return

	case *types.UnionType:
		for _, member := range p.Types {
			if member.AssignableTo(actual) {
				unify(member, actual, bindings)
				return
			}
		}
		return
	}
}

// unwrapOptional returns the value an Optional holds, when the type is one.
func unwrapOptional(t types.Type) (types.Type, bool) {
	if opt, ok := t.(*types.OptionalType); ok {
		return opt.Base, true
	}
	if generic, ok := t.(*types.GenericType); ok && len(generic.Parameters) > 0 {
		// `?T` and a generic named Optional are the same shape; both are handled
		// by `match` and both should infer the same way.
		if generic.Base == "Optional" {
			return generic.Parameters[0], true
		}
	}
	return nil, false
}
