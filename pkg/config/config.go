package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ProjectConfig represents the nil.json configuration file
type ProjectConfig struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Author      string            `json:"author,omitempty"`
	Description string            `json:"description,omitempty"`
	Entry       string            `json:"entry"`
	Targets     []string          `json:"targets"`
	Resources   []string          `json:"resources,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	Build       BuildConfig       `json:"build,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type BuildConfig struct {
	OutputDir   string `json:"output_dir,omitempty"`
	Optimize    bool   `json:"optimize,omitempty"`
	Debug       bool   `json:"debug,omitempty"`
	Minify      bool   `json:"minify,omitempty"`
	StripDebug  bool   `json:"strip_debug,omitempty"`
}

// DefaultConfig returns a default project configuration
func DefaultConfig(name string) *ProjectConfig {
	return &ProjectConfig{
		Name:    name,
		Version: "0.1.0",
		Author:  "Joyshriram Sarkar",
		Entry:   "src/main.nil",
		Targets: []string{"onuron", "linux"},
		Resources: []string{"resources/*"},
		Build: BuildConfig{
			OutputDir: "build",
			Optimize:  true,
			Debug:     false,
		},
	}
}

// LoadConfig reads and parses nil.json from the given directory
func LoadConfig(dir string) (*ProjectConfig, error) {
	configPath := filepath.Join(dir, "nil.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("nil.json not found in %s: %w", dir, err)
	}

	var config ProjectConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse nil.json: %w", err)
	}

	// Validate required fields
	if config.Name == "" {
		return nil, fmt.Errorf("nil.json: 'name' field is required")
	}
	if config.Entry == "" {
		return nil, fmt.Errorf("nil.json: 'entry' field is required")
	}
	if len(config.Targets) == 0 {
		config.Targets = []string{"onuron"}
	}

	// Set defaults
	if config.Build.OutputDir == "" {
		config.Build.OutputDir = "build"
	}
	if config.Version == "" {
		config.Version = "0.1.0"
	}

	return &config, nil
}

// SaveConfig writes the configuration to nil.json
func (c *ProjectConfig) Save(dir string) error {
	configPath := filepath.Join(dir, "nil.json")

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write nil.json: %w", err)
	}

	return nil
}

// GetOutputPath returns the full path for the output .nilax file
func (c *ProjectConfig) GetOutputPath(projectDir string) string {
	outputDir := filepath.Join(projectDir, c.Build.OutputDir)
	filename := fmt.Sprintf("%s-%s.nilax", c.Name, c.Version)
	return filepath.Join(outputDir, filename)
}

// GetEntryPath returns the full path to the entry file
func (c *ProjectConfig) GetEntryPath(projectDir string) string {
	return filepath.Join(projectDir, c.Entry)
}

// Validate checks if the project configuration is valid
func (c *ProjectConfig) Validate(projectDir string) error {
	// Check entry file exists
	entryPath := c.GetEntryPath(projectDir)
	if _, err := os.Stat(entryPath); os.IsNotExist(err) {
		return fmt.Errorf("entry file not found: %s", entryPath)
	}

	// Check targets are valid
	validTargets := map[string]bool{
		"onuron":  true,
		"android": true,
		"ios":     true,
		"linux":   true,
	}
	for _, target := range c.Targets {
		if !validTargets[target] {
			return fmt.Errorf("invalid target: %s (valid: onuron, android, ios, linux)", target)
		}
	}

	return nil
}