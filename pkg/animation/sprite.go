package animation

import (
	"fmt"
	"math"
)

// Sprite represents an animatable visual element
type Sprite struct {
	Name       string
	X          float64
	Y          float64
	Width      float64
	Height     float64
	Scale      float64
	Rotation   float64 // degrees
	Opacity    float64 // 0.0 - 1.0
	Visible    bool
	ZIndex     int
	Children   []*Sprite
	Parent     *Sprite
	Animations []*Animation
}

// NewSprite creates a new sprite
func NewSprite(name string) *Sprite {
	return &Sprite{
		Name:     name,
		X:        0,
		Y:        0,
		Width:    100,
		Height:   100,
		Scale:    1.0,
		Rotation: 0,
		Opacity:  1.0,
		Visible:  true,
		ZIndex:   0,
		Children: []*Sprite{},
	}
}

// SetPosition sets the sprite position
func (s *Sprite) SetPosition(x, y float64) *Sprite {
	s.X = x
	s.Y = y
	return s
}

// SetSize sets the sprite size
func (s *Sprite) SetSize(width, height float64) *Sprite {
	s.Width = width
	s.Height = height
	return s
}

// SetScale sets the sprite scale
func (s *Sprite) SetScale(scale float64) *Sprite {
	s.Scale = scale
	return s
}

// SetRotation sets the sprite rotation
func (s *Sprite) SetRotation(degrees float64) *Sprite {
	s.Rotation = degrees
	return s
}

// SetOpacity sets the sprite opacity
func (s *Sprite) SetOpacity(opacity float64) *Sprite {
	if opacity < 0 {
		opacity = 0
	}
	if opacity > 1 {
		opacity = 1
	}
	s.Opacity = opacity
	return s
}

// AddChild adds a child sprite
func (s *Sprite) AddChild(child *Sprite) *Sprite {
	child.Parent = s
	s.Children = append(s.Children, child)
	return s
}

// Animate applies an animation to the sprite
func (s *Sprite) Animate(anim *Animation) *Sprite {
	s.Animations = append(s.Animations, anim)
	return s
}

// ApplyAnimation applies animation values to sprite properties
func (s *Sprite) ApplyAnimation(anim *Animation, time float64) {
	values := anim.GetValuesAt(time)

	for prop, value := range values {
		switch prop {
		case "x":
			s.X = value
		case "y":
			s.Y = value
		case "scale":
			s.Scale = value
		case "rotation":
			s.Rotation = value
		case "opacity":
			s.Opacity = value
		case "width":
			s.Width = value
		case "height":
			s.Height = value
		}
	}
}

// GetBounds returns the sprite's bounding box
func (s *Sprite) GetBounds() (x, y, w, h float64) {
	scaledW := s.Width * s.Scale
	scaledH := s.Height * s.Scale
	return s.X - scaledW/2, s.Y - scaledH/2, scaledW, scaledH
}

// Intersects checks if this sprite intersects with another
func (s *Sprite) Intersects(other *Sprite) bool {
	x1, y1, w1, h1 := s.GetBounds()
	x2, y2, w2, h2 := other.GetBounds()

	return x1 < x2+w2 && x1+w1 > x2 && y1 < y2+h2 && y1+h1 > y2
}

// DistanceTo returns the distance to another sprite
func (s *Sprite) DistanceTo(other *Sprite) float64 {
	dx := other.X - s.X
	dy := other.Y - s.Y
	return math.Sqrt(dx*dx + dy*dy)
}

// String returns a string representation
func (s *Sprite) String() string {
	return fmt.Sprintf("Sprite(%s: x=%.1f, y=%.1f, scale=%.2f, rot=%.1f°, opacity=%.2f)",
		s.Name, s.X, s.Y, s.Scale, s.Rotation, s.Opacity)
}