package gpu

import "math"

// Vec2 represents a 2D vector
type Vec2 struct {
	X, Y float32
}

// Vec3 represents a 3D vector
type Vec3 struct {
	X, Y, Z float32
}

// Vec4 represents a 4D vector
type Vec4 struct {
	X, Y, Z, W float32
}

// Mat4 represents a 4x4 matrix
type Mat4 struct {
	M [16]float32
}

// Identity returns an identity matrix
func Identity() Mat4 {
	return Mat4{M: [16]float32{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}}
}

// Translate creates a translation matrix
func Translate(x, y, z float32) Mat4 {
	m := Identity()
	m.M[12] = x
	m.M[13] = y
	m.M[14] = z
	return m
}

// Scale creates a scale matrix
func ScaleMat(x, y, z float32) Mat4 {
	m := Identity()
	m.M[0] = x
	m.M[5] = y
	m.M[10] = z
	return m
}

// RotateZ creates a rotation matrix around Z axis
func RotateZ(angle float32) Mat4 {
	rad := float32(angle * math.Pi / 180.0)
	c := float32(math.Cos(float64(rad)))
	s := float32(math.Sin(float64(rad)))

	m := Identity()
	m.M[0] = c
	m.M[1] = s
	m.M[4] = -s
	m.M[5] = c
	return m
}

// Ortho creates an orthographic projection matrix
func Ortho(left, right, bottom, top, near, far float32) Mat4 {
	m := Identity()
	m.M[0] = 2.0 / (right - left)
	m.M[5] = 2.0 / (top - bottom)
	m.M[10] = -2.0 / (far - near)
	m.M[12] = -(right + left) / (right - left)
	m.M[13] = -(top + bottom) / (top - bottom)
	m.M[14] = -(far + near) / (far - near)
	return m
}

// Multiply multiplies two matrices
func (a Mat4) Multiply(b Mat4) Mat4 {
	var result Mat4
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			sum := float32(0)
			for k := 0; k < 4; k++ {
				sum += a.M[i*4+k] * b.M[k*4+j]
			}
			result.M[i*4+j] = sum
		}
	}
	return result
}

// TransformPoint transforms a point by the matrix
func (m Mat4) TransformPoint(v Vec2) Vec2 {
	x := m.M[0]*v.X + m.M[4]*v.Y + m.M[12]
	y := m.M[1]*v.X + m.M[5]*v.Y + m.M[13]
	return Vec2{X: x, Y: y}
}

// Lerp performs linear interpolation
func Lerp(a, b, t float32) float32 {
	return a + (b-a)*t
}

// Clamp clamps a value between min and max
func Clamp(v, min, max float32) float32 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
