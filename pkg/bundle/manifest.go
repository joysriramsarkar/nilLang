package bundle

import (
	"encoding/json"
	"time"
)

// Manifest represents the metadata inside a .nilax bundle
type Manifest struct {
	FormatVersion   int               `json:"format_version"`
	AppName         string            `json:"app_name"`
	AppVersion      string            `json:"app_version"`
	Author          string            `json:"author,omitempty"`
	Description     string            `json:"description,omitempty"`
	EntryBytecode   string            `json:"entry_bytecode"`
	Targets         []string          `json:"targets"`
	CreatedAt       time.Time         `json:"created_at"`
	CompilerVersion string            `json:"compiler_version"`
	Dependencies    map[string]string `json:"dependencies,omitempty"`
	Permissions     []string          `json:"permissions,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	Signature       string            `json:"signature,omitempty"`
}

// NewManifest creates a new manifest with default values
func NewManifest(name, version, author string) *Manifest {
	return &Manifest{
		FormatVersion:   1,
		AppName:         name,
		AppVersion:      version,
		Author:          author,
		EntryBytecode:   "bytecode/main.nabc",
		CreatedAt:       time.Now(),
		CompilerVersion: "0.1.0",
		Targets:         []string{"onuron"},
		Dependencies:    make(map[string]string),
		Permissions:     []string{},
		Metadata:        make(map[string]string),
	}
}

// ToJSON serializes the manifest to JSON
func (m *Manifest) ToJSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// FromJSON deserializes a manifest from JSON
func FromJSON(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate checks if the manifest is valid
func (m *Manifest) Validate() error {
	if m.FormatVersion != 1 {
		return ErrUnsupportedFormat
	}
	if m.AppName == "" {
		return ErrMissingAppName
	}
	if m.EntryBytecode == "" {
		return ErrMissingEntry
	}
	return nil
}

// Bundle errors
var (
	ErrUnsupportedFormat = &BundleError{"unsupported bundle format version"}
	ErrMissingAppName    = &BundleError{"missing app name in manifest"}
	ErrMissingEntry      = &BundleError{"missing entry bytecode in manifest"}
	ErrInvalidBundle     = &BundleError{"invalid .nilax bundle"}
)

type BundleError struct {
	Message string
}

func (e *BundleError) Error() string {
	return e.Message
}
