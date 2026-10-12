package router

import "testing"

func TestStackNavigateBack(t *testing.T) {
	r := New().AddRoute("/home", PatternStack).AddRoute("/detail", PatternStack)
	if err := r.Navigate("/home"); err != nil {
		t.Fatal(err)
	}
	if err := r.Navigate("/detail"); err != nil {
		t.Fatal(err)
	}
	if r.Active() != "/detail" || len(r.Stack()) != 2 {
		t.Fatalf("stack broken: %v", r.Stack())
	}
	if !r.Back() || r.Active() != "/home" {
		t.Fatal("back must pop to home")
	}
	if r.Back() {
		t.Fatal("back at root must return false")
	}
}

func TestTabs(t *testing.T) {
	r := New().AddRoute("/a", PatternTabs).AddRoute("/b", PatternTabs)
	r.Navigate("/a")
	r.Navigate("/b")
	if r.Active() != "/b" || len(r.Stack()) != 0 {
		t.Fatal("tabs must not push stack")
	}
}

func TestSheetAndDialog(t *testing.T) {
	r := New().AddRoute("/home", PatternStack).AddRoute("/sheet", PatternSheet)
	r.Navigate("/home")
	r.Navigate("/sheet")
	if r.Sheet() != "/sheet" {
		t.Fatal("sheet must present")
	}
	if !r.Back() || r.Sheet() != "" {
		t.Fatal("back must dismiss sheet first")
	}
	r.Dialog("/dlg")
	if r.Sheet() != "/dlg" {
		t.Fatal("dialog must present as sheet")
	}
}

func TestUnknownRoute(t *testing.T) {
	r := New()
	if err := r.Navigate("/nope"); err == nil {
		t.Fatal("unknown route must error")
	}
}

func TestDeepLink(t *testing.T) {
	p, params, err := ParseDeepLink("alap://app/detail?id=42&tab=info")
	if err != nil || p != "/detail" || params["id"] != "42" || params["tab"] != "info" {
		t.Fatalf("deeplink broken: %v %v %v", p, params, err)
	}
	if _, _, err := ParseDeepLink("https://x/y"); err == nil {
		t.Fatal("non-alap link must error")
	}
}

func TestStateRestoration(t *testing.T) {
	r := New().AddRoute("/form", PatternStack)
	r.Navigate("/form")
	r.SetState("/form", "draft", "hello")
	snap := r.SaveState()
	r2 := New().AddRoute("/form", PatternStack)
	r2.RestoreState(snap)
	if got := r2.GetState("/form", "draft"); got != "hello" {
		t.Fatalf("restoration broken: %q", got)
	}
}
