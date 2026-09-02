package nilpkg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// InstalledPackage represents a package installed on the system
type InstalledPackage struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Author       string            `json:"author,omitempty"`
	Description  string            `json:"description,omitempty"`
	InstallPath  string            `json:"install_path"`
	InstalledAt  time.Time         `json:"installed_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	Size         int64             `json:"size_bytes"`
	Checksum     string            `json:"checksum"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	Targets      []string          `json:"targets,omitempty"`
	EntryPoint   string            `json:"entry_point"`
	IsActive     bool              `json:"is_active"`
}

// Database manages the installed packages database
type Database struct {
	config   *Config
	packages map[string]*InstalledPackage
	dbPath   string
}

// NewDatabase creates a new package database
func NewDatabase(cfg *Config) (*Database, error) {
	dbPath := filepath.Join(cfg.RootDir, DatabaseFileName)

	db := &Database{
		config:   cfg,
		packages: make(map[string]*InstalledPackage),
		dbPath:   dbPath,
	}

	if err := db.load(); err != nil {
		// If DB doesn't exist, start fresh
		if os.IsNotExist(err) {
			return db, nil
		}
		return nil, err
	}

	return db, nil
}

func (db *Database) load() error {
	data, err := os.ReadFile(db.dbPath)
	if err != nil {
		return err
	}

	var packages map[string]*InstalledPackage
	if err := json.Unmarshal(data, &packages); err != nil {
		return fmt.Errorf("corrupted database: %w", err)
	}

	db.packages = packages
	return nil
}

func (db *Database) save() error {
	data, err := json.MarshalIndent(db.packages, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal database: %w", err)
	}

	return os.WriteFile(db.dbPath, data, 0644)
}

// Add adds a package to the database
func (db *Database) Add(pkg *InstalledPackage) error {
	if _, exists := db.packages[pkg.Name]; exists {
		return fmt.Errorf("package already installed: %s", pkg.Name)
	}

	db.packages[pkg.Name] = pkg
	return db.save()
}

// Remove removes a package from the database
func (db *Database) Remove(name string) error {
	if _, exists := db.packages[name]; !exists {
		return fmt.Errorf("package not found: %s", name)
	}

	delete(db.packages, name)
	return db.save()
}

// Get returns a package by name
func (db *Database) Get(name string) (*InstalledPackage, error) {
	pkg, exists := db.packages[name]
	if !exists {
		return nil, fmt.Errorf("package not found: %s", name)
	}
	return pkg, nil
}

// List returns all installed packages
func (db *Database) List() []*InstalledPackage {
	result := make([]*InstalledPackage, 0, len(db.packages))
	for _, pkg := range db.packages {
		result = append(result, pkg)
	}
	return result
}

// Has checks if a package is installed
func (db *Database) Has(name string) bool {
	_, exists := db.packages[name]
	return exists
}

// Update updates a package record
func (db *Database) Update(pkg *InstalledPackage) error {
	if _, exists := db.packages[pkg.Name]; !exists {
		return fmt.Errorf("package not found: %s", pkg.Name)
	}

	pkg.UpdatedAt = time.Now()
	db.packages[pkg.Name] = pkg
	return db.save()
}

// Count returns the number of installed packages
func (db *Database) Count() int {
	return len(db.packages)
}

// TotalSize returns the total size of all installed packages
func (db *Database) TotalSize() int64 {
	var total int64
	for _, pkg := range db.packages {
		total += pkg.Size
	}
	return total
}