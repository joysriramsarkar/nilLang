package layout

import (
	"strings"
	"testing"
)

func TestConstraintsTightLoose(t *testing.T) {
	tc := Tight(100, 50)
	if !tc.IsTight() {
		t.Fatal("tight must report tight")
	}
	w, h := Loose(200, 200).Constrain(500, -5)
	if w != 200 || h != 0 {
		t.Fatalf("clamp broken: %v,%v", w, h)
	}
}

func TestUnitsResolve(t *testing.T) {
	if got := Dp(10).Resolve(2, 0, 1); got != 20 {
		t.Fatalf("dp broken: %v", got)
	}
	if got := (Length{10, UnitSp}).Resolve(2, 0, 1.5); got != 30 {
		t.Fatalf("sp broken: %v", got)
	}
	if got := (Length{50, UnitPercent}).Resolve(1, 200, 1); got != 100 {
		t.Fatalf("percent broken: %v", got)
	}
}

func TestBoxModelPadding(t *testing.T) {
	l := NewLeaf("a", 100, 50)
	l.Padding = Edges{Start: 10, End: 10}
	sz := l.Measure(Loose(1000, 1000))
	if sz.W != 120 || sz.H != 50 {
		t.Fatalf("padding must grow measured box: %+v", sz)
	}
}

func TestColumnStacks(t *testing.T) {
	f := NewFlex("col", DirColumn)
	f.Add(NewLeaf("a", 100, 20), 0, 0, -1)
	f.Add(NewLeaf("b", 100, 30), 0, 0, -1)
	sz := f.Measure(Loose(1000, 1000))
	if sz.H != 50 || sz.W != 100 {
		t.Fatalf("column broken: %+v", sz)
	}
	f.Arrange(Rect{0, 0, sz.W, sz.H})
	if f.Kids[1].Box.Placed().Y != 20 {
		t.Fatalf("second child must start at y=20: %+v", f.Kids[1].Box.Placed())
	}
}

func TestRowGap(t *testing.T) {
	f := NewFlex("row", DirRow)
	f.Gap = 8
	f.Add(NewLeaf("a", 10, 10), 0, 0, -1)
	f.Add(NewLeaf("b", 10, 10), 0, 0, -1)
	if sz := f.Measure(Loose(1000, 1000)); sz.W != 28 {
		t.Fatalf("row+gap broken: %+v", sz)
	}
}

func TestStackLargestChild(t *testing.T) {
	s := NewStack("s")
	s.Add(NewLeaf("a", 10, 10)).Add(NewLeaf("b", 30, 5))
	if sz := s.Measure(Loose(1000, 1000)); sz.W != 30 || sz.H != 10 {
		t.Fatalf("stack broken: %+v", sz)
	}
}

func TestGridTracks(t *testing.T) {
	g := NewGrid("g", 2)
	for i := 0; i < 4; i++ {
		g.Add(NewLeaf("c", 10, 10), 1)
	}
	sz := g.Measure(Constraints{MaxW: 100, MaxH: 1000})
	if sz.W != 100 {
		t.Fatalf("grid width broken: %+v", sz)
	}
	g.Arrange(Rect{0, 0, sz.W, sz.H})
	// Second row starts after first row height.
	if g.Kids[2].Placed().Y <= 0 {
		t.Fatalf("grid rows must stack: %+v", g.Kids[2].Placed())
	}
}

func TestGridSpan(t *testing.T) {
	g := NewGrid("g", 2)
	g.Add(NewLeaf("wide", 10, 10), 2)
	g.Add(NewLeaf("n", 10, 10), 1)
	g.Measure(Constraints{MaxW: 100, MaxH: 1000})
	// wide spans full width.
	if w := g.Kids[0].Measured().W; w != 100 {
		t.Fatalf("span must fill: %v", w)
	}
}

func TestScrollUnbounded(t *testing.T) {
	s := NewScroll("v", false)
	col := NewFlex("col", DirColumn)
	col.Add(NewLeaf("a", 100, 2000), 0, 0, -1)
	s.SetChild(col)
	sz := s.Measure(Constraints{MaxW: 300, MaxH: 600})
	if sz.H != 600 || col.Measured().H != 2000 {
		t.Fatalf("scroll viewport/content broken: %+v / %+v", sz, col.Measured())
	}
}

