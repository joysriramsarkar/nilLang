package routing

import (
	"fmt"
	"strings"
	"sync"
)

// HandlerFunc is an Alap route handler
type HandlerFunc func(ctx *Context) (interface{}, error)

// Context encapsulates request parameters and response data
type Context struct {
	Method  string                 `json:"method"`
	Path    string                 `json:"path"`
	Params  map[string]string      `json:"params"`
	Query   map[string]string      `json:"query"`
	Body    interface{}            `json:"body,omitempty"`
	Headers map[string]string      `json:"headers,omitempty"`
	Store   map[string]interface{} `json:"store,omitempty"`
}

// NewContext creates a new route context
func NewContext(method, path string) *Context {
	return &Context{
		Method:  strings.ToUpper(method),
		Path:    path,
		Params:  make(map[string]string),
		Query:   make(map[string]string),
		Headers: make(map[string]string),
		Store:   make(map[string]interface{}),
	}
}

// Route defines an application route
type Route struct {
	Method  string
	Pattern string
	parts   []string
	Handler HandlerFunc
}

// Router dispatches requests to matching routes
type Router struct {
	routes []*Route
	mu     sync.RWMutex
}

// NewRouter creates a new application router
func NewRouter() *Router {
	return &Router{
		routes: []*Route{},
	}
}

// AddRoute registers a new route
func (r *Router) AddRoute(method, pattern string, handler HandlerFunc) *Route {
	r.mu.Lock()
	defer r.mu.Unlock()

	parts := splitPath(pattern)
	rt := &Route{
		Method:  strings.ToUpper(method),
		Pattern: pattern,
		parts:   parts,
		Handler: handler,
	}
	r.routes = append(r.routes, rt)
	return rt
}

func (r *Router) GET(pattern string, handler HandlerFunc) *Route {
	return r.AddRoute("GET", pattern, handler)
}

func (r *Router) POST(pattern string, handler HandlerFunc) *Route {
	return r.AddRoute("POST", pattern, handler)
}

func (r *Router) PUT(pattern string, handler HandlerFunc) *Route {
	return r.AddRoute("PUT", pattern, handler)
}

func (r *Router) DELETE(pattern string, handler HandlerFunc) *Route {
	return r.AddRoute("DELETE", pattern, handler)
}

// Match finds a route matching method and path
func (r *Router) Match(method, path string) (*Route, map[string]string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	upperMethod := strings.ToUpper(method)
	pathParts := splitPath(path)

	for _, route := range r.routes {
		if route.Method != "*" && route.Method != upperMethod {
			continue
		}

		if len(route.parts) != len(pathParts) {
			continue
		}

		params := make(map[string]string)
		match := true

		for i := 0; i < len(route.parts); i++ {
			rPart := route.parts[i]
			pPart := pathParts[i]

			if strings.HasPrefix(rPart, "{") && strings.HasSuffix(rPart, "}") {
				paramName := rPart[1 : len(rPart)-1]
				params[paramName] = pPart
			} else if strings.HasPrefix(rPart, ":") {
				paramName := rPart[1:]
				params[paramName] = pPart
			} else if rPart != pPart {
				match = false
				break
			}
		}

		if match {
			return route, params, true
		}
	}

	return nil, nil, false
}

// Dispatch executes the matched handler
func (r *Router) Dispatch(ctx *Context) (interface{}, error) {
	route, params, ok := r.Match(ctx.Method, ctx.Path)
	if !ok {
		return nil, fmt.Errorf("404 Not Found: %s %s", ctx.Method, ctx.Path)
	}

	ctx.Params = params
	return route.Handler(ctx)
}

func splitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return []string{}
	}
	return strings.Split(trimmed, "/")
}

// ─── NAVIGATION STACK ───────────────────────────────────────────────────────

// NavigationStack manages screen transitions (Mobile & Desktop)
type NavigationStack struct {
	history []string
	index   int
	mu      sync.Mutex
}

func NewNavigationStack(initialRoute string) *NavigationStack {
	return &NavigationStack{
		history: []string{initialRoute},
		index:   0,
	}
}

func (ns *NavigationStack) Push(route string) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.history = append(ns.history[:ns.index+1], route)
	ns.index++
}

func (ns *NavigationStack) Pop() (string, bool) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	if ns.index > 0 {
		ns.index--
		return ns.history[ns.index], true
	}
	return "", false
}

func (ns *NavigationStack) Current() string {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	if len(ns.history) == 0 {
		return ""
	}
	return ns.history[ns.index]
}
