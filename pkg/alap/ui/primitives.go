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

// ─── NUMBER INPUT PRIMITIVE ─────────────────────────────────────────────────

// NumberInput represents a stepper numeric input with controls
type NumberInput struct {
	ID       string
	Label    string
	Value    float64
	Min      float64
	Max      float64
	Step     float64
	OnChange string
}

func NewNumberInput(id, label string, val, min, max, step float64) *NumberInput {
	return &NumberInput{ID: id, Label: label, Value: val, Min: min, Max: max, Step: step}
}

func (ni *NumberInput) Build(theme Theme) nilui.Primitive {
	return nilui.NewInput(ni.ID, fmt.Sprintf("%.2f", ni.Value))
}

func (ni *NumberInput) RenderANSI(theme Theme) string {
	return fmt.Sprintf("  %s: [ - | %.2f | + ]", ni.Label, ni.Value)
}

func (ni *NumberInput) RenderHTML(theme Theme) string {
	changeAttr := ""
	if ni.OnChange != "" {
		changeAttr = fmt.Sprintf(` data-alap-change="%s"`, html.EscapeString(ni.OnChange))
	}
	return fmt.Sprintf(`<div class="alap-number-input" style="display:inline-flex; align-items:center; gap:6px;"><button type="button" class="alap-btn-step" onclick="var el=document.getElementById('%s'); el.stepDown(); el.dispatchEvent(new Event('change'))" style="background:#334155; border:none; color:#fff; width:32px; height:32px; border-radius:6px; cursor:pointer; font-weight:bold;">-</button><input id="%s" type="number" value="%.2f" min="%.2f" max="%.2f" step="%.2f" class="alap-input" style="width:80px; text-align:center; padding:6px;"%s /><button type="button" class="alap-btn-step" onclick="var el=document.getElementById('%s'); el.stepUp(); el.dispatchEvent(new Event('change'))" style="background:#334155; border:none; color:#fff; width:32px; height:32px; border-radius:6px; cursor:pointer; font-weight:bold;">+</button></div>`,
		html.EscapeString(ni.ID), html.EscapeString(ni.ID), ni.Value, ni.Min, ni.Max, ni.Step, changeAttr, html.EscapeString(ni.ID))
}

// ─── BADGE PRIMITIVE ────────────────────────────────────────────────────────

// Badge represents a visual status pill (success, warning, danger, info, neutral)
type Badge struct {
	Label   string
	Variant string // "success", "warning", "danger", "info", "neutral"
}

func NewBadge(label, variant string) *Badge {
	return &Badge{Label: label, Variant: variant}
}

func (b *Badge) Build(theme Theme) nilui.Primitive {
	return nilui.NewText("badge", b.Label)
}

func (b *Badge) RenderANSI(theme Theme) string {
	return fmt.Sprintf("(%s)", b.Label)
}

func (b *Badge) RenderHTML(theme Theme) string {
	bg := "rgba(56, 189, 248, 0.15)"
	fg := "#38bdf8"
	switch b.Variant {
	case "success":
		bg = "rgba(74, 222, 128, 0.15)"
		fg = "#4ade80"
	case "warning":
		bg = "rgba(250, 204, 21, 0.15)"
		fg = "#facc15"
	case "danger":
		bg = "rgba(248, 113, 113, 0.15)"
		fg = "#f87171"
	}
	return fmt.Sprintf(`<span class="alap-badge" style="display:inline-block; padding:3px 10px; border-radius:12px; font-size:12px; font-weight:600; background:%s; color:%s;">%s</span>`,
		bg, fg, html.EscapeString(b.Label))
}

// ─── TABLE / DATAGRID PRIMITIVE ─────────────────────────────────────────────

// TableColumn defines a column in a DataGrid
type TableColumn struct {
	Key      string
	Title    string
	Sortable bool
	Align    string // "left", "center", "right"
}

