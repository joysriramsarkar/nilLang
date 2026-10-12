// Package render implements U4: View→Element→Render trees, dirty marking
// with relayout boundaries, a retained display list, a real software
// rasterizer (RGBA pixel buffer + deterministic golden tests), the GPU
// backend interface (Vulkan/Metal/GLES/Software), and a frame timing budget.
package render

import (
	"fmt"
	"image"
	"image/color"
	"sort"
	"sync"
	"time"
)

// ─── Trees (U4.1) ───────────────────────────────────────────────────────────

// ViewNode is the declarative view description.
type ViewNode struct {
	Type     string
	Key      string
	Props    map[string]string
	Children []*ViewNode
}

// ElementNode binds a View to lifecycle state.
type ElementNode struct {
	View     *ViewNode
	Mounted  bool
	Dirty    bool
	Children []*ElementNode
}

// RenderNode is the layout/render object.
type RenderNode struct {
	Element *ElementNode
	X, Y    float64
	W, H    float64
	Needs   bool // needs layout
	Paint   bool // needs paint
}

// BuildElements inflates views to elements.
func BuildElements(v *ViewNode) *ElementNode {
	if v == nil {
		return nil
	}
	e := &ElementNode{View: v, Mounted: true, Dirty: true}
	for _, c := range v.Children {
		e.Children = append(e.Children, BuildElements(c))
	}
	return e
}

// BuildRenders inflates elements to render nodes.
func BuildRenders(e *ElementNode) *RenderNode {
	if e == nil {
		return nil
	}
	r := &RenderNode{Element: e, Needs: true, Paint: true}
	return r
}

// ─── Dirty marking (U4.2) ───────────────────────────────────────────────────

// MarkDirty flags a subtree; relayout boundaries stop upward propagation:
// a node with Boundary=true absorbs the mark (its parent is not dirtied).
type dirtyNode struct {
	Boundary bool
	Parent   *dirtyNode
	Dirty    bool
}

// MarkDirty marks n and propagates up to (not past) boundaries.
func MarkDirty(n *dirtyNode) []*dirtyNode {
	var marked []*dirtyNode
	for cur := n; cur != nil; cur = cur.Parent {
		cur.Dirty = true
		marked = append(marked, cur)
		if cur.Boundary {
			break
		}
	}
	return marked
}

// ─── Display list (U4.3) ────────────────────────────────────────────────────

// Cmd is one retained paint command.
type Cmd struct {
	Kind       string // rect | circle | line | text | clip
	X, Y, W, H float64
	R, G, B, A uint8
	Text       string
}

// DisplayList is the retained command buffer.
type DisplayList struct {
	Cmds []Cmd
}

// Push appends.
func (d *DisplayList) Push(c Cmd) { d.Cmds = append(d.Cmds, c) }

// Clear resets.
func (d *DisplayList) Clear() { d.Cmds = nil }

// ─── Software rasterizer (U4.4) ─────────────────────────────────────────────

// Surface is a real RGBA pixel buffer.
type Surface struct {
	W, H int
	Pix  *image.RGBA
	mu   sync.Mutex
}

// NewSurface creates a W×H surface.
func NewSurface(w, h int) *Surface {
	return &Surface{W: w, H: h, Pix: image.NewRGBA(image.Rect(0, 0, w, h))}
}

// Fill fills with a color.
func (s *Surface) Fill(c color.RGBA) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for y := 0; y < s.H; y++ {
		for x := 0; x < s.W; x++ {
			s.Pix.SetRGBA(x, y, c)
		}
	}
}

func (s *Surface) blend(x, y int, c color.RGBA) {
	if x < 0 || y < 0 || x >= s.W || y >= s.H {
		return
	}
	dst := s.Pix.RGBAAt(x, y)
	a := float64(c.A) / 255
	inv := 1 - a
	s.Pix.SetRGBA(x, y, color.RGBA{
		R: uint8(float64(c.R)*a + float64(dst.R)*inv),
		G: uint8(float64(c.G)*a + float64(dst.G)*inv),
		B: uint8(float64(c.B)*a + float64(dst.B)*inv),
		A: 255,
	})
}

