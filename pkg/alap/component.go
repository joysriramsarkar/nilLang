// Package alap — legacy first-generation SSR model.
//
// Deprecated (ADR-0006): pkg/alap/ui is the single UI model. This file is a
// thin compatibility shim that delegates to pkg/alap/ui so existing callers
// keep compiling while new code uses ui directly. Do not add features here.
package alap

import (
	"github.com/joysriramsarkar/nilLang/pkg/alap/ui"
)

// Component is a legacy alias. Use ui components instead.
//
// Deprecated: use pkg/alap/ui Component implementations.
type Component = ui.Card

// UIElement is a legacy alias for ui.VNode.
//
// Deprecated: use ui.VNode.
type UIElement = ui.VNode

// Theme is a legacy alias for ui.Theme.
//
// Deprecated: use ui.Theme.
type Theme = ui.Theme

// DefaultTheme delegates to ui.DefaultTheme.
//
// Deprecated: use ui.DefaultTheme.
func DefaultTheme() Theme { return ui.DefaultTheme() }

// OnuronTheme delegates to ui.OnuronTheme.
//
// Deprecated: use ui.OnuronTheme.
func OnuronTheme() Theme { return ui.OnuronTheme() }

// NewUIElement creates a legacy element backed by ui.NewElementVNode.
//
// Deprecated: use ui.NewElementVNode.
func NewUIElement(tag string) *UIElement { return ui.NewElementVNode(tag, "", "") }

// Text creates a text element.
//
// Deprecated: use ui.NewTextVNode.
func Text(content string) *UIElement { return ui.NewTextVNode(content) }

// Button creates a button element.
//
// Deprecated: use ui.NewButton.
func Button(label string, _ func()) *UIElement {
	return ui.NewElementVNode("button", "", "")
}

// Column creates a vertical layout.
//
// Deprecated: compose with ui components.
func Column(children ...*UIElement) *UIElement {
	col := ui.NewElementVNode("div", "", "")
	for _, c := range children {
		col.AddChild(c)
	}
	return col
}

// Row creates a horizontal layout.
//
// Deprecated: compose with ui components.
func Row(children ...*UIElement) *UIElement {
	row := ui.NewElementVNode("div", "", "")
	for _, c := range children {
		row.AddChild(c)
	}
	return row
}

// Container creates a container.
//
// Deprecated: compose with ui components.
func Container(children ...*UIElement) *UIElement {
	div := ui.NewElementVNode("div", "", "")
	for _, c := range children {
		div.AddChild(c)
	}
	return div
}

// Image creates an image element.
//
// Deprecated: use ui catalog.
func Image(src string) *UIElement {
	n := ui.NewElementVNode("img", "", "")
	n.SetAttr("src", src)
	return n
}

// Input creates a text input element.
//
// Deprecated: use ui catalog.
func Input(placeholder string) *UIElement {
	n := ui.NewElementVNode("input", "", "")
	n.SetAttr("placeholder", placeholder)
	return n
}

// ListView creates a scrollable list.
//
// Deprecated: use ui catalog.
func ListView(children ...*UIElement) *UIElement {
	list := ui.NewElementVNode("div", "", "")
	for _, c := range children {
		list.AddChild(c)
	}
	return list
}
