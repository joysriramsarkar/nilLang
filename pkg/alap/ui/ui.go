package ui

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"github.com/joysriramsarkar/nilLang/pkg/nilui"
)

// Theme defines color and styling system for Alap UI
type Theme struct {
	Name            string `json:"name"`
	PrimaryColor    string `json:"primary_color"`
	SecondaryColor  string `json:"secondary_color"`
	BackgroundColor string `json:"background_color"`
	SurfaceColor    string `json:"surface_color"`
	TextColor       string `json:"text_color"`
	AccentColor     string `json:"accent_color"`
	BorderRadius    int    `json:"border_radius"`
}

// Standard Themes
func DefaultTheme() Theme {
	return Theme{
		Name:            "Default",
		PrimaryColor:    "#005A9C",
		SecondaryColor:  "#00d4ff",
		BackgroundColor: "#0f172a",
		SurfaceColor:    "#1e293b",
		TextColor:       "#f8fafc",
		AccentColor:     "#38bdf8",
		BorderRadius:    8,
	}
}

func OnuronTheme() Theme {
	return Theme{
		Name:            "Onuron OS",
		PrimaryColor:    "#00d4ff",
		SecondaryColor:  "#1a1a2e",
		BackgroundColor: "#0a0a0f",
		SurfaceColor:    "#161622",
		TextColor:       "#ffffff",
		AccentColor:     "#00ffcc",
		BorderRadius:    12,
	}
}

// Component is the Alap high-level UI component interface
type Component interface {
	Build(theme Theme) nilui.Primitive
	RenderANSI(theme Theme) string
	RenderHTML(theme Theme) string
}

// ─── PAGE COMPONENT ─────────────────────────────────────────────────────────

type Page struct {
	Title      string
	Navigation *Navigation
	Content    []Component
	FooterText string
}

func NewPage(title string) *Page {
	return &Page{
		Title:   title,
		Content: []Component{},
	}
}

func (p *Page) SetNav(nav *Navigation) *Page {
	p.Navigation = nav
	return p
}

func (p *Page) Add(c Component) *Page {
	p.Content = append(p.Content, c)
	return p
}

func (p *Page) SetFooter(text string) *Page {
	p.FooterText = text
	return p
}

func (p *Page) Build(theme Theme) nilui.Primitive {
	layout := nilui.NewLayout("page-layout", nilui.LayoutColumn)

	// Header
	header := nilui.NewText("page-title", p.Title)
	header.Bold = true
	header.Color = theme.PrimaryColor
	layout.Add(header)

	// Content
	for _, c := range p.Content {
		layout.Add(c.Build(theme))
	}

	if p.FooterText != "" {
		footer := nilui.NewText("page-footer", p.FooterText)
		footer.Color = "#64748b"
		layout.Add(footer)
	}

	return layout
}

func (p *Page) RenderANSI(theme Theme) string {
	var sb strings.Builder
	sb.WriteString("\033[1;36m╔══════════════════════════════════════════════════════════════════╗\033[0m\n")
	sb.WriteString(fmt.Sprintf("\033[1;36m║  %s - Alap UI Page\033[0m\n", p.Title))
	sb.WriteString("\033[1;36m╠══════════════════════════════════════════════════════════════════╣\033[0m\n")

	if p.Navigation != nil {
		sb.WriteString(p.Navigation.RenderANSI(theme))
		sb.WriteString("\n")
	}

	for _, c := range p.Content {
		sb.WriteString(c.RenderANSI(theme))
		sb.WriteString("\n")
	}

	if p.FooterText != "" {
		sb.WriteString(fmt.Sprintf("\033[90m— %s —\033[0m\n", p.FooterText))
	}
	sb.WriteString("\033[1;36m╚══════════════════════════════════════════════════════════════════╝\033[0m\n")
	return sb.String()
}

func (p *Page) RenderHTML(theme Theme) string {
	return p.RenderSSR(theme, nil)
}

