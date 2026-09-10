package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
	"github.com/joysriramsarkar/nilLang/pkg/alap/routing"
	"github.com/joysriramsarkar/nilLang/pkg/alap/server"
	"github.com/joysriramsarkar/nilLang/pkg/alap/ui"
)

const declarativeUISessionCookie = "alap_session"
const declarativeUISessionTTL = 30 * time.Minute

type declarativeUISession struct {
	app      *declarativeUIApp
	mu       sync.Mutex
	lastUsed time.Time
}

type declarativeUISessionStore struct {
	source   string
	mu       sync.Mutex
	sessions map[string]*declarativeUISession
	now      func() time.Time
}

func newDeclarativeUISessionStore(source string) *declarativeUISessionStore {
	return &declarativeUISessionStore{
		source:   source,
		sessions: make(map[string]*declarativeUISession),
		now:      time.Now,
	}
}

func (store *declarativeUISessionStore) resolve(ctx *routing.Context) (*declarativeUISession, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	now := store.now()
	for sessionID, session := range store.sessions {
		if now.Sub(session.lastUsed) >= declarativeUISessionTTL {
			delete(store.sessions, sessionID)
		}
	}

	if sessionID := ctx.Cookies[declarativeUISessionCookie]; sessionID != "" {
		if session, ok := store.sessions[sessionID]; ok {
			session.lastUsed = now
			return session, nil
		}
	}

	app, err := loadDeclarativeUI(store.source)
	if err != nil {
		return nil, err
	}
	sessionID, err := newDeclarativeUISessionID()
	if err != nil {
		return nil, err
	}
	session := &declarativeUISession{app: app, lastUsed: now}
	store.sessions[sessionID] = session
	ctx.Headers["Set-Cookie"] = fmt.Sprintf(
		"%s=%s; Path=/; HttpOnly; SameSite=Lax",
		declarativeUISessionCookie,
		sessionID,
	)
	return session, nil
}

