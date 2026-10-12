package bundle

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/config"
)

func testConfig() *config.ProjectConfig {
	return &config.ProjectConfig{
		Name:    "repro-app",
		Version: "0.1.0",
		Author:  "Tester",
		Targets: []string{"android"},
		Entry:   "main.nil",
	}
}

func buildOnce(t *testing.T, dir string, buildTime time.Time) []byte {
	t.Helper()
	cfg := testConfig()
	b := NewBuilder(cfg, dir)
	b.SetBuildTime(buildTime)
	b.AddFile("bytecode/main.nabc", []byte{0x01, 0x02, 0x03, 0x04})
	b.AddFile("src/main.nil", []byte("println(\"hello\");"))
	b.AddFile("assets/icon.png", []byte("PNGDATA"))
	out := filepath.Join(dir, "out.nilax")
	if err := b.Build(out); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	return data
}

// TestReproducibleBuildHashIdentical (L8.9/L9.6): two builds of identical
// inputs with a pinned timestamp must be byte- and hash-identical.
func TestReproducibleBuildHashIdentical(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	a := buildOnce(t, dir1, fixed)
	b := buildOnce(t, dir2, fixed)

	if !bytes.Equal(a, b) {
		t.Fatalf("builds differ: %d vs %d bytes", len(a), len(b))
	}
	ha := sha256.Sum256(a)
	hb := sha256.Sum256(b)
	if ha != hb {
		t.Fatalf("hashes differ: %x vs %x", ha, hb)
	}
}

// TestReproducibleBuildTimestampPinned checks the pinned time actually lands
// in the manifest instead of the wall clock.
func TestReproducibleBuildTimestampPinned(t *testing.T) {
	dir := t.TempDir()
	fixed := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	buildOnce(t, dir, fixed)

	reader, err := OpenBundle(filepath.Join(dir, "out.nilax"))
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	defer reader.Close()
	if !reader.GetManifest().CreatedAt.Equal(fixed) {
		t.Fatalf("manifest timestamp not pinned: got %v want %v",
			reader.GetManifest().CreatedAt, fixed)
	}
}
