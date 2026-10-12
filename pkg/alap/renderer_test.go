package alap

import (
	"strings"
	"testing"
)

// Legacy shim must delegate to the single ui model (ADR-0006) and must not
// reintroduce a parallel renderer with XSS holes.
func TestLegacyShimDelegatesToSingleModel(t *testing.T) {
	r := NewRenderer(DefaultTheme())
	root := NewUIElement("Column")
	root.ID = "col-1"
	malicious := Text("<script>alert('xss')</script>")
	malicious.ID = "txt-1"
	root.AddChild(malicious)
	r.SetRoot(root)
	html := r.RenderToSSR(map[string]interface{}{"user": "Sarkar"})
	if strings.Contains(html, "<script>alert('xss')</script>") {
		t.Errorf("XSS was not escaped in legacy shim path")
	}
	if !strings.Contains(html, "window.__NILANG_INITIAL_STATE__") {
		t.Errorf("expected __NILANG_INITIAL_STATE__ in SSR output")
	}
	if Text("x") == nil || Column() == nil || Row() == nil || Container() == nil {
		t.Errorf("legacy constructors must not return nil")
	}
	if Image("a.png") == nil || Input("ph") == nil || ListView() == nil {
		t.Errorf("legacy element constructors must not return nil")
	}
}
