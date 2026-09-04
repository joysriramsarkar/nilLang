package nilui

import (
	"fmt"
	"strings"
	"sync"
)

// EventType represents the kind of low-level UI event
type EventType string

const (
	EventClick    EventType = "click"
	EventKeyPress EventType = "keypress"
	EventChange   EventType = "change"
	EventResize   EventType = "resize"
	EventHover    EventType = "hover"
	EventFocus    EventType = "focus"
)

// Event is a low-level event payload
type Event struct {
	Type     EventType              `json:"type"`
	TargetID string                 `json:"target_id"`
	Data     map[string]interface{} `json:"data,omitempty"`
}

// EventHandler is a callback for handling low-level events
type EventHandler func(e Event)

// Primitive is the base interface for low-level nil/ui elements
type Primitive interface {
	ID() string
	RenderANSI() string
	RenderHTML() string
}

// LayoutType defines arrangement of elements
type LayoutType string

const (
	LayoutColumn LayoutType = "column"
	LayoutRow    LayoutType = "row"
	LayoutStack  LayoutType = "stack"
)

// Surface is a low-level 2D canvas/container surface
type Surface struct {
	id         string
	Width      int
	Height     int
	Background string
	Border     string
	Children   []Primitive
	handlers   map[EventType][]EventHandler
	mu         sync.RWMutex
}

// NewSurface creates a new Surface primitive
func NewSurface(id string, width, height int) *Surface {
	return &Surface{
		id:         id,
		Width:      width,
		Height:     height,
		Background: "transparent",
		Children:   []Primitive{},
		handlers:   make(map[EventType][]EventHandler),
	}
}

func (s *Surface) ID() string { return s.id }

func (s *Surface) Add(p Primitive) *Surface {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Children = append(s.Children, p)
	return s
}

func (s *Surface) On(evt EventType, h EventHandler) *Surface {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[evt] = append(s.handlers[evt], h)
	return s
}

func (s *Surface) Dispatch(e Event) {
	s.mu.RLock()
	handlers := s.handlers[e.Type]
	s.mu.RUnlock()

	for _, h := range handlers {
		h(e)
	}

	for _, child := range s.Children {
		if cs, ok := child.(*Surface); ok {
			cs.Dispatch(e)
		}
	}
}

func (s *Surface) RenderANSI() string {
	var sb strings.Builder
	for _, child := range s.Children {
		sb.WriteString(child.RenderANSI())
		sb.WriteString("\n")
	}
	return sb.String()
}

func (s *Surface) RenderHTML() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<div id="%s" class="nil-surface" style="background:%s;">`, s.id, s.Background))
	for _, child := range s.Children {
		sb.WriteString(child.RenderHTML())
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

// Window represents the top-level window primitive
type Window struct {
	Title   string   `json:"title"`
	Width   int      `json:"width"`
	Height  int      `json:"height"`
	Surface *Surface `json:"surface"`
}

// NewWindow creates a new Window primitive
func NewWindow(title string, width, height int) *Window {
	return &Window{
		Title:   title,
		Width:   width,
		Height:  height,
		Surface: NewSurface("root-window", width, height),
	}
}

// Text represents low-level formatted text
type Text struct {
	id    string
	Value string
	Color string
	Bold  bool
}

func NewText(id, value string) *Text {
	return &Text{id: id, Value: value, Color: "#ffffff"}
}

func (t *Text) ID() string { return t.id }

func (t *Text) RenderANSI() string {
	if t.Bold {
		return fmt.Sprintf("\033[1m%s\033[0m", t.Value)
	}
	return t.Value
}

func (t *Text) RenderHTML() string {
	weight := "normal"
	if t.Bold {
		weight = "bold"
	}
	return fmt.Sprintf(`<span id="%s" class="nil-text" style="color:%s; font-weight:%s;">%s</span>`,
		t.id, t.Color, weight, t.Value)
}

// Input represents low-level text input box
type Input struct {
	id          string
	Placeholder string
	Value       string
}

func NewInput(id, placeholder string) *Input {
	return &Input{id: id, Placeholder: placeholder}
}

func (i *Input) ID() string { return i.id }

func (i *Input) RenderANSI() string {
	val := i.Value
	if val == "" {
		val = "[" + i.Placeholder + "]"
	}
	return fmt.Sprintf("\033[36m%s\033[0m", val)
}

func (i *Input) RenderHTML() string {
	return fmt.Sprintf(`<input id="%s" class="nil-input" type="text" placeholder="%s" value="%s" />`,
		i.id, i.Placeholder, i.Value)
}

// Layout represents an organized arrangement of primitives
type Layout struct {
	id       string
	Type     LayoutType
	Gap      int
	Children []Primitive
}

func NewLayout(id string, lt LayoutType) *Layout {
	return &Layout{
		id:       id,
		Type:     lt,
		Gap:      8,
		Children: []Primitive{},
	}
}

func (l *Layout) ID() string { return l.id }

func (l *Layout) Add(child Primitive) *Layout {
	l.Children = append(l.Children, child)
	return l
}

func (l *Layout) RenderANSI() string {
	var sb strings.Builder
	for idx, c := range l.Children {
		sb.WriteString(c.RenderANSI())
		if l.Type == LayoutColumn && idx < len(l.Children)-1 {
			sb.WriteString("\n")
		} else if l.Type == LayoutRow && idx < len(l.Children)-1 {
			sb.WriteString("  ")
		}
	}
	return sb.String()
}

func (l *Layout) RenderHTML() string {
	dir := "column"
	if l.Type == LayoutRow {
		dir = "row"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<div id="%s" class="nil-layout" style="display:flex; flex-direction:%s; gap:%dpx;">`,
		l.id, dir, l.Gap))
	for _, c := range l.Children {
		sb.WriteString(c.RenderHTML())
	}
	sb.WriteString(`</div>`)
	return sb.String()
}
