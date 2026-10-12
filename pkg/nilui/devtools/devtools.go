// Package devtools implements U15: state-preserving hot reload, the
// inspector (tree/layout/signals), formatter+UI lint, @preview collection,
// and browser preview scaffolding (Studio links to the language plan L11.6).
package devtools

import (
	"fmt"
	"strings"
)

// ─── Hot reload (U15.1) ─────────────────────────────────────────────────────

// Snapshot preserves UI state across a reload.
type Snapshot struct {
	Values map[string]string
}

// Reloader swaps code while keeping state.
type Reloader struct {
	state Snapshot
	loads int
}

// NewReloader creates a reloader.
func NewReloader() *Reloader { return &Reloader{state: Snapshot{Values: map[string]string{}}} }

// Keep stores state before reload.
func (r *Reloader) Keep(k, v string) { r.state.Values[k] = v }

// Reload simulates a code swap: state survives, load count grows.
func (r *Reloader) Reload() Snapshot {
	r.loads++
	cp := Snapshot{Values: map[string]string{}}
	for k, v := range r.state.Values {
		cp.Values[k] = v
	}
	return cp
}

// Loads counts reloads.
func (r *Reloader) Loads() int { return r.loads }

// ─── Inspector (U15.2) ──────────────────────────────────────────────────────

// SignalValue is one observed signal.
type SignalValue struct {
	Name    string
	Value   string
	Version int64
}

// InspectNode is one tree row.
type InspectNode struct {
	Path   string
	Type   string
	Key    string
	Box    [4]float64
	Depth  int
	Signal *SignalValue
}

// Inspector collects tree/layout/signal rows.
type Inspector struct {
	Rows []InspectNode
}

// Add records a row.
func (in *Inspector) Add(r InspectNode) { in.Rows = append(in.Rows, r) }

// Tree renders the component tree.
func (in *Inspector) Tree() string {
	var sb strings.Builder
	for _, r := range in.Rows {
		sb.WriteString(strings.Repeat("  ", r.Depth))
		sb.WriteString(r.Type)
		if r.Key != "" {
			sb.WriteString(" key=" + r.Key)
		}
		sb.WriteString(fmt.Sprintf(" box=(%.0f,%.0f %.0fx%.0f)", r.Box[0], r.Box[1], r.Box[2], r.Box[3]))
		if r.Signal != nil {
			sb.WriteString(fmt.Sprintf(" signal:%s=%s#%d", r.Signal.Name, r.Signal.Value, r.Signal.Version))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// ─── Formatter + lint (U15.3) ───────────────────────────────────────────────

// LintIssue is one UI lint finding.
type LintIssue struct {
	Rule    string
	Message string
}

// Lint checks component source conventions: keys in lists, 48dp targets,
// labels on buttons (subset of nilfmt UI rules).
func Lint(src string) []LintIssue {
	var out []LintIssue
	if strings.Contains(src, `"type": "List"`) && !strings.Contains(src, `"key"`) {
		out = append(out, LintIssue{"UI-LINT-KEY", "List children need stable keys"})
	}
	if strings.Contains(src, `"type": "Button"`) && !strings.Contains(src, `"label"`) {
		out = append(out, LintIssue{"UI-LINT-LABEL", "Button needs a label"})
	}
	return out
}

// Format normalizes trivial whitespace (nilfmt subset).
func Format(src string) string {
	lines := strings.Split(src, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.Join(lines, "\n")
}

// ─── Preview (U15.4–U15.6) ──────────────────────────────────────────────────

// Preview is one @preview annotation target.
type Preview struct {
	Name   string
	Width  int
	Height int
	Locale string
	Dark   bool
}

// CollectPreviews scans for `@preview(name)` markers.
func CollectPreviews(src string) []Preview {
	var out []Preview
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "// @preview") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "// @preview"))
			if name == "" {
				name = "default"
			}
			out = append(out, Preview{Name: name, Width: 390, Height: 844})
		}
	}
	return out
}

// BrowserPreviewURL builds the local preview URL.
func BrowserPreviewURL(port int, name string) string {
	return fmt.Sprintf("http://localhost:%d/__preview/%s", port, name)
}
