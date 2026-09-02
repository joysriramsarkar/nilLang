package nilpkg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RegistryPackage represents a package in the registry
type RegistryPackage struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	DownloadURL string   `json:"download_url"`
	Checksum    string   `json:"checksum"`
	Size        int64    `json:"size_bytes"`
	Targets     []string `json:"targets"`
	PublishedAt time.Time `json:"published_at"`
	Downloads   int64    `json:"downloads"`
}

// Registry manages the package registry
type Registry struct {
	config   *Config
	index    map[string]*RegistryPackage
	indexPath string
}

// NewRegistry creates a new package registry
func NewRegistry(cfg *Config) (*Registry, error) {
	indexPath := filepath.Join(cfg.RegistryDir, "index.json")

	reg := &Registry{
		config:    cfg,
		index:     make(map[string]*RegistryPackage),
		indexPath: indexPath,
	}

	if err := reg.load(); err != nil {
		// If index doesn't exist, start with empty registry
		return reg, nil
	}

	return reg, nil
}

func (r *Registry) load() error {
	data, err := os.ReadFile(r.indexPath)
	if err != nil {
		return err
	}

	var index map[string]*RegistryPackage
	if err := json.Unmarshal(data, &index); err != nil {
		return fmt.Errorf("corrupted registry index: %w", err)
	}

	r.index = index
	return nil
}

func (r *Registry) save() error {
	data, err := json.MarshalIndent(r.index, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.indexPath, data, 0644)
}

// Add adds a package to the local registry
func (r *Registry) Add(pkg *RegistryPackage) error {
	r.index[pkg.Name] = pkg
	return r.save()
}

// Remove removes a package from the local registry
func (r *Registry) Remove(name string) error {
	delete(r.index, name)
	return r.save()
}

// Get returns a package from the registry
func (r *Registry) Get(name string) (*RegistryPackage, error) {
	pkg, exists := r.index[name]
	if !exists {
		return nil, fmt.Errorf("package not found in registry: %s", name)
	}
	return pkg, nil
}

// Search searches for packages matching a query
func (r *Registry) Search(query string) []*RegistryPackage {
	var results []*RegistryPackage
	for _, pkg := range r.index {
		if matchesQuery(pkg, query) {
			results = append(results, pkg)
		}
	}
	return results
}

// List returns all packages in the registry
func (r *Registry) List() []*RegistryPackage {
	result := make([]*RegistryPackage, 0, len(r.index))
	for _, pkg := range r.index {
		result = append(result, pkg)
	}
	return result
}

func matchesQuery(pkg *RegistryPackage, query string) bool {
	query = toLower(query)
	return contains(toLower(pkg.Name), query) ||
		contains(toLower(pkg.Description), query) ||
		contains(toLower(pkg.Author), query)
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}