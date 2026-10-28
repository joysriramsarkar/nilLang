package nilpkg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/signing"
)

// A config file that omits verify_signatures must not silently downgrade
// signature verification: LoadConfig unmarshals over DefaultConfig, so an
// absent field keeps its default.
func TestLoadConfigKeepsDefaultsForOmittedFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".nilang")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A minimal/legacy config: only one field set, the rest omitted.
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(`{"parallel_jobs": 9}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.VerifySignatures {
		t.Error("VerifySignatures downgraded to false by omission; want default true")
	}
	if !cfg.AutoUpdate {
		t.Error("AutoUpdate lost its default true")
	}
	if cfg.ParallelJobs != 9 {
		t.Errorf("ParallelJobs = %d, want 9 (explicit file value wins)", cfg.ParallelJobs)
	}
	if cfg.RootDir == "" || cfg.PackagesDir == "" {
		t.Error("directory fields lost their defaults")
	}
	if got := cfg.TrustedKeys[signing.OnuronTeamKeyID]; got != signing.OnuronTeamPublicKeyHex {
		t.Errorf("default trusted key missing/altered: %q", got)
	}
}

// An explicit false in the file is a choice and must be honoured.
func TestLoadConfigHonoursExplicitVerifySignaturesFalse(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".nilang")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte(`{"verify_signatures": false}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.VerifySignatures {
		t.Error("explicit verify_signatures=false was overridden by the default")
	}
}
