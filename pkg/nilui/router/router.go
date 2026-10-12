// Package router implements U10: a declarative Nilang-side router with
// stack/tabs/sheet patterns, platform back-button handling, dialog/sheet
// routes, deep links, and state restoration.
package router

import (
	"fmt"
	"strings"
)

// Pattern is stack/tabs/sheet navigation style.
type Pattern string

const (
	// PatternStack pushes screens.
	PatternStack Pattern = "stack"
	// PatternTabs switches tabs.
	PatternTabs Pattern = "tabs"
	// PatternSheet presents a sheet.
	PatternSheet Pattern = "sheet"
)

// Route declares one destination.
type Route struct {
	Path    string
	Pattern Pattern
	State   map[string]string
}

// Router holds the navigation state.
type Router struct {
	routes   map[string]Route
	stack    []string
	tabs     []string
	active   string
	sheet    string
	restored map[string]map[string]string
}

// New creates a router.
func New() *Router {
	return &Router{routes: map[string]Route{}, restored: map[string]map[string]string{}}
}

// AddRoute registers a destination.
func (r *Router) AddRoute(path string, p Pattern) *Router {
	r.routes[path] = Route{Path: path, Pattern: p, State: map[string]string{}}
	if p == PatternTabs {
		r.tabs = append(r.tabs, path)
	}
	return r
}

// Navigate goes to a path (U10.1/U10.2).
func (r *Router) Navigate(path string) error {
	rt, ok := r.routes[path]
	if !ok {
		return fmt.Errorf("UI10: unknown route %q", path)
	}
	switch rt.Pattern {
	case PatternTabs:
		r.active = path
	case PatternSheet:
		r.sheet = path
	default:
		r.stack = append(r.stack, path)
		r.active = path
	}
	return nil
}

// Back handles the platform back button (U10.3): sheet first, then stack.
func (r *Router) Back() bool {
	if r.sheet != "" {
		r.sheet = ""
		return true
	}
	if len(r.stack) > 1 {
		r.stack = r.stack[:len(r.stack)-1]
		r.active = r.stack[len(r.stack)-1]
		return true
	}
	return false
}

// Dialog presents a dialog route (U10.4).
func (r *Router) Dialog(path string) error {
	if _, ok := r.routes[path]; !ok {
		r.routes[path] = Route{Path: path, Pattern: PatternSheet}
	}
	r.sheet = path
	return nil
}

// Active returns the current path.
func (r *Router) Active() string { return r.active }

// Sheet returns the presented sheet/dialog.
func (r *Router) Sheet() string { return r.sheet }

// Stack returns a copy of the stack.
func (r *Router) Stack() []string { return append([]string{}, r.stack...) }

// ParseDeepLink parses alap://app/<path>?k=v (U10.5).
func ParseDeepLink(link string) (string, map[string]string, error) {
	if !strings.HasPrefix(link, "alap://") {
		return "", nil, fmt.Errorf("UI10: bad deep link %q", link)
	}
	rest := strings.TrimPrefix(link, "alap://")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("UI10: bad deep link %q", link)
	}
	pathPart := "/" + parts[1]
	path := pathPart
	params := map[string]string{}
	if i := strings.Index(pathPart, "?"); i >= 0 {
		path = pathPart[:i]
		for _, kv := range strings.Split(pathPart[i+1:], "&") {
			k, v, _ := strings.Cut(kv, "=")
			params[k] = v
		}
	}
	return path, params, nil
}

// SaveState snapshots per-route state (U10.6).
func (r *Router) SaveState() map[string]map[string]string {
	out := map[string]map[string]string{}
	for p, rt := range r.routes {
		cp := map[string]string{}
		for k, v := range rt.State {
			cp[k] = v
		}
		out[p] = cp
	}
	return out
}

// RestoreState reloads a snapshot (U10.6).
func (r *Router) RestoreState(snap map[string]map[string]string) {
	for p, st := range snap {
		if rt, ok := r.routes[p]; ok {
			rt.State = st
			r.routes[p] = rt
		} else {
			r.restored[p] = st
		}
	}
}

// SetState sets route state.
func (r *Router) SetState(path, k, v string) {
	if rt, ok := r.routes[path]; ok {
		if rt.State == nil {
			rt.State = map[string]string{}
		}
		rt.State[k] = v
		r.routes[path] = rt
	}
}

// GetState reads route state (restored included).
func (r *Router) GetState(path, k string) string {
	if rt, ok := r.routes[path]; ok {
		return rt.State[k]
	}
	return r.restored[path][k]
}
