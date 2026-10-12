// Package components implements U7: the 54-component catalog.
//
// Foundation(6) + Layout(11) + Input(11) + Display(9) + Feedback(6) +
// Navigation(6) + Alap-special(5). Every entry carries its contract (props,
// slots, modifiers, events, states), an a11y role/label, lifecycle hooks,
// and theme-aware rendering. Nilang binding flows through
// cmd/nil convertHashToComponent (unknown → UI0140 diagnostic).
package components

import (
	"fmt"
	"html"
	"sort"
	"strings"

	"github.com/joysriramsarkar/nilLang/pkg/nilui/theme"
)

// Category groups the catalog.
type Category string

const (
	CatFoundation  Category = "Foundation"
	CatLayout      Category = "Layout"
	CatInput       Category = "Input"
	CatDisplay     Category = "Display"
	CatFeedback    Category = "Feedback"
	CatNavigation  Category = "Navigation"
	CatAlapSpecial Category = "AlapSpecial"
)

// Contract is the mandatory per-component contract (UI_SPEC.md §7).
type Contract struct {
	Type      string
	Category  Category
	Props     []string // required props (first element convention: primary)
	Slots     []string
	Modifiers []string
	Events    []string
	States    []string
	Role      string // a11y role
	LabelProp string // prop used as accessible label
}

// Def is a catalog entry with lifecycle + rendering.
type Def struct {
	Contract Contract
	mounted  int
}

// Mount runs mount lifecycle.
func (d *Def) Mount() { d.mounted++ }

// Unmount runs unmount lifecycle.
func (d *Def) Unmount() {
	if d.mounted > 0 {
		d.mounted--
	}
}

// Mounted reports live count.
func (d *Def) Mounted() int { return d.mounted }

// Validate checks required props (U7.3).
func (d *Def) Validate(props map[string]string) error {
	for _, p := range d.Contract.Props {
		if strings.HasSuffix(p, "?") {
			continue
		}
		if _, ok := props[p]; !ok {
			return fmt.Errorf("UI0102: %s missing required prop %q", d.Contract.Type, p)
		}
	}
	return nil
}

// A11yLabel derives the screen-reader label (U7 contract).
func (d *Def) A11yLabel(props map[string]string) string {
	if d.Contract.LabelProp != "" {
		if v, ok := props[d.Contract.LabelProp]; ok && v != "" {
			return v
		}
	}
	return d.Contract.Type
}

// RenderHTML renders theme-aware (U7.5: light/dark differ via roles).
func (d *Def) RenderHTML(props map[string]string, r theme.Roles) string {
	label := html.EscapeString(d.A11yLabel(props))
	return fmt.Sprintf(`<div role="%s" aria-label="%s" data-component="%s" style="background:%s;color:%s;">%s</div>`,
		d.Contract.Role, label, d.Contract.Type, r.Surface, r.OnSurface, label)
}

// Registry holds all 54.
var Registry = map[string]*Def{}

func reg(typ string, cat Category, props, slots, mods, evts, states []string, role, labelProp string) {
	Registry[typ] = &Def{Contract: Contract{
		Type: typ, Category: cat, Props: props, Slots: slots,
		Modifiers: mods, Events: evts, States: states, Role: role, LabelProp: labelProp,
	}}
}

func stdMods() []string { return []string{"padding", "margin", "background", "opacity"} }
func stdStates() []string {
	return []string{"default", "hover", "pressed", "disabled", "focused"}
}

