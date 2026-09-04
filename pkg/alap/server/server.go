package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/routing"
)

// Middleware is an interceptor function wrapping route execution
type Middleware func(next routing.HandlerFunc) routing.HandlerFunc

// Endpoint represents a service endpoint
type Endpoint struct {
	Method       string              `json:"method"`
	Path         string              `json:"path"`
	Handler      routing.HandlerFunc `json:"-"`
	RequiresAuth bool                `json:"requires_auth"`
	RateLimit    int                 `json:"rate_limit"` // requests per minute
}

// Service represents an Alap Server Service container (Section 19)
type Service struct {
	Name        string       `json:"name"`
	BasePath    string       `json:"base_path"`
	Endpoints   []*Endpoint  `json:"endpoints"`
	Middlewares []Middleware `json:"-"`
	router      *routing.Router
	cache       map[string]cacheEntry
	cacheMu     sync.RWMutex
}

type cacheEntry struct {
	data      interface{}
	expiresAt time.Time
}

// NewService creates a new server service
func NewService(name, basePath string) *Service {
	s := &Service{
		Name:        name,
		BasePath:    strings.TrimSuffix(basePath, "/"),
		Endpoints:   []*Endpoint{},
		Middlewares: []Middleware{},
		router:      routing.NewRouter(),
		cache:       make(map[string]cacheEntry),
	}
	// Default middlewares: Logger and Metrics
	s.Use(LoggingMiddleware)
	return s
}

// Use attaches a middleware to the service
func (s *Service) Use(mw Middleware) *Service {
	s.Middlewares = append(s.Middlewares, mw)
	return s
}

// AddEndpoint registers an endpoint under the service
func (s *Service) AddEndpoint(method, path string, handler routing.HandlerFunc) *Endpoint {
	fullPath := s.BasePath + "/" + strings.TrimPrefix(path, "/")

	ep := &Endpoint{
		Method:  strings.ToUpper(method),
		Path:    fullPath,
		Handler: handler,
	}

	// Chain middlewares
	wrappedHandler := handler
	for i := len(s.Middlewares) - 1; i >= 0; i-- {
		wrappedHandler = s.Middlewares[i](wrappedHandler)
	}

	s.router.AddRoute(method, fullPath, wrappedHandler)
	s.Endpoints = append(s.Endpoints, ep)
	return ep
}

// HandleRequest processes an incoming HTTP-style request
func (s *Service) HandleRequest(method, path string, headers map[string]string, body interface{}) (string, int, error) {
	ctx := routing.NewContext(method, path)
	ctx.Headers = headers
	ctx.Body = body

	res, err := s.router.Dispatch(ctx)
	if err != nil {
		if strings.HasPrefix(err.Error(), "404") {
			return `{"error": "not found"}`, 404, nil
		}
		if strings.HasPrefix(err.Error(), "401") {
			return `{"error": "unauthorized"}`, 401, nil
		}
		return fmt.Sprintf(`{"error": %q}`, err.Error()), 500, err
	}

	jsonBytes, err := json.Marshal(res)
	if err != nil {
		return `{"error": "serialization failed"}`, 500, err
	}

	return string(jsonBytes), 200, nil
}

// CacheGet retrieves a cached response
func (s *Service) CacheGet(key string) (interface{}, bool) {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	entry, ok := s.cache[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

// CacheSet stores a cached response
func (s *Service) CacheSet(key string, data interface{}, ttl time.Duration) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cache[key] = cacheEntry{
		data:      data,
		expiresAt: time.Now().Add(ttl),
	}
}

// ─── STANDARD MIDDLEWARES ───────────────────────────────────────────────────

// LoggingMiddleware logs request execution
func LoggingMiddleware(next routing.HandlerFunc) routing.HandlerFunc {
	return func(ctx *routing.Context) (interface{}, error) {
		start := time.Now()
		res, err := next(ctx)
		duration := time.Since(start)
		_ = duration // For metrics/logging
		return res, err
	}
}

// AuthMiddleware enforces bearer token verification
func AuthMiddleware(validToken string) Middleware {
	return func(next routing.HandlerFunc) routing.HandlerFunc {
		return func(ctx *routing.Context) (interface{}, error) {
			token := ctx.Headers["Authorization"]
			if token != "Bearer "+validToken {
				return nil, fmt.Errorf("401: unauthorized")
			}
			return next(ctx)
		}
	}
}