// RenderSSR renders the Page with state hydration script
func (p *Page) RenderSSR(theme Theme, state map[string]interface{}) string {
	var sb strings.Builder
	escapedTitle := html.EscapeString(p.Title)
	sb.WriteString(fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: %s; color: %s; margin: 0; padding: 24px; }
  .alap-card { background: %s; border-radius: %dpx; padding: 16px; margin-bottom: 16px; border: 1px solid rgba(255,255,255,0.1); }
  .alap-table { width: 100%%; border-collapse: collapse; margin: 16px 0; }
  .alap-table th, .alap-table td { padding: 10px 14px; text-align: left; border-bottom: 1px solid rgba(255,255,255,0.1); }
  .alap-table th { background: rgba(255,255,255,0.05); color: %s; }
  .alap-badge { display: inline-block; padding: 4px 8px; border-radius: 4px; font-size: 12px; font-weight: bold; background: %s; color: #fff; }
  .alap-btn { background: %s; color: white; border: none; padding: 10px 18px; border-radius: %dpx; cursor: pointer; font-weight: bold; }
  .alap-input { background: rgba(0,0,0,0.3); border: 1px solid rgba(255,255,255,0.2); color: white; padding: 10px; border-radius: 6px; width: 100%%; box-sizing: border-box; }
</style>
</head>
<body>
<div style="max-width: 900px; margin: 0 auto;">
  <h1 style="color:%s;">%s</h1>
`, escapedTitle, theme.BackgroundColor, theme.TextColor, theme.SurfaceColor, theme.BorderRadius, theme.AccentColor, theme.PrimaryColor, theme.PrimaryColor, theme.BorderRadius, theme.AccentColor, escapedTitle))

	if p.Navigation != nil {
		sb.WriteString(p.Navigation.RenderHTML(theme))
	}

	for _, c := range p.Content {
		sb.WriteString(c.RenderHTML(theme))
	}

	if p.FooterText != "" {
		sb.WriteString(fmt.Sprintf(`<footer style="margin-top:40px; color:#64748b; font-size:13px; text-align:center;">%s</footer>`, html.EscapeString(p.FooterText)))
	}

	// State Hydration Script
	if state != nil {
		stateJSON, err := json.Marshal(state)
		if err == nil {
			sb.WriteString(fmt.Sprintf("\n<script id=\"__NILANG_STATE__\" type=\"application/json\">%s</script>\n", string(stateJSON)))
			sb.WriteString("<script>\n")
			sb.WriteString("  window.__NILANG_INITIAL_STATE__ = JSON.parse(document.getElementById('__NILANG_STATE__').textContent);\n")
			sb.WriteString("</script>\n")
		}
	}

	sb.WriteString(`</div></body></html>`)
	return sb.String()
}

// ─── CARD COMPONENT ─────────────────────────────────────────────────────────

type Card struct {
	Title       string
	Description string
	Body        string
	ActionLabel string
	OnAction    func()
}

func NewCard(title, body string) *Card {
	return &Card{Title: title, Body: body}
}

func (c *Card) Build(theme Theme) nilui.Primitive {
	layout := nilui.NewLayout("card-layout", nilui.LayoutColumn)
	t := nilui.NewText("card-title", c.Title)
	t.Bold = true
	t.Color = theme.AccentColor
	layout.Add(t)

	b := nilui.NewText("card-body", c.Body)
	layout.Add(b)
	return layout
}

func (c *Card) RenderANSI(theme Theme) string {
	return fmt.Sprintf("  ┌─ \033[1m%s\033[0m\n  │  %s\n  └───────────────", c.Title, c.Body)
}

func (c *Card) RenderHTML(theme Theme) string {
	return fmt.Sprintf(`<div class="alap-card"><h3>%s</h3><p>%s</p></div>`, html.EscapeString(c.Title), html.EscapeString(c.Body))
}

// ─── NAVIGATION COMPONENT ───────────────────────────────────────────────────

type NavItem struct {
	Label string
	Path  string
}

type Navigation struct {
	Brand string
	Items []NavItem
}

func NewNavigation(brand string) *Navigation {
	return &Navigation{Brand: brand, Items: []NavItem{}}
}

func (n *Navigation) AddItem(label, path string) *Navigation {
	n.Items = append(n.Items, NavItem{Label: label, Path: path})
	return n
}

func (n *Navigation) Build(theme Theme) nilui.Primitive {
	row := nilui.NewLayout("nav-row", nilui.LayoutRow)
	b := nilui.NewText("nav-brand", n.Brand)
	b.Bold = true
	b.Color = theme.AccentColor
	row.Add(b)

	for _, item := range n.Items {
		it := nilui.NewText("nav-item-"+item.Path, item.Label)
		row.Add(it)
	}
	return row
}

func (n *Navigation) RenderANSI(theme Theme) string {
	var items []string
	for _, item := range n.Items {
		items = append(items, item.Label+" ("+item.Path+")")
	}
	return fmt.Sprintf("\033[33m[%s]\033[0m %s", n.Brand, strings.Join(items, " | "))
}

func (n *Navigation) RenderHTML(theme Theme) string {
	var sb strings.Builder
	sb.WriteString(`<nav style="display:flex; gap:20px; align-items:center; margin-bottom:24px; padding-bottom:12px; border-bottom:1px solid rgba(255,255,255,0.1);">`)
	sb.WriteString(fmt.Sprintf(`<strong style="font-size:18px; color:%s;">%s</strong>`, theme.AccentColor, html.EscapeString(n.Brand)))
	for _, item := range n.Items {
		sb.WriteString(fmt.Sprintf(`<a href="%s" style="color:%s; text-decoration:none;">%s</a>`, html.EscapeString(item.Path), theme.TextColor, html.EscapeString(item.Label)))
	}
	sb.WriteString(`</nav>`)
	return sb.String()
}

// ─── TABLE COMPONENT ────────────────────────────────────────────────────────

type Table struct {
	Headers []string
	Rows    [][]string
}

func NewTable(headers ...string) *Table {
	return &Table{Headers: headers, Rows: [][]string{}}
}

func (t *Table) AddRow(cells ...string) *Table {
	t.Rows = append(t.Rows, cells)
	return t
}

func (t *Table) Build(theme Theme) nilui.Primitive {
	col := nilui.NewLayout("table-layout", nilui.LayoutColumn)
	headText := nilui.NewText("th", strings.Join(t.Headers, " | "))
	headText.Bold = true
	col.Add(headText)

	for _, r := range t.Rows {
		col.Add(nilui.NewText("tr", strings.Join(r, " | ")))
	}
	return col
}

func (t *Table) RenderANSI(theme Theme) string {
	var sb strings.Builder
	sb.WriteString("\033[1m")
	sb.WriteString(strings.Join(t.Headers, "\t│ "))
	sb.WriteString("\033[0m\n")
	sb.WriteString(strings.Repeat("─", 40))
	sb.WriteByte('\n')
	for _, r := range t.Rows {
		sb.WriteString(strings.Join(r, "\t│ "))
		sb.WriteByte('\n')
	}
	return sb.String()
}

func (t *Table) RenderHTML(theme Theme) string {
	var sb strings.Builder
	sb.WriteString(`<table class="alap-table"><thead><tr>`)
	for _, h := range t.Headers {
		sb.WriteString(fmt.Sprintf(`<th>%s</th>`, html.EscapeString(h)))
	}
	sb.WriteString(`</tr></thead><tbody>`)
	for _, r := range t.Rows {
		sb.WriteString(`<tr>`)
		for _, c := range r {
			sb.WriteString(fmt.Sprintf(`<td>%s</td>`, html.EscapeString(c)))
		}
		sb.WriteString(`</tr>`)
	}
	sb.WriteString(`</tbody></table>`)
	return sb.String()
}

// ─── FORM COMPONENT ─────────────────────────────────────────────────────────

type FormField struct {
	Label       string
	Name        string
	Placeholder string
	Value       string
}

type Form struct {
	Title       string
	Fields      []FormField
	SubmitLabel string
}

func NewForm(title string) *Form {
	return &Form{
		Title:       title,
		Fields:      []FormField{},
		SubmitLabel: "Submit",
	}
}

func (f *Form) AddField(label, name, placeholder string) *Form {
	f.Fields = append(f.Fields, FormField{
		Label:       label,
		Name:        name,
		Placeholder: placeholder,
	})
	return f
}

func (f *Form) Build(theme Theme) nilui.Primitive {
	col := nilui.NewLayout("form-layout", nilui.LayoutColumn)
	head := nilui.NewText("form-title", f.Title)
	head.Bold = true
	col.Add(head)

	for _, fld := range f.Fields {
		col.Add(nilui.NewText("lbl-"+fld.Name, fld.Label))
		col.Add(nilui.NewInput("fld-"+fld.Name, fld.Placeholder))
	}
	return col
}

func (f *Form) RenderANSI(theme Theme) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\033[1;33m[Form: %s]\033[0m\n", f.Title))
	for _, fld := range f.Fields {
		sb.WriteString(fmt.Sprintf("  %s: [%s]\n", fld.Label, fld.Placeholder))
	}
	sb.WriteString(fmt.Sprintf("  <Button: %s>\n", f.SubmitLabel))
	return sb.String()
}

func (f *Form) RenderHTML(theme Theme) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<div class="alap-card"><h3>%s</h3><form style="display:flex; flex-direction:column; gap:12px;">`, html.EscapeString(f.Title)))
	for _, fld := range f.Fields {
		sb.WriteString(fmt.Sprintf(`<div><label style="display:block; margin-bottom:4px; font-size:14px;">%s</label><input class="alap-input" name="%s" placeholder="%s" /></div>`,
			html.EscapeString(fld.Label), html.EscapeString(fld.Name), html.EscapeString(fld.Placeholder)))
	}
	sb.WriteString(fmt.Sprintf(`<button type="submit" class="alap-btn">%s</button></form></div>`, html.EscapeString(f.SubmitLabel)))
	return sb.String()
}