func init() {
	// Foundation (6).
	reg("Page", CatFoundation, []string{"title"}, []string{"content"}, stdMods(), []string{"appear"}, []string{"default"}, "document", "title")
	reg("Text", CatFoundation, []string{"value"}, nil, stdMods(), nil, []string{"default"}, "text", "value")
	reg("RichText", CatFoundation, []string{"value"}, nil, stdMods(), nil, []string{"default"}, "text", "value")
	reg("Box", CatFoundation, nil, []string{"child"}, stdMods(), nil, []string{"default"}, "group", "")
	reg("Spacer", CatFoundation, nil, nil, []string{"size"}, nil, []string{"default"}, "separator", "")
	reg("Divider", CatFoundation, nil, nil, stdMods(), nil, []string{"default"}, "separator", "")
	// Layout (11).
	for _, t := range []string{"Column", "Row", "Stack", "Grid", "ScrollView", "SafeArea", "Padding", "Align", "SizedBox", "Flex", "Layout"} {
		reg(t, CatLayout, nil, []string{"children"}, append(stdMods(), "gap", "align"), []string{"layout"}, []string{"default"}, "group", "")
	}
	// Input (11).
	reg("Button", CatInput, []string{"label"}, nil, stdMods(), []string{"click"}, stdStates(), "button", "label")
	reg("IconButton", CatInput, []string{"icon"}, nil, stdMods(), []string{"click"}, stdStates(), "button", "icon")
	reg("TextField", CatInput, []string{"placeholder?"}, nil, stdMods(), []string{"change", "submit"}, append(stdStates(), "error"), "textbox", "placeholder?")
	reg("SearchField", CatInput, []string{"placeholder?"}, nil, stdMods(), []string{"change", "submit"}, stdStates(), "searchbox", "placeholder?")
	reg("PasswordField", CatInput, []string{"placeholder?"}, nil, stdMods(), []string{"change"}, append(stdStates(), "error"), "textbox", "placeholder?")
	reg("Checkbox", CatInput, []string{"label"}, nil, stdMods(), []string{"toggle"}, []string{"checked", "unchecked", "disabled"}, "checkbox", "label")
	reg("CheckboxGroup", CatInput, nil, []string{"options"}, stdMods(), []string{"change"}, stdStates(), "group", "")
	reg("RadioGroup", CatInput, nil, []string{"options"}, stdMods(), []string{"change"}, stdStates(), "radiogroup", "")
	reg("Switch", CatInput, []string{"label"}, nil, stdMods(), []string{"toggle"}, []string{"on", "off", "disabled"}, "switch", "label")
	reg("Slider", CatInput, []string{"value"}, nil, stdMods(), []string{"change"}, stdStates(), "slider", "value")
	reg("Stepper", CatInput, []string{"value"}, nil, stdMods(), []string{"increment", "decrement"}, stdStates(), "spinbutton", "value")
	// Display (9).
	reg("Image", CatDisplay, []string{"src"}, nil, stdMods(), nil, []string{"default", "loading", "error"}, "img", "alt?")
	reg("Icon", CatDisplay, []string{"name"}, nil, stdMods(), nil, []string{"default"}, "img", "name")
	reg("List", CatDisplay, nil, []string{"items"}, stdMods(), []string{"scroll"}, []string{"default"}, "list", "")
	reg("LazyColumn", CatDisplay, nil, []string{"items"}, stdMods(), []string{"scroll"}, []string{"default"}, "list", "")
	reg("LazyRow", CatDisplay, nil, []string{"items"}, stdMods(), []string{"scroll"}, []string{"default"}, "list", "")
	reg("DataGrid", CatDisplay, nil, []string{"columns", "rows"}, stdMods(), []string{"sort", "select"}, []string{"default"}, "grid", "")
	reg("ProgressIndicator", CatDisplay, []string{"value?"}, nil, stdMods(), nil, []string{"determinate", "indeterminate"}, "progressbar", "")
	reg("ActivityIndicator", CatDisplay, nil, nil, stdMods(), nil, []string{"spinning"}, "status", "")
	reg("Badge", CatDisplay, []string{"label"}, nil, stdMods(), nil, []string{"default"}, "status", "label")
	// Feedback (6).
	reg("Dialog", CatFeedback, []string{"title"}, []string{"content", "actions"}, stdMods(), []string{"dismiss", "confirm"}, []string{"open", "closed"}, "dialog", "title")
	reg("Modal", CatFeedback, []string{"title"}, []string{"content"}, stdMods(), []string{"dismiss"}, []string{"open", "closed"}, "dialog", "title")
	reg("BottomSheet", CatFeedback, []string{"title?"}, []string{"content"}, stdMods(), []string{"dismiss", "expand"}, []string{"collapsed", "expanded"}, "dialog", "title?")
	reg("Snackbar", CatFeedback, []string{"message"}, []string{"action?"}, stdMods(), []string{"dismiss", "action"}, []string{"visible"}, "status", "message")
	reg("Tooltip", CatFeedback, []string{"message"}, []string{"child"}, stdMods(), []string{"show", "hide"}, []string{"visible"}, "tooltip", "message")
	reg("Popover", CatFeedback, []string{"title?"}, []string{"content"}, stdMods(), []string{"dismiss"}, []string{"open", "closed"}, "dialog", "title?")
	// Navigation (6).
	reg("NavigationStack", CatNavigation, nil, []string{"routes"}, stdMods(), []string{"push", "pop"}, []string{"default"}, "navigation", "")
	reg("AppBar", CatNavigation, []string{"title"}, []string{"actions?"}, stdMods(), []string{"back"}, []string{"default"}, "banner", "title")
	reg("BottomNavigation", CatNavigation, nil, []string{"items"}, stdMods(), []string{"select"}, []string{"default"}, "navigation", "")
	reg("TabBar", CatNavigation, nil, []string{"tabs"}, stdMods(), []string{"select"}, []string{"default"}, "tablist", "")
	reg("PageView", CatNavigation, nil, []string{"pages"}, stdMods(), []string{"swipe"}, []string{"default"}, "group", "")
	reg("Route", CatNavigation, []string{"path"}, []string{"content"}, stdMods(), []string{"enter", "leave"}, []string{"active"}, "link", "path")
	// Alap-special (5).
	reg("Canvas", CatAlapSpecial, nil, nil, stdMods(), []string{"draw"}, []string{"default"}, "img", "")
	reg("Chart", CatAlapSpecial, []string{"kind"}, []string{"series"}, stdMods(), []string{"select"}, []string{"default"}, "img", "kind")
	reg("SemanticGroup", CatAlapSpecial, []string{"label"}, []string{"children"}, stdMods(), nil, []string{"default"}, "group", "label")
	reg("FocusScope", CatAlapSpecial, nil, []string{"children"}, stdMods(), []string{"focus", "blur"}, []string{"default"}, "group", "")
	reg("AnimatedContainer", CatAlapSpecial, nil, []string{"child"}, append(stdMods(), "duration"), []string{"animate"}, []string{"default", "animating"}, "group", "")
}

// Names returns sorted catalog names.
func Names() []string {
	out := make([]string, 0, len(Registry))
	for k := range Registry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ByCategory lists a category.
func ByCategory(c Category) []*Def {
	var out []*Def
	for _, d := range Registry {
		if d.Contract.Category == c {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Contract.Type < out[j].Contract.Type })
	return out
}
