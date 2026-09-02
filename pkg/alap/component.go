package alap

import (
	"fmt"
	"strings"
)

// Component represents a UI component in the Alap framework
type Component struct {
	Name       string
	Properties map[string]interface{}
	State      map[string]interface{}
	Children   []*Component
	Parent     *Component
	EventHandlers map[string]func(args ...interface{})
	RenderFunc func() *UIElement
}

// NewComponent creates a new component
func NewComponent(name string) *Component {
	return &Component{
		Name:          name,
		Properties:    make(map[string]interface{}),
		State:         make(map[string]interface{}),
		Children:      []*Component{},
		EventHandlers: make(map[string]func(args ...interface{})),
	}
}

// SetProperty sets a component property
func (c *Component) SetProperty(key string, value interface{}) *Component {
	c.Properties[key] = value
	return c
}

// GetProperty gets a component property
func (c *Component) GetProperty(key string) interface{} {
	return c.Properties[key]
}

// SetState sets a state value and triggers re-render
func (c *Component) SetState(key string, value interface{}) {
	c.State[key] = value
	c.triggerReRender()
}

// GetState gets a state value
func (c *Component) GetState(key string) interface{} {
	return c.State[key]
}

// AddChild adds a child component
func (c *Component) AddChild(child *Component) *Component {
	child.Parent = c
	c.Children = append(c.Children, child)
	return c
}

// On registers an event handler
func (c *Component) On(event string, handler func(args ...interface{})) *Component {
	c.EventHandlers[event] = handler
	return c
}

// Emit triggers an event
func (c *Component) Emit(event string, args ...interface{}) {
	if handler, exists := c.EventHandlers[event]; exists {
		handler(args...)
	}
}

func (c *Component) triggerReRender() {
	// In a real implementation, this would schedule a re-render
	// For now, just log
	fmt.Printf("[Alap] Re-render triggered for %s\n", c.Name)
}

// UIElement represents a rendered UI element
type UIElement struct {
	Tag        string
	Attributes map[string]interface{}
	Children   []*UIElement
	Text       string
}

// NewUIElement creates a new UI element
func NewUIElement(tag string) *UIElement {
	return &UIElement{
		Tag:        tag,
		Attributes: make(map[string]interface{}),
		Children:   []*UIElement{},
	}
}

// SetAttr sets an attribute
func (e *UIElement) SetAttr(key string, value interface{}) *UIElement {
	e.Attributes[key] = value
	return e
}

// AddChild adds a child element
func (e *UIElement) AddChild(child *UIElement) *UIElement {
	e.Children = append(e.Children, child)
	return e
}

// SetText sets text content
func (e *UIElement) SetText(text string) *UIElement {
	e.Text = text
	return e
}

// Render renders the element tree to a string representation
func (e *UIElement) Render() string {
	var sb strings.Builder
	e.renderTo(&sb, 0)
	return sb.String()
}

func (e *UIElement) renderTo(sb *strings.Builder, indent int) {
	padding := strings.Repeat("  ", indent)

	sb.WriteString(padding)
	sb.WriteString("<")
	sb.WriteString(e.Tag)

	// Write attributes
	for key, val := range e.Attributes {
		sb.WriteString(fmt.Sprintf(" %s=\"%v\"", key, val))
	}

	if len(e.Children) == 0 && e.Text == "" {
		sb.WriteString(" />\n")
		return
	}

	sb.WriteString(">")

	if e.Text != "" {
		sb.WriteString(e.Text)
	}

	sb.WriteString("\n")

	for _, child := range e.Children {
		child.renderTo(sb, indent+1)
	}

	sb.WriteString(padding)
	sb.WriteString("</")
	sb.WriteString(e.Tag)
	sb.WriteString(">\n")
}

// ============================================================
// Built-in UI Components (Alap Widget Library)
// ============================================================

// Text creates a text element
func Text(content string) *UIElement {
	return NewUIElement("Text").SetText(content)
}

// Button creates a button element
func Button(label string, onClick func()) *UIElement {
	return NewUIElement("Button").
		SetAttr("label", label).
		SetAttr("onClick", onClick)
}

// Column creates a vertical layout
func Column(children ...*UIElement) *UIElement {
	col := NewUIElement("Column")
	for _, child := range children {
		col.AddChild(child)
	}
	return col
}

// Row creates a horizontal layout
func Row(children ...*UIElement) *UIElement {
	row := NewUIElement("Row")
	for _, child := range children {
		row.AddChild(child)
	}
	return row
}

// Container creates a container with styling
func Container(children ...*UIElement) *UIElement {
	container := NewUIElement("Container")
	for _, child := range children {
		container.AddChild(child)
	}
	return container
}

// Image creates an image element
func Image(src string) *UIElement {
	return NewUIElement("Image").SetAttr("src", src)
}

// Input creates a text input element
func Input(placeholder string) *UIElement {
	return NewUIElement("Input").SetAttr("placeholder", placeholder)
}

// ListView creates a scrollable list
func ListView(children ...*UIElement) *UIElement {
	list := NewUIElement("ListView")
	for _, child := range children {
		list.AddChild(child)
	}
	return list
}

// ============================================================
// Theme System
// ============================================================

type Theme struct {
	PrimaryColor    string
	SecondaryColor  string
	BackgroundColor string
	TextColor       string
	FontSize        int
	BorderRadius    int
	Padding         int
}

func DefaultTheme() *Theme {
	return &Theme{
		PrimaryColor:    "#00d4ff",
		SecondaryColor:  "#005A9C",
		BackgroundColor: "#0a0a0f",
		TextColor:       "#e0e0e0",
		FontSize:        16,
		BorderRadius:    8,
		Padding:         16,
	}
}

// OnuronTheme returns the Onuron OS default theme
func OnuronTheme() *Theme {
	return &Theme{
		PrimaryColor:    "#00d4ff",
		SecondaryColor:  "#1a1a2e",
		BackgroundColor: "#0a0a0f",
		TextColor:       "#ffffff",
		FontSize:        16,
		BorderRadius:    12,
		Padding:         16,
	}
}