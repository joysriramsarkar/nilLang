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

// ─── INCREMENTAL DOM PATCHING (ADR-0005: single keyed-patch runtime) ─────────

type PatchType string

const (
	PatchReplace     PatchType = "REPLACE"
	PatchProps       PatchType = "PROPS"
	PatchText        PatchType = "TEXT"
	PatchInsertChild PatchType = "INSERT_CHILD"
	PatchRemoveChild PatchType = "REMOVE_CHILD"
	PatchMove        PatchType = "MOVE"
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
	From   int               `json:"from,omitempty"`
	To     int               `json:"to,omitempty"`
}

// Diff compares two VNode trees and returns the minimal list of patches.
// Children with non-empty keys use LIS-based keyed reordering (O(n)) so a
// pure shuffle emits MOVE patches instead of needless REPLACE patches.
func Diff(oldTree, newTree *VNode) []Patch {
	patches := make([]Patch, 0)
	diffRecursive(oldTree, newTree, []int{}, &patches)
	return patches
}

func diffRecursive(oldNode, newNode *VNode, currentPath []int, patches *[]Patch) {
	if oldNode == nil && newNode == nil {
		return
	}
	if oldNode == nil {
		*patches = append(*patches, Patch{
			Type:   PatchReplace,
			Path:   currentPath,
			NodeID: newNode.ID,
			VNode:  newNode,
		})
		return
	}
	if newNode == nil {
		*patches = append(*patches, Patch{
			Type:   PatchReplace,
			Path:   currentPath,
			NodeID: oldNode.ID,
			VNode:  nil,
		})
		return
	}
	if oldNode.Type != newNode.Type || oldNode.Tag != newNode.Tag {
		*patches = append(*patches, Patch{
			Type:   PatchReplace,
			Path:   currentPath,
			NodeID: newNode.ID,
			VNode:  newNode,
		})
		return
	}
	// Same-position different keyed identity -> replace (identity changed).
	if oldNode.Key != "" && newNode.Key != "" && oldNode.Key != newNode.Key {
		// At the child-list level this is handled by keyed reorder; at the
		// node level a key change means a different logical node.
		*patches = append(*patches, Patch{
			Type:   PatchReplace,
			Path:   currentPath,
			NodeID: newNode.ID,
			VNode:  newNode,
		})
		return
	}

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

	propsDiff := make(map[string]string)
	for k, v := range newNode.Attrs {
		if oldV, exists := oldNode.Attrs[k]; !exists || oldV != v {
			propsDiff[k] = v
		}
	}
	for k := range oldNode.Attrs {
		if _, exists := newNode.Attrs[k]; !exists {
			propsDiff[k] = ""
		}
	}
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

	diffChildren(oldNode, newNode, currentPath, patches)
}

