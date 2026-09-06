package routing

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// HandlerFunc is an Alap route handler
type HandlerFunc func(ctx *Context) (interface{}, error)

// Middleware wraps a handler function
type Middleware func(next HandlerFunc) HandlerFunc

// Context encapsulates request parameters and response data
type Context struct {
	Method      string                 `json:"method"`
	Path        string                 `json:"path"`
	Params      map[string]string      `json:"params"`
	Query       map[string]string      `json:"query"`
	Body        interface{}            `json:"body,omitempty"`
	Headers     map[string]string      `json:"headers,omitempty"`
	Cookies     map[string]string      `json:"cookies,omitempty"`
	Store       map[string]interface{} `json:"store,omitempty"`
	StatusCode  int                    `json:"status_code,omitempty"`
	IsAborted   bool                   `json:"is_aborted,omitempty"`
	AbortReason string                 `json:"abort_reason,omitempty"`
	TraceID     string                 `json:"trace_id,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	TenantID    string                 `json:"tenant_id,omitempty"`
}

// NewContext creates a new route context
func NewContext(method, path string) *Context {
	return &Context{
		Method:     strings.ToUpper(method),
		Path:       path,
		Params:     make(map[string]string),
		Query:      make(map[string]string),
		Headers:    make(map[string]string),
		Cookies:    make(map[string]string),
		Store:      make(map[string]interface{}),
		StatusCode: 200,
	}
}

// Param returns a route parameter by key
func (c *Context) Param(key string) string {
	return c.Params[key]
}

// ParamInt parses a parameter as int
func (c *Context) ParamInt(key string) (int, error) {
	val, ok := c.Params[key]
	if !ok {
		return 0, fmt.Errorf("param %s not found", key)
	}
	return strconv.Atoi(val)
}

// AbortWithStatus flags the context as aborted
func (c *Context) AbortWithStatus(code int, reason string) {
	c.IsAborted = true
	c.StatusCode = code
	c.AbortReason = reason
}

// RouteInfo holds metadata about a registered route
type RouteInfo struct {
	Method  string   `json:"method"`
	Pattern string   `json:"pattern"`
	Group   string   `json:"group,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

// Route defines an application route
type Route struct {
	Method      string
	Pattern     string
	parts       []string
	Handler     HandlerFunc
	Middlewares []Middleware
}

// RadixNode represents a node in the Radix Tree Trie
type RadixNode struct {
	part        string
	children    []*RadixNode
	isParam     bool
	paramName   string
	constraint  string
	isWildcard  bool
	handlers    map[string]HandlerFunc
	middlewares map[string][]Middleware
	pattern     string
}

func newRadixNode(part string) *RadixNode {
	return &RadixNode{
		part:        part,
		children:    make([]*RadixNode, 0),
		handlers:    make(map[string]HandlerFunc),
		middlewares: make(map[string][]Middleware),
	}
}

// UUID regex for constraint checking
var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Router dispatches requests using a Radix Tree Trie
type Router struct {
	root        *RadixNode
	routes      []*Route
	routeInfos  []RouteInfo
	middlewares []Middleware
	mu          sync.RWMutex
}

// NewRouter creates a new application router
func NewRouter() *Router {
	return &Router{
		root:        newRadixNode("/"),
		routes:      make([]*Route, 0),
		routeInfos:  make([]RouteInfo, 0),
		middlewares: make([]Middleware, 0),
	}
}

// Use attaches router-level middleware
func (r *Router) Use(mw Middleware) *Router {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.middlewares = append(r.middlewares, mw)
	return r
}

// Group creates a new sub-router with a common path prefix and middleware
func (r *Router) Group(prefix string, middlewares ...Middleware) *RouteGroup {
	return &RouteGroup{
		router:      r,
		prefix:      strings.TrimRight(prefix, "/"),
		middlewares: middlewares,
	}
}

// AddRoute registers a new route in the Radix Tree
func (r *Router) AddRoute(method, pattern string, handler HandlerFunc, middlewares ...Middleware) *Route {
	r.mu.Lock()
	defer r.mu.Unlock()

	method = strings.ToUpper(method)
	cleanPattern := "/" + strings.Trim(pattern, "/")
	parts := splitPath(cleanPattern)

	// Combine router middlewares + route middlewares
	combinedMW := make([]Middleware, 0, len(r.middlewares)+len(middlewares))
	combinedMW = append(combinedMW, r.middlewares...)
	combinedMW = append(combinedMW, middlewares...)

	// Insert into Radix Tree
	curr := r.root
	for _, part := range parts {
		var child *RadixNode
		isWildcard := strings.HasPrefix(part, "*")
		isParam := false
		paramName := ""
		constraint := ""

		if !isWildcard {
			if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
				isParam = true
				raw := part[1 : len(part)-1]
				if idx := strings.Index(raw, ":"); idx != -1 {
					paramName = raw[:idx]
					constraint = raw[idx+1:]
				} else {
					paramName = raw
				}
			} else if strings.HasPrefix(part, ":") {
				isParam = true
				paramName = part[1:]
			}
		} else {
			paramName = strings.TrimPrefix(part, "*")
		}

		// Look for existing child matching part
		for _, c := range curr.children {
			if c.isWildcard == isWildcard && c.isParam == isParam && (!isParam || c.paramName == paramName) && c.part == part {
				child = c
				break
			}
		}

		if child == nil {
			child = newRadixNode(part)
			child.isParam = isParam
			child.paramName = paramName
			child.constraint = constraint
			child.isWildcard = isWildcard
			curr.children = append(curr.children, child)
		}
		curr = child
	}

	curr.handlers[method] = handler
	curr.middlewares[method] = combinedMW
	curr.pattern = cleanPattern

	rt := &Route{
		Method:      method,
		Pattern:     cleanPattern,
		parts:       parts,
		Handler:     handler,
		Middlewares: combinedMW,
	}
	r.routes = append(r.routes, rt)
	r.routeInfos = append(r.routeInfos, RouteInfo{
		Method:  method,
		Pattern: cleanPattern,
	})

	return rt
}

