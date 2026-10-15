package nilpkg

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/joysriramsarkar/nilLang/pkg/bundle"
	"github.com/joysriramsarkar/nilLang/pkg/contract"
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
//
// Verification runs the ecosystem contract's seven stages in addition to the
// integrity checks below: archive safety, manifest identity, payload integrity,
// signer trust, ABI and target compatibility, capability validity, and install
// readiness. Every failure is reported rather than only the first, because a
// package that is both unsigned and built for the wrong target should say so
// once.
//
// The trust posture defaults to TrustIfPresent — a signature is verified when
// present, and an unsigned package is reported as unsigned without being
// refused. Callers that publish or install should ask for TrustRequired
// explicitly; the permissive default exists so that this change does not
// silently break local development, and it is stated here so that nobody
// inherits it by accident.
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

	// Contract validation. A bundle the contract cannot be read for is
	// refused: a verifier that skips this because the contract failed to load
	// is the hole the contract was added to close.
	contractResult := verifyAgainstContract(path, reader)
	result.ContractOK = contractResult.OK
	result.Errors = append(result.Errors, contractResult.Errors...)

	result.Valid = len(result.Errors) == 0

	return result, nil
}

// verifyAgainstContract runs the seven contract stages over a bundle.
func verifyAgainstContract(path string, reader *bundle.Reader) contractCheck {
	var out contractCheck

	set, err := contract.Load()
	if err != nil {
		out.Errors = append(out.Errors, fmt.Sprintf("contract bundle could not be loaded: %v", err))
		return out
	}

	entries := make([]contract.ArchiveEntry, 0, len(reader.ListFiles()))
	for _, name := range reader.ListFiles() {
		data, err := reader.GetFile(name)
		if err != nil {
			continue
		}
		entries = append(entries, contract.ArchiveEntry{
			Path:   name,
			Size:   int64(len(data)),
			SHA256: sha256HexOf(data),
		})
	}

	manifestJSON, err := reader.GetFile("manifest.json")
	if err != nil {
		out.Errors = append(out.Errors, "manifest.json could not be read from the bundle")
		return out
	}

	_, vs := set.ValidateJSON(manifestJSON, contract.ValidateOptions{
		Archive: contract.FileArchive(entries),
		Trust:   contract.TrustIfPresent,
	})
	out.OK = true
	for _, v := range vs {
		out.OK = false
		out.Errors = append(out.Errors, v.Error())
	}
	return out
}

type contractCheck struct {
	OK     bool
	Errors []string
}

// sha256HexOf returns the SHA-256 of data as lowercase hex.
func sha256HexOf(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// VerificationResult holds the result of bundle verification
type VerificationResult struct {
	Path     string
	Checksum string
	Valid    bool
	Errors   []string
	Manifest *bundle.Manifest
	// ContractOK records whether the ecosystem contract's seven stages passed.
	// It is separate from Valid so a caller can tell "this package violates the
	// contract" from "this package is not a readable bundle".
	ContractOK bool
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
