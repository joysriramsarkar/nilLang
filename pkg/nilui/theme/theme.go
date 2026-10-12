// Package theme implements U8: token layers (primitive→semantic→component),
// light/dark color roles, sp typography scale, 4/8/12/16/24/32 spacing,
// shape/elevation, resolution priority (element→parent→app→system), a
// dynamic-color interface, WCAG AA contrast checks, adaptive
// (Cupertino/Material/Harmony) variants, and local overrides.
package theme

import (
	"fmt"
	"math"
)

// ─── Tokens (U8.1) ──────────────────────────────────────────────────────────

// Color is an sRGB hex color.
type Color string

// Tokens are primitive values.
type Tokens struct {
	Colors map[string]Color
	Space  map[int]float64
	Radii  map[string]float64
}

// DefaultTokens returns the primitive layer.
func DefaultTokens() Tokens {
	return Tokens{
		Colors: map[string]Color{
			"blue500": "#005A9C", "cyan400": "#00d4ff", "slate900": "#0f172a",
			"slate800": "#1e293b", "white": "#f8fafc", "sky400": "#38bdf8",
			"red500": "#ef4444", "green500": "#22c55e", "amber400": "#facc15",
		},
		Space: map[int]float64{1: 4, 2: 8, 3: 12, 4: 16, 6: 24, 8: 32},
		Radii: map[string]float64{"sm": 4, "md": 8, "lg": 12, "xl": 16},
	}
}

// ─── Roles (U8.2) ───────────────────────────────────────────────────────────

// Roles are semantic colors.
type Roles struct {
	Primary, OnPrimary Color
	Surface, OnSurface Color
	Background, OnBack Color
	Error, OnError     Color
	Accent             Color
	Dark               bool
}

// LightRoles builds the light scheme.
func LightRoles(t Tokens) Roles {
	return Roles{Primary: t.Colors["blue500"], OnPrimary: t.Colors["white"],
		Surface: t.Colors["white"], OnSurface: t.Colors["slate900"],
		Background: t.Colors["white"], OnBack: t.Colors["slate900"],
		Error: t.Colors["red500"], OnError: t.Colors["white"],
		Accent: t.Colors["sky400"]}
}

// DarkRoles builds the dark (Onuron) scheme.
func DarkRoles(t Tokens) Roles {
	return Roles{Primary: t.Colors["cyan400"], OnPrimary: t.Colors["slate900"],
		Surface: t.Colors["slate800"], OnSurface: t.Colors["white"],
		Background: t.Colors["slate900"], OnBack: t.Colors["white"],
		Error: t.Colors["red500"], OnError: t.Colors["white"],
		Accent: t.Colors["sky400"], Dark: true}
}

// ─── Typography (U8.3) ──────────────────────────────────────────────────────

// TypeScale maps display→body in sp.
type TypeScale struct {
	Display, Headline, Title, Body, Label float64
}

// DefaultType returns the sp scale.
func DefaultType() TypeScale {
	return TypeScale{Display: 34, Headline: 24, Title: 18, Body: 14, Label: 12}
}

// ─── Shape/elevation (U8.5) ─────────────────────────────────────────────────

// Shape names corner radii.
type Shape struct {
	Radius float64
}

// Elevation is dp shadow depth.
type Elevation float64

// ─── Theme + resolution (U8.6/U8.10) ────────────────────────────────────────

// Theme is one resolved theme node.
type Theme struct {
	Tokens Tokens
	Roles  Roles
	Type   TypeScale
	Shape  Shape
	Elev   Elevation
	Parent *Theme
}

// Resolve walks element→parent→app→system (nil parent = system default).
func (t *Theme) Resolve() Roles {
	if t == nil {
		return DarkRoles(DefaultTokens())
	}
	r := t.Roles
	if r.Primary == "" && t.Parent != nil {
		p := t.Parent.Resolve()
		if r.Surface == "" {
			r.Surface = p.Surface
		}
		if r.Primary == "" {
			r.Primary = p.Primary
		}
		if r.OnSurface == "" {
			r.OnSurface = p.OnSurface
		}
	}
	return r
}

// Override returns a child theme with local roles (U8.10).
func (t *Theme) Override(r Roles) *Theme {
	return &Theme{Tokens: t.Tokens, Roles: r, Type: t.Type, Parent: t}
}

// Spacing reads the 4/8 scale (U8.4).
func (t *Theme) Spacing(step int) float64 {
	if v, ok := t.Tokens.Space[step]; ok {
		return v
	}
	return 0
}

// ─── Dynamic color (U8.7) ───────────────────────────────────────────────────

// DynamicSource is implemented by Android 12+ Material You etc.
type DynamicSource interface{ SeedColor() Color }

// DynamicRoles tints roles from a seed color.
func DynamicRoles(seed Color, dark bool) Roles {
	t := DefaultTokens()
	base := LightRoles(t)
	if dark {
		base = DarkRoles(t)
	}
	base.Primary = seed
	base.Accent = seed
	return base
}

// ─── Contrast (U8.8, WCAG AA 4.5:1) ─────────────────────────────────────────

func lum(c Color) float64 {
	var r, g, b int
	_, _ = fmt.Sscanf(string(c), "#%02x%02x%02x", &r, &g, &b)
	f := func(v int) float64 {
		x := float64(v) / 255
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*f(r) + 0.7152*f(g) + 0.0722*f(b)
}

// Contrast returns the WCAG ratio of fg on bg.
func Contrast(fg, bg Color) float64 {
	l1, l2 := lum(fg), lum(bg)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

// AA passes at 4.5:1 (normal text).
func AA(fg, bg Color) bool { return Contrast(fg, bg) >= 4.5 }

// ─── Adaptive (U8.9) ────────────────────────────────────────────────────────

// Platform variants.
type Platform int

const (
	// PlatformMaterial is Android.
	PlatformMaterial Platform = iota
	// PlatformCupertino is iOS.
	PlatformCupertino
	// PlatformHarmony is HarmonyOS.
	PlatformHarmony
)

// Adaptive tweaks per platform (corner radius + elevation conventions).
func (t *Theme) Adaptive(p Platform) *Theme {
	cp := *t
	switch p {
	case PlatformCupertino:
		cp.Shape.Radius = 10
		cp.Elev = 0
	case PlatformHarmony:
		cp.Shape.Radius = 12
		cp.Elev = 2
	default:
		cp.Shape.Radius = 8
		cp.Elev = 1
	}
	cp.Parent = t
	return &cp
}
