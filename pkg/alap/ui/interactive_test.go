package ui

import (
	"strings"
	"testing"
)

func TestPageRenderSSRIncludesInteractiveRoot(t *testing.T) {
	page := NewPage("Interactive").Add(NewButton("increment", "Increment"))
	button := page.Content[0].(*Button)
	button.OnClick = `increment<&"`
	button.Payload = map[string]interface{}{"name": `Ada & "Lin"`}

	html := page.RenderSSR(DefaultTheme(), map[string]interface{}{"count": int64(0)})
	for _, expected := range []string{
		"data-alap-root",
		`data-alap-click="increment&lt;&amp;&#34;"`,
		`data-alap-payload="{&#34;name&#34;:&#34;Ada \u0026 \&#34;Lin\&#34;&#34;}"`,
		`<script id="__NILANG_STATE__"`,
		`<script src="/alap-runtime.js"></script>`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("SSR HTML does not contain %q: %s", expected, html)
		}
	}
	if strings.Contains(html, "onclick=") {
		t.Fatalf("SSR HTML contains executable inline onclick: %s", html)
	}
}
