package alap

import (
	"strings"
	"testing"
)

func TestRendererSanitizationAndSSR(t *testing.T) {
	renderer := NewRenderer(DefaultTheme())

	root := NewUIElement("Column").SetID("col-1")
	// Text containing malicious XSS script
	maliciousText := NewUIElement("Text").SetID("txt-1").SetText("<script>alert('xss')</script>")
	root.AddChild(maliciousText)

	// Button with quotes and brackets in label
	btn := NewUIElement("Button").SetID("btn-1").SetAttr("label", "Click \"Me\" & Win <Prize>")
	root.AddChild(btn)

	renderer.SetRoot(root)

	// Test SSR with initial state
	state := map[string]interface{}{
		"user":  "Sarkar",
		"roles": []string{"admin", "developer"},
	}
	html := renderer.RenderToSSR(state)

	// Verify XSS script was escaped
	if strings.Contains(html, "<script>alert('xss')</script>") {
		t.Errorf("XSS was not escaped! Output: %s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;") {
		t.Errorf("expected escaped text in HTML, got: %s", html)
	}

	// Verify state injection
	if !strings.Contains(html, `window.__NILANG_INITIAL_STATE__`) {
		t.Errorf("expected __NILANG_INITIAL_STATE__ in SSR output")
	}
	if !strings.Contains(html, `"user":"Sarkar"`) {
		t.Errorf("expected user state in SSR output")
	}
}
