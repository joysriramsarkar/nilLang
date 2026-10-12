// Package parity implements U14.7: the cross-platform parity suite. One
// test core runs against every backend (software raster + stub Vulkan/Metal/
// GLES) so the three native targets cannot drift from the tested behavior.
// Flakiness policy (U14.9) and the device matrix (U14.8) live in docs below;
// the perf budget CI test (U14.10) is TestPerfBudget.
package parity

import (
	"image/color"
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/nilui/render"
)

// Case is one parity case: build a display list, run everywhere, compare.
type Case struct {
	Name  string
	Build func() *render.DisplayList
	Check func(s *render.Surface) error
}

func coreCases() []Case {
	rect := Case{Name: "rect", Build: func() *render.DisplayList {
		dl := &render.DisplayList{}
		dl.Push(render.Cmd{Kind: "rect", X: 2, Y: 2, W: 8, H: 8, R: 255, G: 255, B: 255, A: 255})
		return dl
	}}
	circle := Case{Name: "circle", Build: func() *render.DisplayList {
		dl := &render.DisplayList{}
		dl.Push(render.Cmd{Kind: "circle", X: 15, Y: 15, W: 8, R: 255, A: 255})
		return dl
	}}
	text := Case{Name: "text", Build: func() *render.DisplayList {
		dl := &render.DisplayList{}
		dl.Push(render.Cmd{Kind: "text", X: 1, Y: 1, Text: "HIX", R: 255, G: 255, B: 255, A: 255})
		return dl
	}}
	return []Case{rect, circle, text}
}

func backends() []render.GPUBackend {
	return []render.GPUBackend{
		&render.SoftwareBackend{},
		render.NewStub(render.BackendVulkan),
		render.NewStub(render.BackendMetal),
		render.NewStub(render.BackendGLES),
	}
}

// TestParity runs the core on all backends and requires identical pixels.
func TestParity(t *testing.T) {
	for _, c := range coreCases() {
		var reference []uint8
		for i, b := range backends() {
			if err := b.Init(40, 30); err != nil {
				t.Fatalf("%s/%v init: %v", c.Name, b.Name(), err)
			}
			b.Readback().Fill(color.RGBA{15, 23, 42, 255})
			if err := b.Submit(c.Build()); err != nil {
				t.Fatalf("%s/%v submit: %v", c.Name, b.Name(), err)
			}
			pix := append([]uint8{}, b.Readback().Pix.Pix...)
			if i == 0 {
				reference = pix
			} else if !equalBytes(reference, pix) {
				t.Fatalf("%s: backend %v pixels differ from software", c.Name, b.Name())
			}
			b.Close()
		}
	}
}

// TestPerfBudget is the CI perf gate (U14.10): full-frame raster + layout
// workload must fit the 60Hz budget at p95.
func TestPerfBudget(t *testing.T) {
	ft := render.NewFrameTimer(render.Budget60Hz * time.Nanosecond)
	s := render.NewSurface(390, 844)
	dl := &render.DisplayList{}
	for i := 0; i < 200; i++ {
		dl.Push(render.Cmd{Kind: "rect", X: float64(i % 380), Y: float64((i * 7) % 830), W: 12, H: 12, G: 200, A: 255})
	}
	for i := 0; i < 30; i++ {
		ft.Time(func() {
			s.Fill(color.RGBA{15, 23, 42, 255})
			s.Execute(dl)
		})
	}
	t.Logf("perf: %s", ft.Report())
	if ft.P95() > render.Budget60Hz*time.Nanosecond {
		t.Fatalf("frame over 60Hz budget: %s", ft.Report())
	}
}

func equalBytes(a, b []uint8) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
