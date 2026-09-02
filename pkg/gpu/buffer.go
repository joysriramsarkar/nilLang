package gpu

// BufferType represents the type of GPU buffer
type BufferType int

const (
	BufferVertex BufferType = iota
	BufferIndex
	BufferUniform
	BufferStorage
)

// BufferUsage represents buffer usage hints
type BufferUsage int

const (
	UsageStaticDraw BufferUsage = iota
	UsageDynamicDraw
	UsageStreamDraw
)

// Buffer represents a GPU buffer
type Buffer struct {
	ID       uint32
	Type     BufferType
	Usage    BufferUsage
	Size     int
	Data     []byte
	Mapped   bool
}

// Vertex represents a vertex for rendering
type Vertex struct {
	Position   Vec2
	Color      Color
	TexCoord   Vec2
	BorderRadius float32
}

// VertexBuffer manages vertex data
type VertexBuffer struct {
	vertices []Vertex
	indices  []uint32
	dirty    bool
}

// NewVertexBuffer creates a new vertex buffer
func NewVertexBuffer() *VertexBuffer {
	return &VertexBuffer{
		vertices: []Vertex{},
		indices:  []uint32{},
	}
}

// AddQuad adds a quad (two triangles) to the vertex buffer
func (vb *VertexBuffer) AddQuad(x, y, w, h float32, color Color, texCoords [4]Vec2) {
	baseIndex := uint32(len(vb.vertices))

	// Four corners
	vb.vertices = append(vb.vertices,
		Vertex{Position: Vec2{x, y}, Color: color, TexCoord: texCoords[0]},
		Vertex{Position: Vec2{x + w, y}, Color: color, TexCoord: texCoords[1]},
		Vertex{Position: Vec2{x + w, y + h}, Color: color, TexCoord: texCoords[2]},
		Vertex{Position: Vec2{x, y + h}, Color: color, TexCoord: texCoords[3]},
	)

	// Two triangles
	vb.indices = append(vb.indices,
		baseIndex, baseIndex+1, baseIndex+2,
		baseIndex, baseIndex+2, baseIndex+3,
	)

	vb.dirty = true
}

// AddRoundedQuad adds a rounded rectangle quad
func (vb *VertexBuffer) AddRoundedQuad(x, y, w, h, radius float32, color Color) {
	baseIndex := uint32(len(vb.vertices))
	texCoords := [4]Vec2{{0, 0}, {1, 0}, {1, 1}, {0, 1}}

	vb.vertices = append(vb.vertices,
		Vertex{Position: Vec2{x, y}, Color: color, TexCoord: texCoords[0], BorderRadius: radius},
		Vertex{Position: Vec2{x + w, y}, Color: color, TexCoord: texCoords[1], BorderRadius: radius},
		Vertex{Position: Vec2{x + w, y + h}, Color: color, TexCoord: texCoords[2], BorderRadius: radius},
		Vertex{Position: Vec2{x, y + h}, Color: color, TexCoord: texCoords[3], BorderRadius: radius},
	)

	vb.indices = append(vb.indices,
		baseIndex, baseIndex+1, baseIndex+2,
		baseIndex, baseIndex+2, baseIndex+3,
	)

	vb.dirty = true
}

// Clear clears the vertex buffer
func (vb *VertexBuffer) Clear() {
	vb.vertices = vb.vertices[:0]
	vb.indices = vb.indices[:0]
	vb.dirty = true
}

// GetVertexCount returns the number of vertices
func (vb *VertexBuffer) GetVertexCount() int {
	return len(vb.vertices)
}

// GetIndexCount returns the number of indices
func (vb *VertexBuffer) GetIndexCount() int {
	return len(vb.indices)
}

// UniformBuffer represents a uniform buffer
type UniformBuffer struct {
	Projection Mat4
	Model      Mat4
	Scissor    Vec4
	Time       float32
	Resolution Vec2
}

// NewUniformBuffer creates a new uniform buffer
func NewUniformBuffer(width, height int) *UniformBuffer {
	return &UniformBuffer{
		Projection: Ortho(0, float32(width), float32(height), 0, -1, 1),
		Model:      Identity(),
		Scissor:    Vec4{0, 0, float32(width), float32(height)},
		Resolution: Vec2{float32(width), float32(height)},
	}
}