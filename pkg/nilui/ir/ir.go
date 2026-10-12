// Package ir implements U1: the typed, versioned, serializable UI IR.
//
// Node{Type, Key, Props, Slots, Children, Events, Modifiers, Span, Version}
// is validated by Validate (diagnostics UI0101–UI0150, see UI_SPEC.md §3.4)
// and round-trips through the .nilax (JSON) bundle format.
package ir

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Version is the current IR schema version.
const Version = 1

// Diagnostic codes U1.6 (UI0101–UI0150). Only the used subset is enumerated;
// the range is reserved in UI_SPEC.md.
const (
	CodeUnknownType   = "UI0101"
	CodeMissingProp   = "UI0102"
	CodeDuplicateKey  = "UI0103"
	CodeWrongChild    = "UI0104"
	CodeUnknownProp   = "UI0105"
	CodeBadPropType   = "UI0106"
	CodeMissingKey    = "UI0107"
	CodeBadEvent      = "UI0108"
	CodeVersionSkew   = "UI0109"
	CodeSpanLost      = "UI0110"
	CodeBridgeUnknown = "UI0140"
)

// Diagnostic is a schema validation failure. Never silent: validation
// returns every diagnostic instead of dropping nodes.
type Diagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path"`
}

// Error implements error.
func (d Diagnostic) Error() string { return d.Code + " at " + d.Path + ": " + d.Message }

// Span records where a node came from (U1.4: IDE "this component is where?").
type Span struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

// Event is a named handler binding.
type Event struct {
	Name    string `json:"name"`
	Handler string `json:"handler"`
}

// Node is one typed UI IR node.
type Node struct {
	Type      string            `json:"type"`
	Key       string            `json:"key,omitempty"`
	Props     map[string]string `json:"props,omitempty"`
	Slots     map[string]*Node  `json:"slots,omitempty"`
	Children  []*Node           `json:"children,omitempty"`
	Events    []Event           `json:"events,omitempty"`
	Modifiers []string          `json:"modifiers,omitempty"`
	Span      Span              `json:"span,omitempty"`
	Version   int               `json:"version"`
}

// New creates a versioned node.
func New(typ string) *Node {
	return &Node{Type: typ, Props: map[string]string{}, Version: Version}
}

// WithKey sets a stable list key (U1.3).
func (n *Node) WithKey(k string) *Node { n.Key = k; return n }

// WithProp sets a prop.
func (n *Node) WithProp(k, v string) *Node {
	if n.Props == nil {
		n.Props = map[string]string{}
	}
	n.Props[k] = v
	return n
}

// Add appends a child.
func (n *Node) Add(c *Node) *Node {
	if c != nil {
		n.Children = append(n.Children, c)
	}
	return n
}

// On binds an event handler.
func (n *Node) On(name, handler string) *Node {
	n.Events = append(n.Events, Event{Name: name, Handler: handler})
	return n
}

// At attaches source location.
func (n *Node) At(file string, line, col int) *Node {
	n.Span = Span{File: file, Line: line, Column: col}
	return n
}

// ─── Schema (U1.1/U1.2) ─────────────────────────────────────────────────────

// PropDef describes one allowed prop.
type PropDef struct {
	Required bool
	// Allowed, when non-empty, enumerates legal values.
	Allowed []string
}

// TypeDef is the schema row for one component type.
type TypeDef struct {
	RequiredProps []string
	OptionalProps []string
	// ChildTypes, when non-empty, restricts direct children types.
	ChildTypes []string
	// NeedsKey requires Key on list children of this type.
	NeedsKey bool
}

// CatalogSchema is the minimal built-in schema. Components register more.
var CatalogSchema = map[string]TypeDef{}

func def(typ string, required, optional []string) {
	CatalogSchema[typ] = TypeDef{RequiredProps: required, OptionalProps: optional}
}

func init() {
	for _, t := range []string{"Page", "Text", "RichText", "Box", "Spacer", "Divider",
		"Column", "Row", "Stack", "Grid", "ScrollView", "SafeArea", "Padding",
		"Align", "SizedBox", "Flex", "Layout", "Button", "IconButton", "TextField",
		"SearchField", "PasswordField", "Checkbox", "CheckboxGroup", "RadioGroup",
		"Switch", "Slider", "Stepper", "Image", "Icon", "List", "LazyColumn",
		"LazyRow", "DataGrid", "ProgressIndicator", "ActivityIndicator", "Badge",
		"Dialog", "Modal", "BottomSheet", "Snackbar", "Tooltip", "Popover",
		"NavigationStack", "AppBar", "BottomNavigation", "TabBar", "PageView", "Route",
		"Canvas", "Chart", "SemanticGroup", "FocusScope", "AnimatedContainer",
		"Card", "Table", "Form", "Dashboard", "Tabs", "Navigation"} {
		def(t, nil, []string{"title", "label", "body", "id", "variant", "style"})
	}
	// Stricter rows for core types.
	CatalogSchema["Text"] = TypeDef{RequiredProps: []string{"value"}, OptionalProps: []string{"id", "style"}}
	CatalogSchema["Button"] = TypeDef{RequiredProps: []string{"label"}, OptionalProps: []string{"id", "event", "variant", "disabled", "style"}}
	CatalogSchema["Page"] = TypeDef{RequiredProps: []string{"title"}, OptionalProps: []string{"footer"}}
	CatalogSchema["Image"] = TypeDef{RequiredProps: []string{"src"}, OptionalProps: []string{"id", "alt"}}
	CatalogSchema["List"] = TypeDef{OptionalProps: []string{"id"}, NeedsKey: true}
	CatalogSchema["LazyColumn"] = TypeDef{OptionalProps: []string{"id"}, NeedsKey: true}
}

