package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/routing"
)

// Middleware is an interceptor function wrapping route execution
type Middleware = routing.Middleware

// Endpoint represents a service endpoint
type Endpoint struct {
	Method       string              `json:"method"`
	Path         string              `json:"path"`
	Handler      routing.HandlerFunc `json:"-"`
	RequiresAuth bool                `json:"requires_auth"`
	RateLimit    int                 `json:"rate_limit"` // requests per minute
}

// ResponseBody defines custom response types (HTML, JSON, raw)
type ResponseBody interface {
	ContentType() string
	Bytes() ([]byte, error)
}

// HTMLResponse renders raw HTML with text/html content type
type HTMLResponse struct {
	HTML string
}

func (h HTMLResponse) ContentType() string {
	return "text/html; charset=utf-8"
}

func (h HTMLResponse) Bytes() ([]byte, error) {
	return []byte(h.HTML), nil
}

// JSONResponse explicitly defines a JSON response
type JSONResponse struct {
	Data interface{}
}

func (j JSONResponse) ContentType() string {
	return "application/json; charset=utf-8"
}

func (j JSONResponse) Bytes() ([]byte, error) {
	return json.Marshal(j.Data)
}

// TextResponse renders plain text
type TextResponse struct {
	Text string
}

func (t TextResponse) ContentType() string {
	return "text/plain; charset=utf-8"
}

func (t TextResponse) Bytes() ([]byte, error) {
	return []byte(t.Text), nil
}

// CSSResponse renders stylesheet with text/css content type
type CSSResponse struct {
	CSS string
}

func (c CSSResponse) ContentType() string {
	return "text/css; charset=utf-8"
}

func (c CSSResponse) Bytes() ([]byte, error) {
	return []byte(c.CSS), nil
}

// JSResponse renders javascript with application/javascript content type
type JSResponse struct {
	JS string
}

func (j JSResponse) ContentType() string {
	return "application/javascript; charset=utf-8"
}

func (j JSResponse) Bytes() ([]byte, error) {
	return []byte(j.JS), nil
}


// Service represents an Alap Server Service container
type Service struct {
	Name        string              `json:"name"`
	BasePath    string              `json:"base_path"`
	Endpoints   []*Endpoint         `json:"endpoints"`
	Middlewares []Middleware        `json:"-"`
	Router      *routing.Router     `json:"-"`
	cache       map[string]cacheEntry
	cacheMu     sync.RWMutex
	httpServer  *http.Server
	serverMu    sync.Mutex
}

type cacheEntry struct {
	data      interface{}
	expiresAt time.Time
}

// NewService creates a new server service with enterprise defaults
func NewService(name, basePath string) *Service {
	s := &Service{
		Name:        name,
		BasePath:    strings.TrimSuffix(basePath, "/"),
		Endpoints:   make([]*Endpoint, 0),
		Middlewares: make([]Middleware, 0),
		Router:      routing.NewRouter(),
		cache:       make(map[string]cacheEntry),
	}

	// Default enterprise middleware chain: Recovery -> RequestID -> SecurityHeaders -> Logger
	s.Use(RecoveryMiddleware)
	s.Use(RequestIDMiddleware)
	s.Use(SecurityHeadersMiddleware)
	s.Use(LoggingMiddleware)

	return s
}

// Use attaches a middleware to the service
func (s *Service) Use(mw Middleware) *Service {
	s.Middlewares = append(s.Middlewares, mw)
	return s
}

// Group creates a nested route group under the service base path
func (s *Service) Group(prefix string, middlewares ...Middleware) *routing.RouteGroup {
	fullPrefix := s.BasePath + "/" + strings.Trim(prefix, "/")
	return s.Router.Group(fullPrefix, middlewares...)
}

// AddEndpoint registers an endpoint under the service
func (s *Service) AddEndpoint(method, path string, handler routing.HandlerFunc) *Endpoint {
	fullPath := s.BasePath + "/" + strings.TrimPrefix(path, "/")

	ep := &Endpoint{
		Method:  strings.ToUpper(method),
		Path:    fullPath,
		Handler: handler,
	}

	// Combine service-level middlewares with route handler
	s.Router.AddRoute(method, fullPath, handler, s.Middlewares...)
	s.Endpoints = append(s.Endpoints, ep)
	return ep
}

// GET shorthand
func (s *Service) GET(path string, handler routing.HandlerFunc) *Endpoint {
	return s.AddEndpoint("GET", path, handler)
}

// POST shorthand
func (s *Service) POST(path string, handler routing.HandlerFunc) *Endpoint {
	return s.AddEndpoint("POST", path, handler)
}

// PUT shorthand
func (s *Service) PUT(path string, handler routing.HandlerFunc) *Endpoint {
	return s.AddEndpoint("PUT", path, handler)
}

