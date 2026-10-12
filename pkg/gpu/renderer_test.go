package gpu

import (
	"testing"
)

// U4.6: the gpu package must have tests (>0), and availability detection
// must be explicit rather than hardcoded.
func TestBackendDetectionExplicit(t *testing.T) {
	t.Setenv("NIL_VULKAN", "0")
	t.Setenv("NIL_METAL", "0")
	if isVulkanAvailable() || isMetalAvailable() {
		t.Fatal("forced-off backends must report unavailable")
	}
	if got := DetectBestBackend(); got != BackendOpenGLES {
		t.Fatalf("fallback must be GLES, got %v", got)
	}
	t.Setenv("NIL_VULKAN", "1")
	if !isVulkanAvailable() {
		t.Fatal("forced-on vulkan must report available")
	}
	if got := DetectBestBackend(); got != BackendVulkan {
		t.Fatalf("vulkan must win when available, got %v", got)
	}
}

func TestCreateAllBackends(t *testing.T) {
	for _, b := range []BackendType{BackendVulkan, BackendMetal, BackendOpenGLES, BackendSoftware} {
		r, err := CreateRenderer(b)
		if err != nil || r == nil {
			t.Fatalf("backend %v must construct: %v", b, err)
		}
		if r.GetBackend() != b {
			t.Fatalf("backend mismatch: %v", r.GetBackend())
		}
		if err := r.Init(64, 48); err != nil {
			t.Fatalf("init %v: %v", b, err)
		}
		if !r.IsInitialized() {
			t.Fatalf("%v must report initialized", b)
		}
		if err := r.BeginFrame(); err != nil {
			t.Fatalf("begin %v: %v", b, err)
		}
		r.Clear(0, 0, 0, 1)
		r.DrawRect(1, 1, 10, 10, ColorWhite)
		r.DrawCircle(20, 20, 5, ColorRed)
		r.DrawLine(0, 0, 10, 10, ColorGreen, 1)
		if err := r.EndFrame(); err != nil {
			t.Fatalf("end %v: %v", b, err)
		}
		if err := r.Present(); err != nil {
			t.Fatalf("present %v: %v", b, err)
		}
		if err := r.Destroy(); err != nil {
			t.Fatalf("destroy %v: %v", b, err)
		}
	}
}

func TestSoftwareClear(t *testing.T) {
	r := NewSoftwareRenderer()
	if err := r.Init(4, 4); err != nil {
		t.Fatal(err)
	}
	r.Clear(1, 0, 0, 1)
	for _, c := range r.framebuffer {
		if c != ColorRed {
			t.Fatalf("clear must paint all pixels red: %+v", c)
		}
	}
}

func TestMetalTracksWork(t *testing.T) {
	m := NewMetalRenderer()
	if err := m.BeginFrame(); err == nil {
		t.Fatal("frame before init must error")
	}
	if err := m.Init(320, 240); err != nil {
		t.Fatal(err)
	}
	m.DrawRect(0, 0, 5, 5, ColorWhite)
	m.DrawText("hi", 0, 0, 12, ColorWhite)
	if m.Draws() != 2 {
		t.Fatalf("draws must count: %d", m.Draws())
	}
	if err := m.BeginFrame(); err != nil {
		t.Fatal(err)
	}
	if err := m.Present(); err != nil {
		t.Fatal(err)
	}
	if m.Frames() != 1 {
		t.Fatalf("frames must count: %d", m.Frames())
	}
}

func TestColorFromHex(t *testing.T) {
	c := ColorFromHex("#00d4ff")
	if c.R != 0 || c.B != 1 {
		t.Fatalf("hex parse broken: %+v", c)
	}
}