// Register adds a component type to the schema.
func Register(typ string, d TypeDef) { CatalogSchema[typ] = d }

// Validate checks a tree and returns every diagnostic (U1.2). An empty
// result means valid. Unknown types, missing props, duplicate keys, wrong
// child types, bad events, and version skew are all reported — nothing is
// silently dropped.
func Validate(n *Node) []Diagnostic {
	var out []Diagnostic
	validateNode(n, "/"+n.Type, map[string]int{}, &out)
	return out
}

func validateNode(n *Node, path string, sibKeys map[string]int, out *[]Diagnostic) {
	if n.Version != Version {
		*out = append(*out, Diagnostic{Code: CodeVersionSkew, Path: path,
			Message: fmt.Sprintf("version %d, want %d", n.Version, Version)})
	}
	d, known := CatalogSchema[n.Type]
	if !known {
		*out = append(*out, Diagnostic{Code: CodeUnknownType, Path: path,
			Message: fmt.Sprintf("unknown component type %q", n.Type)})
	}
	allowed := map[string]bool{}
	for _, p := range d.RequiredProps {
		allowed[p] = true
		if _, ok := n.Props[p]; !ok && known {
			*out = append(*out, Diagnostic{Code: CodeMissingProp, Path: path,
				Message: fmt.Sprintf("missing required prop %q for %q", p, n.Type)})
		}
	}
	for _, p := range d.OptionalProps {
		allowed[p] = true
	}
	for p := range n.Props {
		if known && !allowed[p] {
			*out = append(*out, Diagnostic{Code: CodeUnknownProp, Path: path,
				Message: fmt.Sprintf("unknown prop %q for %q", p, n.Type)})
		}
	}
	if len(d.ChildTypes) > 0 {
		allowedChildren := map[string]bool{}
		for _, t := range d.ChildTypes {
			allowedChildren[t] = true
		}
		for i, c := range n.Children {
			if !allowedChildren[c.Type] {
				*out = append(*out, Diagnostic{Code: CodeWrongChild, Path: path,
					Message: fmt.Sprintf("child %d type %q not allowed in %q", i, c.Type, n.Type)})
			}
		}
	}
	// Stable keys (U1.3): duplicate keys diagnosed; missing keys in
	// key-needing parents diagnosed.
	keys := map[string]int{}
	for i, c := range n.Children {
		if c.Key != "" {
			if prev, dup := keys[c.Key]; dup {
				*out = append(*out, Diagnostic{Code: CodeDuplicateKey, Path: path,
					Message: fmt.Sprintf("duplicate key %q (children %d and %d)", c.Key, prev, i)})
			} else {
				keys[c.Key] = i
			}
		} else if d.NeedsKey {
			*out = append(*out, Diagnostic{Code: CodeMissingKey, Path: path,
				Message: fmt.Sprintf("child %d of %q needs a stable key", i, n.Type)})
		}
		_ = sibKeys
		validateNode(c, fmt.Sprintf("%s/%s[%d]", path, c.Type, i), map[string]int{}, out)
	}
	for _, e := range n.Events {
		if e.Name == "" || e.Handler == "" {
			*out = append(*out, Diagnostic{Code: CodeBadEvent, Path: path,
				Message: "event binding needs name and handler"})
		}
	}
	if n.Span.File == "" {
		*out = append(*out, Diagnostic{Code: CodeSpanLost, Path: path,
			Message: "node carries no source span"})
	}
}

// ─── .nilax round-trip (U1.5) ───────────────────────────────────────────────

// Marshal serializes a tree deterministically (sorted map keys via encoding/json
// on maps is deterministic since Go 1.12? No — use explicit ordering).
func Marshal(n *Node) ([]byte, error) {
	return json.Marshal(canonical(n))
}

// Unmarshal parses a .nilax bundle.
func Unmarshal(data []byte) (*Node, error) {
	var n Node
	if err := json.Unmarshal(data, &n); err != nil {
		return nil, err
	}
	return &n, nil
}

func canonical(n *Node) *Node {
	if n == nil {
		return nil
	}
	cp := *n
	if n.Props != nil {
		keys := make([]string, 0, len(n.Props))
		for k := range n.Props {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		cp.Props = make(map[string]string, len(n.Props))
		for _, k := range keys {
			cp.Props[k] = n.Props[k]
		}
	}
	cp.Children = make([]*Node, len(n.Children))
	for i, c := range n.Children {
		cp.Children[i] = canonical(c)
	}
	return &cp
}