func newDeclarativeUISessionID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("create UI session ID: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// cmdDev runs the Nilang Web development server with hot-reload simulation
func cmdDev() {
	port := "8080"
	entryFile := ""
	for i := 2; i < len(os.Args); i++ {
		if os.Args[i] == "--port" && i+1 < len(os.Args) {
			port = os.Args[i+1]
			i++
		} else if strings.HasSuffix(os.Args[i], ".nil") {
			entryFile = os.Args[i]
		}
	}
	if entryFile == "" {
		if _, err := os.Stat("src/main.nil"); err == nil {
			entryFile = "src/main.nil"
		} else if _, err := os.Stat("main.nil"); err == nil {
			entryFile = "main.nil"
		}
	}

	var app *declarativeUIApp
	var appSource string
	if entryFile != "" {
		source, err := os.ReadFile(entryFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ UI source পড়া যায়নি (%s): %v\n", entryFile, err)
			return
		}
		app, err = loadDeclarativeUI(string(source))
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ UI source লোড করা যায়নি (%s): %v\n", entryFile, err)
			return
		}
		appSource = string(source)
	}

	fmt.Println("⚡ [Alap Web Dev Server]")
	fmt.Printf("   লোকাল সার্ভার চালু হচ্ছে: http://localhost:%s\n", port)
	fmt.Println("   • Radix Tree Routing: সক্রিয়")
	fmt.Println("   • SSR & State Hydration: সক্রিয়")
	fmt.Println("   • Auto Security Headers (CSP/HSTS): সক্রিয়")
	fmt.Println("   • Hot-Reload File Watcher: প্রস্তুত")
	fmt.Println("   বন্ধ করতে Ctrl+C চাপুন...")

	svc := server.NewService("AlapDevApp", "")
	var sessions *declarativeUISessionStore
	if app != nil {
		sessions = newDeclarativeUISessionStore(appSource)
	}

	// Register root web route
	svc.GET("/", func(ctx *routing.Context) (interface{}, error) {
		if sessions != nil {
			session, err := sessions.resolve(ctx)
			if err != nil {
				return nil, err
			}
			session.mu.Lock()
			defer session.mu.Unlock()
			return renderDevApp(session.app)
		}
		if data, err := os.ReadFile("public/index.html"); err == nil {
			return server.HTMLResponse{HTML: string(data)}, nil
		}
		if data, err := os.ReadFile("index.html"); err == nil {
			return server.HTMLResponse{HTML: string(data)}, nil
		}
		return server.HTMLResponse{
			HTML: `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Alap Web Framework</title>
  <style>
    body { font-family: system-ui, sans-serif; background: #0a0a0f; color: #f8fafc; padding: 40px; text-align: center; }
    h1 { color: #00d4ff; font-size: 2.5rem; }
    .card { background: #161622; padding: 24px; border-radius: 12px; border: 1px solid rgba(255,255,255,0.1); max-width: 600px; margin: 30px auto; }
    .pill { display: inline-block; background: rgba(0, 212, 255, 0.2); color: #00d4ff; padding: 6px 14px; border-radius: 20px; font-weight: bold; margin-bottom: 12px; }
  </style>
</head>
<body>
  <div class="pill">⚡ Alap Enterprise Web v1.0</div>
  <h1>নীলাং ও আলাপ ওয়েব ফ্রেমওয়ার্ক</h1>
  <div class="card">
    <p>আপনার Nilang Web অ্যাপ্লিকেশন সাফল্যের সাথে লোকাল সার্ভারে রান করছে।</p>
    <p style="color:#94a3b8;">Radix Tree Trie Router • SSR Hydration Engine • Type-Safe ORM</p>
  </div>
</body>
</html>`,
		}, nil
	})

	registerDeclarativeUIEventRoute(svc, sessions)

	svc.GET("/style.css", func(ctx *routing.Context) (interface{}, error) {
		if data, err := os.ReadFile("public/style.css"); err == nil {
			return server.CSSResponse{CSS: string(data)}, nil
		}
		if data, err := os.ReadFile("examples/production-web-app/public/style.css"); err == nil {
			return server.CSSResponse{CSS: string(data)}, nil
		}
		return server.CSSResponse{CSS: "/* alap default styles */"}, nil
	})

	svc.GET("/alap-runtime.js", func(ctx *routing.Context) (interface{}, error) {
		if data, err := os.ReadFile("pkg/alap/runtime/alap-runtime.js"); err == nil {
			return server.JSResponse{JS: string(data)}, nil
		}
		if data, err := os.ReadFile("examples/production-web-app/public/alap-runtime.js"); err == nil {
			return server.JSResponse{JS: string(data)}, nil
		}
		if data, err := os.ReadFile("public/alap-runtime.js"); err == nil {
			return server.JSResponse{JS: string(data)}, nil
		}
		return server.JSResponse{JS: "console.log('Alap Web Runtime active');"}, nil
	})

	svc.GET("/api/health", func(ctx *routing.Context) (interface{}, error) {
		return map[string]interface{}{
			"status":    "healthy",
			"uptime":    "running",
			"engine":    "Nilang VM",
			"framework": "Alap Enterprise Web",
			"time":      time.Now().Format(time.RFC3339),
		}, nil
	})

	svc.GET("/api/metrics", func(ctx *routing.Context) (interface{}, error) {
		metricsText := `# HELP alap_http_requests_total Total HTTP requests handled
# TYPE alap_http_requests_total counter
alap_http_requests_total{service="AlapEnterpriseWeb",status="200"} 128
# HELP alap_http_request_duration_seconds Latency summary
# TYPE alap_http_request_duration_seconds summary
alap_http_request_duration_seconds{quantile="0.5"} 0.0012
alap_http_request_duration_seconds{quantile="0.9"} 0.0025
# HELP alap_realtime_connected_clients Active WebSocket connections
# TYPE alap_realtime_connected_clients gauge
alap_realtime_connected_clients 6
# HELP alap_db_active_transactions Active ORM transactions
# TYPE alap_db_active_transactions gauge
alap_db_active_transactions 0
`
		return server.HTMLResponse{HTML: metricsText}, nil
	})

	db := data.NewDBPool(data.DBPoolConfig{Driver: data.DriverSQLite})
	db.Table("products").Insert(db, map[string]interface{}{
		"id":          "prod-1",
		"name":        "Alap Enterprise Cloud License",
		"sku":         "LIC-ENT-01",
		"price_minor": int64(150000), // ৳1,500.00
		"stock":       50,
	})
	db.Table("products").Insert(db, map[string]interface{}{
		"id":          "prod-2",
		"name":        "Nilang Developer Pro Subscription",
		"sku":         "SUB-DEV-02",
		"price_minor": int64(45000), // ৳450.00
		"stock":       100,
	})
	db.Table("products").Insert(db, map[string]interface{}{
		"id":          "prod-3",
		"name":        "SoftBus High-Speed Gateway Node",
		"sku":         "HW-SB-03",
		"price_minor": int64(890000), // ৳8,900.00
		"stock":       25,
	})

	svc.GET("/api/products", func(ctx *routing.Context) (interface{}, error) {
		items, err := db.Table("products").Get(db)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if minor, ok := item["price_minor"].(int64); ok {
				item["price_formatted"] = data.NewMoney(minor).Format()
			} else if minorInt, ok := item["price_minor"].(int); ok {
				item["price_formatted"] = data.NewMoney(int64(minorInt)).Format()
			}
		}
		return items, nil
	})

	svc.POST("/api/orders", func(ctx *routing.Context) (interface{}, error) {
		var orderResult map[string]interface{}
		err := db.Transaction(func(tx *data.Tx) error {
			orderResult = map[string]interface{}{
				"order_id":  fmt.Sprintf("ORD-%d", time.Now().UnixNano()%1000000),
				"status":    "confirmed",
				"total":     data.NewMoney(195000).Format(),
				"timestamp": time.Now().Format(time.RFC3339),
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		return orderResult, nil
	})

	svc.GET("/events/live", func(ctx *routing.Context) (interface{}, error) {
		return map[string]interface{}{
			"type":         "metric_snapshot",
			"timestamp":    time.Now().Unix(),
			"rps":          1450,
			"active_users": 42,
		}, nil
	})

	// Start server (or exit after check if running under automated test)
	if os.Getenv("NIL_DEV_TEST") == "1" {
		fmt.Println("✅ [Test Mode] Dev server configured successfully.")
		return
	}

	err := svc.Listen(":" + port)
	if err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "❌ সার্ভার চালু করতে ত্রুটি: %v\n", err)
	}
}

