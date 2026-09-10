package nilpkg

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const LockfileName = "nil.lock"

// LockedPackage describes an exact pinned package version and integrity hash
type LockedPackage struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Source       string            `json:"source,omitempty"`
	Checksum     string            `json:"checksum"` // sha256:<hex>
	Dependencies map[string]string `json:"dependencies,omitempty"`
	Signature    string            `json:"signature,omitempty"`
}

// Lockfile represents the content of nil.lock
type Lockfile struct {
	Version     int                       `json:"version"`
	GeneratedAt time.Time                 `json:"generated_at"`
	Packages    map[string]*LockedPackage `json:"packages"`
}

func NewLockfile() *Lockfile {
	return &Lockfile{
		Version:     1,
		GeneratedAt: time.Now().UTC(),
		Packages:    make(map[string]*LockedPackage),
	}
}

// LoadLockfile loads and parses nil.lock from the specified path
func LoadLockfile(path string) (*Lockfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var lf Lockfile
	if err := json.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("corrupt lockfile %s: %w", path, err)
	}

	if lf.Packages == nil {
		lf.Packages = make(map[string]*LockedPackage)
	}

	return &lf, nil
}

// Save writes the lockfile to disk with deterministic JSON formatting
func (lf *Lockfile) Save(path string) error {
	lf.GeneratedAt = time.Now().UTC()
	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal lockfile: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

// AddPackage records or updates a locked package entry
func (lf *Lockfile) AddPackage(pkg *LockedPackage) {
	if lf.Packages == nil {
		lf.Packages = make(map[string]*LockedPackage)
	}
	lf.Packages[pkg.Name] = pkg
}

// GetPackage retrieves a locked package by name
func (lf *Lockfile) GetPackage(name string) (*LockedPackage, bool) {
	if lf.Packages == nil {
		return nil, false
	}
	pkg, ok := lf.Packages[name]
	return pkg, ok
}

// VerifyPackageChecksum verifies that a file matches the checksum in the lockfile
func (lf *Lockfile) VerifyPackageChecksum(name string, filePath string) error {
	pkg, ok := lf.GetPackage(name)
	if !ok {
		return fmt.Errorf("package %q not recorded in lockfile", name)
	}

	actualHash, err := ComputeFileChecksum(filePath)
	if err != nil {
		return fmt.Errorf("failed to hash package file %s: %w", filePath, err)
	}

	if actualHash != pkg.Checksum {
		return fmt.Errorf("integrity violation for %s: expected %s, got %s", name, pkg.Checksum, actualHash)
	}

	return nil
}

// CheckFrozen ensures that the lockfile satisfies the manifest without requiring updates.
// If any manifest dependency is missing, or versions differ, it returns an error.
func (lf *Lockfile) CheckFrozen(manifestDeps map[string]string) error {
	for name, reqVer := range manifestDeps {
		locked, ok := lf.Packages[name]
		if !ok {
			return fmt.Errorf("frozen lockfile violation: package %q is declared in manifest but missing in %s", name, LockfileName)
		}

		if reqVer != "" && reqVer != "*" && !strings.HasPrefix(reqVer, "^") && !strings.HasPrefix(reqVer, "~") {
			if locked.Version != reqVer {
				return fmt.Errorf("frozen lockfile violation: package %q version mismatch (manifest requests %s, lockfile specifies %s)", name, reqVer, locked.Version)
			}
		}

		if locked.Checksum == "" {
			return fmt.Errorf("frozen lockfile violation: package %q is missing SHA-256 integrity checksum in %s", name, LockfileName)
		}
	}
	return nil
}

// ComputeFileChecksum returns sha256:<hex> of a given file
func ComputeFileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// ComputeDirectoryChecksum computes a deterministic combined sha256 of all files in a directory
func ComputeDirectoryChecksum(dirPath string) (string, error) {
	var files []string
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, relErr := filepath.Rel(dirPath, path)
			if relErr != nil {
				return relErr
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	sort.Strings(files)

	h := sha256.New()
	for _, rel := range files {
		fullPath := filepath.Join(dirPath, filepath.FromSlash(rel))
		f, err := os.Open(fullPath)
		if err != nil {
			return "", err
		}
		// Write relative path to hash
		h.Write([]byte(rel + "\n"))
		if _, err := io.Copy(h, f); err != nil {
			f.Close()
			return "", err
		}
		f.Close()
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
