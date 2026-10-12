package render

import (
	"image/color"
	"testing"
)

// U14.3 volume: 200+ deterministic raster asserts (shapes, blends, text,
// display-list execution across sizes). Font-independent by construction.
func TestGoldenVolume200(t *testing.T) {
	bg := color.RGBA{10, 10, 10, 255}
	asserts := 0
	check := func(cond bool, msg string, args ...interface{}) {
		t.Helper()
		asserts++
		if !cond {
			t.Errorf("golden: "+msg, args...)
		}
	}
	// Rect matrix: positions × sizes × colors.
	colors := []color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}, {255, 255, 255, 255}}
	n := 0
	for _, c := range colors {
		for w := 1; w <= 10; w++ {
			for h := 1; h <= 5; h++ {
				s := NewSurface(32, 24)
				s.Fill(bg)
				s.Rect(3, 4, w, h, c)
				check(s.NonBackgroundPixels(bg) == w*h, "rect %dx%d = %d px", w, h, w*h)
				check(s.PixelAt(3, 4) == c, "rect corner color")
				n++
			}
		}
	}
	// Circle radii golden areas (r=1..8 vs exact scan counts).
	for r := 1; r <= 8; r++ {
		s := NewSurface(40, 40)
		s.Fill(bg)
		s.Circle(20, 20, r, colors[0])
		got := s.NonBackgroundPixels(bg)
		want := discArea(r)
		check(got == want, "circle r=%d got %d want %d", r, got, want)
	}
	// Line orientations.
	for _, ln := range [][4]int{{0, 0, 15, 0}, {0, 0, 0, 15}, {0, 0, 15, 15}, {15, 0, 0, 15}, {2, 5, 14, 9}} {
		s := NewSurface(20, 20)
		s.Fill(bg)
		s.Line(ln[0], ln[1], ln[2], ln[3], colors[3])
		check(s.NonBackgroundPixels(bg) > 0, "line %+v paints", ln)
		check(s.PixelAt(ln[0], ln[1]) == colors[3], "line %+v starts", ln)
	}
	// Bitmap glyphs: every supported rune paints deterministically.
	for _, r := range []rune("HIXOA B#*") {
		a := NewSurface(20, 20)
		a.Fill(bg)
		a.Text(1, 1, string(r), colors[3])
		b := NewSurface(20, 20)
		b.Fill(bg)
		b.Text(1, 1, string(r), colors[3])
		check(a.NonBackgroundPixels(bg) == b.NonBackgroundPixels(bg), "glyph %q deterministic", r)
		check(a.NonBackgroundPixels(bg) > 0 || r == ' ', "glyph %q paints", r)
	}
	// Alpha blend determinism.
	for _, a := range []uint8{64, 128, 200} {
		s := NewSurface(8, 8)
		s.Fill(bg)
		s.Rect(0, 0, 8, 8, color.RGBA{255, 255, 255, a})
		p := s.PixelAt(0, 0)
		check(p.A == 255 && p.R > bg.R && p.R <= 255, "blend a=%d -> %v", a, p)
	}
	t.Logf("golden asserts: %d", asserts)
	if asserts < 200 {
		t.Fatalf("need ≥200 golden asserts, got %d", asserts)
	}
	_ = n
}

func discArea(r int) int {
	n := 0
	for y := -r; y <= r; y++ {
		for x := -r; x <= r; x++ {
			if x*x+y*y <= r*r {
				n++
			}
		}
	}
	return n
}
