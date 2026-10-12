package ast

import (
	"bytes"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/token"
)

// ─── DECLARATIVE TYPES ───────────────────────────────────────────────────────
//
// struct, enum, and match are the language's algebraic data types. They are
// declarations rather than expressions, because a type name is in scope for the
// whole file and because their shape is what the type checker needs before any
// expression is checked.

// StructDecl is `struct Name { field: Type, ... }`.
//
// Fields keep their declaration order: it is the order a literal is built in, the
// order `Inspect` prints, and the order a host serialising the value sees.
type StructDecl struct {
	Token  token.Token // token.STRUCT
	Name   *Identifier
	Fields []StructFieldDecl
}

// StructFieldDecl is one field of a struct declaration.
//
// TypeAnnotation is the written type, kept as text because a type is resolved
// against the file's scope rather than evaluated.
type StructFieldDecl struct {
	Token          token.Token
	Name           *Identifier
	TypeAnnotation string
}

func (sd *StructDecl) statementNode()       {}
func (sd *StructDecl) TokenLiteral() string { return sd.Token.Literal }
func (sd *StructDecl) String() string {
	var out bytes.Buffer
	out.WriteString("struct " + sd.Name.Value + " { ")
	parts := make([]string, 0, len(sd.Fields))
	for _, f := range sd.Fields {
		parts = append(parts, f.String())
	}
	out.WriteString(strings.Join(parts, ", "))
	out.WriteString(" }")
	return out.String()
}

func (f StructFieldDecl) String() string {
	return f.Name.Value + ": " + f.TypeAnnotation
}

// StructLiteral is `Name { field: value, ... }`.
//
// It is distinct from a HashLiteral because the name selects a declared struct:
// the type checker verifies the fields against that declaration, and an unknown
// struct name is an error rather than a hash with an extra key.
type StructLiteral struct {
	Token  token.Token
	Name   *Identifier
	Fields []StructFieldExpr
}

// StructFieldExpr is one `field: value` entry of a struct literal.
type StructFieldExpr struct {
	Token token.Token
	Name  *Identifier
	Value Expression
}

func (sl *StructLiteral) expressionNode()      {}
func (sl *StructLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StructLiteral) String() string {
	parts := make([]string, 0, len(sl.Fields))
	for _, f := range sl.Fields {
		parts = append(parts, f.Name.Value+": "+f.Value.String())
	}
	return sl.Name.Value + " { " + strings.Join(parts, ", ") + " }"
}

// EnumDecl is `enum Name { Variant, Variant(Int), ... }`.
//
// A variant with a payload carries one value, which is enough for the shapes a
// result type actually takes (Success(T), Failure(E)) without needing a general
// algebraic data type.
type EnumDecl struct {
	Token    token.Token // token.ENUM
	Name     *Identifier
	Variants []EnumVariantDecl
}

// EnumVariantDecl is one variant of an enum.
//
// PayloadType is empty for a payload-free variant.
type EnumVariantDecl struct {
	Token       token.Token
	Name        *Identifier
	PayloadType string
}

// HasPayload reports whether the variant carries a value.
func (v EnumVariantDecl) HasPayload() bool { return v.PayloadType != "" }

func (ed *EnumDecl) statementNode()       {}
func (ed *EnumDecl) TokenLiteral() string { return ed.Token.Literal }
func (ed *EnumDecl) String() string {
	parts := make([]string, 0, len(ed.Variants))
	for _, v := range ed.Variants {
		if v.HasPayload() {
			parts = append(parts, v.Name.Value+"("+v.PayloadType+")")
			continue
		}
		parts = append(parts, v.Name.Value)
	}
	return "enum " + ed.Name.Value + " { " + strings.Join(parts, ", ") + " }"
}

// EnumVariantExpr is `Name` or `Name(payload)`, producing an enum value.
type EnumVariantExpr struct {
	Token token.Token
	Name  *Identifier

	// Payload is nil for a payload-free variant.
	Payload Expression
}

func (ev *EnumVariantExpr) expressionNode()      {}
func (ev *EnumVariantExpr) TokenLiteral() string { return ev.Token.Literal }
func (ev *EnumVariantExpr) String() string {
	if ev.Payload == nil {
		return ev.Name.Value
	}
	return ev.Name.Value + "(" + ev.Payload.String() + ")"
}

// MatchExpression is `match subject { pattern => expression, ... }`.
//
// It is an expression rather than a statement because every arm produces a value
// and `let x = match ...;` is the common shape. A match used as a statement
// discards that value.
type MatchExpression struct {
	Token   token.Token
	Subject Expression
	Arms    []MatchArm
}

// MatchArm is `pattern => expression`, optionally guarded with `if`.
//
// The guard exists because some cases cannot be expressed as a pattern without
// either a nested match or a boolean field on the enum, and nesting a match
// inside a match arm is the same thing written twice.
type MatchArm struct {
	Token   token.Token
	Pattern MatchPattern
	Guard   Expression
	Body    Expression
}

