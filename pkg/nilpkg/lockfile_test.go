package nilpkg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLockfileIntegrityAndFrozen(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Create a dummy package artifact
	pkgFile := filepath.Join(tempDir, "std-http-1.2.0.nilax")
	pkgContent := "package std-http binary content v1.2.0"
	if err := os.WriteFile(pkgFile, []byte(pkgContent), 0644); err != nil {
		t.Fatalf("failed to write dummy package: %v", err)
	}

	checksum, err := ComputeFileChecksum(pkgFile)
	if err != nil {
		t.Fatalf("failed to compute checksum: %v", err)
	}

	// 2. Create lockfile
	lf := NewLockfile()
	lf.AddPackage(&LockedPackage{
		Name:     "std-http",
		Version:  "1.2.0",
		Source:   "https://registry.nilang.dev/pkg/std-http",
		Checksum: checksum,
	})

	lockfilePath := filepath.Join(tempDir, LockfileName)
	if err := lf.Save(lockfilePath); err != nil {
		t.Fatalf("failed to save lockfile: %v", err)
	}

	// 3. Load lockfile and verify checksum
	loadedLf, err := LoadLockfile(lockfilePath)
	if err != nil {
		t.Fatalf("failed to load lockfile: %v", err)
	}

	if err := loadedLf.VerifyPackageChecksum("std-http", pkgFile); err != nil {
		t.Fatalf("expected valid checksum, got: %v", err)
	}

	// 4. Tamper with package file and ensure verification fails
	tamperedContent := "package std-http tampered content"
	if err := os.WriteFile(pkgFile, []byte(tamperedContent), 0644); err != nil {
		t.Fatalf("failed to write tampered package: %v", err)
	}

	if err := loadedLf.VerifyPackageChecksum("std-http", pkgFile); err == nil {
		t.Fatal("expected checksum verification error for tampered package, got nil")
	}

	// 5. Test CheckFrozen
	// Match manifest:
	manifest := map[string]string{
		"std-http": "1.2.0",
	}
	if err := loadedLf.CheckFrozen(manifest); err != nil {
		t.Fatalf("CheckFrozen failed on valid match: %v", err)
	}

	// Conflict manifest:
	conflictManifest := map[string]string{
		"std-http": "2.0.0",
	}
	if err := loadedLf.CheckFrozen(conflictManifest); err == nil {
		t.Fatal("expected CheckFrozen error on version conflict, got nil")
	}

	// Missing dependency in lockfile:
	missingManifest := map[string]string{
		"std-http": "1.2.0",
		"std-json": "0.9.0",
	}
	if err := loadedLf.CheckFrozen(missingManifest); err == nil {
		t.Fatal("expected CheckFrozen error on missing dependency, got nil")
	}
}
