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
