package ui

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"github.com/joysriramsarkar/nilLang/pkg/nilui"
)

// ─── DECLARATIVE UI PRIMITIVES (web-implications.md Section 14) ─────────────

// Button represents a modern clickable button
type Button struct {
	ID       string
	Label    string
	Variant  string // "primary", "secondary", "danger", "glass"
	Icon     string
	OnClick  string // NilLang component event name
	Payload  interface{}
	Disabled bool
}

func NewButton(id, label string) *Button {
	return &Button{ID: id, Label: label, Variant: "primary"}
}

func (b *Button) Build(theme Theme) nilui.Primitive {
	btn := nilui.NewButton(b.ID, b.Label)
	return btn
}

func (b *Button) RenderANSI(theme Theme) string {
	return fmt.Sprintf("\033[1;36m[%s %s]\033[0m", b.Icon, b.Label)
}

func (b *Button) RenderHTML(theme Theme) string {
	cls := "alap-btn"
	if b.Variant != "" {
		cls += " alap-btn-" + b.Variant
	}
	dis := ""
	if b.Disabled {
		dis = " disabled"
	}
	clickAttr := ""
	if b.OnClick != "" {
		clickAttr = fmt.Sprintf(` data-alap-click="%s"`, html.EscapeString(b.OnClick))
	}
	payloadAttr := ""
	if b.Payload != nil {
		if payload, err := json.Marshal(b.Payload); err == nil {
			payloadAttr = fmt.Sprintf(` data-alap-payload="%s"`, html.EscapeString(string(payload)))
		}
	}
	iconHtml := ""
	if b.Icon != "" {
		iconHtml = fmt.Sprintf(`<span class="btn-icon">%s</span> `, b.Icon)
	}
	return fmt.Sprintf(`<button id="%s" class="%s"%s%s%s>%s%s</button>`,
		html.EscapeString(b.ID), cls, clickAttr, payloadAttr, dis, iconHtml, html.EscapeString(b.Label))
}

// Input represents an editable text/search/numeric input
type InputType string

const (
	InputText   InputType = "text"
	InputNumber InputType = "number"
	InputMoney  InputType = "money"
	InputSearch InputType = "search"
)

type Input struct {
	ID          string
	Type        InputType
	Label       string
	Placeholder string
	Value       string
	Prefix      string
}

func NewInput(id string, inputType InputType, label, placeholder string) *Input {
	return &Input{
		ID:          id,
		Type:        inputType,
		Label:       label,
		Placeholder: placeholder,
	}
}

func (in *Input) Build(theme Theme) nilui.Primitive {
	return nilui.NewInput(in.ID, in.Placeholder)
}

func (in *Input) RenderANSI(theme Theme) string {
	return fmt.Sprintf("  %s: [%s %s]", in.Label, in.Prefix, in.Placeholder)
}

func (in *Input) RenderHTML(theme Theme) string {
	htmlType := "text"
	if in.Type == InputNumber {
		htmlType = "number"
	}
	var sb strings.Builder
	sb.WriteString(`<div class="alap-input-group" style="margin-bottom:12px;">`)
	if in.Label != "" {
		sb.WriteString(fmt.Sprintf(`<label for="%s" style="display:block; font-size:13px; margin-bottom:4px; color:#94a3b8;">%s</label>`,
			html.EscapeString(in.ID), html.EscapeString(in.Label)))
	}
	sb.WriteString(`<div style="position:relative; display:flex; align-items:center;">`)
	if in.Prefix != "" {
		sb.WriteString(fmt.Sprintf(`<span style="position:absolute; left:12px; color:#64748b; font-weight:bold;">%s</span>`, html.EscapeString(in.Prefix)))
	}
	padLeft := "10px"
	if in.Prefix != "" {
		padLeft = "32px"
	}
	sb.WriteString(fmt.Sprintf(`<input id="%s" type="%s" class="alap-input" placeholder="%s" value="%s" style="padding-left:%s;" />`,
		html.EscapeString(in.ID), htmlType, html.EscapeString(in.Placeholder), html.EscapeString(in.Value), padLeft))
	sb.WriteString(`</div></div>`)
	return sb.String()
}

// Modal represents a popup dialog or slide-out drawer
type Modal struct {
	ID       string
	Title    string
	Content  []Component
	Visible  bool
	IsDrawer bool
}

func NewModal(id, title string) *Modal {
	return &Modal{ID: id, Title: title, Content: make([]Component, 0)}
}

func (m *Modal) Add(c Component) *Modal {
	m.Content = append(m.Content, c)
	return m
}

func (m *Modal) Build(theme Theme) nilui.Primitive {
	col := nilui.NewLayout("modal-"+m.ID, nilui.LayoutColumn)
	col.Add(nilui.NewText("title", m.Title))
	for _, c := range m.Content {
		col.Add(c.Build(theme))
	}
	return col
}