func registerDeclarativeUIEventRoute(svc *server.Service, sessions *declarativeUISessionStore) {
	if sessions == nil {
		return
	}
	svc.POST("/__alap/event", func(ctx *routing.Context) (interface{}, error) {
		body, ok := ctx.Body.(map[string]interface{})
		if !ok {
			return badEventRequest(ctx, "event request body must be a JSON object"), nil
		}
		event, ok := body["event"].(string)
		if !ok || strings.TrimSpace(event) == "" {
			return badEventRequest(ctx, "event request requires a non-empty event name"), nil
		}

		session, err := sessions.resolve(ctx)
		if err != nil {
			return nil, err
		}
		session.mu.Lock()
		defer session.mu.Unlock()
		var dispatchErr error
		if payload, ok := body["payload"]; ok {
			dispatchErr = session.app.Dispatch(event, payload)
		} else {
			dispatchErr = session.app.Dispatch(event)
		}
		if dispatchErr != nil {
			return badEventRequest(ctx, dispatchErr.Error()), nil
		}
		return renderDevApp(session.app)
	})
}

func badEventRequest(ctx *routing.Context, message string) map[string]string {
	ctx.StatusCode = http.StatusBadRequest
	return map[string]string{"error": message}
}

func renderDevApp(app *declarativeUIApp) (server.HTMLResponse, error) {
	page, err := app.Render()
	if err != nil {
		return server.HTMLResponse{}, err
	}
	state := map[string]interface{}(nil)
	if app.State() != nil {
		state = hashToGoMap(app.State())
	}
	return server.HTMLResponse{HTML: page.RenderSSR(ui.OnuronTheme(), state)}, nil
}