// DELETE shorthand
func (s *Service) DELETE(path string, handler routing.HandlerFunc) *Endpoint {
	return s.AddEndpoint("DELETE", path, handler)
}

// HandleRequest processes an incoming HTTP-style in-memory request (backwards compatible)
func (s *Service) HandleRequest(method, path string, headers map[string]string, body interface{}) (string, int, error) {
	ctx := routing.NewContext(method, path)
	if headers != nil {
		ctx.Headers = headers
	}
	ctx.Body = body

	res, err := s.Router.Dispatch(ctx)
	if err != nil {
		if strings.HasPrefix(err.Error(), "404") {
			return `{"error": "not found"}`, 404, nil
		}
		if strings.HasPrefix(err.Error(), "401") {
			return `{"error": "unauthorized"}`, 401, nil
		}
		if strings.HasPrefix(err.Error(), "403") {
			return `{"error": "forbidden"}`, 403, nil
		}
		if strings.HasPrefix(err.Error(), "429") {
			return `{"error": "too many requests"}`, 429, nil
		}
		return fmt.Sprintf(`{"error": %q}`, err.Error()), 500, err
	}

	if ctx.IsAborted {
		return fmt.Sprintf(`{"error": %q}`, ctx.AbortReason), ctx.StatusCode, nil
	}

	// Handle response types
	if respBody, ok := res.(ResponseBody); ok {
		b, err := respBody.Bytes()
		if err != nil {
			return `{"error": "serialization failed"}`, 500, err
		}
		code := ctx.StatusCode
		if code == 0 {
			code = 200
		}
		return string(b), code, nil
	}

	jsonBytes, err := json.Marshal(res)
	if err != nil {
		return `{"error": "serialization failed"}`, 500, err
	}

	code := ctx.StatusCode
	if code == 0 {
		code = 200
	}
	return string(jsonBytes), code, nil
}

// ServeHTTP implements net/http.Handler for the live web server
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := routing.NewContext(r.Method, r.URL.Path)

	// Copy headers
	for k, vals := range r.Header {
		if len(vals) > 0 {
			ctx.Headers[k] = vals[0]
		}
	}

	// Copy query params
	for k, vals := range r.URL.Query() {
		if len(vals) > 0 {
			ctx.Query[k] = vals[0]
		}
	}

	// Copy cookies
	for _, cookie := range r.Cookies() {
		ctx.Cookies[cookie.Name] = cookie.Value
	}

	// Parse JSON body if present
	if r.Body != nil && r.ContentLength > 0 {
		var bodyData interface{}
		bodyBytes, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		if len(bodyBytes) > 0 {
			if err := json.Unmarshal(bodyBytes, &bodyData); err == nil {
				ctx.Body = bodyData
			} else {
				ctx.Body = string(bodyBytes)
			}
		}
	}

	res, err := s.Router.Dispatch(ctx)

	// Write response headers from context
	for k, v := range ctx.Headers {
		if strings.HasPrefix(k, "X-") ||
			strings.HasPrefix(k, "Access-Control-") ||
			k == "Content-Security-Policy" ||
			k == "Strict-Transport-Security" {
			w.Header().Set(k, v)
		}
	}

	if err != nil {
		statusCode := 500
		if strings.HasPrefix(err.Error(), "404") {
			statusCode = 404
		} else if strings.HasPrefix(err.Error(), "401") {
			statusCode = 401
		} else if strings.HasPrefix(err.Error(), "403") {
			statusCode = 403
		} else if strings.HasPrefix(err.Error(), "429") {
			statusCode = 429
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	statusCode := ctx.StatusCode
	if statusCode == 0 {
		statusCode = 200
	}

	if respBody, ok := res.(ResponseBody); ok {
		w.Header().Set("Content-Type", respBody.ContentType())
		w.WriteHeader(statusCode)
		b, _ := respBody.Bytes()
		_, _ = w.Write(b)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(res)
}

// Listen starts the live HTTP server on specified address
func (s *Service) Listen(addr string) error {
	s.serverMu.Lock()
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	s.serverMu.Unlock()

	return s.httpServer.ListenAndServe()
}

// Shutdown stops the live HTTP server gracefully
func (s *Service) Shutdown(ctx context.Context) error {
	s.serverMu.Lock()
	defer s.serverMu.Unlock()
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
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

// ─── STANDARD ENTERPRISE MIDDLEWARES ────────────────────────────────────────

// RecoveryMiddleware catches panics, formats error response, prevents crashes
func RecoveryMiddleware(next routing.HandlerFunc) routing.HandlerFunc {
	return func(ctx *routing.Context) (res interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				_ = stack
				err = fmt.Errorf("500: internal server panic: %v", r)
				ctx.AbortWithStatus(500, fmt.Sprintf("panic: %v", r))
			}
		}()
		return next(ctx)
	}
}

// RequestIDMiddleware injects X-Request-ID
func RequestIDMiddleware(next routing.HandlerFunc) routing.HandlerFunc {
	return func(ctx *routing.Context) (interface{}, error) {
		reqID := ctx.Headers["X-Request-ID"]
		if reqID == "" {
			reqID = generateID()
			ctx.Headers["X-Request-ID"] = reqID
		}
		return next(ctx)
	}
}

// SecurityHeadersMiddleware injects enterprise CSP, HSTS, and X-Frame headers
func SecurityHeadersMiddleware(next routing.HandlerFunc) routing.HandlerFunc {
	return func(ctx *routing.Context) (interface{}, error) {
		ctx.Headers["X-Content-Type-Options"] = "nosniff"
		ctx.Headers["X-Frame-Options"] = "SAMEORIGIN"
		ctx.Headers["Referrer-Policy"] = "strict-origin-when-cross-origin"
		ctx.Headers["Content-Security-Policy"] = "default-src 'self'; img-src 'self' data: https:; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'"
		ctx.Headers["Strict-Transport-Security"] = "max-age=31536000; includeSubDomains"
		return next(ctx)
	}
}

// LoggingMiddleware logs request execution duration and trace info
func LoggingMiddleware(next routing.HandlerFunc) routing.HandlerFunc {
	return func(ctx *routing.Context) (interface{}, error) {
		start := time.Now()
		res, err := next(ctx)
		duration := time.Since(start)
		_ = duration
		return res, err
	}
}

// TracingMiddleware injects distributed trace ID
func TracingMiddleware(next routing.HandlerFunc) routing.HandlerFunc {
	return func(ctx *routing.Context) (interface{}, error) {
		traceID := ctx.Headers["X-Trace-ID"]
		if traceID == "" {
			traceID = generateID()
			ctx.Headers["X-Trace-ID"] = traceID
		}
		ctx.TraceID = traceID
		return next(ctx)
	}
}

// CORSConfig defines Cross-Origin Resource Sharing policy
type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	AllowCredentials bool
}

