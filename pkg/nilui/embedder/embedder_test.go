package embedder

import (
	"strings"
	"testing"
)

func TestCABIHeader(t *testing.T) {
	for _, fn := range []string{"alap_node_create", "alap_node_measure", "alap_node_arrange", "alap_node_paint", "alap_node_destroy", "alap_event_cb", "alap_set_event_callback"} {
		if !strings.Contains(Header, fn) {
			t.Errorf("header missing %s", fn)
		}
	}
}

func TestNodeLifecycle(t *testing.T) {
	n := Create("Button", "b1")
	n.SetProp("label", "Go")
	n.Measure(0, 0, 1000, 1000)
	if n.W <= 0 || n.H <= 0 {
		t.Fatal("measure must size")
	}
	n.Arrange(10, 20, n.W, n.H)
	if n.Rect != [4]float64{10, 20, n.W, n.H} {
		t.Fatalf("arrange broken: %v", n.Rect)
	}
	ctx := &PaintCtx{W: 100, H: 100}
	n.Paint(ctx)
	if len(ctx.Filled) != 1 || ctx.Filled[0] != n.ID {
		t.Fatal("paint must record node")
	}
	var got string
	n.SetEventCallback(func(id, ev, p string) { got = id + ev + p })
	n.Emit("click", "{}")
	if got == "" {
		t.Fatal("event callback broken")
	}
	n.Destroy()
	if !n.Destroyed() {
		t.Fatal("destroy broken")
	}
}

func TestAdapters(t *testing.T) {
	for _, p := range []Platform{PlatformAndroid, PlatformIOS, PlatformHarmony} {
		a := NewAdapter(p)
		n := Create("Text", "t")
		a.Host(n)
		if a.HostName() == "unknown" {
			t.Fatalf("platform %d needs a host name", p)
		}
		if err := a.EscapeHatch(n.ID, struct{}{}); err != nil {
			t.Fatal(err)
		}
		if n.NativeHandle == nil {
			t.Fatal("escape hatch must attach")
		}
		if err := a.EscapeHatch("missing", nil); err == nil {
			t.Fatal("escape hatch on missing node must error")
		}
	}
}

func TestInsets(t *testing.T) {
	in := Insets{SafeTop: 24, SafeBottom: 48, Keyboard: 100}
	r := in.Apply(0, 0, 400, 800)
	if r[0] != 0 || r[1] != 124 || r[2] != 400 || r[3] != 628 {
		t.Fatalf("insets broken: %v", r)
	}
}