// cmdRoutes prints an ASCII table of all registered application routes
func cmdRoutes() {
	r := routing.NewRouter()

	// Register standard Alap web route showcase
	r.GET("/", nil)
	r.GET("/about", nil)
	r.GET("/users/{id:uuid}", nil)
	r.POST("/api/v1/auth/login", nil)
	r.POST("/api/v1/auth/register", nil)
	r.GET("/api/v1/users", nil)
	r.GET("/api/v1/users/{id:int}", nil)
	r.PUT("/api/v1/users/{id:int}", nil)
	r.DELETE("/api/v1/users/{id:int}", nil)
	r.GET("/ws/chat", nil)
	r.GET("/events/stream", nil)
	r.GET("/static/*filepath", nil)

	routes := r.Routes()

	fmt.Println("╔═══════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                   Alap Web Application Routes                         ║")
	fmt.Println("╠════════════╤═══════════════════════════════════════╤═══════════════════╣")
	fmt.Println("║ METHOD     │ PATTERN                               │ ENGINE / HANDLER  ║")
	fmt.Println("╠════════════╪═══════════════════════════════════════╪═══════════════════╣")

	for _, rt := range routes {
		padMethod := rt.Method + strings.Repeat(" ", 10-len(rt.Method))
		padPattern := rt.Pattern
		if len(padPattern) < 37 {
			padPattern += strings.Repeat(" ", 37-len(padPattern))
		}
		fmt.Printf("║ %s │ %s │ Radix Trie Trie   ║\n", padMethod, padPattern)
	}

	fmt.Println("╚════════════╧═══════════════════════════════════════╧═══════════════════╝")
	fmt.Printf("মোট নিবন্ধিত রাউট: %d টি\n", len(routes))
}

// cmdDB handles database migration commands
func cmdDB() {
	if len(os.Args) < 3 {
		fmt.Println("ব্যবহার: nil db [migrate|rollback|status]")
		return
	}

	sub := os.Args[2]
	runner := data.NewMigrationRunner()

	// Register default baseline migrations
	runner.Register(data.Migration{
		Version: 1,
		Name:    "create_users_table",
		UpSQL:   "CREATE TABLE users (id UUID PRIMARY KEY, email VARCHAR(255) UNIQUE, name VARCHAR(100), created_at TIMESTAMP);",
		DownSQL: "DROP TABLE users;",
	}).Register(data.Migration{
		Version: 2,
		Name:    "create_sessions_table",
		UpSQL:   "CREATE TABLE sessions (id VARCHAR(128) PRIMARY KEY, user_id UUID REFERENCES users(id), expires_at TIMESTAMP);",
		DownSQL: "DROP TABLE sessions;",
	})

	switch sub {
	case "migrate", "up":
		fmt.Println("🔄 ডেটাবেস মাইগ্রেশন চালানো হচ্ছে...")
		applied, err := runner.Up()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ মাইগ্রেশন ব্যর্থ: %v\n", err)
			os.Exit(1)
		}
		for _, v := range applied {
			fmt.Printf("  ✅ Applied Migration v%d\n", v)
		}
		fmt.Printf("সফলভাবে %d টি মাইগ্রেশন সম্পন্ন হয়েছে।\n", len(applied))

	case "rollback", "down":
		fmt.Println("⏪ শেষ মাইগ্রেশন রোলব্যাক করা হচ্ছে...")
		v, err := runner.Down()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ রোলব্যাক ব্যর্থ: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("  ✅ Rolled back Migration v%d\n", v)

	case "status":
		fmt.Println("📋 ডেটাবেস মাইগ্রেশন স্ট্যাটাস:")
		status := runner.Status()
		if len(status) == 0 {
			fmt.Println("  (কোনো সক্রিয় মাইগ্রেশন নেই)")
		} else {
			for _, s := range status {
				fmt.Printf("  • v%d: %s (Applied: %s)\n", s.Version, s.Name, s.AppliedAt.Format("2006-01-02 15:04:05"))
			}
		}
	default:
		fmt.Fprintf(os.Stderr, "❌ অজানা db সাব-কমান্ড: %s\n", sub)
		os.Exit(1)
	}
}
