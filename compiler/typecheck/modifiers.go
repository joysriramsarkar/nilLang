package typecheck

import (
	"fmt"
	"sort"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/capability"
	"github.com/joysriramsarkar/nilLang/compiler/types"
)

// Function modifiers, the `requires` clause, and the capability `unsafe` buys.
//
// # What each one means here
//
// `pure fn` is a claim the developer makes and the checker verifies as far as it
// can: a call to an effectful function inside one is E0202. It cannot prove the
// claim, which is why it is written rather than inferred.
//
// `unsafe fn` says the function performs capability-gated operations and takes
// responsibility for them. Inside it, a gated operation does not need its
// capability granted to the project. The exemption is per function and has to be
// written down, so a reader can see where it applies — and the project must grant
// `unsafe` itself, so it cannot be had by accident.
//
// `requires "cap"` says the function cannot do its job without that capability.
// It is checked at every call site, not at run time, because a function whose
// capability is missing fails the moment it is called; that is knowable before
// the program starts.
//
// # Why the capability is normalised
//
// Nilang has two vocabularies for the same capability: a lowercase
// operation-oriented name (`filesystem`) and a capitalised resource-oriented one
// (`Filesystem`, from the host side). `capability.NormalizeName` reconciles them,
// and using it here means a project that granted `Filesystem` satisfies a clause
// that says `filesystem`. A clause written in the wrong vocabulary is otherwise a
// failure that looks like a missing grant.

// UnsafeCapability is the capability a project must hold before any `unsafe fn`
// is allowed to exist.
const UnsafeCapability = "unsafe"

// CodeRequiresUndeclaredCapability is reported when a call site's callee requires
// a capability the project has not been granted.
//
// It shares E0201 with the run-time capability denial deliberately: both say the
// same thing — an operation needs a capability the project does not have — and
// they are the same mistake at two different times.
const CodeRequiresUndeclaredCapability = capability.Code

// functionEffects renders a function's declared effects.
//
// A `pure fn` is pure by definition, which is what makes the E0202 check above
// able to skip it. Anything else is recorded with the modifiers that produced it,
// so a diagnostic can name why rather than only that the call is impure.
func functionEffects(fn *ast.FunctionLiteral) []string {
	if fn == nil {
		return nil
	}
	switch {
	case fn.IsPure:
		return []string{"pure"}
	case fn.IsUnsafe:
		return []string{"unsafe"}
	default:
		return []string{"effectful"}
	}
}

// checkRequiresClause verifies that the project holds what the callee needs.
func (c *Checker) checkRequiresClause(fn *types.FunctionType, call *ast.CallExpression) {
	if fn == nil || len(fn.Requires) == 0 {
		return
	}

	// `unsafe fn` is itself a gated declaration: the project has to have opted
	// in before any function may take the exemption.
	if fn.IsUnsafe && !c.HasCapability(UnsafeCapability) {
		c.report(CodeRequiresUndeclaredCapability,
			fmt.Sprintf("calling an `unsafe fn` requires the %q capability, which the project has not been granted",
				UnsafeCapability),
			call.Token.Line, call.Token.Column)
	}

	// An unsafe function declares that it copes with the capabilities it uses, so
	// its clause is not checked against the project.
	if fn.IsUnsafe {
		return
	}

	var missing []string
	for _, required := range fn.Requires {
		if !c.HasCapability(required) {
			missing = append(missing, required)
		}
	}
	if len(missing) == 0 {
		return
	}

	sort.Strings(missing)
	c.report(CodeRequiresUndeclaredCapability,
		fmt.Sprintf("this call needs the %s, which the project has not been granted",
			capabilityList(missing)),
		call.Token.Line, call.Token.Column)
}

// capabilityList renders a capability list so it reads as English inside a
// sentence: "the filesystem capability" for one, "the filesystem and network
// capabilities" for two, and a comma-separated list beyond that.
//
// It exists because the obvious version — counting and calling it "capability"
// or "capabilities" — produces "the 1 capability filesystem", which reads like a
// machine that gave up halfway.
func capabilityList(names []string) string {
	switch len(names) {
	case 0:
		return "capability"
	case 1:
		return names[0] + " capability"
	case 2:
		return names[0] + " and " + names[1] + " capabilities"
	default:
		return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1] + " capabilities"
	}
}

// checkFunctionRequires verifies a function's own clause at its declaration.
//
// A declaration is checked as well as each call because a function nobody calls
// still declares an intention, and a typo in a capability name would otherwise
// sit unnoticed until the day something called it.
func (c *Checker) checkFunctionRequires(fn *ast.FunctionLiteral) {
	if fn == nil {
		return
	}

	// The unsafe exemption is gated on its own, not on the presence of a
	// clause: unsafe fn f() { } takes the exemption just as much as one that
	// also happens to name a capability.
	if fn.IsUnsafe && !c.HasCapability(UnsafeCapability) {
		c.report(CodeRequiresUndeclaredCapability,
			fmt.Sprintf("`unsafe fn %s` requires the %q capability, which the project has not been granted",
				functionName(fn.Name), UnsafeCapability),
			fn.Token.Line, fn.Token.Column)
		return
	}

	for _, required := range fn.Requires {
		if c.HasCapability(required) {
			continue
		}
		c.report(CodeRequiresUndeclaredCapability,
			fmt.Sprintf("%s requires the %q capability, which the project has not been granted",
				describeFunction(fn.Name), required),
			fn.Token.Line, fn.Token.Column)
	}
}

// checkModifierUsage rejects a combination that says two contradictory things.
func (c *Checker) checkModifierUsage(fn *ast.FunctionLiteral) {
	if fn == nil {
		return
	}

	if fn.IsPure && len(fn.Requires) > 0 {
		c.report("E0202",
			fmt.Sprintf("%s is declared `pure` but requires a capability, which is a side effect",
				describeFunction(fn.Name)),
			fn.Token.Line, fn.Token.Column)
	}
}

// functionName names a function without the "function" prefix, for a message
// that already supplies its own.
func functionName(name string) string {
	if name == "" {
		return "a function literal"
	}
	return name
}

// CapabilityIsKnown reports whether a name is one the capability system knows.
//
// It is separate from HasCapability because a typo and a deliberate omission are
// different mistakes: the first should be reported, the second is sometimes the
// point of a build that does not use the feature yet.
func CapabilityIsKnown(name string) bool {
	return capability.IsKnown(capability.NormalizeName(name))
}
