package gpu

import (
	"fmt"
)

// ShaderType represents the type of shader
type ShaderType int

const (
	ShaderVertex ShaderType = iota
	ShaderFragment
	ShaderCompute
	ShaderGeometry
)

// Shader represents a compiled shader
type Shader struct {
	ID       uint32
	Type     ShaderType
	Source   string
	Compiled bool
}

// ShaderProgram represents a linked shader program
type ShaderProgram struct {
	ID       uint32
	Vertex   *Shader
	Fragment *Shader
	Uniforms map[string]int
	Compiled bool
}

// ShaderCompiler compiles shaders
type ShaderCompiler struct {
	backend BackendType
	cache   map[string]*ShaderProgram
}

// NewShaderCompiler creates a new shader compiler
func NewShaderCompiler(backend BackendType) *ShaderCompiler {
	return &ShaderCompiler{
		backend: backend,
		cache:   make(map[string]*ShaderProgram),
	}
}

// CompileVertex compiles a vertex shader
func (sc *ShaderCompiler) CompileVertex(source string) (*Shader, error) {
	shader := &Shader{
		Type:   ShaderVertex,
		Source: source,
	}

	// In a real implementation, this would call the GPU API
	// For Vulkan: vkCreateShaderModule
	// For Metal: MTLDevice.makeLibrary
	// For OpenGL: glCreateShader + glShaderSource + glCompileShader

	shader.Compiled = true
	shader.ID = generateShaderID()

	return shader, nil
}

// CompileFragment compiles a fragment shader
func (sc *ShaderCompiler) CompileFragment(source string) (*Shader, error) {
	shader := &Shader{
		Type:   ShaderFragment,
		Source: source,
	}

	shader.Compiled = true
	shader.ID = generateShaderID()

	return shader, nil
}

// LinkProgram links vertex and fragment shaders into a program
func (sc *ShaderCompiler) LinkProgram(vertex, fragment *Shader) (*ShaderProgram, error) {
	if !vertex.Compiled || !fragment.Compiled {
		return nil, fmt.Errorf("shaders must be compiled before linking")
	}

	program := &ShaderProgram{
		Vertex:   vertex,
		Fragment: fragment,
		Uniforms: make(map[string]int),
		Compiled: true,
	}

	program.ID = generateProgramID()

	return program, nil
}

// CompileAndLink compiles and links a shader program
func (sc *ShaderCompiler) CompileAndLink(vertexSource, fragmentSource, name string) (*ShaderProgram, error) {
	// Check cache
	if cached, ok := sc.cache[name]; ok {
		return cached, nil
	}

	vertex, err := sc.CompileVertex(vertexSource)
	if err != nil {
		return nil, fmt.Errorf("vertex shader compilation failed: %w", err)
	}

	fragment, err := sc.CompileFragment(fragmentSource)
	if err != nil {
		return nil, fmt.Errorf("fragment shader compilation failed: %w", err)
	}

	program, err := sc.LinkProgram(vertex, fragment)
	if err != nil {
		return nil, fmt.Errorf("program linking failed: %w", err)
	}

	sc.cache[name] = program
	return program, nil
}

// Default shaders
const BasicVertexShader = `
#version 450

layout(location = 0) in vec2 inPosition;
layout(location = 1) in vec4 inColor;
layout(location = 2) in vec2 inTexCoord;

layout(binding = 0) uniform UniformBufferObject {
    mat4 projection;
    mat4 model;
} ubo;

layout(location = 0) out vec4 fragColor;
layout(location = 1) out vec2 fragTexCoord;

void main() {
    gl_Position = ubo.projection * ubo.model * vec4(inPosition, 0.0, 1.0);
    fragColor = inColor;
    fragTexCoord = inTexCoord;
}
`

const BasicFragmentShader = `
#version 450

layout(location = 0) in vec4 fragColor;
layout(location = 1) in vec2 fragTexCoord;

layout(binding = 1) uniform sampler2D texSampler;

layout(location = 0) out vec4 outColor;

void main() {
    vec4 texColor = texture(texSampler, fragTexCoord);
    outColor = fragColor * texColor;
}
`

const UIVertexShader = `
#version 450

layout(location = 0) in vec2 inPosition;
layout(location = 1) in vec4 inColor;
layout(location = 2) in vec2 inTexCoord;
layout(location = 3) in float inBorderRadius;

layout(binding = 0) uniform UniformBufferObject {
    mat4 projection;
    mat4 model;
    vec4 scissor;
} ubo;

layout(location = 0) out vec4 fragColor;
layout(location = 1) out vec2 fragTexCoord;
layout(location = 2) out float fragBorderRadius;
layout(location = 3) out vec2 fragPosition;

void main() {
    gl_Position = ubo.projection * ubo.model * vec4(inPosition, 0.0, 1.0);
    fragColor = inColor;
    fragTexCoord = inTexCoord;
    fragBorderRadius = inBorderRadius;
    fragPosition = inPosition;
}
`

const UIFragmentShader = `
#version 450

layout(location = 0) in vec4 fragColor;
layout(location = 1) in vec2 fragTexCoord;
layout(location = 2) in float fragBorderRadius;
layout(location = 3) in vec2 fragPosition;

layout(binding = 1) uniform sampler2D texSampler;

layout(location = 0) out vec4 outColor;

float roundedBoxSDF(vec2 center, vec2 size, float radius) {
    return length(max(abs(center) - size + radius, 0.0)) - radius;
}

void main() {
    vec4 texColor = texture(texSampler, fragTexCoord);
    vec4 color = fragColor * texColor;
    
    // Apply rounded corners
    if (fragBorderRadius > 0.0) {
        vec2 center = fragPosition - vec2(0.5);
        float dist = roundedBoxSDF(center, vec2(0.5), fragBorderRadius);
        float alpha = 1.0 - smoothstep(-0.005, 0.005, dist);
        color.a *= alpha;
    }
    
    outColor = color;
}
`

const TextVertexShader = `
#version 450

layout(location = 0) in vec2 inPosition;
layout(location = 1) in vec2 inTexCoord;

layout(binding = 0) uniform UniformBufferObject {
    mat4 projection;
    mat4 model;
} ubo;

layout(location = 0) out vec2 fragTexCoord;

void main() {
    gl_Position = ubo.projection * ubo.model * vec4(inPosition, 0.0, 1.0);
    fragTexCoord = inTexCoord;
}
`

const TextFragmentShader = `
#version 450

layout(location = 0) in vec2 fragTexCoord;

layout(binding = 1) uniform sampler2D fontAtlas;
layout(binding = 2) uniform TextUniform {
    vec4 color;
} textUbo;

layout(location = 0) out vec4 outColor;

void main() {
    float alpha = texture(fontAtlas, fragTexCoord).r;
    outColor = vec4(textUbo.color.rgb, textUbo.color.a * alpha);
}
`

var shaderIDCounter uint32
var programIDCounter uint32

func generateShaderID() uint32 {
	shaderIDCounter++
	return shaderIDCounter
}

func generateProgramID() uint32 {
	programIDCounter++
	return programIDCounter
}
