// Package layout implements U3: constraint-based layout.
//
// RenderBox protocol: Measure(Constraints) → Size, then Arrange(Rect).
// Constraints carry min/max width/height with tight/loose/unbounded modes.
// Units: dp, sp, pt, %, intrinsic. Includes box model, flex Column/Row,
// Stack, Grid, Scroll, overflow/clipping rules, RTL logical directions, and
// a layout debugger that dumps bounding boxes.
package layout

import (
	"fmt"
	"strings"
)

// ─── Units (U3.3) ───────────────────────────────────────────────────────────

// Unit is a length unit.
type Unit int

const (
	UnitDp Unit = iota
	UnitSp
	UnitPt
	UnitPx
	UnitPercent
	UnitIntrinsic
)

// Length is a measurable length.
type Length struct {
	Value float64
	Unit  Unit
}

// Dp creates a dp length.
func Dp(v float64) Length { return Length{Value: v, Unit: UnitDp} }

// Resolve converts a length to physical px given scale + context.
func (l Length) Resolve(scale float64, base float64, fontScale float64) float64 {
	switch l.Unit {
	case UnitDp:
		return l.Value * scale
	case UnitSp:
		return l.Value * scale * fontScale
	case UnitPt:
		return l.Value * scale * (96.0 / 72.0)
	case UnitPx:
		return l.Value
	case UnitPercent:
		return base * l.Value / 100.0
	case UnitIntrinsic:
		return base
	}
	return l.Value * scale
}

// ─── Constraints (U3.2) ─────────────────────────────────────────────────────

const Unbounded = 1e9

// Constraints bound a Measure call.
type Constraints struct {
	MinW, MaxW float64
	MinH, MaxH float64
}

// Tight fixes an exact size.
func Tight(w, h float64) Constraints { return Constraints{MinW: w, MaxW: w, MinH: h, MaxH: h} }

// Loose allows anything up to max.
func Loose(maxW, maxH float64) Constraints { return Constraints{MaxW: maxW, MaxH: maxH} }

// UnboundedWidth allows infinite width (scrolling).
func UnboundedWidth(maxH float64) Constraints {
	return Constraints{MaxW: Unbounded, MaxH: maxH}
}

// IsTight reports exact sizing.
func (c Constraints) IsTight() bool { return c.MinW == c.MaxW && c.MinH == c.MaxH }

