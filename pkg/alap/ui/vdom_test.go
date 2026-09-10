package ui_test

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/alap/ui"
)

func TestVNodeRenderHTML(t *testing.T) {
	node := ui.NewElementVNode("div", "app-root", "root-key")
	node.SetAttr("class", "container").SetAttr("style", "padding:20px;")
	node.SetEvent("click", "handleRootClick")

	btn := ui.NewElementVNode("button", "btn-1", "")
	btn.SetAttr("class", "btn-primary")
	btn.AddChild(ui.NewTextVNode("Click Me"))

	node.AddChild(btn)

	rendered := node.RenderHTML()
	expectedSubstrings := []string{
		`<div id="app-root" data-key="root-key" class="container" style="padding:20px;" data-alap-click="handleRootClick">`,
		`<button id="btn-1" class="btn-primary">Click Me</button>`,
		`</div>`,
	}

	for _, sub := range expectedSubstrings {
		if !testingContains(rendered, sub) {
			t.Errorf("expected rendered HTML to contain %q, got %q", sub, rendered)
		}
	}
}

func TestVDOMDiffPropsAndText(t *testing.T) {
	oldTree := ui.NewElementVNode("div", "box", "")
	oldTree.SetAttr("class", "card old-theme")
	oldTree.AddChild(ui.NewTextVNode("Hello Old World"))

	newTree := ui.NewElementVNode("div", "box", "")
	newTree.SetAttr("class", "card new-theme")
	newTree.AddChild(ui.NewTextVNode("Hello New World"))

	patches := ui.Diff(oldTree, newTree)

	if len(patches) != 2 {
		t.Fatalf("expected 2 patches (props and text), got %d: %+v", len(patches), patches)
	}

	hasPropsPatch := false
	hasTextPatch := false
	for _, p := range patches {
		if p.Type == ui.PatchProps {
			hasPropsPatch = true
			if p.Props["class"] != "card new-theme" {
				t.Errorf("expected class 'card new-theme', got %q", p.Props["class"])
			}
		}
		if p.Type == ui.PatchText {
			hasTextPatch = true
			if p.Text != "Hello New World" {
				t.Errorf("expected text 'Hello New World', got %q", p.Text)
			}
		}
	}

	if !hasPropsPatch || !hasTextPatch {
		t.Errorf("expected both props and text patch, got props=%v, text=%v", hasPropsPatch, hasTextPatch)
	}
}

func TestVDOMDiffChildrenInsertAndRemove(t *testing.T) {
	// 1. Insert child
	oldTree := ui.NewElementVNode("ul", "list", "")
	oldTree.AddChild(ui.NewElementVNode("li", "item-1", ""))

	newTree := ui.NewElementVNode("ul", "list", "")
	newTree.AddChild(ui.NewElementVNode("li", "item-1", ""))
	newTree.AddChild(ui.NewElementVNode("li", "item-2", ""))

	patches := ui.Diff(oldTree, newTree)
	if len(patches) != 1 || patches[0].Type != ui.PatchInsertChild {
		t.Fatalf("expected 1 INSERT_CHILD patch, got: %+v", patches)
	}

	// 2. Remove child
	patches2 := ui.Diff(newTree, oldTree)
	if len(patches2) != 1 || patches2[0].Type != ui.PatchRemoveChild {
		t.Fatalf("expected 1 REMOVE_CHILD patch, got: %+v", patches2)
	}
}

func TestSerializePatches(t *testing.T) {
	patches := []ui.Patch{
		{
			Type:   ui.PatchText,
			Path:   []int{0, 1},
			NodeID: "label-1",
			Text:   "Updated Total: ৳500",
		},
	}

	bytes, err := ui.SerializePatches(patches)
	if err != nil {
		t.Fatalf("SerializePatches error: %v", err)
	}
	if len(bytes) == 0 {
		t.Fatal("expected non-empty serialized JSON")
	}
}

func testingContains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || stringContains(s, sub))
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
