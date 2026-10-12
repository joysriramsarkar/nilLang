package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joysriramsarkar/nilLang/pkg/capability"
	"github.com/joysriramsarkar/nilLang/pkg/profile"
)

// ProjectConfig represents the nil.json configuration file
type ProjectConfig struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Author       string            `json:"author,omitempty"`
	Description  string            `json:"description,omitempty"`
	Entry        string            `json:"entry"`
	Profile      string            `json:"profile,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Targets      []string          `json:"targets"`
	Resources    []string          `json:"resources,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	Build        BuildConfig       `json:"build,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Scripts      map[string]string `json:"scripts,omitempty"`
}

type BuildConfig struct {
	OutputDir  string `json:"output_dir,omitempty"`
	Optimize   bool   `json:"optimize,omitempty"`
	Debug      bool   `json:"debug,omitempty"`
	Minify     bool   `json:"minify,omitempty"`
	StripDebug bool   `json:"strip_debug,omitempty"`

	// Libraries are native libraries to link, as they would be written on a
	// compiler's command line: `m`, `pthread`, `ws2_32`.
	//
	// # Why a declaration cannot say which
	//
	// `extern "C" fn sin(x: Float) -> Float;` names a *symbol*. `sin` lives in
	// `libm` on Linux, in the C runtime on Windows, and in `libSystem` on macOS —
	// three different answers from one declaration, and the linker is what knows
	// which. So the program says what it uses and the project says where it lives,
	// and a build that inferred one from the other would either link every library
	// a program mentions or fail to link one it needs.
	//
	// The spellings accepted are `m`, `-lm` and `libm.so`; see
	// `aot.LibraryFlags`.
	Libraries []string `json:"libraries,omitempty"`

	// CSources are C files this project compiles and links into its program,
	// relative to the project directory. A directory means every `.c` file in it,
	// one level deep.
	//
	// # Why a project compiles its own C
	//
	// Because `extern` declares a *symbol* and a library says where an existing
	// symbol lives, and most FFI work is neither: a project's own C is written
	// alongside the Nilang that calls it, and there is no library for it until the
	// build makes one.
	//
	// Compiled by the same toolchain the runtime is, found by capability rather than
	// by name. Asking for `clang` by name would fail on a machine where clang can
	// read IR but cannot compile C — which is exactly this one — with a message
	// about a missing SDK, when the machine has a perfectly good compiler under
	// another name.
	CSources []string `json:"c_sources,omitempty"`

	// CFlags are passed to the C compiler for `CSources` and to the linker,
	// unexamined. `-I`, `-D` and `-std=` all belong here.
	CFlags []string `json:"c_flags,omitempty"`
}

// DefaultConfig returns a default project configuration
func DefaultConfig(name string) *ProjectConfig {
	return &ProjectConfig{
		Name:         name,
		Version:      "0.1.0",
		Author:       "Joysriram Sarkar",
		Entry:        "src/main.nil",
		Profile:      "os",
		Capabilities: []string{"Network", "Filesystem"},
		Targets:      []string{"onuron", "linux"},
		Resources:    []string{"resources/*"},
		Build: BuildConfig{
			OutputDir: "build",
			Optimize:  true,
			Debug:     false,
		},
		Scripts: map[string]string{
			"start": "nil run",
			"dev":   "nil run src/main.nil",
			"build": "nil build",
		},
	}
}

// utf8BOM is the three bytes a UTF-8 byte-order mark is written as.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// trimBOM removes a leading UTF-8 byte-order mark.
//
// # Why this is here rather than "use a BOM-aware reader"
//
// Because a BOM in a JSON file is not the file's problem — the JSON starts at the
// first `{`, and `encoding/json` stops at the first byte it cannot use. Every tool
// on Windows writes one by default: PowerShell's `Out-File`, `Set-Content
// -Encoding utf8`, older Visual Studio, and most editors' "save with encoding"
// menus. A reader that rejects those files makes `nil.json` unwritable from the
// tools a Windows developer already has, and the error it produces — "invalid
// character U+FEFF looking for beginning of value" — names a character nobody
// deliberately typed.
//
// The lexer already skips a BOM for the same reason and with the same reasoning;
// two readers in one project ought to agree about what a BOM is.
func trimBOM(data []byte) []byte {
	if len(data) >= len(utf8BOM) &&
		data[0] == utf8BOM[0] && data[1] == utf8BOM[1] && data[2] == utf8BOM[2] {
		return data[len(utf8BOM):]
	}
	return data
}

// LoadConfig reads and parses nil.json from the given directory
func LoadConfig(dir string) (*ProjectConfig, error) {
	configPath := filepath.Join(dir, "nil.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("nil.json not found in %s: %w", dir, err)
	}

	var config ProjectConfig
	if err := json.Unmarshal(trimBOM(data), &config); err != nil {
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
	if config.Profile == "" {
		config.Profile = "os"
	}
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
		"onuron":       true,
		"android":      true,
		"linux":        true,
		"windows":      true,
		"win":          true,
		"win64":        true,
		"macos":        true,
		"darwin":       true,
		"darwin-arm64": true,
		"darwin-amd64": true,
		"wasm":         true,
		"web":          true,
		"server":       true,
		"data":         true,
	}
	for _, target := range c.Targets {
		normalizedTarget := strings.ToLower(strings.TrimSpace(target))
		if !validTargets[normalizedTarget] {
			return fmt.Errorf("invalid target: %s (valid: onuron, android, linux, windows, macos, darwin, wasm, web, server, data)", target)
		}
	}

	return nil
}

// ValidateCapabilities checks if declared capabilities match the profile
func (c *ProjectConfig) ValidateCapabilities() (*capability.VerificationResult, error) {
	return profile.ValidateProfileCaps(c.Profile, c.Capabilities)
}
