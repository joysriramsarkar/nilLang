package ui_test

import (
	"os"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/alap/ui"
)

// U0.3: shuffle of keyed children must emit MOVE, never REPLACE.
func TestKeyedReorderNoNeedlessReplace(t *testing.T) {
	mk := func(keys ...string) *ui.VNode {
		ul := ui.NewElementVNode("ul", "list", "")
		for i, k := range keys {
			li := ui.NewElementVNode("li", "item-"+k, k)
			li.AddChild(ui.NewTextVNode("row " + k))
			_ = i
			ul.AddChild(li)
		}
		return ul
	}
	oldTree := mk("a", "b", "c", "d")
	newTree := mk("d", "b", "a", "c") // pure shuffle

	patches := ui.Diff(oldTree, newTree)
	for _, p := range patches {
		if p.Type == ui.PatchReplace {
			t.Fatalf("shuffle must not REPLACE, got: %+v (all: %+v)", p, patches)
		}
	}
	foundMove := false
	for _, p := range patches {
		if p.Type == ui.PatchMove {
			foundMove = true
		}
	}
	if !foundMove {
		t.Fatalf("expected at least one MOVE patch for shuffle, got: %+v", patches)
	}
}

// U0.3: identical keyed trees produce zero patches.
func TestKeyedIdenticalNoPatches(t *testing.T) {
	mk := func() *ui.VNode {
		ul := ui.NewElementVNode("ul", "list", "")
		for _, k := range []string{"a", "b", "c"} {
			li := ui.NewElementVNode("li", "item-"+k, k)
			li.AddChild(ui.NewTextVNode("row " + k))
			ul.AddChild(li)
		}
		return ul
	}
	if patches := ui.Diff(mk(), mk()); len(patches) != 0 {
		t.Fatalf("expected 0 patches, got %+v", patches)
	}
}

// U0.3: keyed insert+remove still detected alongside moves.
func TestKeyedInsertRemove(t *testing.T) {
	oldTree := ui.NewElementVNode("ul", "list", "")
	for _, k := range []string{"a", "b", "c"} {
		oldTree.AddChild(ui.NewElementVNode("li", "item-"+k, k))
	}
	newTree := ui.NewElementVNode("ul", "list", "")
	for _, k := range []string{"b", "c", "e"} {
		newTree.AddChild(ui.NewElementVNode("li", "item-"+k, k))
	}
	patches := ui.Diff(oldTree, newTree)
	var sawIns, sawRem bool
	for _, p := range patches {
		if p.Type == ui.PatchInsertChild {
			sawIns = true
		}
		if p.Type == ui.PatchRemoveChild {
			sawRem = true
		}
	}
	if !sawIns || !sawRem {
		t.Fatalf("expected insert+remove, got %+v", patches)
	}
}

// U0.2: single canonical runtime — SSR inline JS and the served file agree,
// and applyPatches is really present and called-able (smoke check on source).
func TestRuntimeSingleSource(t *testing.T) {
	disk, err := os.ReadFile("../../pkg/alap/runtime/alap-runtime.js")
	if err != nil {
		disk, err = os.ReadFile("pkg/alap/runtime/alap-runtime.js")
	}
	if err != nil {
		t.Skipf("runtime file not found from cwd: %v", err)
	}
	if ui.AlapRuntimeJS != string(disk) {
		t.Errorf("AlapRuntimeJS const drifted from pkg/alap/runtime/alap-runtime.js; re-sync")
	}
	for _, want := range []string{"applyPatches", "MOVE", "INSERT_CHILD", "REMOVE_CHILD", "AlapRuntime"} {
		if !strings.Contains(ui.AlapRuntimeJS, want) {
			t.Errorf("canonical runtime missing %q", want)
		}
	}
	if strings.Contains(ui.AlapRuntimeJS, "replaceWith(document.importNode") {
		// legacy fallback branch is allowed but the patch path must exist
		if !strings.Contains(ui.AlapRuntimeJS, "applyPatchResponse") {
			t.Errorf("expected patch-response path alongside legacy fallback")
		}
	}
}
