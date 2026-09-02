package nilpkg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultPackageName = "nilpkg"
	ConfigFileName     = "config.json"
	DatabaseFileName   = "db.json"
	PackagesDirName    = "packages"
	RegistryDirName    = "registry"
	CacheDirName       = "cache"
)

// Config represents nilpkg configuration
type Config struct {
	RootDir       string   `json:"root_dir"`
	PackagesDir   string   `json:"packages_dir"`
	RegistryDir   string   `json:"registry_dir"`
	CacheDir      string   `json:"cache_dir"`
	Repositories  []string `json:"repositories"`
	AutoUpdate    bool     `json:"auto_update"`
	VerifySignatures bool  `json:"verify_signatures"`
	ParallelJobs  int      `json:"parallel_jobs"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	rootDir := filepath.Join(homeDir, ".nilang")

	return &Config{
		RootDir:          rootDir,
		PackagesDir:      filepath.Join(rootDir, PackagesDirName),
		RegistryDir:      filepath.Join(rootDir, RegistryDirName),
		CacheDir:         filepath.Join(rootDir, CacheDirName),
		Repositories:     []string{"https://registry.nilang.dev"},
		AutoUpdate:       true,
		VerifySignatures: false, // Enable when signing is implemented
		ParallelJobs:     4,
	}
}

// LoadConfig loads configuration from ~/.nilang/config.json
func LoadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".nilang", ConfigFileName)

	// If config doesn't exist, create default
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := DefaultConfig()
		if err := cfg.Save(); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

// Save writes the configuration to disk
func (c *Config) Save() error {
	// Create directories
	dirs := []string{c.RootDir, c.PackagesDir, c.RegistryDir, c.CacheDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	configPath := filepath.Join(c.RootDir, ConfigFileName)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(configPath, data, 0644)
}

// GetPackagePath returns the installation path for a package
func (c *Config) GetPackagePath(name, version string) string {
	return filepath.Join(c.PackagesDir, name, version)
}

// GetCurrentPath returns the current version symlink path
func (c *Config) GetCurrentPath(name string) string {
	return filepath.Join(c.PackagesDir, name, "current")
}

// AddRepository adds a new repository URL
func (c *Config) AddRepository(url string) error {
	for _, repo := range c.Repositories {
		if repo == url {
			return fmt.Errorf("repository already exists: %s", url)
		}
	}
	c.Repositories = append(c.Repositories, url)
	return c.Save()
}

// RemoveRepository removes a repository URL
func (c *Config) RemoveRepository(url string) error {
	for i, repo := range c.Repositories {
		if repo == url {
			c.Repositories = append(c.Repositories[:i], c.Repositories[i+1:]...)
			return c.Save()
		}
	}
	return fmt.Errorf("repository not found: %s", url)
}