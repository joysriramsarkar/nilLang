// Package a11y implements U11: the semantic tree (role/label/hint/value/
// state), screen-reader order, dynamic type, contrast/reduced-transparency
// helpers, 48dp target checks, reduced-motion wiring, and the TalkBack/
// VoiceOver checklist (see CHECKLIST.md + TestA11yGate_AllComponents in
// components).
package a11y

import (
	"fmt"
	"sort"
)

// Role is a semantic role (U11.2).
type Role string

const (
	RoleButton   Role = "button"
	RoleLink     Role = "link"
	RoleHeader   Role = "header"
	RoleImage    Role = "image"
	RoleCheckbox Role = "checkbox"
	RoleSwitch   Role = "switch"
	RoleSlider   Role = "slider"
	RoleText     Role = "text"
	RoleList     Role = "list"
	RoleDialog   Role = "dialog"
	RoleStatus   Role = "status"
	RoleGroup    Role = "group"
)

// State is a control state.
type State string

const (
	StateEnabled  State = "enabled"
	StateDisabled State = "disabled"
	StateChecked  State = "checked"
	StateSelected State = "selected"
)

// Node is one semantic-tree node (U11.1).
type Node struct {
	ID    string
	Role  Role
	Label string
	Hint  string
	Value string
	State []State
	Order int
	// TargetDp is the touch target for the 48dp gate (U11.7).
	TargetDp [2]float64
	Hidden   bool
}

// Describe renders the screen-reader announcement.
func (n Node) Describe() string {
	s := string(n.Role) + ": " + n.Label
	if n.Value != "" {
		s += ", " + n.Value
	}
	if n.Hint != "" {
		s += " (" + n.Hint + ")"
	}
	for _, st := range n.State {
		s += " [" + string(st) + "]"
	}
	return s
}

// TargetOK enforces the 48×48dp minimum (U11.7 gate).
func (n Node) TargetOK() bool { return n.TargetDp[0] >= 48 && n.TargetDp[1] >= 48 }

// Tree is an ordered semantic tree.
type Tree struct {
	Nodes []*Node
}

// Add inserts.
func (t *Tree) Add(n *Node) *Tree { t.Nodes = append(t.Nodes, n); return t }

// Order returns visible nodes in screen-reader order (U11.3).
func (t *Tree) Order() []*Node {
	var vis []*Node
	for _, n := range t.Nodes {
		if !n.Hidden {
			vis = append(vis, n)
		}
	}
	sort.SliceStable(vis, func(i, j int) bool { return vis[i].Order < vis[j].Order })
	return vis
}

// Validate checks role/label presence + targets (U11.9 gate input).
func (t *Tree) Validate() []string {
	var out []string
	seen := map[int]bool{}
	for _, n := range t.Nodes {
		if n.Hidden {
			continue
		}
		if n.Role == "" {
			out = append(out, fmt.Sprintf("%s: missing role", n.ID))
		}
		if n.Label == "" {
			out = append(out, fmt.Sprintf("%s: missing label", n.ID))
		}
		if seen[n.Order] {
			out = append(out, fmt.Sprintf("%s: duplicate order %d", n.ID, n.Order))
		}
		seen[n.Order] = true
		if n.Role == RoleButton && !n.TargetOK() {
			out = append(out, fmt.Sprintf("%s: touch target %v below 48dp", n.ID, n.TargetDp))
		}
	}
	return out
}

// ─── Dynamic type (U11.4) ───────────────────────────────────────────────────

// ScaleFont scales sp by the a11y font factor (capped for layout sanity).
func ScaleFont(sp, factor float64) float64 {
	if factor < 1 {
		factor = 1
	}
	if factor > 2 {
		factor = 2
	}
	return sp * factor
}

// ─── Reduced transparency (U11.5) ───────────────────────────────────────────

// OpaqueColor removes alpha when reduce-transparency is on.
func OpaqueColor(r, g, b, a uint8, reduce bool) (uint8, uint8, uint8, uint8) {
	if reduce {
		return r, g, b, 255
	}
	return r, g, b, a
}
