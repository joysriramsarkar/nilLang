package gpu

import (
	"fmt"
	"image"
)

// TextureFormat represents the pixel format
type TextureFormat int

const (
	TextureRGBA8 TextureFormat = iota
	TextureRGB8
	TextureR8
	TextureDepth24Stencil8
	TextureDepth32F
)

// TextureFilter represents texture filtering mode
type TextureFilter int

const (
	FilterNearest TextureFilter = iota
	FilterLinear
	FilterLinearMipmap
)

// TextureWrap represents texture wrapping mode
type TextureWrap int

const (
	WrapClamp TextureWrap = iota
	WrapRepeat
	WrapMirror
)

// Texture represents a GPU texture
type Texture struct {
	ID       uint32
	Width    int
	Height   int
	Format   TextureFormat
	Filter   TextureFilter
	Wrap     TextureWrap
	Mipmaps  int
	Data     []byte
}

// TextureManager manages textures
type TextureManager struct {
	backend  BackendType
	textures map[uint32]*Texture
	nextID   uint32
	maxSize  int
}

// NewTextureManager creates a new texture manager
func NewTextureManager(backend BackendType) *TextureManager {
	return &TextureManager{
		backend:  backend,
		textures: make(map[uint32]*Texture),
		nextID:   1,
		maxSize:  4096,
	}
}

// CreateTexture creates a new texture
func (tm *TextureManager) CreateTexture(width, height int, format TextureFormat) (*Texture, error) {
	if width > tm.maxSize || height > tm.maxSize {
		return nil, fmt.Errorf("texture size exceeds maximum (%dx%d)", tm.maxSize, tm.maxSize)
	}

	texture := &Texture{
		ID:     tm.nextID,
		Width:  width,
		Height: height,
		Format: format,
		Filter: FilterLinear,
		Wrap:   WrapClamp,
	}

	tm.nextID++
	tm.textures[texture.ID] = texture

	return texture, nil
}

// CreateTextureFromImage creates a texture from a Go image
func (tm *TextureManager) CreateTextureFromImage(img image.Image) (*Texture, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	texture, err := tm.CreateTexture(width, height, TextureRGBA8)
	if err != nil {
		return nil, err
	}

	// Extract pixel data
	data := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, a := img.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			idx := (y*width + x) * 4
			data[idx] = uint8(r >> 8)
			data[idx+1] = uint8(g >> 8)
			data[idx+2] = uint8(b >> 8)
			data[idx+3] = uint8(a >> 8)
		}
	}

	texture.Data = data
	return texture, nil
}

// UpdateTexture updates texture data
func (tm *TextureManager) UpdateTexture(texture *Texture, data []byte) error {
	expectedSize := texture.Width * texture.Height * 4
	if len(data) != expectedSize {
		return fmt.Errorf("data size mismatch: expected %d, got %d", expectedSize, len(data))
	}
	texture.Data = data
	return nil
}

// DeleteTexture deletes a texture
func (tm *TextureManager) DeleteTexture(id uint32) error {
	if _, exists := tm.textures[id]; !exists {
		return fmt.Errorf("texture not found: %d", id)
	}
	delete(tm.textures, id)
	return nil
}

// GetTexture returns a texture by ID
func (tm *TextureManager) GetTexture(id uint32) *Texture {
	return tm.textures[id]
}

// CreateSolidColor creates a solid color texture
func (tm *TextureManager) CreateSolidColor(width, height int, color Color) (*Texture, error) {
	texture, err := tm.CreateTexture(width, height, TextureRGBA8)
	if err != nil {
		return nil, err
	}

	data := make([]byte, width*height*4)
	r := uint8(color.R * 255)
	g := uint8(color.G * 255)
	b := uint8(color.B * 255)
	a := uint8(color.A * 255)

	for i := 0; i < width*height; i++ {
		data[i*4] = r
		data[i*4+1] = g
		data[i*4+2] = b
		data[i*4+3] = a
	}

	texture.Data = data
	return texture, nil
}