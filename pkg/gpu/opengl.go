package gpu

import "fmt"

// OpenGLRenderer implements the Renderer interface using OpenGL ES
type OpenGLRenderer struct {
	initialized bool
	width       int
	height      int

	shaderCompiler *ShaderCompiler
	textureManager *TextureManager
	vertexBuffer   *VertexBuffer
	uniformBuffer  *UniformBuffer
	transformStack []Mat4

	capabilities *Capabilities
}

func NewOpenGLRenderer() *OpenGLRenderer {
	return &OpenGLRenderer{
		transformStack: []Mat4{Identity()},
		capabilities: &Capabilities{
			MaxTextureSize:   4096,
			MaxRenderTargets: 4,
			SupportsCompute:  false,
			APIVersion:       "3.2",
			DeviceName:       "OpenGL ES Device",
			VendorName:       "Nilang Graphics",
		},
	}
}

func (r *OpenGLRenderer) Init(width, height int) error {
	r.width = width
	r.height = height

	fmt.Println("  [OpenGL ES] Initializing...")
	// Would call glViewport, create context, etc.

	r.shaderCompiler = NewShaderCompiler(BackendOpenGLES)
	r.textureManager = NewTextureManager(BackendOpenGLES)
	r.vertexBuffer = NewVertexBuffer()
	r.uniformBuffer = NewUniformBuffer(width, height)

	r.initialized = true
	fmt.Println("  [OpenGL ES] ✅ Initialization complete")
	return nil
}

func (r *OpenGLRenderer) Destroy() error {
	r.initialized = false
	return nil
}

func (r *OpenGLRenderer) BeginFrame() error {
	r.vertexBuffer.Clear()
	return nil
}

func (r *OpenGLRenderer) EndFrame() error { return nil }
func (r *OpenGLRenderer) Present() error  { return nil }

func (r *OpenGLRenderer) Clear(red, g, b, a float32) {
	// glClearColor + glClear
}

