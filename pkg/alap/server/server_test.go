package server

import (
	"strings"
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/routing"
)

func TestAlapServerService(t *testing.T) {
	// UserService matching Section 19 of refactor.md
	service := NewService("UserService", "/api")

	type User struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	// GET /api/users/{id}
	service.AddEndpoint("GET", "/users/{id}", func(ctx *routing.Context) (interface{}, error) {
		id := ctx.Params["id"]
		return User{ID: id, Name: "User " + id}, nil
	})

	// Test successful request
	res, code, err := service.HandleRequest("GET", "/api/users/101", nil, nil)
	if err != nil || code != 200 {
		t.Fatalf("request failed: code=%d err=%v", code, err)
	}
	if !strings.Contains(res, `"id":"101"`) || !strings.Contains(res, `"name":"User 101"`) {
		t.Errorf("response body invalid: %s", res)
	}

	// Test 404
	_, code404, _ := service.HandleRequest("GET", "/api/nonexistent", nil, nil)
	if code404 != 404 {
		t.Errorf("expected 404, got %d", code404)
	}

	// Test Caching
	service.CacheSet("user:101", User{ID: "101", Name: "Cached User"}, 1*time.Minute)
	cached, found := service.CacheGet("user:101")
	if !found || cached.(User).Name != "Cached User" {
		t.Errorf("cache get failed")
	}
}

func TestAuthMiddleware(t *testing.T) {
	service := NewService("SecureService", "/api")
	service.Use(AuthMiddleware("secret123"))

	service.AddEndpoint("GET", "/secret", func(ctx *routing.Context) (interface{}, error) {
		return map[string]string{"secret": "data"}, nil
	})

	// Unauthorized request
	_, code, _ := service.HandleRequest("GET", "/api/secret", nil, nil)
	if code != 401 {
		t.Errorf("expected 401 unauthorized, got %d", code)
	}

	// Authorized request
	headers := map[string]string{"Authorization": "Bearer secret123"}
	res, codeAuth, err := service.HandleRequest("GET", "/api/secret", headers, nil)
	if err != nil || codeAuth != 200 || !strings.Contains(res, "secret") {
		t.Errorf("authorized request failed: code=%d err=%v res=%s", codeAuth, err, res)
	}
}
