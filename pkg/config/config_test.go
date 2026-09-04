package config

import (
	"os"
	"testing"
)

func TestProjectConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nil-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := DefaultConfig("test-app")
	cfg.Profile = "web"
	cfg.Capabilities = []string{"Network", "Crypto"}

	if err := cfg.Save(tempDir); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	loaded, err := LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loaded.Name != "test-app" || loaded.Profile != "web" {
		t.Errorf("loaded config mismatched: %+v", loaded)
	}

	// Validate capabilities for web profile (should be valid)
	res, err := loaded.ValidateCapabilities()
	if err != nil || !res.Valid {
		t.Errorf("expected valid web capabilities, got err=%v res=%+v", err, res)
	}

	// Add forbidden capability on web
	loaded.Capabilities = append(loaded.Capabilities, "Process")
	resInvalid, err := loaded.ValidateCapabilities()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resInvalid.Valid {
		t.Errorf("expected Process to be invalid on web profile")
	}
}
