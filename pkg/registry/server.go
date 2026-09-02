package registry

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Server represents the registry HTTP server
type Server struct {
	config  *ServerConfig
	storage *Storage
	mux     *http.ServeMux
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	DataDir     string `json:"data_dir"`
	MaxUploadMB int    `json:"max_upload_mb"`
	EnableAuth  bool   `json:"enable_auth"`
	LogLevel    string `json:"log_level"`
}

// DefaultServerConfig returns default server configuration
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Host:        "0.0.0.0",
		Port:        8080,
		DataDir:     "./registry-data",
		MaxUploadMB: 100,
		EnableAuth:  false,
		LogLevel:    "info",
	}
}

// NewServer creates a new registry server
func NewServer(config *ServerConfig) (*Server, error) {
	storage, err := NewStorage(config.DataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	server := &Server{
		config:  config,
		storage: storage,
		mux:     http.NewServeMux(),
	}

	server.setupRoutes()
	return server, nil
}

func (s *Server) setupRoutes() {
	// Health check
	s.mux.HandleFunc("/health", s.handleHealth)

	// API v1 routes
	s.mux.HandleFunc("/api/v1/packages", s.handlePackages)
	s.mux.HandleFunc("/api/v1/packages/search", s.handleSearch)
	s.mux.HandleFunc("/api/v1/packages/publish", s.handlePublish)
	s.mux.HandleFunc("/api/v1/packages/stats", s.handleStats)
	s.mux.HandleFunc("/api/v1/packages/", s.handlePackageByID)

	// Download endpoint
	s.mux.HandleFunc("/api/v1/download/", s.handleDownload)

	// User management
	s.mux.HandleFunc("/api/v1/users/register", s.handleRegister)
	s.mux.HandleFunc("/api/v1/users/login", s.handleLogin)

	// Web UI (basic)
	s.mux.HandleFunc("/", s.handleWebUI)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	log.Printf("🚀 Nilang Registry Server starting on %s", addr)
	log.Printf("📦 Data directory: %s", s.config.DataDir)
	log.Printf("🔐 Authentication: %v", s.config.EnableAuth)

	server := &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return server.ListenAndServe()
}

// --- Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "healthy",
		"version": "0.1.0",
		"time":    time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handlePackages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		packages := s.storage.ListAll()
		response := &PackageListResponse{
			Packages: packages,
			Total:    len(packages),
			Page:     1,
			PerPage:  len(packages),
		}
		s.jsonResponse(w, http.StatusOK, response)
	default:
		s.errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		s.errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req PackageSearchRequest

	if r.Method == http.MethodGet {
		query := r.URL.Query()
		req.Query = query.Get("q")
		req.Author = query.Get("author")
		req.Keyword = query.Get("keyword")
		req.SortBy = query.Get("sort")
		req.Order = query.Get("order")

		if page, err := strconv.Atoi(query.Get("page")); err == nil {
			req.Page = page
		} else {
			req.Page = 1
		}
		if perPage, err := strconv.Atoi(query.Get("per_page")); err == nil {
			req.PerPage = perPage
		} else {
			req.PerPage = 20
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.errorResponse(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	results := s.storage.SearchPackages(&req)
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"packages": results,
		"total":    len(results),
		"query":    req.Query,
	})
}

func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse multipart form
	maxUpload := int64(s.config.MaxUploadMB) * 1024 * 1024
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "failed to parse form: "+err.Error())
		return
	}

	// Get metadata
	metadataStr := r.FormValue("metadata")
	var publishReq PublishRequest
	if err := json.Unmarshal([]byte(metadataStr), &publishReq); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "invalid metadata")
		return
	}

	// Get uploaded file
	file, _, err := r.FormFile("package")
	if err != nil {
		s.errorResponse(w, http.StatusBadRequest, "package file required")
		return
	}
	defer file.Close()

	// Save uploaded file
	uploadDir := filepath.Join(s.config.DataDir, "uploads")
	filename := fmt.Sprintf("%s-%s.nilax", publishReq.Name, publishReq.Version)
	filePath := filepath.Join(uploadDir, filename)

	dst, err := os.Create(filePath)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to save file")
		return
	}
	defer dst.Close()

	written, err := dst.ReadFrom(file)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to write file")
		return
	}

	// Create package info
	pkg := &PackageInfo{
		Name:         publishReq.Name,
		Version:      publishReq.Version,
		Description:  publishReq.Description,
		Author:       publishReq.Author,
		AuthorEmail:  publishReq.AuthorEmail,
		License:      publishReq.License,
		DownloadURL:  fmt.Sprintf("/api/v1/download/%s/%s", publishReq.Name, publishReq.Version),
		Checksum:     publishReq.Checksum,
		Size:         written,
		Targets:      publishReq.Targets,
		Dependencies: publishReq.Dependencies,
		Keywords:     publishReq.Keywords,
		PublishedAt:  time.Now(),
		UpdatedAt:    time.Now(),
		Signature:    publishReq.Signature,
	}

	if err := s.storage.AddPackage(pkg); err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "failed to register package")
		return
	}

	response := &PublishResponse{
		Success:     true,
		PackageID:   pkg.ID,
		DownloadURL: pkg.DownloadURL,
		Message:     fmt.Sprintf("Package %s v%s published successfully", pkg.Name, pkg.Version),
	}

	s.jsonResponse(w, http.StatusCreated, response)
}

