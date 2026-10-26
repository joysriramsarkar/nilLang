package nilpkg

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/bundle"
	"github.com/joysriramsarkar/nilLang/pkg/config"
	"github.com/joysriramsarkar/nilLang/pkg/signing"
)

// TestPublishInstallVerifyRoundTrip (L9.7): key generation (nilkey) → bundle
// build → sign → registry publish → install → verify, end to end.
func TestPublishInstallVerifyRoundTrip(t *testing.T) {
	root := t.TempDir()

	// Isolated nilpkg home so the test never touches ~/.nilang.
	cfg := &Config{
		RootDir:          filepath.Join(root, "nilang"),
		Repositories:     []string{"test-local"},
		VerifySignatures: true,
		ParallelJobs:     1,
	}
	cfg.PackagesDir = filepath.Join(cfg.RootDir, PackagesDirName)
	cfg.RegistryDir = filepath.Join(cfg.RootDir, RegistryDirName)
	cfg.CacheDir = filepath.Join(cfg.RootDir, CacheDirName)
	for _, dir := range []string{cfg.RootDir, cfg.PackagesDir, cfg.RegistryDir, cfg.CacheDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// 1. nilkey: generate an Ed25519 identity.
	kp, ki, err := signing.GenerateKeyPair("tester", "tester@example.com", "signing")
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}
	signer := signing.NewSigner(kp, ki)

	// 2. Build a deterministic .nilax bundle (the "publish" artefact).
	projCfg := &config.ProjectConfig{
		Name: "roundtrip-pkg", Version: "1.0.0", Author: "Tester",
		Targets: []string{"onuron"}, Entry: "main.nil",
	}
	builder := bundle.NewBuilder(projCfg, root)
	builder.SetBuildTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	builder.AddFile("bytecode/main.nabc", []byte{0x01, 0x02, 0x03})
	builder.AddFile("src/main.nil", []byte("println(\"hi\");"))
	bundlePath := filepath.Join(root, "roundtrip-pkg.nilax")
	if err := builder.Build(bundlePath); err != nil {
		t.Fatalf("bundle build failed: %v", err)
	}

	// 3. Sign the artefact with nilkey.
	sig, err := signer.SignFile(bundlePath)
	if err != nil {
		t.Fatalf("SignFile failed: %v", err)
	}
	if err := signing.VerifySignature(kp.GetPublicKeyHex(), sig); err != nil {
		t.Fatalf("signature verify failed: %v", err)
	}
	// The sidecar travels with the bundle, and the key is trusted: the
	// installer refuses unsigned packages with VerifySignatures=true.
	sigJSON, err := sig.ToJSON()
	if err != nil {
		t.Fatalf("signature serialisation failed: %v", err)
	}
	if err := os.WriteFile(bundlePath+".sig", sigJSON, 0644); err != nil {
		t.Fatalf("writing the sidecar signature failed: %v", err)
	}
	if cfg.TrustedKeys == nil {
		cfg.TrustedKeys = map[string]string{}
	}
	cfg.TrustedKeys[kp.KeyID] = kp.GetPublicKeyHex()

	// 4. Publish: record the signed artefact in the registry index.
	reg, err := NewRegistry(cfg)
	if err != nil {
		t.Fatalf("NewRegistry failed: %v", err)
	}
	st, _ := os.Stat(bundlePath)
	if err := reg.Add(&RegistryPackage{
		Name: "roundtrip-pkg", Version: "1.0.0", Description: "round-trip fixture",
		Author: "Tester", Checksum: sig.Checksum, Size: st.Size(),
		Targets: []string{"onuron"},
	}); err != nil {
		t.Fatalf("registry publish failed: %v", err)
	}
	got, err := reg.Get("roundtrip-pkg")
	if err != nil || got.Checksum != sig.Checksum {
		t.Fatalf("registry lookup failed: %+v %v", got, err)
	}

	// 5. Install from the published file.
	db, err := NewDatabase(cfg)
	if err != nil {
		t.Fatalf("NewDatabase failed: %v", err)
	}
	inst := NewInstaller(cfg, db)
	res, err := inst.InstallFromFile(bundlePath)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if !res.Success || res.PackageName != "roundtrip-pkg" {
		t.Fatalf("bad install result: %+v", res)
	}

	// 6. Verify the installed artefact: checksum must match the signature.
	verifier := NewVerifier(cfg)
	vr, err := verifier.VerifyBundle(bundlePath)
	if err != nil || !vr.Valid {
		t.Fatalf("verify failed: %+v %v", vr, err)
	}
	if vr.Checksum != sig.Checksum {
		t.Fatalf("checksum drift: bundle %s vs signature %s", vr.Checksum, sig.Checksum)
	}
	if vr.Manifest.AppName != "roundtrip-pkg" {
		t.Fatalf("wrong manifest app: %s", vr.Manifest.AppName)
	}
}