// Rect fills an axis-aligned rect.
func (s *Surface) Rect(x, y, w, h int, c color.RGBA) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for j := y; j < y+h; j++ {
		for i := x; i < x+w; i++ {
			s.blendLocked(i, j, c)
		}
	}
}

func (s *Surface) blendLocked(x, y int, c color.RGBA) {
	if x < 0 || y < 0 || x >= s.W || y >= s.H {
		return
	}
	dst := s.Pix.RGBAAt(x, y)
	a := float64(c.A) / 255
	inv := 1 - a
	s.Pix.SetRGBA(x, y, color.RGBA{
		R: uint8(float64(c.R)*a + float64(dst.R)*inv),
		G: uint8(float64(c.G)*a + float64(dst.G)*inv),
		B: uint8(float64(c.B)*a + float64(dst.B)*inv),
		A: 255,
	})
}

// Circle fills a disc (midpoint scan).
func (s *Surface) Circle(cx, cy, r int, c color.RGBA) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= r*r {
				s.blendLocked(x, y, c)
			}
		}
	}
}

// Line draws a thick-ish line (Bresenham, 1px).
func (s *Surface) Line(x0, y0, x1, y1 int, c color.RGBA) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx := 1
	if x0 > x1 {
		sx = -1
	}
	sy := 1
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		s.blendLocked(x0, y0, c)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Glyph draws a deterministic 5x7 bitmap glyph for ASCII (font-independent
// golden tests: shape asserts, not font rendering).
var glyphFont = map[rune][7]uint8{
	'H': {0x11, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11},
	'I': {0x0E, 0x04, 0x04, 0x04, 0x04, 0x04, 0x0E},
	'X': {0x11, 0x11, 0x0A, 0x04, 0x0A, 0x11, 0x11},
	'O': {0x0E, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0E},
	'#': {0x0A, 0x1F, 0x0A, 0x0A, 0x1F, 0x0A, 0x0A},
	'*': {0x00, 0x04, 0x15, 0x0E, 0x15, 0x04, 0x00},
	'A': {0x0E, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11},
	'B': {0x1E, 0x11, 0x11, 0x1E, 0x11, 0x11, 0x1E},
	' ': {0, 0, 0, 0, 0, 0, 0},
}

// Text draws text with the bitmap font at integer scale.
func (s *Surface) Text(x, y int, text string, c color.RGBA) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cx := x
	for _, r := range text {
		g, ok := glyphFont[r]
		if !ok {
			g = glyphFont['#']
		}
		for row := 0; row < 7; row++ {
			for col := 0; col < 5; col++ {
				if g[row]&(1<<(4-col)) != 0 {
					s.blendLocked(cx+col, y+row, c)
				}
			}
		}
		cx += 6
	}
}

// Execute runs a display list onto the surface.
func (s *Surface) Execute(dl *DisplayList) {
	for _, c := range dl.Cmds {
		col := color.RGBA{c.R, c.G, c.B, c.A}
		switch c.Kind {
		case "rect":
			s.Rect(int(c.X), int(c.Y), int(c.W), int(c.H), col)
		case "circle":
			s.Circle(int(c.X), int(c.Y), int(c.W), col)
		case "line":
			s.Line(int(c.X), int(c.Y), int(c.W), int(c.H), col)
		case "text":
			s.Text(int(c.X), int(c.Y), c.Text, col)
		}
	}
}

// NonBackgroundPixels counts pixels differing from bg (shape assert helper).
func (s *Surface) NonBackgroundPixels(bg color.RGBA) int {
	n := 0
	for y := 0; y < s.H; y++ {
		for x := 0; x < s.W; x++ {
			if s.Pix.RGBAAt(x, y) != bg {
				n++
			}
		}
	}
	return n
}

// PixelAt returns one pixel.
func (s *Surface) PixelAt(x, y int) color.RGBA { return s.Pix.RGBAAt(x, y) }

// ─── Backend interface (U4.5) ───────────────────────────────────────────────

