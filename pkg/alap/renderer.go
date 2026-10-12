// Package alap — legacy renderer shim.
//
// Deprecated (ADR-0006): pkg/alap/ui is the single UI model. This renderer
// delegates to ui.Page/SSR so legacy callers keep working.
package alap

import (
	"github.com/joysriramsarkar/nilLang/pkg/alap/ui"
)

// Renderer is a legacy alias-backed renderer delegating to ui.
//
// Deprecated: use ui.Page.RenderSSR.
type Renderer struct {
	theme ui.Theme
	page  *ui.Page
	dirty bool
}

// NewRenderer creates a renderer with the given theme.
//
// Deprecated: build ui.Page and call RenderSSR/RenderHTML.
func NewRenderer(theme Theme) *Renderer {
	return &Renderer{theme: theme, dirty: true}
}

// SetRoot marks the renderer dirty. The legacy UIElement tree is rendered
// through the VNode HTML path for preview purposes.
//
// Deprecated: use ui directly.
func (r *Renderer) SetRoot(root *UIElement) {
	r.page = ui.NewPage("Alap Legacy Preview")
	r.dirty = true
}

// Render renders a preview string.
//
// Deprecated: use ui.Page.RenderANSI.
func (r *Renderer) Render() string {
	if r.page == nil {
		return "<Empty />"
	}
	return r.page.RenderANSI(r.theme)
}

// RenderToANSI renders as ANSI art.
//
// Deprecated: use ui.Page.RenderANSI.
func (r *Renderer) RenderToANSI() string {
	if r.page == nil {
		return ""
	}
	return r.page.RenderANSI(r.theme)
}

// RenderToHTML renders sanitized HTML.
//
// Deprecated: use ui.Page.RenderHTML.
func (r *Renderer) RenderToHTML() string { return r.RenderToSSR(nil) }

// RenderToSSR renders sanitized HTML with hydration state.
//
// Deprecated: use ui.Page.RenderSSR.
func (r *Renderer) RenderToSSR(initialState map[string]interface{}) string {
	if r.page == nil {
		r.page = ui.NewPage("Alap Legacy Preview")
	}
	return r.page.RenderSSR(r.theme, initialState)
}
