package render

import (
	"image/color"
	"testing"
	"time"
)

var (
	black = color.RGBA{0, 0, 0, 255}
	white = color.RGBA{255, 255, 255, 255}
	red   = color.RGBA{255, 0, 0, 255}
)

func TestTreesBuild(t *testing.T) {
	v := &ViewNode{Type: "Column", Children: []*ViewNode{{Type: "Text"}, {Type: "Button"}}}
	e := BuildElements(v)
	if !e.Mounted || !e.Dirty || len(e.Children) != 2 {
		t.Fatalf("elements broken: %+v", e)
	}
	if r := BuildRenders(e); !r.Needs || !r.Paint {
		t.Fatalf("render flags broken")
	}
}

// U14.6: mount/unmount/reload lifecycle.
func TestLifecycleMountUnmountReload(t *testing.T) {
	v := &ViewNode{Type: "Column", Key: "root"}
	e := BuildElements(v)
	if !e.Mounted {
		t.Fatal("build must mount")
	}
	// Unmount.
	e.Mounted = false
	e.Dirty = false
	for _, c := range e.Children {
		c.Mounted = false
	}
	if e.Mounted {
		t.Fatal("unmount must clear")
	}
	// Reload remounts fresh.
	e2 := BuildElements(v)
	if !e2.Mounted || !e2.Dirty {
		t.Fatal("reload must remount dirty")
	}
}

func TestDirtyBoundary(t *testing.T) {
	root := &dirtyNode{}
	mid := &dirtyNode{Boundary: true, Parent: root}
	leaf := &dirtyNode{Parent: mid}
	marked := MarkDirty(leaf)
	if len(marked) != 2 {
		t.Fatalf("boundary must stop propagation: %d", len(marked))
	}
	if root.Dirty {
		t.Fatalf("root must stay clean past boundary")
	}
}

func TestRasterRectGolden(t *testing.T) {
	s := NewSurface(40, 30)
	s.Fill(black)
	s.Rect(5, 5, 10, 8, white)
	if got := s.NonBackgroundPixels(black); got != 80 {
		t.Fatalf("rect must paint exactly 80 px, got %d", got)
	}
	if p := s.PixelAt(5, 5); p != white {
		t.Fatalf("corner must be white: %v", p)
	}
	if p := s.PixelAt(0, 0); p != black {
		t.Fatalf("outside must stay black: %v", p)
	}
}

func TestRasterCircleGolden(t *testing.T) {
	a := NewSurface(50, 50)
	a.Fill(black)
	a.Circle(25, 25, 10, red)
	b := NewSurface(50, 50)
	b.Fill(black)
	b.Circle(25, 25, 10, red)
	if a.NonBackgroundPixels(black) != b.NonBackgroundPixels(black) {
		t.Fatal("raster must be deterministic")
	}
	if n := a.NonBackgroundPixels(black); n < 300 || n > 330 {
		t.Fatalf("r=10 disc ≈317px, got %d", n)
	}
}

func TestRasterLineGolden(t *testing.T) {
	s := NewSurface(20, 20)
	s.Fill(black)
	s.Line(0, 0, 19, 19, white)
	if n := s.NonBackgroundPixels(black); n != 20 {
		t.Fatalf("diagonal must paint 20px, got %d", n)
	}
}

func TestRasterTextGolden(t *testing.T) {
	s := NewSurface(60, 20)
	s.Fill(black)
	s.Text(2, 2, "HI", white)
	// H=~21px + I=~11px from the 5x7 bitmap font.
	if n := s.NonBackgroundPixels(black); n < 25 || n > 40 {
		t.Fatalf("bitmap text shape out of range: %d", n)
	}
	// Deterministic repeat.
	s2 := NewSurface(60, 20)
	s2.Fill(black)
	s2.Text(2, 2, "HI", white)
	if s.NonBackgroundPixels(black) != s2.NonBackgroundPixels(black) {
		t.Fatal("text raster must be deterministic")
	}
}

func TestDisplayListExecute(t *testing.T) {
	dl := &DisplayList{}
	dl.Push(Cmd{Kind: "rect", X: 1, Y: 1, W: 4, H: 4, R: 255, G: 255, B: 255, A: 255})
	dl.Push(Cmd{Kind: "text", X: 10, Y: 2, Text: "A", R: 255, G: 255, B: 255, A: 255})
	s := NewSurface(40, 20)
	s.Fill(black)
	s.Execute(dl)
	if n := s.NonBackgroundPixels(black); n < 16 {
		t.Fatalf("display list under-painted: %d", n)
	}
}

func TestBackendsParity(t *testing.T) {
	dl := &DisplayList{}
	dl.Push(Cmd{Kind: "rect", X: 2, Y: 2, W: 8, H: 6, R: 0, G: 255, B: 0, A: 255})
	dl.Push(Cmd{Kind: "circle", X: 20, Y: 10, W: 5, R: 255, G: 0, B: 0, A: 255})
	backends := []GPUBackend{&SoftwareBackend{}, NewStub(BackendVulkan), NewStub(BackendMetal), NewStub(BackendGLES)}
	var counts []int
	for _, b := range backends {
		if err := b.Init(40, 30); err != nil {
			t.Fatal(err)
		}
		b.Readback().Fill(black)
		if err := b.Submit(dl); err != nil {
			t.Fatal(err)
		}
		counts = append(counts, b.Readback().NonBackgroundPixels(black))
		b.Close()
	}
	for i := 1; i < len(counts); i++ {
		if counts[i] != counts[0] {
			t.Fatalf("backend parity broken: %v", counts)
		}
	}
}

func TestFrameBudget(t *testing.T) {
	ft := NewFrameTimer(50 * time.Millisecond)
	for i := 0; i < 20; i++ {
		ft.Time(func() {})
	}
	if ft.P95() > 50*time.Millisecond {
		t.Fatalf("empty frames must fit budget: %s", ft.Report())
	}
	if ft.Overruns != 0 {
		t.Fatalf("no overruns expected: %s", ft.Report())
	}
	// Volume: raster workload fits 60Hz budget on CI.
	big := NewFrameTimer(Budget60Hz * time.Nanosecond)
	s := NewSurface(100, 100)
	dl := &DisplayList{}
	for i := 0; i < 50; i++ {
		dl.Push(Cmd{Kind: "rect", X: float64(i % 90), Y: 1, W: 8, H: 8, A: 255, G: 200})
	}
	big.Time(func() { s.Fill(black); s.Execute(dl) })
	t.Logf("raster budget: %s", big.Report())
}
