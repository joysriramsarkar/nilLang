package ast

import (
	"bytes"

	"github.com/joysriramsarkar/nilLang/compiler/token"
)

// Module visibility.
//
// # The migration rule
//
// A declaration is exported when it is marked `export`. A file that contains no
// `export` at all exports everything, which is what every module did before
// visibility existed.
//
// That rule is not the one a language would choose from the start, and it is worth
// being explicit about why it is the one chosen here: the alternative — an
// unmarked declaration is private — breaks every module that has ever been
// written, all of which predate the keyword. A module opts into strict
// visibility by writing a single `export`; from that point on, anything not
// marked is private. The rule is observable in two directions and is tested in
// both, so it cannot drift into something subtler by accident.

// Visibility is a declaration's module visibility.
type Visibility string

const (
	// VisibilityUnspecified means the declaration said nothing. Whether that
	// exports depends on whether the file uses `export` anywhere.
	VisibilityUnspecified Visibility = ""

	// VisibilityExported means the declaration is part of the module's interface.
	VisibilityExported Visibility = "export"

	// VisibilityPrivate means the declaration is internal, even in a file that
	// uses `export` elsewhere.
	VisibilityPrivate Visibility = "private"
)

// ExportStatement is a declaration with its visibility written on it.
//
// It wraps the declaration rather than adding a field to each declaration kind:
// visibility is one property with one meaning, and a wrapper keeps it from
// becoming six fields that have to be kept in step.
type ExportStatement struct {
	Token      token.Token // token.EXPORT or token.PRIVATE
	Visibility Visibility
	Decl       Statement
}

func (es *ExportStatement) statementNode()       {}
func (es *ExportStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExportStatement) String() string {
	var out bytes.Buffer
	out.WriteString(string(es.Visibility) + " ")
	if es.Decl != nil {
		out.WriteString(es.Decl.String())
	}
	return out.String()
}

// ExportVisibility returns the marker a statement carries, or
// VisibilityUnspecified when it carries none.
//
// It exists so that a consumer does not have to type-assert to look, and so that
// the typed-nil case is handled in one place.
func ExportVisibility(stmt Statement) Visibility {
	if wrapped, ok := stmt.(*ExportStatement); ok && wrapped != nil {
		return wrapped.Visibility
	}
	return VisibilityUnspecified
}

// Unwrap returns the declaration an ExportStatement wraps, or the statement
// itself when it is not wrapped.
//
// Every consumer that wants the declaration rather than the visibility uses this,
// so that a wrapped declaration behaves exactly like an unwrapped one and there
// is only one place that knows about the wrapper.
func Unwrap(stmt Statement) Statement {
	if wrapped, ok := stmt.(*ExportStatement); ok && wrapped != nil && wrapped.Decl != nil {
		return wrapped.Decl
	}
	return stmt
}

// DeclaredName returns the name a statement declares, or "" if it declares none.
//
// A `fn` is named by its literal, everything else by the identifier it binds. A
// declaration that binds nothing — an entity, a component — has no name to export
// and is reported as such by the caller rather than guessed at.
//
// The nil checks are against the concrete pointer, not the interface. A parser
// that fails partway returns a typed nil (`(*LetStatement)(nil)` stored in a
// `Statement`), which is not `== nil` as an interface: checking the interface
// would let that through and the next field access would panic. The parser is
// asked about declarations that failed to parse often enough that this is the
// normal case here, not a corner.
func DeclaredName(stmt Statement) string {
	switch node := Unwrap(stmt).(type) {
	case *LetStatement:
		if node != nil && node.Name != nil {
			return node.Name.Value
		}
	case *StructDecl:
		if node != nil && node.Name != nil {
			return node.Name.Value
		}
	case *EnumDecl:
		if node != nil && node.Name != nil {
			return node.Name.Value
		}
	case *TypeAliasDecl:
		if node != nil && node.Name != nil {
			return node.Name.Value
		}
	case *ExpressionStatement:
		// `export fn f() { }` reaches here as an expression statement when the
		// declaration is a function literal, which is how a function declaration
		// is written.
		if node == nil {
			return ""
		}
		if fn, ok := node.Expression.(*FunctionLiteral); ok && fn != nil {
			return fn.Name
		}
	}
	return ""
}

// ExportedNames reports which of a program's top-level declarations leave the
// module, and whether the file uses `export` at all.
//
// The second value is the whole migration rule: without it a caller could not
// tell "this module exports nothing" from "this module predates visibility".
func ExportedNames(program *Program) (map[string]bool, bool) {
	exported := make(map[string]bool)
	if program == nil {
		return exported, false
	}

	usesExport := false
	for _, stmt := range program.Statements {
		if _, ok := stmt.(*ExportStatement); ok {
			usesExport = true
			break
		}
	}

	if !usesExport {
		// A file that says nothing about visibility exports everything.
		for _, stmt := range program.Statements {
			if name := DeclaredName(stmt); name != "" {
				exported[name] = true
			}
		}
		return exported, false
	}

	for _, stmt := range program.Statements {
		wrapped, ok := stmt.(*ExportStatement)
		if !ok {
			// Unmarked in a file that does use `export`: private.
			continue
		}
		if wrapped.Visibility != VisibilityExported {
			continue
		}
		if name := DeclaredName(wrapped.Decl); name != "" {
			exported[name] = true
		}
	}
	return exported, true
}
