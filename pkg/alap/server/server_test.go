package server

import (
	"net/http/httptest"
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

func TestLiveHTTPServerAndMiddlewares(t *testing.T) {
	svc := NewService("LiveWeb", "/web")

	// Endpoint with recovery test
	svc.AddEndpoint("GET", "/panic", func(ctx *routing.Context) (interface{}, error) {
		panic("simulated fatal crash")
	})

	// HTML rendering endpoint
	svc.AddEndpoint("GET", "/home", func(ctx *routing.Context) (interface{}, error) {
		return HTMLResponse{HTML: "<h1>Welcome to Nilang</h1>"}, nil
	})

	// 1. Test live HTTP with httptest
	req := httptest.NewRequest("GET", "/web/home", nil)
	rec := httptest.NewRecorder()
	svc.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("unexpected content type: %s", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), "<h1>Welcome to Nilang</h1>") {
		t.Errorf("unexpected body: %s", rec.Body.String())
	}
	// Check security headers
	if rec.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Errorf("expected X-Frame-Options: SAMEORIGIN")
	}

	// 2. Test Panic Recovery
	panicReq := httptest.NewRequest("GET", "/web/panic", nil)
	panicRec := httptest.NewRecorder()
	svc.ServeHTTP(panicRec, panicReq)

	if panicRec.Code != 500 {
		t.Errorf("expected 500 for panic, got %d", panicRec.Code)
	}
	if !strings.Contains(panicRec.Body.String(), "panic") {
		t.Errorf("expected panic in response, got: %s", panicRec.Body.String())
	}
}

func TestRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(3, 100*time.Millisecond)

	if !limiter.Allow("ip-1") {
		t.Error("1st request should be allowed")
	}
	if !limiter.Allow("ip-1") {
		t.Error("2nd request should be allowed")
	}
	if !limiter.Allow("ip-1") {
		t.Error("3rd request should be allowed")
	}
	if limiter.Allow("ip-1") {
		t.Error("4th request should be blocked")
	}

	// Different IP should still be allowed
	if !limiter.Allow("ip-2") {
		t.Error("different IP should be allowed")
	}

	// Wait for window to expire
	time.Sleep(120 * time.Millisecond)
	if !limiter.Allow("ip-1") {
		t.Error("request after window should be allowed")
	}
}

func TestCORS(t *testing.T) {
	svc := NewService("CorsApp", "/api")
	svc.Use(CORSMiddleware(CORSConfig{
		AllowOrigins: []string{"https://example.com"},
		AllowMethods: []string{"GET", "POST"},
	}))

	svc.AddEndpoint("GET", "/data", func(ctx *routing.Context) (interface{}, error) {
		return map[string]string{"result": "ok"}, nil
	})

	req := httptest.NewRequest("OPTIONS", "/api/data", nil)
	rec := httptest.NewRecorder()
	svc.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("CORS origin missing or incorrect: %s", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}
