package ui

import (
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strings"
)

// ─── VIRTUAL DOM (VDOM) SPECIFICATION (web-implications.md Section 14) ─────

// VNodeType specifies whether a VNode is an element or raw text
type VNodeType string

const (
	VNodeElement VNodeType = "ELEMENT"
	VNodeText    VNodeType = "TEXT"
)

// VNode represents a lightweight in-memory Virtual DOM node
type VNode struct {
	Type     VNodeType         `json:"type"`
	Tag      string            `json:"tag,omitempty"`
	Key      string            `json:"key,omitempty"`
	ID       string            `json:"id,omitempty"`
	Attrs    map[string]string `json:"attrs,omitempty"`
	Events   map[string]string `json:"events,omitempty"`
	Children []*VNode          `json:"children,omitempty"`
	Text     string            `json:"text,omitempty"`
}

// NewElementVNode creates an element VNode
func NewElementVNode(tag string, id string, key string) *VNode {
	return &VNode{
		Type:     VNodeElement,
		Tag:      tag,
		ID:       id,
		Key:      key,
		Attrs:    make(map[string]string),
		Events:   make(map[string]string),
		Children: make([]*VNode, 0),
	}
}

// NewTextVNode creates a text VNode
func NewTextVNode(text string) *VNode {
	return &VNode{
		Type: VNodeText,
		Text: text,
	}
}

// SetAttr sets an attribute on the VNode
func (v *VNode) SetAttr(key, val string) *VNode {
	if v.Attrs == nil {
		v.Attrs = make(map[string]string)
	}
	v.Attrs[key] = val
	return v
}

// SetEvent sets an event listener identifier on the VNode
func (v *VNode) SetEvent(name, handler string) *VNode {
	if v.Events == nil {
		v.Events = make(map[string]string)
	}
	v.Events[name] = handler
	return v
}

// AddChild appends a child VNode
func (v *VNode) AddChild(child *VNode) *VNode {
	if child != nil {
		v.Children = append(v.Children, child)
	}
	return v
}

// RenderHTML converts a VNode tree directly into an HTML string
func (v *VNode) RenderHTML() string {
	if v.Type == VNodeText {
		return html.EscapeString(v.Text)
	}

	var sb strings.Builder
	sb.WriteString("<")
	sb.WriteString(v.Tag)

	if v.ID != "" {
		sb.WriteString(fmt.Sprintf(` id="%s"`, html.EscapeString(v.ID)))
	}
	if v.Key != "" {
		sb.WriteString(fmt.Sprintf(` data-key="%s"`, html.EscapeString(v.Key)))
	}

	// Sort attributes for deterministic output
	keys := make([]string, 0, len(v.Attrs))
	for k := range v.Attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		val := v.Attrs[k]
		sb.WriteString(fmt.Sprintf(` %s="%s"`, k, html.EscapeString(val)))
	}

	// Render event data tags
	evtKeys := make([]string, 0, len(v.Events))
	for k := range v.Events {
		evtKeys = append(evtKeys, k)
	}
	sort.Strings(evtKeys)
	for _, k := range evtKeys {
		sb.WriteString(fmt.Sprintf(` data-alap-%s="%s"`, k, html.EscapeString(v.Events[k])))
	}

	// Void tags
	voidTags := map[string]bool{"input": true, "img": true, "hr": true, "br": true, "meta": true}
	if voidTags[v.Tag] && len(v.Children) == 0 {
		sb.WriteString(" />")
		return sb.String()
	}

	sb.WriteString(">")
	for _, c := range v.Children {
		sb.WriteString(c.RenderHTML())
	}
	sb.WriteString("</")
	sb.WriteString(v.Tag)
	sb.WriteString(">")
	return sb.String()
}

// ─── INCREMENTAL DOM PATCHING ──────────────────────────────────────────────

// PatchType defines DOM mutation operations
type PatchType string

const (
	PatchReplace     PatchType = "REPLACE"
	PatchProps       PatchType = "PROPS"
	PatchText        PatchType = "TEXT"
	PatchInsertChild PatchType = "INSERT_CHILD"
	PatchRemoveChild PatchType = "REMOVE_CHILD"
)

