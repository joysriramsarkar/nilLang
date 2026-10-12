package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCatalogHasFiveComponents(t *testing.T) {
	catalog := Catalog()
	if len(catalog) != 5 {
		t.Fatalf("expected 5 catalog components, got %d", len(catalog))
	}
	names := make(map[string]bool)
	for _, c := range catalog {
		if c.Name == "" || c.Description == "" || c.Example == "" {
			t.Fatalf("catalog component missing fields: %+v", c)
		}
		names[c.Name] = true
	}
	for _, want := range []string{"Dashboard", "Table", "Form", "Card", "Button"} {
		if !names[want] {
			t.Fatalf("catalog missing %q", want)
		}
	}
}

func TestStudioServesCatalogPage(t *testing.T) {
	server := httptest.NewServer(NewMux())
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "component catalog") {
		t.Fatalf("index page missing catalog marker: %q", body)
	}

	resp, err = http.Get(server.URL + "/api/components")
	if err != nil {
		t.Fatalf("GET /api/components: %v", err)
	}
	defer resp.Body.Close()
	var items []Component
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if len(items) != len(Catalog()) {
		t.Fatalf("API returned %d components, want %d", len(items), len(Catalog()))
	}

	resp, err = http.Get(server.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health: %v", err)
	}
	defer resp.Body.Close()
	health, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(health), `"ok":true`) {
		t.Fatalf("unexpected health body: %q", health)
	}
}
