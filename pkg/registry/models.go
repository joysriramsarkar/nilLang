package registry

import (
	"time"
)

// PackageInfo represents a package in the registry
type PackageInfo struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description,omitempty"`
	Author      string            `json:"author"`
	AuthorEmail string            `json:"author_email,omitempty"`
	License     string            `json:"license,omitempty"`
	Homepage    string            `json:"homepage,omitempty"`
	Repository  string            `json:"repository,omitempty"`
	DownloadURL string            `json:"download_url"`
	Checksum    string            `json:"checksum"`
	Size        int64             `json:"size_bytes"`
	Targets     []string          `json:"targets"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	Keywords    []string          `json:"keywords,omitempty"`
	PublishedAt time.Time         `json:"published_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Downloads   int64             `json:"downloads"`
	Deprecated  bool              `json:"deprecated"`
	Signature   string            `json:"signature,omitempty"`
}

// PackageListResponse represents the response for listing packages
type PackageListResponse struct {
	Packages   []*PackageInfo `json:"packages"`
	Total      int            `json:"total"`
	Page       int            `json:"page"`
	PerPage    int            `json:"per_page"`
	TotalPages int            `json:"total_pages"`
}

// PackageSearchRequest represents search parameters
type PackageSearchRequest struct {
	Query    string   `json:"query"`
	Targets  []string `json:"targets,omitempty"`
	Author   string   `json:"author,omitempty"`
	Keyword  string   `json:"keyword,omitempty"`
	Page     int      `json:"page"`
	PerPage  int      `json:"per_page"`
	SortBy   string   `json:"sort_by"`
	Order    string   `json:"order"`
}

// PublishRequest represents a package publish request
type PublishRequest struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Author      string            `json:"author"`
	AuthorEmail string            `json:"author_email"`
	License     string            `json:"license"`
	Targets     []string          `json:"targets"`
	Dependencies map[string]string `json:"dependencies"`
	Keywords    []string          `json:"keywords"`
	Checksum    string            `json:"checksum"`
	Signature   string            `json:"signature,omitempty"`
}

// PublishResponse represents the response after publishing
type PublishResponse struct {
	Success     bool   `json:"success"`
	PackageID   string `json:"package_id"`
	DownloadURL string `json:"download_url"`
	Message     string `json:"message"`
}

// APIError represents an API error response
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// ServerStats represents server statistics
type ServerStats struct {
	TotalPackages  int64     `json:"total_packages"`
	TotalDownloads int64     `json:"total_downloads"`
	TotalAuthors   int64     `json:"total_authors"`
	Uptime         string    `json:"uptime"`
	LastUpdated    time.Time `json:"last_updated"`
}

// User represents a registry user
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	APIKey    string    `json:"api_key"`
	CreatedAt time.Time `json:"created_at"`
	IsAdmin   bool      `json:"is_admin"`
}

// VersionInfo represents version information
type VersionInfo struct {
	Version     string    `json:"version"`
	Checksum    string    `json:"checksum"`
	Size        int64     `json:"size_bytes"`
	PublishedAt time.Time `json:"published_at"`
	DownloadURL string    `json:"download_url"`
	Deprecated  bool      `json:"deprecated"`
}

// PackageVersions represents all versions of a package
type PackageVersions struct {
	Name     string         `json:"name"`
	Latest   string         `json:"latest"`
	Versions []*VersionInfo `json:"versions"`
}