func (r *OpenGLRenderer) DrawRect(x, y, w, h float32, color Color) {
	texCoords := [4]Vec2{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
	r.vertexBuffer.AddQuad(x, y, w, h, color, texCoords)
}

func (r *OpenGLRenderer) DrawRoundedRect(x, y, w, h, radius float32, color Color) {
	r.vertexBuffer.AddRoundedQuad(x, y, w, h, radius, color)
}

func (r *OpenGLRenderer) DrawCircle(cx, cy, radius float32, color Color) {
	r.vertexBuffer.AddQuad(cx-radius, cy-radius, radius*2, radius*2, color, [4]Vec2{{0, 0}, {1, 0}, {1, 1}, {0, 1}})
}

func (r *OpenGLRenderer) DrawLine(x1, y1, x2, y2 float32, color Color, thickness float32) {
	r.vertexBuffer.AddQuad(x1, y1, x2-x1, thickness, color, [4]Vec2{{0, 0}, {1, 0}, {1, 1}, {0, 1}})
}

func (r *OpenGLRenderer) DrawText(text string, x, y float32, fontSize float32, color Color) {
	charWidth := fontSize * 0.6
	r.vertexBuffer.AddQuad(x, y, charWidth*float32(len(text)), fontSize, color, [4]Vec2{{0, 0}, {1, 0}, {1, 1}, {0, 1}})
}

func (r *OpenGLRenderer) DrawImage(texture *Texture, x, y, w, h float32) {
	texCoords := [4]Vec2{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
	r.vertexBuffer.AddQuad(x, y, w, h, ColorWhite, texCoords)
}

func (r *OpenGLRenderer) PushTransform(transform Mat4) {
	current := r.transformStack[len(r.transformStack)-1]
	r.transformStack = append(r.transformStack, current.Multiply(transform))
}

func (r *OpenGLRenderer) PopTransform() {
	if len(r.transformStack) > 1 {
		r.transformStack = r.transformStack[:len(r.transformStack)-1]
	}
}

func (r *OpenGLRenderer) SetScissor(x, y, w, h int) {
	r.uniformBuffer.Scissor = Vec4{float32(x), float32(y), float32(w), float32(h)}
}

func (r *OpenGLRenderer) GetBackend() BackendType        { return BackendOpenGLES }
func (r *OpenGLRenderer) GetCapabilities() *Capabilities { return r.capabilities }
func (r *OpenGLRenderer) IsInitialized() bool            { return r.initialized }

// SoftwareRenderer is a CPU-based fallback renderer
type SoftwareRenderer struct {
	initialized bool
	width       int
	height      int
	framebuffer []Color
}

func NewSoftwareRenderer() *SoftwareRenderer {
	return &SoftwareRenderer{}
}

func (r *SoftwareRenderer) Init(width, height int) error {
	r.width = width
	r.height = height
	r.framebuffer = make([]Color, width*height)
	r.initialized = true
	return nil
}

func (r *SoftwareRenderer) Destroy() error    { r.initialized = false; return nil }
func (r *SoftwareRenderer) BeginFrame() error { return nil }
func (r *SoftwareRenderer) EndFrame() error   { return nil }
func (r *SoftwareRenderer) Present() error    { return nil }
func (r *SoftwareRenderer) Clear(red, g, b, a float32) {
	color := Color{red, g, b, a}
	for i := range r.framebuffer {
		r.framebuffer[i] = color
	}
}
func (r *SoftwareRenderer) DrawRect(x, y, w, h float32, color Color)                          {}
func (r *SoftwareRenderer) DrawRoundedRect(x, y, w, h, radius float32, color Color)           {}
func (r *SoftwareRenderer) DrawCircle(cx, cy, radius float32, color Color)                    {}
func (r *SoftwareRenderer) DrawLine(x1, y1, x2, y2 float32, color Color, thickness float32)   {}
func (r *SoftwareRenderer) DrawText(text string, x, y float32, fontSize float32, color Color) {}
func (r *SoftwareRenderer) DrawImage(texture *Texture, x, y, w, h float32)                    {}
func (r *SoftwareRenderer) PushTransform(transform Mat4)                                      {}
func (r *SoftwareRenderer) PopTransform()                                                     {}
func (r *SoftwareRenderer) SetScissor(x, y, w, h int)                                         {}
func (r *SoftwareRenderer) GetBackend() BackendType                                           { return BackendSoftware }
func (r *SoftwareRenderer) GetCapabilities() *Capabilities                                    { return &Capabilities{} }
func (r *SoftwareRenderer) IsInitialized() bool                                               { return r.initialized }

// MetalRenderer placeholder for iOS
type MetalRenderer struct {
	initialized bool
}

func NewMetalRenderer() *MetalRenderer                                                     { return &MetalRenderer{} }
func (r *MetalRenderer) Init(width, height int) error                                      { r.initialized = true; return nil }
func (r *MetalRenderer) Destroy() error                                                    { return nil }
func (r *MetalRenderer) BeginFrame() error                                                 { return nil }
func (r *MetalRenderer) EndFrame() error                                                   { return nil }
func (r *MetalRenderer) Present() error                                                    { return nil }
func (r *MetalRenderer) Clear(red, g, b, a float32)                                        {}
func (r *MetalRenderer) DrawRect(x, y, w, h float32, color Color)                          {}
func (r *MetalRenderer) DrawRoundedRect(x, y, w, h, radius float32, color Color)           {}
func (r *MetalRenderer) DrawCircle(cx, cy, radius float32, color Color)                    {}
func (r *MetalRenderer) DrawLine(x1, y1, x2, y2 float32, color Color, thickness float32)   {}
func (r *MetalRenderer) DrawText(text string, x, y float32, fontSize float32, color Color) {}
func (r *MetalRenderer) DrawImage(texture *Texture, x, y, w, h float32)                    {}
func (r *MetalRenderer) PushTransform(transform Mat4)                                      {}
func (r *MetalRenderer) PopTransform()                                                     {}
func (r *MetalRenderer) SetScissor(x, y, w, h int)                                         {}
func (r *MetalRenderer) GetBackend() BackendType                                           { return BackendMetal }
func (r *MetalRenderer) GetCapabilities() *Capabilities                                    { return &Capabilities{} }
func (r *MetalRenderer) IsInitialized() bool                                               { return r.initialized }
