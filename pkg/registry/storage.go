package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Storage handles package data persistence
type Storage struct {
	dataDir     string
	packagesDir string
	mu          sync.RWMutex
	packages    map[string]*PackageInfo
	versions    map[string][]*VersionInfo
	users       map[string]*User
}

// NewStorage creates a new storage instance
func NewStorage(dataDir string) (*Storage, error) {
	storage := &Storage{
		dataDir:     dataDir,
		packagesDir: filepath.Join(dataDir, "packages"),
		packages:    make(map[string]*PackageInfo),
		versions:    make(map[string][]*VersionInfo),
		users:       make(map[string]*User),
	}

	// Create directories
	dirs := []string{
		dataDir,
		storage.packagesDir,
		filepath.Join(dataDir, "uploads"),
		filepath.Join(dataDir, "users"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Load existing data
	if err := storage.load(); err != nil {
		// If no data exists, start fresh
		return storage, nil
	}

	return storage, nil
}

func (s *Storage) load() error {
	// Load packages index
	indexPath := filepath.Join(s.dataDir, "index.json")
	if data, err := os.ReadFile(indexPath); err == nil {
		var packages map[string]*PackageInfo
		if err := json.Unmarshal(data, &packages); err == nil {
			s.packages = packages
		}
	}

	// Load versions
	versionsPath := filepath.Join(s.dataDir, "versions.json")
	if data, err := os.ReadFile(versionsPath); err == nil {
		var versions map[string][]*VersionInfo
		if err := json.Unmarshal(data, &versions); err == nil {
			s.versions = versions
		}
	}

	// Load users
	usersPath := filepath.Join(s.dataDir, "users.json")
	if data, err := os.ReadFile(usersPath); err == nil {
		var users map[string]*User
		if err := json.Unmarshal(data, &users); err == nil {
			s.users = users
		}
	}

	return nil
}

func (s *Storage) save() error {
	// Save packages index
	indexPath := filepath.Join(s.dataDir, "index.json")
	if data, err := json.MarshalIndent(s.packages, "", "  "); err == nil {
		os.WriteFile(indexPath, data, 0644)
	}

	// Save versions
	versionsPath := filepath.Join(s.dataDir, "versions.json")
	if data, err := json.MarshalIndent(s.versions, "", "  "); err == nil {
		os.WriteFile(versionsPath, data, 0644)
	}

	// Save users
	usersPath := filepath.Join(s.dataDir, "users.json")
	if data, err := json.MarshalIndent(s.users, "", "  "); err == nil {
		os.WriteFile(usersPath, data, 0644)
	}

	return nil
}

// AddPackage adds or updates a package
func (s *Storage) AddPackage(pkg *PackageInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pkg.ID = generateID(pkg.Name, pkg.Version)
	pkg.UpdatedAt = time.Now()

	s.packages[pkg.ID] = pkg

	// Add version
	if _, exists := s.versions[pkg.Name]; !exists {
		s.versions[pkg.Name] = []*VersionInfo{}
	}

	version := &VersionInfo{
		Version:     pkg.Version,
		Checksum:    pkg.Checksum,
		Size:        pkg.Size,
		PublishedAt: pkg.PublishedAt,
		DownloadURL: pkg.DownloadURL,
	}
	s.versions[pkg.Name] = append(s.versions[pkg.Name], version)

	return s.save()
}

// GetPackage returns a package by ID
func (s *Storage) GetPackage(id string) (*PackageInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pkg, exists := s.packages[id]
	if !exists {
		return nil, fmt.Errorf("package not found: %s", id)
	}
	return pkg, nil
}

// GetPackageByName returns the latest version of a package
func (s *Storage) GetPackageByName(name string) (*PackageInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var latest *PackageInfo
	for _, pkg := range s.packages {
		if pkg.Name == name {
			if latest == nil || pkg.PublishedAt.After(latest.PublishedAt) {
				latest = pkg
			}
		}
	}

	if latest == nil {
		return nil, fmt.Errorf("package not found: %s", name)
	}
	return latest, nil
}

// SearchPackages searches for packages
func (s *Storage) SearchPackages(req *PackageSearchRequest) []*PackageInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*PackageInfo
	query := strings.ToLower(req.Query)

	for _, pkg := range s.packages {
		if matchesSearch(pkg, query, req) {
			results = append(results, pkg)
		}
	}

	// Sort results
	sort.Slice(results, func(i, j int) bool {
		switch req.SortBy {
		case "downloads":
			return results[i].Downloads > results[j].Downloads
		case "updated":
			return results[i].UpdatedAt.After(results[j].UpdatedAt)
		default:
			return results[i].Name < results[j].Name
		}
	})

	// Pagination
	if req.Page > 0 && req.PerPage > 0 {
		start := (req.Page - 1) * req.PerPage
		end := start + req.PerPage
		if start >= len(results) {
			return []*PackageInfo{}
		}
		if end > len(results) {
			end = len(results)
		}
		results = results[start:end]
	}

	return results
}

func matchesSearch(pkg *PackageInfo, query string, req *PackageSearchRequest) bool {
	if query == "" {
		return true
	}

	// Match name
	if strings.Contains(strings.ToLower(pkg.Name), query) {
		return true
	}

	// Match description
	if strings.Contains(strings.ToLower(pkg.Description), query) {
		return true
	}

	// Match author
	if req.Author != "" && strings.Contains(strings.ToLower(pkg.Author), strings.ToLower(req.Author)) {
		return true
	}

	// Match keywords
	if req.Keyword != "" {
		for _, kw := range pkg.Keywords {
			if strings.Contains(strings.ToLower(kw), strings.ToLower(req.Keyword)) {
				return true
			}
		}
	}

	// Match targets
	if len(req.Targets) > 0 {
		for _, target := range req.Targets {
			for _, pkgTarget := range pkg.Targets {
				if target == pkgTarget {
					return true
				}
			}
		}
	}

	return false
}

// IncrementDownloads increments the download count
func (s *Storage) IncrementDownloads(packageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if pkg, exists := s.packages[packageID]; exists {
		pkg.Downloads++
		return s.save()
	}
	return fmt.Errorf("package not found: %s", packageID)
}

// GetStats returns server statistics
func (s *Storage) GetStats() *ServerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var totalDownloads int64
	authors := make(map[string]bool)

	for _, pkg := range s.packages {
		totalDownloads += pkg.Downloads
		authors[pkg.Author] = true
	}

	return &ServerStats{
		TotalPackages:  int64(len(s.packages)),
		TotalDownloads: totalDownloads,
		TotalAuthors:   int64(len(authors)),
		Uptime:         time.Since(startTime).String(),
		LastUpdated:    time.Now(),
	}
}

// ListAll returns all packages
func (s *Storage) ListAll() []*PackageInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*PackageInfo, 0, len(s.packages))
	for _, pkg := range s.packages {
		result = append(result, pkg)
	}
	return result
}

// DeletePackage removes a package
func (s *Storage) DeletePackage(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.packages[id]; !exists {
		return fmt.Errorf("package not found: %s", id)
	}

	delete(s.packages, id)
	return s.save()
}

func generateID(name, version string) string {
	return fmt.Sprintf("%s@%s", name, version)
}

var startTime = time.Now()