func (m *Modal) RenderANSI(theme Theme) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\033[1;33m[Modal: %s]\033[0m\n", m.Title))
	for _, c := range m.Content {
		sb.WriteString(c.RenderANSI(theme))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (m *Modal) RenderHTML(theme Theme) string {
	var sb strings.Builder
	display := "none"
	if m.Visible {
		display = "flex"
	}
	sb.WriteString(fmt.Sprintf(`<div id="%s" class="alap-modal-overlay" style="display:%s; position:fixed; inset:0; background:rgba(0,0,0,0.6); align-items:center; justify-content:center; z-index:1000; backdrop-filter:blur(4px);">`,
		html.EscapeString(m.ID), display))
	sb.WriteString(fmt.Sprintf(`<div class="alap-card" style="width:90%%; max-width:550px; background:%s; border-radius:%dpx; border:1px solid rgba(255,255,255,0.15);">`,
		theme.SurfaceColor, theme.BorderRadius))
	sb.WriteString(fmt.Sprintf(`<div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;"><h3 style="margin:0; color:%s;">%s</h3><button class="btn-close" onclick="document.getElementById('%s').style.display='none'" style="background:none; border:none; color:#fff; font-size:18px; cursor:pointer;">&times;</button></div>`,
		theme.AccentColor, html.EscapeString(m.Title), html.EscapeString(m.ID)))

	for _, c := range m.Content {
		sb.WriteString(c.RenderHTML(theme))
	}
	sb.WriteString(`</div></div>`)
	return sb.String()
}

// ─── LAYOUT PRIMITIVES (web-implications.md Section 14) ─────────────────────

type SplitPane struct {
	Left      Component
	Right     Component
	LeftRatio int // e.g. 60 for 60/40
}

func NewSplitPane(left, right Component, leftRatio int) *SplitPane {
	if leftRatio <= 0 || leftRatio >= 100 {
		leftRatio = 50
	}
	return &SplitPane{Left: left, Right: right, LeftRatio: leftRatio}
}

func (sp *SplitPane) Build(theme Theme) nilui.Primitive {
	row := nilui.NewLayout("split-pane", nilui.LayoutRow)
	if sp.Left != nil {
		row.Add(sp.Left.Build(theme))
	}
	if sp.Right != nil {
		row.Add(sp.Right.Build(theme))
	}
	return row
}

func (sp *SplitPane) RenderANSI(theme Theme) string {
	var sb strings.Builder
	if sp.Left != nil {
		sb.WriteString(sp.Left.RenderANSI(theme))
	}
	sb.WriteString("\n  ──────────────[SPLIT]──────────────\n")
	if sp.Right != nil {
		sb.WriteString(sp.Right.RenderANSI(theme))
	}
	return sb.String()
}

func (sp *SplitPane) RenderHTML(theme Theme) string {
	rightRatio := 100 - sp.LeftRatio
	var sb strings.Builder
	sb.WriteString(`<div class="alap-split-pane" style="display:grid; grid-template-columns:`)
	sb.WriteString(fmt.Sprintf("%d%% %d%%; gap:16px; width:100%%;", sp.LeftRatio, rightRatio))
	sb.WriteString(`">`)
	sb.WriteString(`<div class="pane-left">`)
	if sp.Left != nil {
		sb.WriteString(sp.Left.RenderHTML(theme))
	}
	sb.WriteString(`</div><div class="pane-right">`)
	if sp.Right != nil {
		sb.WriteString(sp.Right.RenderHTML(theme))
	}
	sb.WriteString(`</div></div>`)
	return sb.String()
}

// ─── POS SPECIFIC DECLARATIVE COMPONENTS (web-implications.md Section 15-16) ─

// ProductCardUI represents a visual item in the POS catalog grid
type ProductCardUI struct {
	ID          string
	Name        string
	NameBn      string
	SKU         string
	PriceStr    string
	StockStr    string
	Unit        string
	Category    string
	Active      bool
	OnClickCode string
}

func (pc *ProductCardUI) Build(theme Theme) nilui.Primitive {
	col := nilui.NewLayout("prod-"+pc.ID, nilui.LayoutColumn)
	t := nilui.NewText("name", pc.Name)
	t.Bold = true
	col.Add(t)
	col.Add(nilui.NewText("price", pc.PriceStr))
	return col
}

func (pc *ProductCardUI) RenderANSI(theme Theme) string {
	return fmt.Sprintf("  [%s] %s - %s (Stock: %s)", pc.SKU, pc.Name, pc.PriceStr, pc.StockStr)
}

func (pc *ProductCardUI) RenderHTML(theme Theme) string {
	clickAttr := ""
	if pc.OnClickCode != "" {
		clickAttr = fmt.Sprintf(` onclick="%s"`, html.EscapeString(pc.OnClickCode))
	}
	nameDisplay := pc.Name
	if pc.NameBn != "" {
		nameDisplay = pc.NameBn
	}
	return fmt.Sprintf(`<div class="alap-product-card" id="card-%s"%s style="background:rgba(255,255,255,0.04); border:1px solid rgba(255,255,255,0.1); border-radius:10px; padding:14px; cursor:pointer; transition:all 0.15s ease;"><div style="font-weight:600; font-size:15px; margin-bottom:6px;">%s</div><div style="font-size:12px; color:#94a3b8; margin-bottom:8px;">%s • %s</div><div style="display:flex; justify-content:space-between; align-items:center;"><span style="font-weight:bold; font-size:16px; color:#38bdf8;">%s</span><span style="font-size:12px; color:#4ade80;">স্টক: %s</span></div></div>`,
		html.EscapeString(pc.ID), clickAttr, html.EscapeString(nameDisplay), html.EscapeString(pc.SKU), html.EscapeString(pc.Unit), html.EscapeString(pc.PriceStr), html.EscapeString(pc.StockStr))
}