func (r *Router) GET(pattern string, handler HandlerFunc, mw ...Middleware) *Route {
	return r.AddRoute("GET", pattern, handler, mw...)
}

func (r *Router) POST(pattern string, handler HandlerFunc, mw ...Middleware) *Route {
	return r.AddRoute("POST", pattern, handler, mw...)
}

func (r *Router) PUT(pattern string, handler HandlerFunc, mw ...Middleware) *Route {
	return r.AddRoute("PUT", pattern, handler, mw...)
}

func (r *Router) PATCH(pattern string, handler HandlerFunc, mw ...Middleware) *Route {
	return r.AddRoute("PATCH", pattern, handler, mw...)
}

func (r *Router) DELETE(pattern string, handler HandlerFunc, mw ...Middleware) *Route {
	return r.AddRoute("DELETE", pattern, handler, mw...)
}

func (r *Router) OPTIONS(pattern string, handler HandlerFunc, mw ...Middleware) *Route {
	return r.AddRoute("OPTIONS", pattern, handler, mw...)
}

func (r *Router) HEAD(pattern string, handler HandlerFunc, mw ...Middleware) *Route {
	return r.AddRoute("HEAD", pattern, handler, mw...)
}

// Routes returns a copy of all registered route info for introspection
func (r *Router) Routes() []RouteInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]RouteInfo, len(r.routeInfos))
	copy(res, r.routeInfos)
	return res
}

// Match finds a route matching method and path in the Radix Tree
func (r *Router) Match(method, path string) (*Route, map[string]string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	method = strings.ToUpper(method)
	cleanPath := "/" + strings.Trim(path, "/")
	parts := splitPath(cleanPath)
	params := make(map[string]string)

	matchedNode := r.search(r.root, parts, 0, params)
	if matchedNode == nil {
		return nil, nil, false
	}

	handler, ok := matchedNode.handlers[method]
	if !ok {
		handler, ok = matchedNode.handlers["*"]
		if !ok {
			// For OPTIONS preflight, if any handler exists on this node, allow dispatch so CORS middleware can respond
			if method == "OPTIONS" && len(matchedNode.handlers) > 0 {
				for _, anyMW := range matchedNode.middlewares {
					return &Route{
						Method:      method,
						Pattern:     matchedNode.pattern,
						Handler:     func(c *Context) (interface{}, error) { return map[string]string{"status": "ok"}, nil },
						Middlewares: anyMW,
					}, params, true
				}
			}
			return nil, nil, false
		}
	}

	rt := &Route{
		Method:      method,
		Pattern:     matchedNode.pattern,
		Handler:     handler,
		Middlewares: matchedNode.middlewares[method],
	}

	return rt, params, true
}

