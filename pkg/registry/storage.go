package registry

import (
	"crypto/subtle"
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
	// owners maps a package name to the username that first published it.
	// Ownership survives restarts and version updates; it is the basis for
	// the publish/delete authorization checks in the server.
	owners map[string]string
}

// NewStorage creates a new storage instance
func NewStorage(dataDir string) (*Storage, error) {
	storage := &Storage{
		dataDir:     dataDir,
		packagesDir: filepath.Join(dataDir, "packages"),
		packages:    make(map[string]*PackageInfo),
		versions:    make(map[string][]*VersionInfo),
		users:       make(map[string]*User),
		owners:      make(map[string]string),
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

	// Load users. The on-disk form is userRecord, not User: User deliberately
	// never serializes its password hash/salt, but those must survive a
	// restart or every login breaks and the "no users yet" bootstrap state
	// (which relaxes authorization) comes back.
	usersPath := filepath.Join(s.dataDir, "users.json")
	if data, err := os.ReadFile(usersPath); err == nil {
		var records map[string]*userRecord
		if err := json.Unmarshal(data, &records); err == nil {
			s.users = make(map[string]*User, len(records))
			for name, rec := range records {
				s.users[name] = rec.toUser()
			}
		}
	}

	// Load package ownership
	ownersPath := filepath.Join(s.dataDir, "owners.json")
	if data, err := os.ReadFile(ownersPath); err == nil {
		var owners map[string]string
		if err := json.Unmarshal(data, &owners); err == nil {
			s.owners = owners
		}
	}
	// Packages published before owners.json existed carry the owner in the
	// index itself; adopt those so an upgrade does not orphan ownership.
	for _, pkg := range s.packages {
		if pkg.Owner != "" {
			if _, ok := s.owners[pkg.Name]; !ok {
				s.owners[pkg.Name] = pkg.Owner
			}
		}
	}

	return nil
}

// userRecord is the persisted form of User. PasswordHash/PasswordSalt ride
// along here — and only here; the User type keeps them out of every API
// response via its json:"-" tags, while this file is written 0600.
type userRecord struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	APIKey       string    `json:"api_key,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	IsAdmin      bool      `json:"is_admin"`
	PasswordHash string    `json:"password_hash"`
	PasswordSalt string    `json:"password_salt"`
}

func recordOfUser(u *User) *userRecord {
	return &userRecord{
		ID:           u.ID,
		Username:     u.Username,
		Email:        u.Email,
		APIKey:       u.APIKey,
		CreatedAt:    u.CreatedAt,
		IsAdmin:      u.IsAdmin,
		PasswordHash: u.PasswordHash,
		PasswordSalt: u.PasswordSalt,
	}
}

func (r *userRecord) toUser() *User {
	return &User{
		ID:           r.ID,
		Username:     r.Username,
		Email:        r.Email,
		APIKey:       r.APIKey,
		CreatedAt:    r.CreatedAt,
		IsAdmin:      r.IsAdmin,
		PasswordHash: r.PasswordHash,
		PasswordSalt: r.PasswordSalt,
	}
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

	// Save users in record form (includes the password hash/salt that User
	// never serializes). The file holds API keys and password hashes, so it
	// is owner-only.
	usersPath := filepath.Join(s.dataDir, "users.json")
	records := make(map[string]*userRecord, len(s.users))
	for name, u := range s.users {
		records[name] = recordOfUser(u)
	}
	if data, err := json.MarshalIndent(records, "", "  "); err == nil {
		os.WriteFile(usersPath, data, 0600)
	}

	// Save package ownership
	ownersPath := filepath.Join(s.dataDir, "owners.json")
	if data, err := json.MarshalIndent(s.owners, "", "  "); err == nil {
		os.WriteFile(ownersPath, data, 0644)
	}

	return nil
}

// AddPackage adds or updates a package
func (s *Storage) AddPackage(pkg *PackageInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pkg.ID = generateID(pkg.Name, pkg.Version)
	pkg.UpdatedAt = time.Now()

	// The first publisher of a name owns it. Later publishes of the same
	// name (already ownership-checked by the server) keep the recorded
	// owner, including when an admin publishes on the owner's behalf.
	if existing, ok := s.owners[pkg.Name]; ok {
		pkg.Owner = existing
	} else if pkg.Owner != "" {
		s.owners[pkg.Name] = pkg.Owner
	}

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

// PackageOwner returns the username that owns a package name. The bool is
// false when the name has never been published by a registered user (it is
// unowned, e.g. published anonymously during bootstrap).
func (s *Storage) PackageOwner(name string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	owner, ok := s.owners[name]
	return owner, ok
}

// UserCount returns how many users are registered. Zero has a specific
// meaning for authorization: the server has never had a user, so the
// bootstrap rules apply.
func (s *Storage) UserCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.users)
}

// GetUser returns the user with the given username.
func (s *Storage) GetUser(username string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[username]
	return u, ok
}

// UserForAPIKey returns the user a key belongs to. The comparison is
// constant-time per key, matching the login path.
func (s *Storage) UserForAPIKey(key string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if key == "" {
		return nil, false
	}
	for _, u := range s.users {
		if subtle.ConstantTimeCompare([]byte(u.APIKey), []byte(key)) == 1 {
			return u, true
		}
	}
	return nil, false
}

// RegisterUser adds a user and persists it immediately, so accounts — and
// the admin flag — survive a restart. The first user ever registered
// becomes admin. It reports false without changing anything when the
// username is taken; the existence check and the insert are atomic under
// the storage lock, so two racing registrations cannot both succeed.
func (s *Storage) RegisterUser(u *User) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[u.Username]; exists {
		return false, nil
	}
	if len(s.users) == 0 {
		u.IsAdmin = true
	}
	s.users[u.Username] = u
	if err := s.save(); err != nil {
		delete(s.users, u.Username)
		return false, err
	}
	return true, nil
}

func generateID(name, version string) string {
	return fmt.Sprintf("%s@%s", name, version)
}

var startTime = time.Now()
