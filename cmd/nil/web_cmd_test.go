package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/routing"
	"github.com/joysriramsarkar/nilLang/pkg/alap/server"
)

func TestDeclarativeUIEventRoutePersistsState(t *testing.T) {
	source := `
component Counter {
    state count = 0;
    render {
        return {
            "type": "Page",
            "title": "Counter",
            "content": [
                {"type": "Card", "title": "Count", "body": "\(count)"},
                {"type": "Button", "id": "increment", "label": "Increment", "event": "increment"}
            ]
        };
    }
    on increment { count = count + 1; }
}`

	svc := server.NewService("test", "")
	registerDeclarativeUIEventRoute(svc, newDeclarativeUISessionStore(source))
	httpServer := httptest.NewServer(svc)
	defer httpServer.Close()
	client := newCookieClient(t)

	for expected := 1; expected <= 2; expected++ {
		body := postEvent(t, client, httpServer.URL, `{"event":"increment"}`)
		wantBody := "<p>" + string(rune('0'+expected)) + "</p>"
		if !strings.Contains(body, wantBody) {
			t.Fatalf("event %d HTML does not contain %s: %s", expected, wantBody, body)
		}
		wantState := `{"count":` + string(rune('0'+expected)) + `}`
		if !strings.Contains(body, wantState) {
			t.Fatalf("event %d HTML does not contain state %s: %s", expected, wantState, body)
		}
	}
}

func TestDeclarativeUIEventRouteIsolatesBrowserSessionsAndPassesPayload(t *testing.T) {
	source := `
component Counter {
    state count = 0;
    state name = "";
    render {
        return {
            "type": "Page",
            "content": [{"type": "Card", "body": "\(name):\(count)"}]
        };
    }
    on increment(payload) {
        count = count + 1;
        name = payload["user"]["name"];
    }
}`
	svc := server.NewService("test", "")
	registerDeclarativeUIEventRoute(svc, newDeclarativeUISessionStore(source))
	httpServer := httptest.NewServer(svc)
	defer httpServer.Close()

	firstBrowser := newCookieClient(t)
	secondBrowser := newCookieClient(t)
	firstBody := postEvent(t, firstBrowser, httpServer.URL, `{"event":"increment","payload":{"user":{"name":"Ada"}}}`)
	firstBody = postEvent(t, firstBrowser, httpServer.URL, `{"event":"increment","payload":{"user":{"name":"Ada"}}}`)
	secondBody := postEvent(t, secondBrowser, httpServer.URL, `{"event":"increment","payload":{"user":{"name":"Lin"}}}`)

	if !strings.Contains(firstBody, "<p>Ada:2</p>") {
		t.Fatalf("first browser state was not retained: %s", firstBody)
	}
	if !strings.Contains(secondBody, "<p>Lin:1</p>") {
		t.Fatalf("second browser did not receive isolated state: %s", secondBody)
	}
}

func TestDeclarativeUIEventRouteRejectsInvalidRequests(t *testing.T) {
	source := `
component Counter {
    render { return {"type": "Page"}; }
    on increment {}
}`

	tests := []struct {
		name string
		body interface{}
	}{
		{name: "non-object body", body: "invalid"},
		{name: "missing event", body: map[string]interface{}{}},
		{name: "unknown event", body: map[string]interface{}{"event": "missing"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			svc := server.NewService("test", "")
			registerDeclarativeUIEventRoute(svc, newDeclarativeUISessionStore(source))
			_, status, _ := svc.HandleRequest("POST", "/__alap/event", nil, test.body)
			if status != 400 {
				t.Fatalf("status = %d, want 400", status)
			}
		})
	}
}

func TestDeclarativeUISessionStoreExpiresIdleSessions(t *testing.T) {
	store := newDeclarativeUISessionStore(`component Counter { render { return {"type": "Page"}; } }`)
	now := time.Date(2026, time.September, 9, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	firstContext := routing.NewContext("GET", "/")
	firstSession, err := store.resolve(firstContext)
	if err != nil {
		t.Fatalf("first resolve returned error: %v", err)
	}
	cookie := firstContext.Headers["Set-Cookie"]
	sessionID := strings.SplitN(strings.TrimPrefix(cookie, declarativeUISessionCookie+"="), ";", 2)[0]

	now = now.Add(declarativeUISessionTTL)
	secondContext := routing.NewContext("GET", "/")
	secondContext.Cookies[declarativeUISessionCookie] = sessionID
	secondSession, err := store.resolve(secondContext)
	if err != nil {
		t.Fatalf("second resolve returned error: %v", err)
	}
	if secondSession == firstSession {
		t.Fatal("expired session was unexpectedly reused")
	}
	if len(store.sessions) != 1 {
		t.Fatalf("session count = %d, want 1 after cleanup", len(store.sessions))
	}
}

func newCookieClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New returned error: %v", err)
	}
	return &http.Client{Jar: jar}
}

func postEvent(t *testing.T, client *http.Client, baseURL, body string) string {
	t.Helper()
	response, err := client.Post(baseURL+"/__alap/event", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST event returned error: %v", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read event response: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("event status = %d, want 200; body=%s", response.StatusCode, responseBody)
	}
	return string(responseBody)
}
