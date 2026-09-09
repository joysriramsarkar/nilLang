package main

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/alap/ui"
)

func TestDeclarativeUIAppDispatchRerenders(t *testing.T) {
	app, err := loadDeclarativeUI(`
component Counter {
    state count: i32 = 0;
    render {
        return {
            "type": "Page",
            "title": "Counter",
            "content": [{"type": "Card", "title": "Count", "body": "\(count)"}]
        };
    }
    on increment {
        count = count + 1;
        emit("changed", count);
    }
}`)
	if err != nil {
		t.Fatalf("loadDeclarativeUI returned error: %v", err)
	}

	before, err := app.Render()
	if err != nil {
		t.Fatalf("initial Render returned error: %v", err)
	}
	if html := before.RenderHTML(ui.DefaultTheme()); !strings.Contains(html, "<p>0</p>") {
		t.Fatalf("initial HTML does not contain count 0: %s", html)
	}

	if err := app.Dispatch("increment"); err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}
	after, err := app.Render()
	if err != nil {
		t.Fatalf("rerender returned error: %v", err)
	}
	if html := after.RenderHTML(ui.DefaultTheme()); !strings.Contains(html, "<p>1</p>") {
		t.Fatalf("updated HTML does not contain count 1: %s", html)
	}
	if got := getHashStr(app.State(), "count"); got != "1" {
		t.Fatalf("state count = %q, want 1", got)
	}
	if got := getHashStr(app.LastEvent(), "name"); got != "changed" {
		t.Fatalf("last event name = %q, want changed", got)
	}
	if got := getHashStr(app.LastEvent(), "payload"); got != "1" {
		t.Fatalf("last event payload = %q, want 1", got)
	}
}

func TestDeclarativeUIAppDispatchesNestedPayload(t *testing.T) {
	app, err := loadDeclarativeUI(`
component Form {
    state result = "";
    render { return {"type": "Page"}; }
    on submit(payload) { result = payload["user"]["name"] + payload["tags"][0]; }
}`)
	if err != nil {
		t.Fatalf("loadDeclarativeUI returned error: %v", err)
	}

	err = app.Dispatch("submit", map[string]interface{}{
		"user": map[string]interface{}{"name": "Ada"},
		"tags": []interface{}{"!"},
	})
	if err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}
	if got := getHashStr(app.State(), "result"); got != "Ada!" {
		t.Fatalf("result = %q, want Ada!", got)
	}
}

func TestDeclarativeUIButtonIncludesPayloadMetadata(t *testing.T) {
	app, err := loadDeclarativeUI(`
component Form {
	render {
		return {
			"type": "Page",
			"content": [{
				"type": "Button",
				"id": "submit",
				"label": "Submit",
				"event": "submit",
				"payload": {"user": {"name": "Ada"}}
			}]
		};
	}
	on submit(payload) {}
}`)
	if err != nil {
		t.Fatalf("loadDeclarativeUI returned error: %v", err)
	}
	page, err := app.Render()
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	html := page.RenderHTML(ui.DefaultTheme())
	if !strings.Contains(html, `data-alap-payload="{&#34;user&#34;:{&#34;name&#34;:&#34;Ada&#34;}}"`) {
		t.Fatalf("button HTML does not include payload metadata: %s", html)
	}
}

func TestDeclarativeUIAppUsesBuildAsRenderFallback(t *testing.T) {
	app, err := loadDeclarativeUI(`
component BuiltPage {
    build {
        return {"type": "Page", "title": "Built", "content": []};
    }
}`)
	if err != nil {
		t.Fatalf("loadDeclarativeUI returned error: %v", err)
	}

	page, err := app.Render()
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if page.Title != "Built" {
		t.Fatalf("page title = %q, want Built", page.Title)
	}
}

func TestDeclarativeUIAppAcceptsStaticPage(t *testing.T) {
	app, err := loadDeclarativeUI(`let page = {"type": "Page", "title": "Static", "content": []};`)
	if err != nil {
		t.Fatalf("loadDeclarativeUI returned error: %v", err)
	}

	page, err := app.Render()
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if page.Title != "Static" {
		t.Fatalf("page title = %q, want Static", page.Title)
	}
	if err := app.Dispatch("click"); err == nil {
		t.Fatal("static Page dispatch unexpectedly succeeded")
	}
}

func TestHashToGoMap(t *testing.T) {
	app, err := loadDeclarativeUI(`component State { state count = 2; render { return {"type": "Page"}; } }`)
	if err != nil {
		t.Fatalf("loadDeclarativeUI returned error: %v", err)
	}

	state := hashToGoMap(app.State())
	if got := state["count"]; got != int64(2) {
		t.Fatalf("hydrated count = %#v, want int64(2)", got)
	}
}