func TestOverflowRules(t *testing.T) {
	r := Rect{0, 0, 100, 100}
	kids := []Rect{{90, 90, 50, 50}}
	if out := CheckOverflow(r, kids, OverflowClip); len(out) != 1 || !strings.Contains(out[0], "clipped") {
		t.Fatalf("clip broken: %v", out)
	}
	if out := CheckOverflow(r, kids, OverflowError); !strings.Contains(out[0], "ERROR") {
		t.Fatalf("error rule broken: %v", out)
	}
	if out := CheckOverflow(r, []Rect{{10, 10, 5, 5}}, OverflowClip); len(out) != 0 {
		t.Fatalf("inside must not report: %v", out)
	}
}

func TestIntrinsicText(t *testing.T) {
	tb := NewText("t", "hello", 10)
	sz := tb.Measure(Loose(1000, 1000))
	if sz.W <= 0 || sz.H <= 0 {
		t.Fatalf("intrinsic must be positive: %+v", sz)
	}
	wide := NewText("t2", "hello world, longer", 10)
	if w2 := wide.Measure(Loose(1000, 1000)); w2.W <= sz.W {
		t.Fatalf("longer text must be wider")
	}
}

func TestRTLRow(t *testing.T) {
	f := NewFlex("row", DirRow)
	f.RTL = true
	f.Add(NewLeaf("a", 10, 10), 0, 0, -1)
	f.Add(NewLeaf("b", 20, 10), 0, 0, -1)
	sz := f.Measure(Loose(1000, 1000))
	f.Arrange(Rect{0, 0, sz.W, sz.H})
	// In RTL the first child sits at the right edge.
	if x := f.Kids[0].Box.Placed().X; x != sz.W-10 {
		t.Fatalf("RTL first child must be right-aligned: %v (w=%v)", x, sz.W)
	}
}

func TestFoldableTwoPane(t *testing.T) {
	narrow := NewTwoPane("tp", NewLeaf("m", 100, 100), NewLeaf("d", 100, 100))
	narrow.Measure(Constraints{MaxW: 400, MaxH: 800})
	narrow.Arrange(Rect{0, 0, 400, 800})
	if narrow.Detail.Placed().W != 0 {
		t.Fatalf("narrow must show master only: %+v", narrow.Detail.Placed())
	}
	wide := NewTwoPane("tp", NewLeaf("m", 100, 100), NewLeaf("d", 100, 100))
	wide.FoldX = 300
	wide.Measure(Constraints{MaxW: 800, MaxH: 800})
	wide.Arrange(Rect{0, 0, 800, 800})
	if wide.Master.Placed().W != 300 || wide.Detail.Placed().X != 300 {
		t.Fatalf("fold must split at 300: %+v %+v", wide.Master.Placed(), wide.Detail.Placed())
	}
}

func TestDebuggerDumpsTree(t *testing.T) {
	f := NewFlex("col", DirColumn)
	f.Add(NewLeaf("a", 5, 5), 0, 0, -1)
	f.Measure(Loose(100, 100))
	f.Arrange(Rect{0, 0, 5, 5})
	d := f.Debug(0)
	if !strings.Contains(d, "Flex:col") || !strings.Contains(d, "Leaf:a") {
		t.Fatalf("debug dump broken:\n%s", d)
	}
}

// Flex distribution sanity across many generated cases (U14 layout volume).
func TestFlexMatrix(t *testing.T) {
	n := 0
	for _, dir := range []FlexDir{DirColumn, DirRow} {
		for kids := 1; kids <= 5; kids++ {
			for gap := 0.0; gap <= 8; gap += 4 {
				f := NewFlex("m", dir)
				f.Gap = gap
				for k := 0; k < kids; k++ {
					f.Add(NewLeaf("c", 10, 5), 0, 0, -1)
				}
				sz := f.Measure(Loose(10000, 10000))
				var want float64
				if dir == DirColumn {
					want = float64(kids*5) + gap*float64(kids-1)
					if sz.H != want {
						t.Fatalf("matrix col kids=%d gap=%v: %+v", kids, gap, sz)
					}
				} else {
					want = float64(kids*10) + gap*float64(kids-1)
					if sz.W != want {
						t.Fatalf("matrix row kids=%d gap=%v: %+v", kids, gap, sz)
					}
				}
				n++
			}
		}
	}
	if n < 20 {
		t.Fatalf("matrix too small: %d", n)
	}
}
