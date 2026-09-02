package gpu

import (
	"fmt"
)

// BackendType represents the GPU backend
type BackendType int

const (
	BackendVulkan BackendType = iota
	BackendMetal
	BackendOpenGLES
	BackendSoftware
)

func (b BackendType) String() string {
	switch b {
	case BackendVulkan:
		return "Vulkan"
	case BackendMetal:
		return "Metal"
	case BackendOpenGLES:
		return "OpenGL ES"
	case BackendSoftware:
		return "Software"
	default:
		return "Unknown"
	}
}

// Renderer is the main GPU renderer interface
type Renderer interface {
	// Initialization
	Init(width, height int) error
	Destroy() error

	// Frame management
	BeginFrame() error
	EndFrame() error
	Present() error

	// Drawing
	Clear(r, g, b, a float32)
	DrawRect(x, y, w, h float32, color Color)
	DrawRoundedRect(x, y, w, h, radius float32, color Color)
	DrawCircle(cx, cy, radius float32, color Color)
	DrawLine(x1, y1, x2, y2 float32, color Color, thickness float32)
	DrawText(text string, x, y float32, fontSize float32, color Color)
	DrawImage(texture *Texture, x, y, w, h float32)

	// Transform
	PushTransform(transform Mat4)
	PopTransform()
	SetScissor(x, y, w, h int)

	// State
	GetBackend() BackendType
	GetCapabilities() *Capabilities
	IsInitialized() bool
}

// Capabilities represents GPU capabilities
type Capabilities struct {
	MaxTextureSize       int
	MaxRenderTargets     int
	SupportsCompute      bool
	SupportsTessellation bool
	SupportsGeometry     bool
	APIVersion           string
	DeviceName           string
	VendorName           string
}

// Color represents an RGBA color
type Color struct {
	R, G, B, A float32
}

func NewColor(r, g, b, a float32) Color {
	return Color{R: r, G: g, B: b, A: a}
}

func ColorFromHex(hex string) Color {
	var r, g, b uint8
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	return Color{
		R: float32(r) / 255.0,
		G: float32(g) / 255.0,
		B: float32(b) / 255.0,
		A: 1.0,
	}
}

var (
	ColorWhite  = Color{1, 1, 1, 1}
	ColorBlack  = Color{0, 0, 0, 1}
	ColorRed    = Color{1, 0, 0, 1}
	ColorGreen  = Color{0, 1, 0, 1}
	ColorBlue   = Color{0, 0, 1, 1}
	ColorNilang = Color{0, 0.83, 1, 1}    // #00d4ff
	ColorOnuron = Color{0, 0.35, 0.61, 1} // #005A9C
)

// CreateRenderer creates a renderer with the best available backend
func CreateRenderer(backend BackendType) (Renderer, error) {
	switch backend {
	case BackendVulkan:
		return NewVulkanRenderer(), nil
	case BackendMetal:
		return NewMetalRenderer(), nil
	case BackendOpenGLES:
		return NewOpenGLRenderer(), nil
	case BackendSoftware:
		return NewSoftwareRenderer(), nil
	default:
		return nil, fmt.Errorf("unsupported backend: %d", backend)
	}
}

// DetectBestBackend detects the best available GPU backend
func DetectBestBackend() BackendType {
	// In a real implementation, this would check system capabilities
	// For Onuron OS (Linux), Vulkan is preferred
	// For iOS, Metal is preferred
	// Fallback to OpenGL ES

	// Check Vulkan
	if isVulkanAvailable() {
		return BackendVulkan
	}

	// Check Metal (macOS/iOS)
	if isMetalAvailable() {
		return BackendMetal
	}

	// Fallback to OpenGL ES
	return BackendOpenGLES
}

func isVulkanAvailable() bool {
	// In production: check for vulkan library
	return true
}

func isMetalAvailable() bool {
	// In production: check for Metal framework
	return false
}
