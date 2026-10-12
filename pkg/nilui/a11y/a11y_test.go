package a11y

import (
	"strings"
	"testing"
)

func TestSemanticDescribe(t *testing.T) {
	n := Node{ID: "b", Role: RoleButton, Label: "Save", Hint: "saves draft", State: []State{StateEnabled}, TargetDp: [2]float64{48, 48}}
	d := n.Describe()
	for _, want := range []string{"button", "Save", "saves draft", "enabled"} {
		if !strings.Contains(d, want) {
			t.Fatalf("describe missing %q: %q", want, d)
		}
	}
}

func TestOrderSkipsHidden(t *testing.T) {
	tr := &Tree{}
	tr.Add(&Node{ID: "a", Order: 2}).Add(&Node{ID: "b", Order: 1}).Add(&Node{ID: "c", Order: 0, Hidden: true})
	ord := tr.Order()
	if len(ord) != 2 || ord[0].ID != "b" || ord[1].ID != "a" {
		t.Fatalf("order broken: %v", ord)
	}
}

func TestValidateGate(t *testing.T) {
	tr := &Tree{}
	tr.Add(&Node{ID: "ok", Role: RoleButton, Label: "Go", Order: 0, TargetDp: [2]float64{48, 48}})
	tr.Add(&Node{ID: "nolabel", Role: RoleButton, Order: 1, TargetDp: [2]float64{48, 48}})
	tr.Add(&Node{ID: "small", Role: RoleButton, Label: "S", Order: 2, TargetDp: [2]float64{24, 24}})
	issues := tr.Validate()
	if len(issues) != 2 {
		t.Fatalf("want 2 issues, got %v", issues)
	}
	good := &Tree{}
	good.Add(&Node{ID: "ok", Role: RoleButton, Label: "Go", Order: 0, TargetDp: [2]float64{48, 48}})
	if issues := good.Validate(); len(issues) != 0 {
		t.Fatalf("clean tree must validate: %v", issues)
	}
}

func TestDynamicType(t *testing.T) {
	if ScaleFont(14, 1.5) != 21 || ScaleFont(14, 5) != 28 || ScaleFont(14, 0.5) != 14 {
		t.Fatal("dynamic type scaling broken")
	}
}

func TestReducedTransparency(t *testing.T) {
	_, _, _, a := OpaqueColor(1, 2, 3, 100, true)
	if a != 255 {
		t.Fatal("reduce-transparency must opacify")
	}
}

func TestRolesCovered(t *testing.T) {
	for _, r := range []Role{RoleButton, RoleLink, RoleHeader, RoleImage, RoleCheckbox} {
		if r == "" {
			t.Fatal("role empty")
		}
	}
}
