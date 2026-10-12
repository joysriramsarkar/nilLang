package theme

import "testing"

func TestRolesAndSpacing(t *testing.T) {
	tk := DefaultTokens()
	l, d := LightRoles(tk), DarkRoles(tk)
	if l.Primary == "" || d.Primary == "" || !d.Dark || l.Dark {
		t.Fatalf("roles broken: %+v %+v", l, d)
	}
	th := &Theme{Tokens: tk, Roles: d, Type: DefaultType()}
	for step, want := range map[int]float64{1: 4, 2: 8, 3: 12, 4: 16, 6: 24, 8: 32} {
		if th.Spacing(step) != want {
			t.Fatalf("spacing %d broken", step)
		}
	}
}

func TestTypeScaleSp(t *testing.T) {
	ts := DefaultType()
	if !(ts.Display > ts.Headline && ts.Headline > ts.Title && ts.Title > ts.Body && ts.Body > ts.Label) {
		t.Fatalf("type scale must descend: %+v", ts)
	}
}

func TestResolutionPriority(t *testing.T) {
	tk := DefaultTokens()
	app := &Theme{Tokens: tk, Roles: DarkRoles(tk)}
	child := &Theme{Tokens: tk, Parent: app} // inherits
	if got := child.Resolve(); got.Primary != app.Roles.Primary {
		t.Fatalf("child must inherit app primary: %+v", got)
	}
	local := child.Override(Roles{Primary: "#ff0000"})
	if got := local.Resolve(); got.Primary != "#ff0000" {
		t.Fatalf("local override must win: %+v", got)
	}
}

func TestContrastAA(t *testing.T) {
	if !AA("#ffffff", "#000000") {
		t.Fatal("white on black must pass AA")
	}
	if AA("#777777", "#888888") {
		t.Fatal("mid grays must fail AA")
	}
	// Dark theme text roles must pass on dark surfaces.
	d := DarkRoles(DefaultTokens())
	if !AA(d.OnSurface, d.Surface) {
		t.Fatalf("dark roles must pass AA: %v on %v", d.OnSurface, d.Surface)
	}
}

func TestAdaptive(t *testing.T) {
	th := &Theme{Tokens: DefaultTokens(), Roles: DarkRoles(DefaultTokens())}
	if c := th.Adaptive(PlatformCupertino); c.Shape.Radius != 10 || c.Elev != 0 {
		t.Fatalf("cupertino broken: %+v", c.Shape)
	}
	if h := th.Adaptive(PlatformHarmony); h.Shape.Radius != 12 {
		t.Fatalf("harmony broken: %+v", h.Shape)
	}
	if m := th.Adaptive(PlatformMaterial); m.Shape.Radius != 8 {
		t.Fatalf("material broken: %+v", m.Shape)
	}
}

func TestDynamicColor(t *testing.T) {
	r := DynamicRoles("#aabbcc", true)
	if r.Primary != "#aabbcc" || !r.Dark {
		t.Fatalf("dynamic broken: %+v", r)
	}
}
