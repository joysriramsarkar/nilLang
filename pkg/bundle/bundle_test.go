package bundle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/config"
)

func TestBundleBuildAndRead(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nilang_bundle_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.ProjectConfig{
		Name:    "test-app",
		Version: "0.1.0",
		Author:  "Tester",
		Targets: []string{"onuron"},
		Entry:   "main.nil",
	}

	builder := NewBuilder(cfg, tempDir)
	builder.AddFile("bytecode/main.nabc", []byte{0x01, 0x02, 0x03, 0x04})

	outputPath := filepath.Join(tempDir, "test-app.nilax")
	err = builder.Build(outputPath)
	if err != nil {
		t.Fatalf("failed to build bundle: %v", err)
	}

	reader, err := OpenBundle(outputPath)
	if err != nil {
		t.Fatalf("failed to open bundle: %v", err)
	}
	defer reader.Close()

	manifest := reader.GetManifest()
	if manifest.AppName != "test-app" {
		t.Errorf("expected name 'test-app', got %s", manifest.AppName)
	}
	if manifest.AppVersion != "0.1.0" {
		t.Errorf("expected version '0.1.0', got %s", manifest.AppVersion)
	}

	bc, err := reader.GetBytecode()
	if err != nil {
		t.Fatalf("failed to get bytecode: %v", err)
	}
	if len(bc) != 4 || bc[0] != 0x01 {
		t.Errorf("wrong bytecode content: %v", bc)
	}
}

func TestAppendedBundleRead(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nilang_append_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.ProjectConfig{
		Name:    "appended-app",
		Version: "1.0.0",
		Targets: []string{"linux", "windows"},
	}

	builder := NewBuilder(cfg, tempDir)
	builder.AddFile("src/main.nil", []byte("puts(\"hello appended\");"))

	bundlePath := filepath.Join(tempDir, "app.nilax")
	if err := builder.Build(bundlePath); err != nil {
		t.Fatalf("failed to build: %v", err)
	}

	bundleBytes, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatal(err)
	}

	// Create a "mock executable" with 100KB of dummy binary code prepended to bundle
	mockExePath := filepath.Join(tempDir, "app.exe")
	dummyBinary := make([]byte, 1024*100)
	for i := range dummyBinary {
		dummyBinary[i] = 0x90 // NOP
	}

	appendedData := append(dummyBinary, bundleBytes...)
	if err := os.WriteFile(mockExePath, appendedData, 0755); err != nil {
		t.Fatal(err)
	}

	// Try reading the bundle from the mock executable
	reader, err := OpenBundle(mockExePath)
	if err != nil {
		t.Fatalf("failed to open appended bundle: %v", err)
	}
	defer reader.Close()

	if reader.GetManifest().AppName != "appended-app" {
		t.Fatalf("expected name 'appended-app', got %s", reader.GetManifest().AppName)
	}

	src, err := reader.GetFile("src/main.nil")
	if err != nil {
		t.Fatalf("failed to get src/main.nil: %v", err)
	}
	if string(src) != "puts(\"hello appended\");" {
		t.Fatalf("wrong src content: %s", string(src))
	}
}
