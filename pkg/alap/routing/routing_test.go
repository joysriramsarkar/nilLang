package routing

import (
	"fmt"
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

func TestRadixTreeConstraintsAndWildcards(t *testing.T) {
	r := NewRouter()

	// Typed constraint: {id:int}
	r.GET("/items/{id:int}", func(ctx *Context) (interface{}, error) {
		id, err := ctx.ParamInt("id")
		if err != nil {
			return nil, err
		}
		return fmt.Sprintf("item-%d", id), nil
	})

	// Typed constraint: {id:uuid}
	r.GET("/records/{id:uuid}", func(ctx *Context) (interface{}, error) {
		return "record:" + ctx.Param("id"), nil
	})

	// Wildcard: /static/*filepath
	r.GET("/static/*filepath", func(ctx *Context) (interface{}, error) {
		return "file:" + ctx.Param("filepath"), nil
	})

	// 1. Valid int
	res, err := r.Dispatch(NewContext("GET", "/items/123"))
	if err != nil || res != "item-123" {
		t.Errorf("expected item-123, got %v, err=%v", res, err)
	}

	// 2. Invalid int constraint should 404
	_, err = r.Dispatch(NewContext("GET", "/items/abc"))
	if err == nil {
		t.Errorf("expected 404 for non-int param")
	}

	// 3. Valid UUID
	validUUID := "123e4567-e89b-12d3-a456-426614174000"
	res, err = r.Dispatch(NewContext("GET", "/records/"+validUUID))
	if err != nil || res != "record:"+validUUID {
		t.Errorf("expected record uuid, got %v, err=%v", res, err)
	}

	// 4. Invalid UUID should 404
	_, err = r.Dispatch(NewContext("GET", "/records/not-a-uuid"))
	if err == nil {
		t.Errorf("expected 404 for invalid uuid")
	}

	// 5. Wildcard match
	res, err = r.Dispatch(NewContext("GET", "/static/css/theme/dark.css"))
	if err != nil || res != "file:css/theme/dark.css" {
		t.Errorf("expected wildcard match, got %v, err=%v", res, err)
	}
}

func TestRouteGroupsAndMiddleware(t *testing.T) {
	r := NewRouter()

	var order []string
	globalMW := func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) (interface{}, error) {
			order = append(order, "global")
			return next(ctx)
		}
	}
	groupMW := func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) (interface{}, error) {
			order = append(order, "group")
			return next(ctx)
		}
	}

	r.Use(globalMW)
	v1 := r.Group("/api/v1", groupMW)
	v1.GET("/profile", func(ctx *Context) (interface{}, error) {
		order = append(order, "handler")
		return "profile-ok", nil
	})

	ctx := NewContext("GET", "/api/v1/profile")
	res, err := r.Dispatch(ctx)
	if err != nil || res != "profile-ok" {
		t.Fatalf("group dispatch failed: res=%v, err=%v", res, err)
	}

	if len(order) != 3 || order[0] != "global" || order[1] != "group" || order[2] != "handler" {
		t.Errorf("unexpected middleware execution order: %v", order)
	}

	// Verify route introspection
	routes := r.Routes()
	if len(routes) != 1 || routes[0].Pattern != "/api/v1/profile" {
		t.Errorf("expected 1 route /api/v1/profile, got %+v", routes)
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
