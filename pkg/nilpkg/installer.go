package nilpkg

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Installer handles package installation and removal
type Installer struct {
	config   *Config
	database *Database
	verifier *Verifier
}

// NewInstaller creates a new package installer
func NewInstaller(cfg *Config, db *Database) *Installer {
	return &Installer{
		config:   cfg,
		database: db,
		verifier: NewVerifier(cfg),
	}
}

// InstallResult holds the result of an installation
type InstallResult struct {
	PackageName string
	Version     string
	InstallPath string
	Size        int64
	Checksum    string
	Duration    time.Duration
	Success     bool
	Error       error
}

// InstallFromFile installs a package from a .nilax file
func (inst *Installer) InstallFromFile(path string) (*InstallResult, error) {
	startTime := time.Now()
	result := &InstallResult{}

	// Step 1: Verify the bundle
	fmt.Print("   [1/5] ভেরিফাই করা হচ্ছে... ")
	verification, err := inst.verifier.VerifyBundle(path)
	if err != nil {
		fmt.Println("❌")
		return nil, fmt.Errorf("verification failed: %w", err)
	}
	if !verification.Valid {
		fmt.Println("❌")
		return nil, fmt.Errorf("invalid bundle: %v", verification.Errors)
	}
	fmt.Println("✅")

	manifest := verification.Manifest
	result.PackageName = manifest.AppName
	result.Version = manifest.AppVersion
	result.Checksum = verification.Checksum

	// Step 2: Check if already installed
	if inst.database.Has(manifest.AppName) {
		existing, _ := inst.database.Get(manifest.AppName)
		if existing.Version == manifest.AppVersion {
			return nil, fmt.Errorf("package %s v%s is already installed",
				manifest.AppName, manifest.AppVersion)
		}
		fmt.Printf("   ⚠️  %s v%s ইতিমধ্যে ইনস্টল আছে, আপডেট হচ্ছে...\n",
			existing.Name, existing.Version)
	}

	// Step 3: Create installation directory
	fmt.Print("   [2/5] ইনস্টল ডিরেক্টরি তৈরি হচ্ছে... ")
	installPath := inst.config.GetPackagePath(manifest.AppName, manifest.AppVersion)
	if err := os.MkdirAll(installPath, 0755); err != nil {
		fmt.Println("❌")
		return nil, fmt.Errorf("failed to create install directory: %w", err)
	}
	fmt.Println("✅")

	// Step 4: Extract bundle
	fmt.Print("   [3/5] বান্ডিল এক্সট্র্যাক্ট হচ্ছে... ")
	size, err := inst.extractBundle(path, installPath)
	if err != nil {
		fmt.Println("❌")
		os.RemoveAll(installPath)
		return nil, fmt.Errorf("extraction failed: %w", err)
	}
	fmt.Println("✅")

	// Step 5: Create "current" symlink
	fmt.Print("   [4/5] কারেন্ট ভার্সন লিংক হচ্ছে... ")
	currentPath := inst.config.GetCurrentPath(manifest.AppName)
	os.Remove(currentPath) // Remove existing symlink
	if err := os.Symlink(manifest.AppVersion, currentPath); err != nil {
		fmt.Println("⚠️")
	} else {
		fmt.Println("✅")
	}

	// Step 6: Register in database
	fmt.Print("   [5/5] ডাটাবেসে রেজিস্টার হচ্ছে... ")
	pkg := &InstalledPackage{
		Name:         manifest.AppName,
		Version:      manifest.AppVersion,
		Author:       manifest.Author,
		Description:  manifest.Description,
		InstallPath:  installPath,
		InstalledAt:  time.Now(),
		UpdatedAt:    time.Now(),
		Size:         size,
		Checksum:     verification.Checksum,
		Dependencies: manifest.Dependencies,
		Targets:      manifest.Targets,
		EntryPoint:   manifest.EntryBytecode,
		IsActive:     true,
	}

	if err := inst.database.Add(pkg); err != nil {
		// If adding fails (e.g., update), try updating
		if err := inst.database.Update(pkg); err != nil {
			fmt.Println("❌")
			return nil, fmt.Errorf("failed to register package: %w", err)
		}
	}
	fmt.Println("✅")

	result.InstallPath = installPath
	result.Size = size
	result.Duration = time.Since(startTime)
	result.Success = true

	return result, nil
}

// extractBundle extracts a .nilax bundle to the target directory
func (inst *Installer) extractBundle(bundlePath, targetDir string) (int64, error) {
	zipReader, err := zip.OpenReader(bundlePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open bundle: %w", err)
	}
	defer zipReader.Close()

	var totalSize int64

	for _, file := range zipReader.File {
		filePath := filepath.Join(targetDir, file.Name)

		// Security: prevent path traversal
		if !isSafePath(targetDir, filePath) {
			return 0, fmt.Errorf("unsafe path in bundle: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			os.MkdirAll(filePath, 0755)
			continue
		}

		// Create parent directory
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return 0, err
		}

		// Extract file
		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return 0, err
		}

		srcFile, err := file.Open()
		if err != nil {
			dstFile.Close()
			return 0, err
		}

		written, err := io.Copy(dstFile, srcFile)
		srcFile.Close()
		dstFile.Close()

		if err != nil {
			return 0, err
		}

		totalSize += written
	}

	return totalSize, nil
}

// Uninstall removes a package from the system
func (inst *Installer) Uninstall(name string) error {
	pkg, err := inst.database.Get(name)
	if err != nil {
		return fmt.Errorf("package not found: %s", name)
	}

	fmt.Printf("   🗑️  %s v%s আনইনস্টল হচ্ছে...\n", pkg.Name, pkg.Version)

	// Remove installation directory
	if err := os.RemoveAll(pkg.InstallPath); err != nil {
		return fmt.Errorf("failed to remove package files: %w", err)
	}

	// Remove current symlink
	currentPath := inst.config.GetCurrentPath(name)
	os.Remove(currentPath)

	// Remove parent directory if empty
	parentDir := filepath.Dir(pkg.InstallPath)
	entries, _ := os.ReadDir(parentDir)
	if len(entries) == 0 {
		os.Remove(parentDir)
	}

	// Remove from database
	if err := inst.database.Remove(name); err != nil {
		return fmt.Errorf("failed to remove from database: %w", err)
	}

	fmt.Printf("   ✅ %s সফলভাবে আনইনস্টল হয়েছে\n", name)
	return nil
}

// Update updates a package to the latest version
func (inst *Installer) Update(name string, newBundlePath string) (*InstallResult, error) {
	// Check if package exists
	if !inst.database.Has(name) {
		return nil, fmt.Errorf("package not installed: %s", name)
	}

	oldPkg, _ := inst.database.Get(name)
	fmt.Printf("   📦 %s v%s → আপডেট হচ্ছে...\n", name, oldPkg.Version)

	// Install new version
	result, err := inst.InstallFromFile(newBundlePath)
	if err != nil {
		return nil, err
	}

	// Remove old version
	oldPath := inst.config.GetPackagePath(name, oldPkg.Version)
	if oldPath != result.InstallPath {
		os.RemoveAll(oldPath)
	}

	return result, nil
}

// isSafePath checks if a path is safe (no directory traversal)
func isSafePath(base, path string) bool {
	absBase, _ := filepath.Abs(base)
	absPath, _ := filepath.Abs(path)
	return len(absPath) >= len(absBase) && absPath[:len(absBase)] == absBase
}

// FormatSize formats bytes to human-readable size
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}