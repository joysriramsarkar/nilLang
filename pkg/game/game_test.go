package game

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/config"
)

func TestResolveEnginesAll(t *testing.T) {
	targets, err := ResolveEngines([]string{"all"})
	if err != nil {
		t.Fatalf("ResolveEngines failed: %v", err)
	}
	if len(targets) != 4 {
		t.Fatalf("expected 4 engines, got %d", len(targets))
	}
	if targets[0].ID != EngineAndroid || targets[3].ID != EngineUnreal {
		t.Fatalf("unexpected engine order: %+v", targets)
	}
}

func TestScaffoldFilesIncludesEngineAdapters(t *testing.T) {
	files, err := ScaffoldFiles(ProjectOptions{
		Name:    "arcade-demo",
		Version: "0.1.0",
		Author:  "Nilang",
		Engines: []string{"unity,godot"},
	})
	if err != nil {
		t.Fatalf("ScaffoldFiles failed: %v", err)
	}

	for _, path := range []string{
		"nil.json",
		"src/main.nil",
		"resources/nilang.game.json",
		"engine/unity/Runtime/NilangBehaviour.cs",
		"engine/godot/nilang.gdextension",
	} {
		if _, ok := files[path]; !ok {
			t.Fatalf("expected scaffold file %s", path)
		}
	}
}

func TestCheckProjectReady(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tempDir, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "src", "main.nil"), []byte("puts(\"ready\");"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"engine/android", "engine/godot"} {
		if err := os.MkdirAll(filepath.Join(tempDir, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}

	cfg := &config.ProjectConfig{
		Name:         "ready-game",
		Version:      "0.1.0",
		Entry:        "src/main.nil",
		Profile:      ProfileID,
		Capabilities: append([]string(nil), DefaultCapabilities...),
		Targets:      []string{"android", "windows"},
		Metadata: map[string]string{
			"engine_adapters": "android,godot",
		},
	}

	report := CheckProject(cfg, tempDir)
	if !report.Ready {
		t.Fatalf("expected ready report, got %+v", report.Checks)
	}
}
