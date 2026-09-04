package routing

import (
	"testing"
)

func TestRouter(t *testing.T) {
	router := NewRouter()

	router.GET("/products", func(ctx *Context) (interface{}, error) {
		return "products list", nil
	})

	router.GET("/users/{id}", func(ctx *Context) (interface{}, error) {
		return "user: " + ctx.Params["id"], nil
	})

	router.POST("/orders", func(ctx *Context) (interface{}, error) {
		return "order created", nil
	})

	// Test 1: Match static route
	ctx1 := NewContext("GET", "/products")
	res1, err := router.Dispatch(ctx1)
	if err != nil || res1 != "products list" {
		t.Errorf("static route dispatch failed: res=%v err=%v", res1, err)
	}

	// Test 2: Match parameterized route
	ctx2 := NewContext("GET", "/users/42")
	res2, err := router.Dispatch(ctx2)
	if err != nil || res2 != "user: 42" {
		t.Errorf("param route dispatch failed: res=%v err=%v", res2, err)
	}

	// Test 3: Match POST route
	ctx3 := NewContext("POST", "/orders")
	res3, err := router.Dispatch(ctx3)
	if err != nil || res3 != "order created" {
		t.Errorf("POST route dispatch failed: res=%v err=%v", res3, err)
	}

	// Test 4: 404 Route
	ctx4 := NewContext("GET", "/unknown")
	_, err4 := router.Dispatch(ctx4)
	if err4 == nil {
		t.Errorf("expected 404 error for unknown route, got nil")
	}
}

func TestNavigationStack(t *testing.T) {
	nav := NewNavigationStack("/home")
	if nav.Current() != "/home" {
		t.Errorf("expected current /home, got %s", nav.Current())
	}

	nav.Push("/profile")
	if nav.Current() != "/profile" {
		t.Errorf("expected current /profile, got %s", nav.Current())
	}

	popped, ok := nav.Pop()
	if !ok || popped != "/home" || nav.Current() != "/home" {
		t.Errorf("pop failed: popped=%s ok=%v current=%s", popped, ok, nav.Current())
	}
}