// Backend is Vulkan/Metal/GLES/Software.
type Backend int

const (
	// BackendSoftware is the test/preview backend.
	BackendSoftware Backend = iota
	// BackendVulkan targets Android/Onuron.
	BackendVulkan
	// BackendMetal targets iOS.
	BackendMetal
	// BackendGLES targets low-end Android.
	BackendGLES
)

// String names the backend.
func (b Backend) String() string {
	switch b {
	case BackendVulkan:
		return "Vulkan"
	case BackendMetal:
		return "Metal"
	case BackendGLES:
		return "GLES"
	default:
		return "Software"
	}
}

// GPUBackend is the real-API-call-free interface (ADR-0001: native adapters
// own the pixels; this interface is for tests/preview/parity).
type GPUBackend interface {
	Name() Backend
	Init(w, h int) error
	Submit(dl *DisplayList) error
	Readback() *Surface
	Close() error
}

// SoftwareBackend implements GPUBackend over the rasterizer.
type SoftwareBackend struct {
	surf *Surface
}

// Name returns Software.
func (s *SoftwareBackend) Name() Backend { return BackendSoftware }

// Init allocates.
func (s *SoftwareBackend) Init(w, h int) error {
	s.surf = NewSurface(w, h)
	return nil
}

// Submit executes.
func (s *SoftwareBackend) Submit(dl *DisplayList) error {
	s.surf.Execute(dl)
	return nil
}

// Readback returns pixels.
func (s *SoftwareBackend) Readback() *Surface { return s.surf }

// Close releases.
func (s *SoftwareBackend) Close() error { s.surf = nil; return nil }

// StubBackend records submissions for Vulkan/Metal/GLES parity tests.
type StubBackend struct {
	backend Backend
	submits []*DisplayList
	surf    *Surface
}

// NewStub creates a stub backend.
func NewStub(b Backend) *StubBackend { return &StubBackend{backend: b} }

// Name returns the backend.
func (s *StubBackend) Name() Backend { return s.backend }

// Init allocates a readback surface.
func (s *StubBackend) Init(w, h int) error { s.surf = NewSurface(w, h); return nil }

// Submit records and rasterizes identically (parity).
func (s *StubBackend) Submit(dl *DisplayList) error {
	cp := *dl
	s.submits = append(s.submits, &cp)
	s.surf.Execute(dl)
	return nil
}

// Readback returns pixels.
func (s *StubBackend) Readback() *Surface { return s.surf }

// Close releases.
func (s *StubBackend) Close() error { return nil }

// Submits counts submissions.
func (s *StubBackend) Submits() int { return len(s.submits) }

// ─── Frame budget (U4.7) ────────────────────────────────────────────────────

// Budget60Hz is 16.7ms; Budget120Hz is 8.3ms.
const (
	Budget60Hz  = 16_700_000 // ns
	Budget120Hz = 8_300_000  // ns
)

// FrameTimer measures frame work against budget.
type FrameTimer struct {
	mu       sync.Mutex
	samples  []time.Duration
	Overruns int
	Budget   time.Duration
}

// NewFrameTimer creates a timer.
func NewFrameTimer(budget time.Duration) *FrameTimer { return &FrameTimer{Budget: budget} }

// Time runs fn and records.
func (f *FrameTimer) Time(fn func()) time.Duration {
	start := time.Now()
	fn()
	d := time.Since(start)
	f.mu.Lock()
	f.samples = append(f.samples, d)
	if d > f.Budget {
		f.Overruns++
	}
	f.mu.Unlock()
	return d
}

// P95 returns the 95th percentile.
func (f *FrameTimer) P95() time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.samples) == 0 {
		return 0
	}
	cp := append([]time.Duration{}, f.samples...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	return cp[int(float64(len(cp)-1)*0.95)]
}

// Report summarizes.
func (f *FrameTimer) Report() string {
	return fmt.Sprintf("frames=%d p95=%s overruns=%d budget=%s",
		len(f.samples), f.P95(), f.Overruns, f.Budget)
}
