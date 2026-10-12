package ir

import (
	"strings"
	"testing"
)

func spanNode(typ string) *Node { return New(typ).At("a.nil", 1, 1).WithProp("title", "t") }

func TestValidateOK(t *testing.T) {
	n := New("Page").At("a.nil", 1, 1).WithProp("title", "Home")
	n.Add(New("Text").At("a.nil", 2, 1).WithProp("value", "hi"))
	if diags := Validate(n); len(diags) != 0 {
		t.Fatalf("want valid, got %v", diags)
	}
}

func TestUnknownTypeDiagnosed(t *testing.T) {
	n := New("FluxCapacitor").At("a.nil", 1, 1)
	diags := Validate(n)
	if !hasCode(diags, CodeUnknownType) {
		t.Fatalf("want UI0101, got %v", diags)
	}
}

func TestMissingPropDiagnosed(t *testing.T) {
	n := New("Text").At("a.nil", 1, 1) // missing value
	diags := Validate(n)
	if !hasCode(diags, CodeMissingProp) {
		t.Fatalf("want UI0102, got %v", diags)
	}
}

func TestDuplicateKeyDiagnosed(t *testing.T) {
	n := spanNode("Column")
	n.Add(New("Text").At("a.nil", 2, 1).WithProp("value", "a").WithKey("k"))
	n.Add(New("Text").At("a.nil", 3, 1).WithProp("value", "b").WithKey("k"))
	if diags := Validate(n); !hasCode(diags, CodeDuplicateKey) {
		t.Fatalf("want UI0103, got %v", diags)
	}
}

func TestMissingKeyDiagnosed(t *testing.T) {
	n := New("List").At("a.nil", 1, 1)
	n.Add(New("Text").At("a.nil", 2, 1).WithProp("value", "a"))
	if diags := Validate(n); !hasCode(diags, CodeMissingKey) {
		t.Fatalf("want UI0107, got %v", diags)
	}
}

func TestUnknownPropAndBadEvent(t *testing.T) {
	n := New("Button").At("a.nil", 1, 1).WithProp("label", "x").WithProp("bogus", "1")
	n.Events = append(n.Events, Event{})
	diags := Validate(n)
	if !hasCode(diags, CodeUnknownProp) || !hasCode(diags, CodeBadEvent) {
		t.Fatalf("want UI0105+UI0108, got %v", diags)
	}
}

func TestVersionSkewAndSpanLost(t *testing.T) {
	n := New("Box")
	n.Version = 999
	diags := Validate(n)
	if !hasCode(diags, CodeVersionSkew) || !hasCode(diags, CodeSpanLost) {
		t.Fatalf("want UI0109+UI0110, got %v", diags)
	}
}

func TestNilaxRoundTrip(t *testing.T) {
	n := New("Page").At("a.nil", 1, 1).WithProp("title", "Home").WithKey("root")
	n.Add(New("Text").At("a.nil", 2, 3).WithProp("value", "স্বাগতম").WithKey("t1").On("click", "go"))
	data, err := Marshal(n)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	data2, _ := Marshal(back)
	if string(data) != string(data2) {
		t.Fatalf("round-trip mismatch:\n%s\n%s", data, data2)
	}
	if back.Children[0].Props["value"] != "স্বাগতম" || back.Children[0].Span.Line != 2 {
		t.Fatalf("round-trip lost data: %+v", back.Children[0])
	}
}

func hasCode(diags []Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code || strings.Contains(d.Error(), code) {
			return true
		}
	}
	return false
}