// ─── DASHBOARD & DATAVIEW ───────────────────────────────────────────────────

type MetricCard struct {
	Label string
	Value string
	Delta string
}

type Dashboard struct {
	Title   string
	Metrics []MetricCard
}

func NewDashboard(title string) *Dashboard {
	return &Dashboard{Title: title, Metrics: []MetricCard{}}
}

func (d *Dashboard) AddMetric(label, value, delta string) *Dashboard {
	d.Metrics = append(d.Metrics, MetricCard{Label: label, Value: value, Delta: delta})
	return d
}

func (d *Dashboard) Build(theme Theme) nilui.Primitive {
	row := nilui.NewLayout("dash-metrics", nilui.LayoutRow)
	for _, m := range d.Metrics {
		t := nilui.NewText("metric", fmt.Sprintf("%s: %s (%s)", m.Label, m.Value, m.Delta))
		row.Add(t)
	}
	return row
}

func (d *Dashboard) RenderANSI(theme Theme) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\033[1;35m📊 %s\033[0m\n", d.Title))
	for _, m := range d.Metrics {
		sb.WriteString(fmt.Sprintf("  • %s: \033[1;32m%s\033[0m (%s)\n", m.Label, m.Value, m.Delta))
	}
	return sb.String()
}

func (d *Dashboard) RenderHTML(theme Theme) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<h2>%s</h2><div style="display:grid; grid-template-columns:repeat(auto-fit, minmax(200px, 1fr)); gap:16px; margin-bottom:24px;">`, html.EscapeString(d.Title)))
	for _, m := range d.Metrics {
		sb.WriteString(fmt.Sprintf(`<div class="alap-card"><div style="color:#94a3b8; font-size:13px;">%s</div><div style="font-size:24px; font-weight:bold; margin:6px 0;">%s</div><div style="color:#4ade80; font-size:12px;">%s</div></div>`,
			html.EscapeString(m.Label), html.EscapeString(m.Value), html.EscapeString(m.Delta)))
	}
	sb.WriteString(`</div>`)
	return sb.String()
}