// CORSMiddleware applies CORS headers
func CORSMiddleware(cfg CORSConfig) Middleware {
	origins := strings.Join(cfg.AllowOrigins, ", ")
	if origins == "" {
		origins = "*"
	}
	methods := strings.Join(cfg.AllowMethods, ", ")
	if methods == "" {
		methods = "GET, POST, PUT, DELETE, OPTIONS, PATCH"
	}
	headers := strings.Join(cfg.AllowHeaders, ", ")
	if headers == "" {
		headers = "Origin, Content-Type, Accept, Authorization, X-Request-ID"
	}

	return func(next routing.HandlerFunc) routing.HandlerFunc {
		return func(ctx *routing.Context) (interface{}, error) {
			ctx.Headers["Access-Control-Allow-Origin"] = origins
			ctx.Headers["Access-Control-Allow-Methods"] = methods
			ctx.Headers["Access-Control-Allow-Headers"] = headers
			if cfg.AllowCredentials {
				ctx.Headers["Access-Control-Allow-Credentials"] = "true"
			}

			if ctx.Method == "OPTIONS" {
				return map[string]string{"status": "ok"}, nil
			}

			return next(ctx)
		}
	}
}

// RateLimiter creates a sliding window rate limiter
type RateLimiter struct {
	maxRequests int
	window      time.Duration
	hits        map[string][]time.Time
	mu          sync.Mutex
}

// NewRateLimiter creates a rate limiter with requests per window
func NewRateLimiter(maxRequests int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		maxRequests: maxRequests,
		window:      window,
		hits:        make(map[string][]time.Time),
	}
}

// Allow checks if client key has requests left in window
func (rl *RateLimiter) Allow(clientKey string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	validTimes := make([]time.Time, 0)
	for _, t := range rl.hits[clientKey] {
		if t.After(cutoff) {
			validTimes = append(validTimes, t)
		}
	}

	if len(validTimes) >= rl.maxRequests {
		rl.hits[clientKey] = validTimes
		return false
	}

	validTimes = append(validTimes, now)
	rl.hits[clientKey] = validTimes
	return true
}

// RateLimitMiddleware enforces rate limiting per client
func RateLimitMiddleware(requestsPerMinute int) Middleware {
	rl := NewRateLimiter(requestsPerMinute, time.Minute)
	return func(next routing.HandlerFunc) routing.HandlerFunc {
		return func(ctx *routing.Context) (interface{}, error) {
			clientKey := ctx.Headers["X-Forwarded-For"]
			if clientKey == "" {
				clientKey = ctx.Headers["Remote-Addr"]
			}
			if clientKey == "" {
				clientKey = "default"
			}

			if !rl.Allow(clientKey) {
				return nil, fmt.Errorf("429: rate limit exceeded (%d req/min)", requestsPerMinute)
			}
			return next(ctx)
		}
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

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