// diffChildren diffs child lists. When both sides carry keys for every
// child, it uses keyed reconciliation with LIS so order-only changes become
// MOVE patches. Otherwise it falls back to positional diff.
func diffChildren(oldNode, newNode *VNode, currentPath []int, patches *[]Patch) {
	oldKids := oldNode.Children
	newKids := newNode.Children
	if allKeyed(oldKids) && allKeyed(newKids) && len(oldKids) > 0 {
		diffKeyedChildren(oldNode, newNode, currentPath, patches)
		return
	}
	oldLen := len(oldKids)
	newLen := len(newKids)
	commonLen := oldLen
	if newLen < commonLen {
		commonLen = newLen
	}
	for i := 0; i < commonLen; i++ {
		childPath := append(append([]int{}, currentPath...), i)
		diffRecursive(oldKids[i], newKids[i], childPath, patches)
	}
	if newLen > oldLen {
		for i := oldLen; i < newLen; i++ {
			*patches = append(*patches, Patch{
				Type:   PatchInsertChild,
				Path:   currentPath,
				Index:  i,
				VNode:  newKids[i],
				NodeID: newNode.ID,
			})
		}
	} else if oldLen > newLen {
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

func allKeyed(kids []*VNode) bool {
	if len(kids) == 0 {
		return false
	}
	for _, k := range kids {
		if k == nil || k.Key == "" {
			return false
		}
	}
	return true
}

// diffKeyedChildren implements O(n) keyed reconciliation:
//  1. index new keys, match old keys (removals for missing, inserts for new);
//  2. matched pairs are diffed recursively (props/text, not replace);
//  3. order changes resolved via Longest Increasing Subsequence — only
//     non-LIS members emit MOVE.
func diffKeyedChildren(oldNode, newNode *VNode, currentPath []int, patches *[]Patch) {
	oldKids := oldNode.Children
	newKids := newNode.Children
	oldIndex := make(map[string]int, len(oldKids))
	for i, k := range oldKids {
		oldIndex[k.Key] = i
	}
	newIndex := make(map[string]int, len(newKids))
	for i, k := range newKids {
		newIndex[k.Key] = i
	}

	// Removals (descending so indices stay valid when applied in order).
	for i := len(oldKids) - 1; i >= 0; i-- {
		if _, ok := newIndex[oldKids[i].Key]; !ok {
			*patches = append(*patches, Patch{
				Type:   PatchRemoveChild,
				Path:   currentPath,
				Index:  i,
				NodeID: oldNode.ID,
			})
		}
	}
	// Inserts.
	for i, k := range newKids {
		if _, ok := oldIndex[k.Key]; !ok {
			*patches = append(*patches, Patch{
				Type:   PatchInsertChild,
				Path:   currentPath,
				Index:  i,
				VNode:  k,
				NodeID: newNode.ID,
			})
		}
	}
	// Recursive diff of matched pairs at their NEW positions.
	for i, k := range newKids {
		oi, ok := oldIndex[k.Key]
		if !ok {
			continue
		}
		childPath := append(append([]int{}, currentPath...), i)
		diffRecursiveKeyed(oldKids[oi], k, childPath, patches)
	}
	// Reorder detection via LIS over old positions in new order.
	seq := make([]int, 0, len(newKids))
	for _, k := range newKids {
		if oi, ok := oldIndex[k.Key]; ok {
			if _, stillThere := newIndex[k.Key]; stillThere {
				seq = append(seq, oi)
			}
		}
	}
	inLIS := longestIncreasingSet(seq)
	// Map new-position -> whether its old position is in LIS.
	pos := 0
	for i, k := range newKids {
		oi, ok := oldIndex[k.Key]
		if !ok {
			continue
		}
		_ = oi
		if !inLIS[pos] {
			*patches = append(*patches, Patch{
				Type:   PatchMove,
				Path:   currentPath,
				NodeID: oldNode.ID,
				From:   oldIndex[k.Key],
				To:     i,
			})
		}
		pos++
	}
}

// diffRecursiveKeyed diffs two nodes known to share identity: never REPLACE
// on key equality, only props/text/children.
func diffRecursiveKeyed(oldNode, newNode *VNode, path []int, patches *[]Patch) {
	if oldNode.Type != newNode.Type || oldNode.Tag != newNode.Tag {
		*patches = append(*patches, Patch{
			Type:   PatchReplace,
			Path:   path,
			NodeID: newNode.ID,
			VNode:  newNode,
		})
		return
	}
	if oldNode.Type == VNodeText && newNode.Type == VNodeText {
		if oldNode.Text != newNode.Text {
			*patches = append(*patches, Patch{Type: PatchText, Path: path, Text: newNode.Text})
		}
		return
	}
	propsDiff := make(map[string]string)
	for k, v := range newNode.Attrs {
		if oldV, ok := oldNode.Attrs[k]; !ok || oldV != v {
			propsDiff[k] = v
		}
	}
	for k := range oldNode.Attrs {
		if _, ok := newNode.Attrs[k]; !ok {
			propsDiff[k] = ""
		}
	}
	if len(propsDiff) > 0 {
		*patches = append(*patches, Patch{Type: PatchProps, Path: path, NodeID: newNode.ID, Props: propsDiff})
	}
	diffChildren(oldNode, newNode, path, patches)
}

// longestIncreasingSet returns a boolean mask marking which elements of seq
// belong to one longest increasing subsequence (O(n log n)).
func longestIncreasingSet(seq []int) []bool {
	n := len(seq)
	mask := make([]bool, n)
	if n == 0 {
		return mask
	}
	tails := make([]int, 0, n)
	prev := make([]int, n)
	tailIdx := make([]int, 0, n)
	for i := range prev {
		prev[i] = -1
	}
	for i, v := range seq {
		lo, hi := 0, len(tails)
		for lo < hi {
			mid := (lo + hi) / 2
			if tails[mid] < v {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		if lo == len(tails) {
			tails = append(tails, v)
			tailIdx = append(tailIdx, i)
		} else {
			tails[lo] = v
			tailIdx[lo] = i
		}
		if lo > 0 {
			prev[i] = tailIdx[lo-1]
		}
	}
	for k := tailIdx[len(tailIdx)-1]; k != -1; k = prev[k] {
		mask[k] = true
	}
	return mask
}

// SerializePatches serializes patches into JSON format for network transport
func SerializePatches(patches []Patch) ([]byte, error) {
	return json.Marshal(patches)
}
