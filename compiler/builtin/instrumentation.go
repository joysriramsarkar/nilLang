// Package builtin classifies builtin and standard library functions by the
// capabilities they need.
//
// It exists so that the compiler, the type checker, and the runtime gate all
// derive their answers from one table rather than each keeping its own list.
// See compiler/capability for the capability names and the grant rule.
package builtin

import (
	"sort"

	compilercap "github.com/joysriramsarkar/nilLang/compiler/capability"
)

// Class describes what the compiler needs to know about a builtin in order to
// emit a call that the runtime can gate.
type Class struct {
	// Name is the builtin or std module function name.
	Name string

	// Requires lists the capabilities the operation needs. An empty list means
	// the operation is pure and always allowed.
	Requires compilercap.Requirement

	// Gated reports whether the call should go through a runtime capability
	// check. Pure operations are emitted as ordinary calls.
	Gated bool
}

// Classify returns the classification of a builtin, and whether it is known.
//
// A pure builtin is one whose requirement list is empty. Everything else is
// gated: a call to an ungated native operation is a hole in the sandbox.
func Classify(name string) (Class, bool) {
	req, known := compilercap.RequirementsFor(name)
	if !known {
		return Class{Name: name}, false
	}
	return Class{
		Name:     name,
		Requires: req,
		Gated:    len(req) > 0,
	}, known
}

// IsPure reports whether an operation needs no capabilities at all.
//
// Unknown operations are not pure. Treating them as pure would make a missing
// table entry indistinguishable from a genuinely harmless builtin.
func IsPure(name string) bool {
	class, known := Classify(name)
	return known && !class.Gated
}

// runtimeEffectful names builtins that perform an observable effect without
// going through the capability table.
//
// The table only lists operations that reach outside the language runtime.
// Concurrency, UI and terminal-control builtins are effectful too — the static
// checker marks them "concurrency", "ui" or "io" rather than "pure" — but
// they need no capability grant. Listing them here is what lets the runtime
// E0202 check agree with the static one for `pure fn` bodies that reach them.
var runtimeEffectful = map[string]bool{
	"withTimeout": true,
	"send":        true,
	"receive":     true,
	"Channel":     true,
	"emit":        true,
	"clear":       true,
}

// IsEffectful reports whether calling name performs an observable effect.
//
// A capability-gated operation is effectful by construction. Anything else is
// effectful when it appears in the runtime-effectful set above. Pure data and
// computation builtins (len, upper, tensor arithmetic, ...) and unknown names
// are not: an unknown name cannot be proven effectful, and refusing it would
// turn a missing table entry into a false effect violation.
func IsEffectful(name string) bool {
	if class, known := Classify(name); known {
		return class.Gated
	}
	return runtimeEffectful[name]
}

// KnownGated lists every operation in the table that the runtime must gate,
// sorted so that diagnostics and listings are stable.
func KnownGated() []Class {
	var out []Class
	for _, name := range compilercap.KnownOperations() {
		if class, ok := Classify(name); ok && class.Gated {
			out = append(out, class)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