// DataGrid represents a responsive, structured tabular view
type DataGrid struct {
	ID         string
	Columns    []TableColumn
	Rows       []map[string]string
	PageNumber int
	TotalPages int
}

func NewDataGrid(id string, columns []TableColumn) *DataGrid {
	return &DataGrid{
		ID:         id,
		Columns:    columns,
		Rows:       make([]map[string]string, 0),
		PageNumber: 1,
		TotalPages: 1,
	}
}

func (dg *DataGrid) AddRow(row map[string]string) *DataGrid {
	dg.Rows = append(dg.Rows, row)
	return dg
}

func (dg *DataGrid) Build(theme Theme) nilui.Primitive {
	col := nilui.NewLayout(dg.ID, nilui.LayoutColumn)
	for _, r := range dg.Rows {
		col.Add(nilui.NewText("row", fmt.Sprintf("%v", r)))
	}
	return col
}

func (dg *DataGrid) RenderANSI(theme Theme) string {
	var sb strings.Builder
	for _, c := range dg.Columns {
		sb.WriteString(fmt.Sprintf("%s\t│ ", c.Title))
	}
	sb.WriteString("\n────────────────────────────────────────\n")
	for _, r := range dg.Rows {
		for _, c := range dg.Columns {
			sb.WriteString(fmt.Sprintf("%s\t│ ", r[c.Key]))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func (dg *DataGrid) RenderHTML(theme Theme) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<div class="alap-table-wrapper" style="overflow-x:auto; border:1px solid rgba(255,255,255,0.1); border-radius:%dpx;">`, theme.BorderRadius))
	sb.WriteString(`<table class="alap-table" style="width:100%; border-collapse:collapse; font-size:14px; text-align:left;">`)
	sb.WriteString(`<thead style="background:rgba(255,255,255,0.06);"><tr>`)
	for _, c := range dg.Columns {
		align := "left"
		if c.Align != "" {
			align = c.Align
		}
		sb.WriteString(fmt.Sprintf(`<th style="padding:12px 16px; border-bottom:1px solid rgba(255,255,255,0.1); color:#94a3b8; text-align:%s;">%s</th>`, align, html.EscapeString(c.Title)))
	}
	sb.WriteString(`</tr></thead><tbody>`)
	for i, r := range dg.Rows {
		bg := "transparent"
		if i%2 == 1 {
			bg = "rgba(255,255,255,0.02)"
		}
		sb.WriteString(fmt.Sprintf(`<tr style="background:%s; border-bottom:1px solid rgba(255,255,255,0.05);">`, bg))
		for _, c := range dg.Columns {
			align := "left"
			if c.Align != "" {
				align = c.Align
			}
			sb.WriteString(fmt.Sprintf(`<td style="padding:12px 16px; text-align:%s;">%s</td>`, align, html.EscapeString(r[c.Key])))
		}
		sb.WriteString(`</tr>`)
	}
	sb.WriteString(`</tbody></table></div>`)
	return sb.String()
}

// ─── TABS PRIMITIVE ─────────────────────────────────────────────────────────

// TabItem represents an individual tab title and child content
type TabItem struct {
	ID      string
	Title   string
	Content Component
}

// Tabs represents a multi-tab panel switcher
type Tabs struct {
	ID        string
	Items     []TabItem
	ActiveTab string
}

func NewTabs(id string) *Tabs {
	return &Tabs{ID: id, Items: make([]TabItem, 0)}
}

func (t *Tabs) AddTab(id, title string, content Component) *Tabs {
	t.Items = append(t.Items, TabItem{ID: id, Title: title, Content: content})
	if t.ActiveTab == "" {
		t.ActiveTab = id
	}
	return t
}

func (t *Tabs) Build(theme Theme) nilui.Primitive {
	col := nilui.NewLayout(t.ID, nilui.LayoutColumn)
	for _, item := range t.Items {
		if item.Content != nil {
			col.Add(item.Content.Build(theme))
		}
	}
	return col
}

func (t *Tabs) RenderANSI(theme Theme) string {
	var sb strings.Builder
	for _, item := range t.Items {
		if item.ID == t.ActiveTab {
			sb.WriteString(fmt.Sprintf("\033[1;36m[%s]\033[0m ", item.Title))
		} else {
			sb.WriteString(fmt.Sprintf("[%s] ", item.Title))
		}
	}
	sb.WriteString("\n")
	for _, item := range t.Items {
		if item.ID == t.ActiveTab && item.Content != nil {
			sb.WriteString(item.Content.RenderANSI(theme))
		}
	}
	return sb.String()
}

func (t *Tabs) RenderHTML(theme Theme) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<div class="alap-tabs-container" id="%s">`, html.EscapeString(t.ID)))
	sb.WriteString(`<div class="alap-tab-headers" style="display:flex; border-bottom:1px solid rgba(255,255,255,0.1); margin-bottom:16px;">`)
	for _, item := range t.Items {
		activeBorder := "border-bottom:2px solid transparent; color:#94a3b8;"
		if item.ID == t.ActiveTab {
			activeBorder = fmt.Sprintf("border-bottom:2px solid %s; color:#fff; font-weight:bold;", theme.AccentColor)
		}
		sb.WriteString(fmt.Sprintf(`<button class="alap-tab-btn" onclick="document.querySelectorAll('#%s .alap-tab-pane').forEach(p=>p.style.display='none'); document.getElementById('tab-%s').style.display='block';" style="background:none; border:none; padding:10px 18px; cursor:pointer; font-size:14px; %s">%s</button>`,
			html.EscapeString(t.ID), html.EscapeString(item.ID), activeBorder, html.EscapeString(item.Title)))
	}
	sb.WriteString(`</div>`)

	for _, item := range t.Items {
		display := "none"
		if item.ID == t.ActiveTab {
			display = "block"
		}
		sb.WriteString(fmt.Sprintf(`<div id="tab-%s" class="alap-tab-pane" style="display:%s;">`, html.EscapeString(item.ID), display))
		if item.Content != nil {
			sb.WriteString(item.Content.RenderHTML(theme))
		}
		sb.WriteString(`</div>`)
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

// ─── DATE PICKER PRIMITIVE ──────────────────────────────────────────────────

// DatePicker represents a calendar date input
type DatePicker struct {
	ID       string
	Label    string
	Value    string // YYYY-MM-DD
	OnChange string
}

func NewDatePicker(id, label, defaultDate string) *DatePicker {
	return &DatePicker{ID: id, Label: label, Value: defaultDate}
}

func (dp *DatePicker) Build(theme Theme) nilui.Primitive {
	return nilui.NewInput(dp.ID, dp.Value)
}

func (dp *DatePicker) RenderANSI(theme Theme) string {
	return fmt.Sprintf("  %s: [%s 📅]", dp.Label, dp.Value)
}

func (dp *DatePicker) RenderHTML(theme Theme) string {
	changeAttr := ""
	if dp.OnChange != "" {
		changeAttr = fmt.Sprintf(` data-alap-change="%s"`, html.EscapeString(dp.OnChange))
	}
	return fmt.Sprintf(`<div class="alap-datepicker-group" style="margin-bottom:12px;"><label style="display:block; font-size:13px; margin-bottom:4px; color:#94a3b8;">%s</label><input id="%s" type="date" value="%s" class="alap-input" style="padding:8px 12px;"%s /></div>`,
		html.EscapeString(dp.Label), html.EscapeString(dp.ID), html.EscapeString(dp.Value), changeAttr)
}

// ─── CART PANEL UI PRIMITIVE ────────────────────────────────────────────────

// CartLineItemUI represents an item in the cart panel
type CartLineItemUI struct {
	ProductID string
	Name      string
	Quantity  string
	PriceStr  string
	TotalStr  string
}

// CartPanelUI represents the real-time POS cart display
type CartPanelUI struct {
	Items         []CartLineItemUI
	SubtotalStr   string
	DiscountStr   string
	TaxStr        string
	GrandTotalStr string
	OnCheckout    string
	OnClear       string
}

func (cp *CartPanelUI) Build(theme Theme) nilui.Primitive {
	col := nilui.NewLayout("cart-panel", nilui.LayoutColumn)
	col.Add(nilui.NewText("total", "Total: "+cp.GrandTotalStr))
	return col
}

func (cp *CartPanelUI) RenderANSI(theme Theme) string {
	var sb strings.Builder
	sb.WriteString("\n🛒 Cart Summary\n───────────────────────────────\n")
	for _, it := range cp.Items {
		sb.WriteString(fmt.Sprintf("  %s x %s = %s\n", it.Name, it.Quantity, it.TotalStr))
	}
	sb.WriteString(fmt.Sprintf("───────────────────────────────\nGrand Total: %s\n", cp.GrandTotalStr))
	return sb.String()
}

func (cp *CartPanelUI) RenderHTML(theme Theme) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<div class="alap-cart-panel" style="background:%s; border-radius:%dpx; border:1px solid rgba(255,255,255,0.1); padding:16px; display:flex; flex-direction:column; height:100%%;">`,
		theme.SurfaceColor, theme.BorderRadius))
	sb.WriteString(`<h3 style="margin-top:0; margin-bottom:14px; font-size:16px; display:flex; align-items:center; gap:8px;">🛒 Cart Items</h3>`)
	sb.WriteString(`<div class="cart-items-list" style="flex:1; overflow-y:auto; margin-bottom:16px;">`)

	if len(cp.Items) == 0 {
		sb.WriteString(`<div style="color:#64748b; text-align:center; padding:30px 0;">Cart is empty</div>`)
	} else {
		for _, it := range cp.Items {
			sb.WriteString(fmt.Sprintf(`<div style="display:flex; justify-content:space-between; align-items:center; padding:8px 0; border-bottom:1px solid rgba(255,255,255,0.05);"><div style="font-size:14px;"><strong>%s</strong><div style="font-size:12px; color:#94a3b8;">%s @ %s</div></div><div style="font-weight:bold; color:#38bdf8;">%s</div></div>`,
				html.EscapeString(it.Name), html.EscapeString(it.Quantity), html.EscapeString(it.PriceStr), html.EscapeString(it.TotalStr)))
		}
	}
	sb.WriteString(`</div>`)

	// Totals block
	sb.WriteString(`<div class="cart-totals" style="border-top:1px solid rgba(255,255,255,0.1); padding-top:12px; margin-bottom:16px;">`)
	sb.WriteString(fmt.Sprintf(`<div style="display:flex; justify-content:space-between; margin-bottom:4px; font-size:13px; color:#94a3b8;"><span>Subtotal:</span><span>%s</span></div>`, html.EscapeString(cp.SubtotalStr)))
	if cp.DiscountStr != "" && cp.DiscountStr != "৳0.00" {
		sb.WriteString(fmt.Sprintf(`<div style="display:flex; justify-content:space-between; margin-bottom:4px; font-size:13px; color:#f87171;"><span>Discount:</span><span>-%s</span></div>`, html.EscapeString(cp.DiscountStr)))
	}
	sb.WriteString(fmt.Sprintf(`<div style="display:flex; justify-content:space-between; margin-bottom:8px; font-size:13px; color:#94a3b8;"><span>Tax/VAT:</span><span>%s</span></div>`, html.EscapeString(cp.TaxStr)))
	sb.WriteString(fmt.Sprintf(`<div style="display:flex; justify-content:space-between; font-size:18px; font-weight:bold; color:#4ade80;"><span>Grand Total:</span><span>%s</span></div>`, html.EscapeString(cp.GrandTotalStr)))
	sb.WriteString(`</div>`)

	// Action buttons
	checkoutAttr := ""
	if cp.OnCheckout != "" {
		checkoutAttr = fmt.Sprintf(` data-alap-click="%s"`, html.EscapeString(cp.OnCheckout))
	}
	sb.WriteString(fmt.Sprintf(`<button class="alap-btn alap-btn-primary"%s style="width:100%%; padding:12px; font-size:16px; font-weight:bold; border-radius:8px; background:#00d4ff; color:#000; border:none; cursor:pointer;">Proceed to Checkout</button>`, checkoutAttr))
	sb.WriteString(`</div>`)
	return sb.String()
}

// ─── RECEIPT PREVIEW UI ─────────────────────────────────────────────────────

// ReceiptPreviewUI displays a digital paper simulation of an ESC/POS thermal receipt
type ReceiptPreviewUI struct {
	StoreName     string
	StoreSubname  string
	InvoiceNumber string
	DateStr       string
	CashierName   string
	Lines         []string
	GrandTotalStr string
	PaidStr       string
	ChangeStr     string
}

func (rp *ReceiptPreviewUI) Build(theme Theme) nilui.Primitive {
	col := nilui.NewLayout("receipt-preview", nilui.LayoutColumn)
	col.Add(nilui.NewText("store", rp.StoreName))
	return col
}

func (rp *ReceiptPreviewUI) RenderANSI(theme Theme) string {
	return fmt.Sprintf("Receipt #%s | %s | %s", rp.InvoiceNumber, rp.StoreName, rp.GrandTotalStr)
}

func (rp *ReceiptPreviewUI) RenderHTML(theme Theme) string {
	var sb strings.Builder
	sb.WriteString(`<div class="alap-receipt-paper" style="background:#fff; color:#000; font-family:'Courier New', monospace; padding:24px 20px; width:280px; margin:auto; border-radius:4px; box-shadow:0 4px 15px rgba(0,0,0,0.3); font-size:12px;">`)
	sb.WriteString(fmt.Sprintf(`<div style="text-align:center; font-weight:bold; font-size:16px;">%s</div>`, html.EscapeString(rp.StoreName)))
	if rp.StoreSubname != "" {
		sb.WriteString(fmt.Sprintf(`<div style="text-align:center; font-size:11px; margin-bottom:10px;">%s</div>`, html.EscapeString(rp.StoreSubname)))
	}
	sb.WriteString(`<div style="border-top:1px dashed #000; margin:8px 0;"></div>`)
	sb.WriteString(fmt.Sprintf(`<div>Inv: %s</div><div>Date: %s</div><div>Cashier: %s</div>`,
		html.EscapeString(rp.InvoiceNumber), html.EscapeString(rp.DateStr), html.EscapeString(rp.CashierName)))
	sb.WriteString(`<div style="border-top:1px dashed #000; margin:8px 0;"></div>`)

	for _, line := range rp.Lines {
		sb.WriteString(fmt.Sprintf(`<div>%s</div>`, html.EscapeString(line)))
	}

	sb.WriteString(`<div style="border-top:1px dashed #000; margin:8px 0;"></div>`)
	sb.WriteString(fmt.Sprintf(`<div style="display:flex; justify-content:space-between; font-weight:bold; font-size:14px;"><span>TOTAL:</span><span>%s</span></div>`, html.EscapeString(rp.GrandTotalStr)))
	sb.WriteString(fmt.Sprintf(`<div style="display:flex; justify-content:space-between;"><span>PAID:</span><span>%s</span></div>`, html.EscapeString(rp.PaidStr)))
	sb.WriteString(fmt.Sprintf(`<div style="display:flex; justify-content:space-between;"><span>CHANGE:</span><span>%s</span></div>`, html.EscapeString(rp.ChangeStr)))
	sb.WriteString(`<div style="border-top:1px dashed #000; margin:12px 0 8px 0;"></div>`)
	sb.WriteString(`<div style="text-align:center; font-size:11px;">ধন্যবাদ! আবার আসবেন।</div>`)
	sb.WriteString(`</div>`)
	return sb.String()
}
