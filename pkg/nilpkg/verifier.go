package nilpkg

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/joysriramsarkar/nilLang/pkg/bundle"
)

// Verifier handles package integrity verification
type Verifier struct {
	config *Config
}

// NewVerifier creates a new package verifier
func NewVerifier(cfg *Config) *Verifier {
	return &Verifier{config: cfg}
}

// VerifyFile calculates and verifies the SHA-256 checksum of a file
func (v *Verifier) VerifyFile(path string, expectedChecksum string) error {
	actualChecksum, err := v.CalculateChecksum(path)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s",
			expectedChecksum, actualChecksum)
	}

	return nil
}

// CalculateChecksum calculates the SHA-256 checksum of a file
func (v *Verifier) CalculateChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// VerifyBundle performs comprehensive verification on a .nilax bundle
func (v *Verifier) VerifyBundle(path string) (*VerificationResult, error) {
	result := &VerificationResult{
		Path:   path,
		Errors: []string{},
	}

	// Check file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		result.Errors = append(result.Errors, "file not found")
		result.Valid = false
		return result, nil
	}

	// Calculate checksum
	checksum, err := v.CalculateChecksum(path)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("checksum error: %v", err))
		result.Valid = false
		return result, nil
	}
	result.Checksum = checksum

	// Try to open as a valid bundle
	reader, err := bundle.OpenBundle(path)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("invalid bundle: %v", err))
		result.Valid = false
		return result, nil
	}
	defer reader.Close()

	// Verify manifest
	manifest := reader.GetManifest()
	if err := manifest.Validate(); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("invalid manifest: %v", err))
		result.Valid = false
		return result, nil
	}

	// Verify bytecode exists
	if _, err := reader.GetBytecode(); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("missing bytecode: %v", err))
		result.Valid = false
		return result, nil
	}

	result.Manifest = manifest
	result.Valid = len(result.Errors) == 0

	return result, nil
}

// VerificationResult holds the result of bundle verification
type VerificationResult struct {
	Path     string
	Checksum string
	Valid    bool
	Errors   []string
	Manifest *bundle.Manifest
}

func (vr *VerificationResult) String() string {
	status := "✅ VALID"
	if !vr.Valid {
		status = "❌ INVALID"
	}

	result := fmt.Sprintf("📦 Bundle Verification: %s\n", status)
	result += fmt.Sprintf("   Path: %s\n", vr.Path)
	result += fmt.Sprintf("   Checksum: %s\n", vr.Checksum)

	if vr.Manifest != nil {
		result += fmt.Sprintf("   App: %s v%s\n", vr.Manifest.AppName, vr.Manifest.AppVersion)
		result += fmt.Sprintf("   Author: %s\n", vr.Manifest.Author)
		result += fmt.Sprintf("   Targets: %v\n", vr.Manifest.Targets)
	}

	if len(vr.Errors) > 0 {
		result += "\n   Errors:\n"
		for _, err := range vr.Errors {
			result += fmt.Sprintf("   - %s\n", err)
		}
	}

	return result
}