func (s *Server) handlePackageByID(w http.ResponseWriter, r *http.Request) {
	// Extract package name from URL: /api/v1/packages/{name}
	path := r.URL.Path
	name := path[len("/api/v1/packages/"):]

	if name == "" {
		s.errorResponse(w, http.StatusBadRequest, "package name required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		pkg, err := s.storage.GetPackageByName(name)
		if err != nil {
			s.errorResponse(w, http.StatusNotFound, "package not found: "+name)
			return
		}
		s.jsonResponse(w, http.StatusOK, pkg)

	case http.MethodDelete:
		if err := s.storage.DeletePackage(name); err != nil {
			s.errorResponse(w, http.StatusNotFound, "package not found: "+name)
			return
		}
		s.jsonResponse(w, http.StatusOK, map[string]string{"message": "deleted"})

	default:
		s.errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	// Extract name and version from URL: /api/v1/download/{name}/{version}
	path := r.URL.Path[len("/api/v1/download/"):]
	parts := splitPath(path)

	if len(parts) < 2 {
		s.errorResponse(w, http.StatusBadRequest, "invalid download path")
		return
	}

	name := parts[0]
	version := parts[1]

	pkg, err := s.storage.GetPackageByName(name)
	if err != nil {
		s.errorResponse(w, http.StatusNotFound, "package not found")
		return
	}

	if pkg.Version != version {
		s.errorResponse(w, http.StatusNotFound, "version not found")
		return
	}

	// Increment download count
	s.storage.IncrementDownloads(pkg.ID)

	// Serve the file
	filename := fmt.Sprintf("%s-%s.nilax", name, version)
	filePath := filepath.Join(s.config.DataDir, "uploads", filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		s.errorResponse(w, http.StatusNotFound, "package file not found")
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	http.ServeFile(w, r, filePath)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := s.storage.GetStats()
	s.jsonResponse(w, http.StatusOK, stats)
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// TODO: Implement user registration
	s.jsonResponse(w, http.StatusCreated, map[string]string{
		"message": "user registration not yet implemented",
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// TODO: Implement user login
	s.jsonResponse(w, http.StatusOK, map[string]string{
		"message": "user login not yet implemented",
	})
}

func (s *Server) handleWebUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(webUITemplate))
}

// --- Helpers ---

func (s *Server) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) errorResponse(w http.ResponseWriter, status int, message string) {
	s.jsonResponse(w, status, &APIError{
		Code:    status,
		Message: message,
	})
}

func splitPath(path string) []string {
	var parts []string
	current := ""
	for _, ch := range path {
		if ch == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

const webUITemplate = `<!DOCTYPE html>
<html lang="bn">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Nilang Package Registry</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: 'Segoe UI', system-ui, sans-serif; background: #0a0a0f; color: #e0e0e0; }
        .container { max-width: 1200px; margin: 0 auto; padding: 2rem; }
        header { text-align: center; padding: 3rem 0; border-bottom: 1px solid #1a1a2e; }
        h1 { font-size: 2.5rem; color: #00d4ff; margin-bottom: 0.5rem; }
        .subtitle { color: #888; font-size: 1.1rem; }
        .search-box { margin: 2rem 0; text-align: center; }
        .search-box input { width: 60%; padding: 12px 20px; font-size: 1rem; border: 2px solid #1a1a2e; border-radius: 8px; background: #111; color: #fff; }
        .search-box input:focus { outline: none; border-color: #00d4ff; }
        .stats { display: flex; gap: 2rem; justify-content: center; margin: 2rem 0; }
        .stat-card { background: #111; padding: 1.5rem 2rem; border-radius: 12px; text-align: center; border: 1px solid #1a1a2e; }
        .stat-card .number { font-size: 2rem; color: #00d4ff; font-weight: bold; }
        .stat-card .label { color: #888; margin-top: 0.5rem; }
        .packages { margin-top: 2rem; }
        .package-card { background: #111; padding: 1.5rem; border-radius: 12px; margin-bottom: 1rem; border: 1px solid #1a1a2e; }
        .package-card:hover { border-color: #00d4ff; }
        .package-name { font-size: 1.2rem; color: #00d4ff; }
        .package-version { color: #888; margin-left: 0.5rem; }
        .package-desc { color: #aaa; margin-top: 0.5rem; }
        .package-meta { color: #666; margin-top: 0.5rem; font-size: 0.9rem; }
        footer { text-align: center; padding: 2rem; color: #666; border-top: 1px solid #1a1a2e; margin-top: 3rem; }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>📦 Nilang Registry</h1>
            <p class="subtitle">নীলাং প্যাকেজ রেজিস্ট্রি • Alap Framework • Onuron OS</p>
        </header>

        <div class="search-box">
            <input type="text" id="search" placeholder="প্যাকেজ খুঁজুন... (Enter চাপুন)" onkeypress="handleSearch(event)">
        </div>

        <div class="stats" id="stats">
            <div class="stat-card">
                <div class="number" id="total-packages">-</div>
                <div class="label">মোট প্যাকেজ</div>
            </div>
            <div class="stat-card">
                <div class="number" id="total-downloads">-</div>
                <div class="label">মোট ডাউনলোড</div>
            </div>
            <div class="stat-card">
                <div class="number" id="total-authors">-</div>
                <div class="label">মোট লেখক</div>
            </div>
        </div>

        <div class="packages" id="packages">
            <p style="text-align: center; color: #666;">লোড হচ্ছে...</p>
        </div>

        <footer>
            <p>Nilang Registry v0.1.0 • Powered by Onuron OS</p>
            <p>© 2026 Joyshriram Sarkar</p>
        </footer>
    </div>

    <script>
        async function loadStats() {
            try {
                const res = await fetch('/api/v1/packages/stats');
                const stats = await res.json();
                document.getElementById('total-packages').textContent = stats.total_packages;
                document.getElementById('total-downloads').textContent = stats.total_downloads;
                document.getElementById('total-authors').textContent = stats.total_authors;
            } catch (e) { console.error(e); }
        }

        async function loadPackages(query = '') {
            try {
                let url = '/api/v1/packages';
                if (query) url = '/api/v1/packages/search?q=' + encodeURIComponent(query);

                const res = await fetch(url);
                const data = await res.json();
                const packages = data.packages || [];

                const container = document.getElementById('packages');
                if (packages.length === 0) {
                    container.innerHTML = '<p style="text-align: center; color: #666;">কোনো প্যাকেজ পাওয়া যায়নি</p>';
                    return;
                }

                container.innerHTML = packages.map(function(pkg) {
                    return '<div class="package-card">' +
                        '<span class="package-name">' + pkg.name + '</span>' +
                        '<span class="package-version">v' + pkg.version + '</span>' +
                        '<div class="package-desc">' + (pkg.description || 'No description') + '</div>' +
                        '<div class="package-meta">' +
                            'লেখক: ' + pkg.author + ' | ডাউনলোড: ' + pkg.downloads + ' | টার্গেট: ' + (pkg.targets || []).join(', ') +
                        '</div>' +
                    '</div>';
                }).join('');
            } catch (e) { console.error(e); }
        }

        function handleSearch(event) {
            if (event.key === 'Enter') {
                const query = event.target.value;
                loadPackages(query);
            }
        }

        loadStats();
        loadPackages();
    </script>
</body>
</html>`