// Patch represents a discrete DOM mutation operation
type Patch struct {
	Type   PatchType         `json:"type"`
	Path   []int             `json:"path"`
	NodeID string            `json:"node_id,omitempty"`
	VNode  *VNode            `json:"vnode,omitempty"`
	Props  map[string]string `json:"props,omitempty"`
	Text   string            `json:"text,omitempty"`
	Index  int               `json:"index,omitempty"`
}

// Diff compares two VNode trees and returns the minimal list of patches
func Diff(oldTree, newTree *VNode) []Patch {
	patches := make([]Patch, 0)
	diffRecursive(oldTree, newTree, []int{}, &patches)
	return patches
}

func diffRecursive(oldNode, newNode *VNode, currentPath []int, patches *[]Patch) {
	if oldNode == nil && newNode == nil {
		return
	}

	// 1. If old node doesn't exist, replace
	if oldNode == nil {
		*patches = append(*patches, Patch{
			Type:   PatchReplace,
			Path:   currentPath,
			NodeID: newNode.ID,
			VNode:  newNode,
		})
		return
	}

	// 2. If new node doesn't exist, remove
	if newNode == nil {
		*patches = append(*patches, Patch{
			Type:   PatchReplace,
			Path:   currentPath,
			NodeID: oldNode.ID,
			VNode:  nil,
		})
		return
	}

	// 3. Different types or tags -> complete replacement
	if oldNode.Type != newNode.Type || oldNode.Tag != newNode.Tag || (oldNode.Key != "" && newNode.Key != "" && oldNode.Key != newNode.Key) {
		*patches = append(*patches, Patch{
			Type:   PatchReplace,
			Path:   currentPath,
			NodeID: newNode.ID,
			VNode:  newNode,
		})
		return
	}

	// 4. Text node comparison
	if oldNode.Type == VNodeText && newNode.Type == VNodeText {
		if oldNode.Text != newNode.Text {
			*patches = append(*patches, Patch{
				Type: PatchText,
				Path: currentPath,
				Text: newNode.Text,
			})
		}
		return
	}

	// 5. Props & Attributes comparison
	propsDiff := make(map[string]string)
	// Check updated/new attributes
	for k, v := range newNode.Attrs {
		if oldV, exists := oldNode.Attrs[k]; !exists || oldV != v {
			propsDiff[k] = v
		}
	}
	// Check removed attributes (marked with empty string)
	for k := range oldNode.Attrs {
		if _, exists := newNode.Attrs[k]; !exists {
			propsDiff[k] = ""
		}
	}
	// Check event updates
	for k, v := range newNode.Events {
		if oldV, exists := oldNode.Events[k]; !exists || oldV != v {
			propsDiff["data-alap-"+k] = v
		}
	}
	for k := range oldNode.Events {
		if _, exists := newNode.Events[k]; !exists {
			propsDiff["data-alap-"+k] = ""
		}
	}

	if len(propsDiff) > 0 {
		*patches = append(*patches, Patch{
			Type:   PatchProps,
			Path:   currentPath,
			NodeID: newNode.ID,
			Props:  propsDiff,
		})
	}

	// 6. Children comparison
	oldLen := len(oldNode.Children)
	newLen := len(newNode.Children)
	commonLen := oldLen
	if newLen < commonLen {
		commonLen = newLen
	}

	// Diff common children
	for i := 0; i < commonLen; i++ {
		childPath := make([]int, len(currentPath)+1)
		copy(childPath, currentPath)
		childPath[len(currentPath)] = i
		diffRecursive(oldNode.Children[i], newNode.Children[i], childPath, patches)
	}

	// Extra children in new tree -> INSERT
	if newLen > oldLen {
		for i := oldLen; i < newLen; i++ {
			*patches = append(*patches, Patch{
				Type:   PatchInsertChild,
				Path:   currentPath,
				Index:  i,
				VNode:  newNode.Children[i],
				NodeID: newNode.ID,
			})
		}
	} else if oldLen > newLen {
		// Excess children in old tree -> REMOVE from the end
		for i := oldLen - 1; i >= newLen; i-- {
			*patches = append(*patches, Patch{
				Type:   PatchRemoveChild,
				Path:   currentPath,
				Index:  i,
				NodeID: oldNode.ID,
			})
		}
	}
}

// SerializePatches serializes patches into JSON format for network transport
func SerializePatches(patches []Patch) ([]byte, error) {
	return json.Marshal(patches)
}
