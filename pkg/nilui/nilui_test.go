package nilui

import (
	"strings"
	"testing"
)

func TestNilUIPrimitives(t *testing.T) {
	window := NewWindow("Test Window", 800, 600)
	if window.Title != "Test Window" {
		t.Errorf("expected title 'Test Window', got %s", window.Title)
	}

	layout := NewLayout("layout-1", LayoutColumn)
	text := NewText("txt-1", "Hello nil/ui")
	text.Bold = true
	input := NewInput("inp-1", "Type here...")

	layout.Add(text).Add(input)
	window.Surface.Add(layout)

	// Event handling
	clicked := false
	window.Surface.On(EventClick, func(e Event) {
		clicked = true
	})
	window.Surface.Dispatch(Event{Type: EventClick, TargetID: "surface"})
	if !clicked {
		t.Errorf("expected click event to be handled")
	}

	ansi := window.Surface.RenderANSI()
	if !strings.Contains(ansi, "Hello nil/ui") {
		t.Errorf("ANSI render missing text: %s", ansi)
	}

	html := window.Surface.RenderHTML()
	if !strings.Contains(html, "nil-surface") || !strings.Contains(html, "Hello nil/ui") {
		t.Errorf("HTML render invalid: %s", html)
	}
}
