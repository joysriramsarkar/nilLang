package devtools

import (
	"strings"
	"testing"
)

func TestHotReloadPreservesState(t *testing.T) {
	r := NewReloader()
	r.Keep("count", "41")
	r.Keep("draft", "hello")
	snap := r.Reload()
	if snap.Values["count"] != "41" || snap.Values["draft"] != "hello" {
		t.Fatalf("state must survive reload: %v", snap)
	}
	if r.Loads() != 1 {
		t.Fatal("loads must count")
	}
}

func TestInspectorTree(t *testing.T) {
	in := &Inspector{}
	in.Add(InspectNode{Path: "/Page", Type: "Page", Depth: 0, Box: [4]float64{0, 0, 390, 844}})
	in.Add(InspectNode{Path: "/Page/Text", Type: "Text", Key: "t", Depth: 1, Signal: &SignalValue{"s", "3", 2}})
	tree := in.Tree()
	if !strings.Contains(tree, "Page") || !strings.Contains(tree, "signal:s=3#2") {
		t.Fatalf("inspector broken:\n%s", tree)
	}
}

func TestLint(t *testing.T) {
	issues := Lint(`{"type": "List", "content": []}`)
	if len(issues) != 1 || issues[0].Rule != "UI-LINT-KEY" {
		t.Fatalf("key lint broken: %v", issues)
	}
	issues = Lint(`{"type": "Button", "id": "x"}`)
	if len(issues) != 1 {
		t.Fatalf("label lint broken: %v", issues)
	}
	if got := Lint(`{"type": "Button", "label": "Go"}`); len(got) != 0 {
		t.Fatalf("clean must pass: %v", got)
	}
}

func TestFormat(t *testing.T) {
	if got := Format("a  \nb\t"); got != "a\nb" {
		t.Fatalf("format broken: %q", got)
	}
}

func TestPreview(t *testing.T) {
	src := "// @preview(card)\n// @preview\nx"
	pv := CollectPreviews(src)
	if len(pv) != 2 || pv[0].Name != "(card)" || pv[1].Name != "default" {
		t.Fatalf("previews broken: %+v", pv)
	}
	if BrowserPreviewURL(8080, "card") != "http://localhost:8080/__preview/card" {
		t.Fatal("preview url broken")
	}
}