func (me *MatchExpression) expressionNode()      {}
func (me *MatchExpression) TokenLiteral() string { return me.Token.Literal }
func (me *MatchExpression) String() string {
	parts := make([]string, 0, len(me.Arms))
	for _, arm := range me.Arms {
		parts = append(parts, arm.String())
	}
	return "match " + me.Subject.String() + " { " + strings.Join(parts, ", ") + " }"
}

func (a MatchArm) String() string {
	body := a.Pattern.String() + " => " + a.Body.String()
	if a.Guard != nil {
		return a.Pattern.String() + " if " + a.Guard.String() + " => " + a.Body.String()
	}
	return body
}

// MatchPattern is one alternative in a match arm.
//
// The concrete forms are a wildcard, a literal, a binding, an enum variant
// (optionally binding its payload), a struct pattern, and a list of those
// separated by `|`. Exactly one of the fields is set, which keeps "which kind of
// pattern is this" a single comparison rather than a set of flags that could
// disagree.
type MatchPattern struct {
	Token token.Token

	// Wildcard is set for `_`.
	Wildcard bool

	// Literal matches a constant value.
	Literal Expression

	// Binding names the subject without inspecting it. Written `x`, which
	// cannot be confused with a bare literal pattern because a bare identifier
	// pattern always binds.
	Binding *Identifier

	// EnumVariant matches one enum variant, optionally binding its payload.
	EnumVariant *MatchEnumPattern

	// StructPattern matches a struct value with named or `_` fields.
	StructPattern *MatchStructPattern

	// Alternatives holds the `|` separated forms; at least one entry.
	Alternatives []MatchPattern
}

// MatchEnumPattern is `Variant` or `Variant(binding)` in a pattern.
type MatchEnumPattern struct {
	Variant *Identifier

	// PayloadBinding names the variant's payload. It is nil when the payload is
	// ignored, and a wildcard when it is discarded explicitly.
	PayloadBinding *Identifier
	DiscardPayload bool
}

// MatchStructPattern is `Name { field: pattern, ... }` in a pattern.
type MatchStructPattern struct {
	Name   *Identifier
	Fields []MatchStructFieldPattern
}

// MatchStructFieldPattern binds one field of a struct pattern.
type MatchStructFieldPattern struct {
	Name    *Identifier
	Pattern MatchPattern
}

func (p MatchPattern) String() string {
	switch {
	case p.Wildcard:
		return "_"
	case p.Binding != nil:
		return p.Binding.Value
	case p.Literal != nil:
		return p.Literal.String()
	case p.EnumVariant != nil:
		out := p.EnumVariant.Variant.Value
		switch {
		case p.EnumVariant.PayloadBinding != nil:
			out += "(" + p.EnumVariant.PayloadBinding.Value + ")"
		case p.EnumVariant.DiscardPayload:
			out += "(_)"
		}
		return out
	case p.StructPattern != nil:
		parts := make([]string, 0, len(p.StructPattern.Fields))
		for _, f := range p.StructPattern.Fields {
			parts = append(parts, f.Name.Value+": "+f.Pattern.String())
		}
		return p.StructPattern.Name.Value + " { " + strings.Join(parts, ", ") + " }"
	case len(p.Alternatives) > 0:
		parts := make([]string, 0, len(p.Alternatives))
		for _, alt := range p.Alternatives {
			parts = append(parts, alt.String())
		}
		return strings.Join(parts, " | ")
	default:
		return "_"
	}
}

// ─── TYPE ALIASES ────────────────────────────────────────────────────────────

// TypeAliasDecl is `type Name = Target`, giving a domain name to an existing
// type rather than introducing a new one.
type TypeAliasDecl struct {
	Token  token.Token // token.TYPE
	Name   *Identifier
	Target string
}

func (t *TypeAliasDecl) statementNode()       {}
func (t *TypeAliasDecl) TokenLiteral() string { return t.Token.Literal }
func (t *TypeAliasDecl) String() string {
	return "type " + t.Name.Value + " = " + t.Target
}

// WhileLetStatement is `while let <pattern> = <expr> { body }`.
//
// It repeats for as long as the pattern keeps matching, binding the pattern's
// names each pass. That is the shape a consuming loop has — an iterator that
// yields an Optional until it is empty — and writing it as a plain `while` means
// the same `isSome` test and unwrap on every line of every caller.
type WhileLetStatement struct {
	Token token.Token // token.WHILE

	// Pattern is matched against Value on every pass.
	Pattern MatchPattern

	// Value is the expression the pattern is matched against.
	//
	// It is evaluated once, before the loop, and its result is reused: an
	// iterator advanced inside the condition would be advanced twice per pass.
	Value Expression

	Body *BlockStatement
}

func (wl *WhileLetStatement) statementNode()       {}
func (wl *WhileLetStatement) TokenLiteral() string { return wl.Token.Literal }
func (wl *WhileLetStatement) String() string {
	out := "while let " + wl.Pattern.String() + " = " + wl.Value.String() + " { "
	if wl.Body != nil {
		out += wl.Body.String()
	}
	return out + " }"
}