func (r *Router) search(node *RadixNode, parts []string, index int, params map[string]string) *RadixNode {
	if index == len(parts) {
		if len(node.handlers) > 0 {
			return node
		}
		return nil
	}

	part := parts[index]

	// 1. Check exact literal children first
	for _, child := range node.children {
		if !child.isParam && !child.isWildcard && child.part == part {
			if res := r.search(child, parts, index+1, params); res != nil {
				return res
			}
		}
	}

	// 2. Check parameterized children with constraints
	for _, child := range node.children {
		if child.isParam {
			if matchesConstraint(part, child.constraint) {
				params[child.paramName] = part
				if res := r.search(child, parts, index+1, params); res != nil {
					return res
				}
				delete(params, child.paramName)
			}
		}
	}

	// 3. Check wildcard children
	for _, child := range node.children {
		if child.isWildcard {
			remaining := strings.Join(parts[index:], "/")
			params[child.paramName] = remaining
			return child
		}
	}

	return nil
}

func matchesConstraint(val, constraint string) bool {
	if constraint == "" {
		return true
	}
	switch constraint {
	case "int":
		_, err := strconv.Atoi(val)
		return err == nil
	case "uuid":
		return uuidRegex.MatchString(val)
	case "string":
		return true
	default:
		return true
	}
}

// Dispatch executes the matched handler wrapped with middlewares
func (r *Router) Dispatch(ctx *Context) (interface{}, error) {
	route, params, ok := r.Match(ctx.Method, ctx.Path)
	if !ok {
		return nil, fmt.Errorf("404 Not Found: %s %s", ctx.Method, ctx.Path)
	}

	ctx.Params = params

	// Compose middleware onion
	handler := route.Handler
	for i := len(route.Middlewares) - 1; i >= 0; i-- {
		handler = route.Middlewares[i](handler)
	}

	return handler(ctx)
}

func splitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return []string{}
	}
	return strings.Split(trimmed, "/")
}

// ─── ROUTE GROUP ────────────────────────────────────────────────────────────

// RouteGroup allows grouping routes with common prefix and middleware
type RouteGroup struct {
	router      *Router
	prefix      string
	middlewares []Middleware
}

// Group creates a nested sub-group
func (rg *RouteGroup) Group(prefix string, middlewares ...Middleware) *RouteGroup {
	combined := make([]Middleware, 0, len(rg.middlewares)+len(middlewares))
	combined = append(combined, rg.middlewares...)
	combined = append(combined, middlewares...)

	return &RouteGroup{
		router:      rg.router,
		prefix:      rg.prefix + "/" + strings.Trim(prefix, "/"),
		middlewares: combined,
	}
}

// Use adds middleware to the route group
func (rg *RouteGroup) Use(mw Middleware) *RouteGroup {
	rg.middlewares = append(rg.middlewares, mw)
	return rg
}

func (rg *RouteGroup) GET(path string, handler HandlerFunc, mw ...Middleware) *Route {
	return rg.addRoute("GET", path, handler, mw...)
}

func (rg *RouteGroup) POST(path string, handler HandlerFunc, mw ...Middleware) *Route {
	return rg.addRoute("POST", path, handler, mw...)
}

func (rg *RouteGroup) PUT(path string, handler HandlerFunc, mw ...Middleware) *Route {
	return rg.addRoute("PUT", path, handler, mw...)
}

func (rg *RouteGroup) DELETE(path string, handler HandlerFunc, mw ...Middleware) *Route {
	return rg.addRoute("DELETE", path, handler, mw...)
}

func (rg *RouteGroup) addRoute(method, path string, handler HandlerFunc, mw ...Middleware) *Route {
	fullPath := rg.prefix + "/" + strings.TrimPrefix(path, "/")
	combined := make([]Middleware, 0, len(rg.middlewares)+len(mw))
	combined = append(combined, rg.middlewares...)
	combined = append(combined, mw...)

	rt := rg.router.AddRoute(method, fullPath, handler, combined...)
	return rt
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