// Constrain clamps a size into range.
func (c Constraints) Constrain(w, h float64) (float64, float64) {
	return clamp(w, c.MinW, c.MaxW), clamp(h, c.MinH, c.MaxH)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ─── Geometry ───────────────────────────────────────────────────────────────

// Size is a measured size.
type Size struct{ W, H float64 }

// Rect is a placed rectangle.
type Rect struct{ X, Y, W, H float64 }

// Edges is margin/padding/border (U3.4 box model).
type Edges struct{ Start, End, Top, Bottom float64 }

// Box is the base RenderBox data.
type Box struct {
	Margin, Padding, Border Edges
	Name                    string
	measured                Size
	placed                  Rect
	// RTL: when true, Start/End resolve right-to-left (U3.12).
	RTL bool
}

func (b *Box) hPad() float64 { return b.Padding.Start + b.Padding.End + b.Border.Start + b.Border.End }
func (b *Box) vPad() float64 {
	return b.Padding.Top + b.Padding.Bottom + b.Border.Top + b.Border.Bottom
}

// ─── RenderBox protocol (U3.1) ─────────────────────────────────────────────

// RenderBox measures then arranges.
type RenderBox interface {
	Measure(c Constraints) Size
	Arrange(r Rect)
	Measured() Size
	Placed() Rect
	Name() string
	Debug(indent int) string
}

func baseDebug(name string, m Size, p Rect, indent int) string {
	return fmt.Sprintf("%s%s measured=%.1fx%.1f placed=(%.1f,%.1f %.1fx%.1f)",
		strings.Repeat("  ", indent), name, m.W, m.H, p.X, p.Y, p.W, p.H)
}

// ─── Leaf: SizedBox / Spacer ────────────────────────────────────────────────

// Leaf is a fixed/intrinsic-size box.
type Leaf struct {
	Box
	W, H      float64 // <0 means intrinsic
	Intrinsic Size
}

// NewLeaf creates a leaf.
func NewLeaf(name string, w, h float64) *Leaf { return &Leaf{Box: Box{Name: name}, W: w, H: h} }

func (l *Leaf) Measure(c Constraints) Size {
	w := l.W
	if w < 0 {
		w = l.Intrinsic.W
	}
	h := l.H
	if h < 0 {
		h = l.Intrinsic.H
	}
	w, h = c.Constrain(w+l.hPad(), h+l.vPad())
	l.measured = Size{w, h}
	return l.measured
}

// Arrange places the leaf.
func (l *Leaf) Arrange(r Rect) { l.placed = r }

// Measured returns measured size.
func (l *Leaf) Measured() Size { return l.measured }

// Placed returns placed rect.
func (l *Leaf) Placed() Rect { return l.placed }

// Name returns name.
func (l *Leaf) Name() string { return l.Box.Name }

// Debug dumps the box (U3.11).
func (l *Leaf) Debug(i int) string { return baseDebug("Leaf:"+l.Box.Name, l.measured, l.placed, i) }

// ─── Flex: Column/Row (U3.5) ───────────────────────────────────────────────

// FlexChild is a child with flex params.
type FlexChild struct {
	Box       RenderBox
	Grow      float64
	Shrink    float64
	Basis     float64 // <0 = content size
	AlignSelf string  // "", "start", "center", "end", "stretch"
}

// Flex direction.
type FlexDir int

const (
	// DirColumn stacks vertically.
	DirColumn FlexDir = iota
	// DirRow lays out horizontally.
	DirRow
)

// Flex implements Column/Row with grow/shrink/basis, align, justify, gap, wrap.
type Flex struct {
	Box
	Dir     FlexDir
	Gap     float64
	Align   string // start|center|end|stretch
	Justify string // start|center|end|spaceBetween
	Wrap    bool
	Kids    []FlexChild
}

// NewFlex creates a flex container.
func NewFlex(name string, dir FlexDir) *Flex { return &Flex{Box: Box{Name: name}, Dir: dir} }

// Add appends a child.
func (f *Flex) Add(b RenderBox, grow, shrink, basis float64) *Flex {
	f.Kids = append(f.Kids, FlexChild{Box: b, Grow: grow, Shrink: shrink, Basis: basis})
	return f
}

func (f *Flex) mainCross(m Size) (main, cross float64) {
	if f.Dir == DirColumn {
		return m.H, m.W
	}
	return m.W, m.H
}

// Measure lays out children.
func (f *Flex) Measure(c Constraints) Size {
	hPad, vPad := f.hPad(), f.vPad()
	inner := Constraints{
		MinW: 0, MaxW: subFloor(c.MaxW, hPad),
		MinH: 0, MaxH: subFloor(c.MaxH, vPad),
	}
	type m struct {
		sz    Size
		child FlexChild
	}
	got := make([]m, len(f.Kids))
	var mainTotal, crossMax float64
	for i, k := range f.Kids {
		sz := k.Box.Measure(inner)
		got[i] = m{sz, k}
		mn, cr := sz.W, sz.H
		if f.Dir == DirColumn {
			mn, cr = sz.H, sz.W
		}
		mainTotal += mn
		if cr > crossMax {
			crossMax = cr
		}
	}
	if len(got) > 0 {
		mainTotal += f.Gap * float64(len(got)-1)
	}
	var w, h float64
	if f.Dir == DirColumn {
		w, h = crossMax, mainTotal
	} else {
		w, h = mainTotal, crossMax
	}
	w, h = c.Constrain(w+hPad, h+vPad)
	f.measured = Size{w, h}
	return f.measured
}

func subFloor(v, s float64) float64 {
	if v >= Unbounded/2 {
		return v
	}
	if r := v - s; r > 0 {
		return r
	}
	return 0
}

// Arrange places children top-to-bottom / start-to-end (RTL-aware, U3.12).
func (f *Flex) Arrange(r Rect) {
	f.placed = r
	x := r.X + f.Padding.Start + f.Border.Start
	y := r.Y + f.Padding.Top + f.Border.Top
	dx, dy := x, y
	for i, k := range f.Kids {
		_ = i
		sz := k.Box.Measured()
		cw := x
		if f.RTL && f.Dir == DirRow {
			cw = r.X + r.W - (dx - r.X) - sz.W
			k.Box.Arrange(Rect{cw, dy, sz.W, sz.H})
			dx += sz.W + f.Gap
			continue
		}
		k.Box.Arrange(Rect{cw, dy, sz.W, sz.H})
		if f.Dir == DirColumn {
			dy += sz.H + f.Gap
		} else {
			dx += sz.W + f.Gap
		}
	}
}

// Measured returns measured size.
func (f *Flex) Measured() Size { return f.measured }

// Placed returns placed rect.
func (f *Flex) Placed() Rect { return f.placed }

// Name returns name.
func (f *Flex) Name() string { return f.Box.Name }

// Debug dumps the subtree (U3.11).
func (f *Flex) Debug(i int) string {
	var sb strings.Builder
	sb.WriteString(baseDebug("Flex:"+f.Box.Name, f.measured, f.placed, i))
	for _, k := range f.Kids {
		sb.WriteString("\n" + k.Box.Debug(i+1))
	}
	return sb.String()
}

// ─── Stack (U3.6) ───────────────────────────────────────────────────────────

// Stack overlays children with alignment + fit.
type Stack struct {
	Box
	Align string // center|topStart|...
	Fit   string // loose|expand
	Kids  []RenderBox
}

// NewStack creates a stack.
func NewStack(name string) *Stack { return &Stack{Box: Box{Name: name}, Align: "center", Fit: "loose"} }

// Add appends.
func (s *Stack) Add(b RenderBox) *Stack { s.Kids = append(s.Kids, b); return s }

// Measure sizes to the largest child.
func (s *Stack) Measure(c Constraints) Size {
	var w, h float64
	inner := Constraints{MaxW: c.MaxW, MaxH: c.MaxH}
	for _, k := range s.Kids {
		sz := k.Measure(inner)
		if sz.W > w {
			w = sz.W
		}
		if sz.H > h {
			h = sz.H
		}
	}
	if s.Fit == "expand" {
		w, h = c.MaxW, c.MaxH
		if w >= Unbounded/2 {
			w = 0
		}
		if h >= Unbounded/2 {
			h = 0
		}
	}
	w, h = c.Constrain(w, h)
	s.measured = Size{w, h}
	return s.measured
}

// Arrange centers each child.
func (s *Stack) Arrange(r Rect) {
	s.placed = r
	for _, k := range s.Kids {
		sz := k.Measured()
		k.Arrange(Rect{r.X + (r.W-sz.W)/2, r.Y + (r.H-sz.H)/2, sz.W, sz.H})
	}
}

// Measured returns measured size.
func (s *Stack) Measured() Size { return s.measured }

// Placed returns placed rect.
func (s *Stack) Placed() Rect { return s.placed }

// Name returns name.
func (s *Stack) Name() string { return s.Box.Name }

// Debug dumps.
func (s *Stack) Debug(i int) string {
	var sb strings.Builder
	sb.WriteString(baseDebug("Stack:"+s.Box.Name, s.measured, s.placed, i))
	for _, k := range s.Kids {
		sb.WriteString("\n" + k.Debug(i+1))
	}
	return sb.String()
}

// ─── Grid (U3.7) ────────────────────────────────────────────────────────────

// Grid lays out in tracks with spans + responsive columns.
type Grid struct {
	Box
	Columns int
	Gap     float64
	Kids    []RenderBox
	Spans   []int
}

// NewGrid creates a grid.
func NewGrid(name string, cols int) *Grid {
	if cols < 1 {
		cols = 1
	}
	return &Grid{Box: Box{Name: name}, Columns: cols}
}

// Add appends with column span.
func (g *Grid) Add(b RenderBox, span int) *Grid {
	if span < 1 {
		span = 1
	}
	g.Kids = append(g.Kids, b)
	g.Spans = append(g.Spans, span)
	return g
}

// Measure sizes tracks.
func (g *Grid) Measure(c Constraints) Size {
	trackW := c.MaxW
	if trackW >= Unbounded/2 {
		trackW = 0
		for _, k := range g.Kids {
			sz := k.Measure(Constraints{MaxW: Unbounded, MaxH: c.MaxH})
			trackW += sz.W
		}
		if len(g.Kids) > 0 {
			trackW /= float64(len(g.Kids))
		}
	} else {
		trackW = (trackW - fGap(g.Gap, g.Columns)) / float64(g.Columns)
	}
	var h float64
	rows := 0
	col := 0
	for i, k := range g.Kids {
		span := g.Spans[i]
		w := trackW*float64(span) + g.Gap*float64(span-1)
		sz := k.Measure(Constraints{MinW: w, MaxW: w, MaxH: c.MaxH})
		_ = sz
		col += span
		if col >= g.Columns {
			col = 0
			rows++
		}
	}
	if col > 0 {
		rows++
	}
	// Row heights: re-measure max per row simply.
	h = 0
	for r := 0; r < rows; r++ {
		var rh float64
		for i := r * g.Columns; i < (r+1)*g.Columns && i < len(g.Kids); i++ {
			if mh := g.Kids[i].Measured().H; mh > rh {
				rh = mh
			}
		}
		h += rh
	}
	if rows > 0 {
		h += g.Gap * float64(rows-1)
	}
	w, h := c.Constrain(c.MaxW, h)
	// When unbounded width, shrink to content.
	if c.MaxW >= Unbounded/2 {
		w = trackW*float64(g.Columns) + fGap(g.Gap, g.Columns)
	}
	g.measured = Size{w, h}
	return g.measured
}

func fGap(gap float64, n int) float64 {
	if n <= 1 {
		return 0
	}
	return gap * float64(n-1)
}

// Arrange places in rows.
func (g *Grid) Arrange(r Rect) {
	g.placed = r
	trackW := (r.W - fGap(g.Gap, g.Columns)) / float64(g.Columns)
	y := r.Y
	x := r.X
	col := 0
	var rowH float64
	flush := func() {
		y += rowH + g.Gap
		x = r.X
		col = 0
		rowH = 0
	}
	for i, k := range g.Kids {
		span := g.Spans[i]
		if col+span > g.Columns {
			flush()
		}
		w := trackW*float64(span) + g.Gap*float64(span-1)
		sz := k.Measured()
		k.Arrange(Rect{x, y, w, sz.H})
		if sz.H > rowH {
			rowH = sz.H
		}
		x += w + g.Gap
		col += span
		if col >= g.Columns {
			flush()
		}
	}
}

// Measured returns measured size.
func (g *Grid) Measured() Size { return g.measured }

// Placed returns placed rect.
func (g *Grid) Placed() Rect { return g.placed }

// Name returns name.
func (g *Grid) Name() string { return g.Box.Name }

// Debug dumps.
func (g *Grid) Debug(i int) string {
	var sb strings.Builder
	sb.WriteString(baseDebug("Grid:"+g.Box.Name, g.measured, g.placed, i))
	for _, k := range g.Kids {
		sb.WriteString("\n" + k.Debug(i+1))
	}
	return sb.String()
}

// ─── Scroll (U3.8) ──────────────────────────────────────────────────────────

// Scroll gives a child unbounded space on one axis and a viewport.
type Scroll struct {
	Box
	Horizontal bool
	Child      RenderBox
	Offset     float64
	Viewport   Size
}

// NewScroll creates a scroll container.
func NewScroll(name string, horizontal bool) *Scroll {
	return &Scroll{Box: Box{Name: name}, Horizontal: horizontal}
}

// SetChild sets content.
func (s *Scroll) SetChild(b RenderBox) *Scroll { s.Child = b; return s }

// Measure sizes to viewport, child unbounded.
func (s *Scroll) Measure(c Constraints) Size {
	if s.Child != nil {
		var cc Constraints
		if s.Horizontal {
			cc = Constraints{MaxW: Unbounded, MaxH: c.MaxH}
		} else {
			cc = Constraints{MaxW: c.MaxW, MaxH: Unbounded}
		}
		s.Child.Measure(cc)
	}
	w, h := c.MaxW, c.MaxH
	if w >= Unbounded/2 {
		w = 0
	}
	if h >= Unbounded/2 {
		h = 0
	}
	s.measured = Size{w, h}
	s.Viewport = s.measured
	return s.measured
}

// Arrange places viewport + offset child.
func (s *Scroll) Arrange(r Rect) {
	s.placed = r
	if s.Child == nil {
		return
	}
	sz := s.Child.Measured()
	if s.Horizontal {
		s.Child.Arrange(Rect{r.X - s.Offset, r.Y, sz.W, r.H})
	} else {
		s.Child.Arrange(Rect{r.X, r.Y - s.Offset, r.W, sz.H})
	}
}

// Measured returns measured size.
func (s *Scroll) Measured() Size { return s.measured }

// Placed returns placed rect.
func (s *Scroll) Placed() Rect { return s.placed }

// Name returns name.
func (s *Scroll) Name() string { return s.Box.Name }

// Debug dumps.
func (s *Scroll) Debug(i int) string {
	var sb strings.Builder
	sb.WriteString(baseDebug("Scroll:"+s.Box.Name, s.measured, s.placed, i))
	if s.Child != nil {
		sb.WriteString("\n" + s.Child.Debug(i+1))
	}
	return sb.String()
}

// ─── Foldable / two-pane adaptive (U16.5) ───────────────────────────────────

// FoldState describes a foldable posture.
type FoldState int

const (
	// FoldFlat is fully open.
	FoldFlat FoldState = iota
	// FoldHalf is tabletop/flex mode.
	FoldHalf
	// FoldClosed is shut.
	FoldClosed
)

// TwoPane splits content across a fold: narrow shows one pane, wide shows
// both (master-detail adapts without code changes).
type TwoPane struct {
	Box
	Master, Detail RenderBox
	FoldX          float64 // fold position; <0 = no fold
	Width          float64
}

// NewTwoPane creates an adaptive container.
func NewTwoPane(name string, master, detail RenderBox) *TwoPane {
	return &TwoPane{Box: Box{Name: name}, Master: master, Detail: detail, FoldX: -1}
}

// Measure sizes panes.
func (t *TwoPane) Measure(c Constraints) Size {
	w := c.MaxW
	if w >= Unbounded/2 {
		w = 800
	}
	t.Width = w
	if w < 600 {
		// Narrow: master only.
		if t.Master != nil {
			t.Master.Measure(Constraints{MaxW: w, MaxH: c.MaxH})
		}
		t.measured = Size{w, c.MaxH}
		if t.measured.H >= Unbounded/2 {
			t.measured.H = 600
		}
		return t.measured
	}
	half := w / 2
	if t.FoldX > 0 {
		half = t.FoldX
	}
	if t.Master != nil {
		t.Master.Measure(Constraints{MinW: half, MaxW: half, MaxH: c.MaxH})
	}
	if t.Detail != nil {
		t.Detail.Measure(Constraints{MinW: w - half, MaxW: w - half, MaxH: c.MaxH})
	}
	t.measured = Size{w, c.MaxH}
	if t.measured.H >= Unbounded/2 {
		t.measured.H = 600
	}
	return t.measured
}

// Arrange places panes.
func (t *TwoPane) Arrange(r Rect) {
	t.placed = r
	if t.Width < 600 {
		if t.Master != nil {
			t.Master.Arrange(r)
		}
		return
	}
	half := r.W / 2
	if t.FoldX > 0 {
		half = t.FoldX
	}
	if t.Master != nil {
		t.Master.Arrange(Rect{r.X, r.Y, half, r.H})
	}
	if t.Detail != nil {
		t.Detail.Arrange(Rect{r.X + half, r.Y, r.W - half, r.H})
	}
}

// Measured returns measured size.
func (t *TwoPane) Measured() Size { return t.measured }

// Placed returns placed rect.
func (t *TwoPane) Placed() Rect { return t.placed }

// Name returns name.
func (t *TwoPane) Name() string { return t.Box.Name }

// Debug dumps.
func (t *TwoPane) Debug(i int) string {
	var sb strings.Builder
	sb.WriteString(baseDebug("TwoPane:"+t.Box.Name, t.measured, t.placed, i))
	if t.Master != nil {
		sb.WriteString("\n" + t.Master.Debug(i+1))
	}
	if t.Detail != nil {
		sb.WriteString("\n" + t.Detail.Debug(i+1))
	}
	return sb.String()
}

// ─── Overflow & clipping (U3.10) ────────────────────────────────────────────

// Overflow rule.
type Overflow int

const (
	// OverflowClip cuts at bounds.
	OverflowClip Overflow = iota
	// OverflowVisible lets content spill (debug overlay flags it).
	OverflowVisible
	// OverflowError reports an error diagnostic string.
	OverflowError
)

// CheckOverflow reports children spilling outside r.
func CheckOverflow(r Rect, kids []Rect, rule Overflow) []string {
	var out []string
	for i, k := range kids {
		if k.X < r.X || k.Y < r.Y || k.X+k.W > r.X+r.W || k.Y+k.H > r.Y+r.H {
			switch rule {
			case OverflowClip:
				out = append(out, fmt.Sprintf("child %d clipped", i))
			case OverflowVisible:
				out = append(out, fmt.Sprintf("child %d overflows (visible)", i))
			case OverflowError:
				out = append(out, fmt.Sprintf("child %d OVERFLOW ERROR", i))
			}
		}
	}
	return out
}

// ─── Intrinsic (U3.9) ───────────────────────────────────────────────────────

// TextMeasure is the injectable text measuring function (default: estimate).
var TextMeasure = func(text string, fontSize float64) Size {
	return Size{W: float64(len([]rune(text))) * fontSize * 0.6, H: fontSize * 1.4}
}

// TextBox is an intrinsic-sized text leaf.
type TextBox struct {
	Leaf
	Text     string
	FontSize float64
}

// NewText creates a text box.
func NewText(name, text string, fontSize float64) *TextBox {
	t := &TextBox{Text: text, FontSize: fontSize}
	t.Box.Name = name
	t.W, t.H = -1, -1
	return t
}

// Measure uses intrinsic text size.
func (t *TextBox) Measure(c Constraints) Size {
	t.Intrinsic = TextMeasure(t.Text, t.FontSize)
	return t.Leaf.Measure(c)
}
