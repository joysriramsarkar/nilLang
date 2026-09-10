package android

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/config"
)

func TestAndroidBuildAPKPackaging(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Create dummy project
	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}

	mainNil := filepath.Join(srcDir, "main.nil")
	if err := os.WriteFile(mainNil, []byte("println(\"Hello Android from Nilang!\");"), 0644); err != nil {
		t.Fatalf("failed to write main.nil: %v", err)
	}

	buildDir := filepath.Join(tempDir, "build")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatalf("failed to create build dir: %v", err)
	}
	bundleNilax := filepath.Join(buildDir, "testapp-1.0.0.nilax")
	if err := os.WriteFile(bundleNilax, []byte("NILAX_DUMMY_BYTECODE_BUNDLE"), 0644); err != nil {
		t.Fatalf("failed to write bundle: %v", err)
	}

	// 2. Setup project config
	cfg := &config.ProjectConfig{
		Name:    "testapp",
		Version: "1.0.0",
		Entry:   "src/main.nil",
		Build: config.BuildConfig{
			OutputDir: "build",
		},
	}

	outputAPK := filepath.Join(tempDir, "bin", "testapp.apk")

	// 3. Build APK
	res, err := BuildAPK(cfg, tempDir, outputAPK)
	if err != nil {
		t.Fatalf("BuildAPK failed: %v", err)
	}

	if res.Path != outputAPK {
		t.Errorf("expected output path %s, got %s", outputAPK, res.Path)
	}

	// 4. Verify APK zip contents
	zr, err := zip.OpenReader(outputAPK)
	if err != nil {
		t.Fatalf("failed to open generated APK: %v", err)
	}
	defer zr.Close()

	foundAppNil := false
	foundAppNilax := false

	for _, f := range zr.File {
		if f.Name == "assets/app.nil" {
			foundAppNil = true
		}
		if f.Name == "assets/app.nilax" {
			foundAppNilax = true
		}
	}

	if !foundAppNil {
		t.Error("assets/app.nil missing from generated APK")
	}
	if !foundAppNilax {
		t.Error("assets/app.nilax missing from generated APK")
	}
}
