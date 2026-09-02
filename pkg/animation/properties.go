package animation

// AnimatableProperty defines a property that can be animated
type AnimatableProperty struct {
	Name         string
	MinValue     float64
	MaxValue     float64
	DefaultValue float64
	Unit         string
}

// Standard animatable properties
var (
	PropX = AnimatableProperty{
		Name: "x", MinValue: -10000, MaxValue: 10000, DefaultValue: 0, Unit: "px",
	}
	PropY = AnimatableProperty{
		Name: "y", MinValue: -10000, MaxValue: 10000, DefaultValue: 0, Unit: "px",
	}
	PropWidth = AnimatableProperty{
		Name: "width", MinValue: 0, MaxValue: 10000, DefaultValue: 100, Unit: "px",
	}
	PropHeight = AnimatableProperty{
		Name: "height", MinValue: 0, MaxValue: 10000, DefaultValue: 100, Unit: "px",
	}
	PropScale = AnimatableProperty{
		Name: "scale", MinValue: 0, MaxValue: 100, DefaultValue: 1, Unit: "",
	}
	PropRotation = AnimatableProperty{
		Name: "rotation", MinValue: -3600, MaxValue: 3600, DefaultValue: 0, Unit: "deg",
	}
	PropOpacity = AnimatableProperty{
		Name: "opacity", MinValue: 0, MaxValue: 1, DefaultValue: 1, Unit: "",
	}
	PropBorderRadius = AnimatableProperty{
		Name: "borderRadius", MinValue: 0, MaxValue: 1000, DefaultValue: 0, Unit: "px",
	}
	PropBackgroundColor = AnimatableProperty{
		Name: "backgroundColor", MinValue: 0, MaxValue: 16777215, DefaultValue: 0, Unit: "color",
	}
	PropFontSize = AnimatableProperty{
		Name: "fontSize", MinValue: 1, MaxValue: 1000, DefaultValue: 16, Unit: "px",
	}
	PropPadding = AnimatableProperty{
		Name: "padding", MinValue: 0, MaxValue: 1000, DefaultValue: 0, Unit: "px",
	}
	PropMargin = AnimatableProperty{
		Name: "margin", MinValue: -1000, MaxValue: 1000, DefaultValue: 0, Unit: "px",
	}
	PropShadowBlur = AnimatableProperty{
		Name: "shadowBlur", MinValue: 0, MaxValue: 1000, DefaultValue: 0, Unit: "px",
	}
	PropShadowOffsetX = AnimatableProperty{
		Name: "shadowOffsetX", MinValue: -1000, MaxValue: 1000, DefaultValue: 0, Unit: "px",
	}
	PropShadowOffsetY = AnimatableProperty{
		Name: "shadowOffsetY", MinValue: -1000, MaxValue: 1000, DefaultValue: 0, Unit: "px",
	}
)

// GetProperty returns a property by name
func GetProperty(name string) *AnimatableProperty {
	props := []AnimatableProperty{
		PropX, PropY, PropWidth, PropHeight, PropScale,
		PropRotation, PropOpacity, PropBorderRadius,
		PropBackgroundColor, PropFontSize, PropPadding,
		PropMargin, PropShadowBlur, PropShadowOffsetX, PropShadowOffsetY,
	}

	for _, prop := range props {
		if prop.Name == name {
			return &prop
		}
	}
	return nil
}

// Clamp clamps a value to the property's range
func (p *AnimatableProperty) Clamp(value float64) float64 {
	if value < p.MinValue {
		return p.MinValue
	}
	if value > p.MaxValue {
		return p.MaxValue
	}
	return value
}
