package alap

import (
	"fmt"
	"strings"
)

// Renderer handles UI rendering
type Renderer struct {
	theme *Theme
	root  *UIElement
	dirty bool
}

// NewRenderer creates a new renderer
func NewRenderer(theme *Theme) *Renderer {
	if theme == nil {
		theme = DefaultTheme()
	}
	return &Renderer{
		theme: theme,
		dirty: true,
	}
}

// SetRoot sets the root element
func (r *Renderer) SetRoot(root *UIElement) {
	r.root = root
	r.dirty = true
}

// Render renders the UI tree
func (r *Renderer) Render() string {
	if r.root == nil {
		return "<Empty />"
	}
	return r.root.Render()
}

// RenderToANSI renders the UI tree as ANSI art (for terminal)
func (r *Renderer) RenderToANSI() string {
	if r.root == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("╔══════════════════════════════════════╗\n")
	sb.WriteString("║     Alap UI Renderer - Onuron OS     ║\n")
	sb.WriteString("╠══════════════════════════════════════╣\n")
	r.renderElementToANSI(&sb, r.root, 1)
	sb.WriteString("╚══════════════════════════════════════╝\n")
	return sb.String()
}

func (r *Renderer) renderElementToANSI(sb *strings.Builder, elem *UIElement, depth int) {
	padding := strings.Repeat("  ", depth)

	switch elem.Tag {
	case "Text":
		sb.WriteString(fmt.Sprintf("%s📝 %s\n", padding, elem.Text))
	case "Button":
		label := elem.Attributes["label"]
		sb.WriteString(fmt.Sprintf("%s🔘 [%v]\n", padding, label))
	case "Column":
		sb.WriteString(fmt.Sprintf("%s📐 Column:\n", padding))
		for _, child := range elem.Children {
			r.renderElementToANSI(sb, child, depth+1)
		}
	case "Row":
		sb.WriteString(fmt.Sprintf("%s📏 Row:\n", padding))
		for _, child := range elem.Children {
			r.renderElementToANSI(sb, child, depth+1)
		}
	case "Container":
		sb.WriteString(fmt.Sprintf("%s📦 Container:\n", padding))
		for _, child := range elem.Children {
			r.renderElementToANSI(sb, child, depth+1)
		}
	case "Image":
		src := elem.Attributes["src"]
		sb.WriteString(fmt.Sprintf("%s🖼️  Image(%v)\n", padding, src))
	case "Input":
		placeholder := elem.Attributes["placeholder"]
		sb.WriteString(fmt.Sprintf("%s⌨️  Input[%v]\n", padding, placeholder))
	case "ListView":
		sb.WriteString(fmt.Sprintf("%s📋 ListView:\n", padding))
		for _, child := range elem.Children {
			r.renderElementToANSI(sb, child, depth+1)
		}
	default:
		sb.WriteString(fmt.Sprintf("%s<%s>\n", padding, elem.Tag))
		for _, child := range elem.Children {
			r.renderElementToANSI(sb, child, depth+1)
		}
	}
}

// RenderToHTML renders the UI tree as HTML (for web preview)
func (r *Renderer) RenderToHTML() string {
	if r.root == nil {
		return "<html><body></body></html>"
	}

	var sb strings.Builder
	sb.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
	sb.WriteString("  <meta charset=\"UTF-8\">\n")
	sb.WriteString("  <title>Alap UI Preview</title>\n")
	sb.WriteString("  <style>\n")
	sb.WriteString(fmt.Sprintf("    body { background: %s; color: %s; font-family: system-ui; }\n",
		r.theme.BackgroundColor, r.theme.TextColor))
	sb.WriteString(fmt.Sprintf("    .column { display: flex; flex-direction: column; gap: %dpx; }\n", r.theme.Padding))
	sb.WriteString(fmt.Sprintf("    .row { display: flex; flex-direction: row; gap: %dpx; }\n", r.theme.Padding))
	sb.WriteString(fmt.Sprintf("    .button { background: %s; color: white; padding: %dpx; border-radius: %dpx; border: none; cursor: pointer; }\n",
		r.theme.PrimaryColor, r.theme.Padding/2, r.theme.BorderRadius))
	sb.WriteString(fmt.Sprintf("    .container { padding: %dpx; }\n", r.theme.Padding))
	sb.WriteString("  </style>\n")
	sb.WriteString("</head>\n<body>\n")
	r.renderElementToHTML(&sb, r.root, 1)
	sb.WriteString("</body>\n</html>")

	return sb.String()
}

func (r *Renderer) renderElementToHTML(sb *strings.Builder, elem *UIElement, depth int) {
	padding := strings.Repeat("  ", depth)

	switch elem.Tag {
	case "Text":
		sb.WriteString(fmt.Sprintf("%s<p>%s</p>\n", padding, elem.Text))
	case "Button":
		label := elem.Attributes["label"]
		sb.WriteString(fmt.Sprintf("%s<button class=\"button\">%v</button>\n", padding, label))
	case "Column":
		sb.WriteString(fmt.Sprintf("%s<div class=\"column\">\n", padding))
		for _, child := range elem.Children {
			r.renderElementToHTML(sb, child, depth+1)
		}
		sb.WriteString(fmt.Sprintf("%s</div>\n", padding))
	case "Row":
		sb.WriteString(fmt.Sprintf("%s<div class=\"row\">\n", padding))
		for _, child := range elem.Children {
			r.renderElementToHTML(sb, child, depth+1)
		}
		sb.WriteString(fmt.Sprintf("%s</div>\n", padding))
	case "Container":
		sb.WriteString(fmt.Sprintf("%s<div class=\"container\">\n", padding))
		for _, child := range elem.Children {
			r.renderElementToHTML(sb, child, depth+1)
		}
		sb.WriteString(fmt.Sprintf("%s</div>\n", padding))
	case "Image":
		src := elem.Attributes["src"]
		sb.WriteString(fmt.Sprintf("%s<img src=\"%v\" />\n", padding, src))
	case "Input":
		placeholder := elem.Attributes["placeholder"]
		sb.WriteString(fmt.Sprintf("%s<input type=\"text\" placeholder=\"%v\" />\n", padding, placeholder))
	default:
		sb.WriteString(fmt.Sprintf("%s<div>\n", padding))
		for _, child := range elem.Children {
			r.renderElementToHTML(sb, child, depth+1)
		}
		sb.WriteString(fmt.Sprintf("%s</div>\n", padding))
	}
}
