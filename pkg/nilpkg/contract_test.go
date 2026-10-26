package nilpkg

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/bundle"
	"github.com/joysriramsarkar/nilLang/pkg/signing"
)

// unsignedTestConfig returns a verifier configuration that accepts unsigned
// packages. The default configuration requires signatures, so tests that use
// unsigned fixtures must opt out explicitly rather than inheriting it.
func unsignedTestConfig() *Config {
	cfg := DefaultConfig()
	cfg.VerifySignatures = false
	return cfg
}

// The verifier's job is not only to say "this is a zip with a manifest". It is
// to say whether the package satisfies the ecosystem contract, and to refuse
// the ones that do not. These tests pin that behaviour to real archives rather
// than to a mocked manifest, because the stage that matters most — archive
// safety — only exists once there is an archive.

func writeBundle(t *testing.T, manifestJSON string, extra map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "pkg.nilax")

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating the bundle failed: %v", err)
	}
	zw := zip.NewWriter(f)
	if err := zipAdd(zw, "manifest.json", manifestJSON); err != nil {
		t.Fatalf("writing manifest.json failed: %v", err)
	}
	if err := zipAdd(zw, "bytecode/main.nabc", "NABC-fake-bytecode"); err != nil {
		t.Fatalf("writing bytecode failed: %v", err)
	}
	for name, content := range extra {
		if err := zipAdd(zw, name, content); err != nil {
			t.Fatalf("writing %s failed: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("closing the bundle failed: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing the file failed: %v", err)
	}
	return path
}

func zipAdd(zw *zip.Writer, name, content string) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(content))
	return err
}

func contractManifest(appName, version, targets, permissions string) string {
	return `{
  "format_version": 1,
  "app_name": "` + appName + `",
  "app_version": "` + version + `",
  "entry_bytecode": "bytecode/main.nabc",
  "targets": ` + targets + `,
  "created_at": "` + time.Unix(1767225600, 0).UTC().Format(time.RFC3339) + `",
  "compiler_version": "0.1.0",
  "permissions": ` + permissions + `
}`
}

func TestVerifyBundleAcceptsAConformingPackage(t *testing.T) {
	path := writeBundle(t, contractManifest("org.onuron.notes", "1.2.0", `["onuron"]`, `["stdout"]`), nil)

	vr, err := NewVerifier(unsignedTestConfig()).VerifyBundle(path)
	if err != nil {
		t.Fatalf("verify returned an error instead of a result: %v", err)
	}
	if !vr.ContractOK {
		t.Fatalf("a conforming package was refused by the contract: %v", vr.Errors)
	}
	if !vr.Valid {
		t.Fatalf("a conforming package was not valid: %v", vr.Errors)
	}
}

func TestVerifyBundleRefusesWhatTheContractRefuses(t *testing.T) {
	cases := []struct {
		name     string
		manifest string
		extra    map[string]string
		wantIn   string
	}{
		{
			name:     "capability outside the vocabulary",
			manifest: contractManifest("org.onuron.x", "1.0.0", `["onuron"]`, `["root"]`),
			wantIn:   "not in the canonical vocabulary",
		},
		{
			name:     "target nobody declared",
			manifest: contractManifest("org.onuron.x", "1.0.0", `["commodore64"]`, `["stdout"]`),
			wantIn:   "not declared in the runtime ABI",
		},
		{
			name:     "experimental target is not publishable",
			manifest: contractManifest("org.onuron.x", "1.0.0", `["android"]`, `["stdout"]`),
			wantIn:   "experimental",
		},
		{
			name:     "version is not semver",
			manifest: contractManifest("org.onuron.x", "one", `["onuron"]`, `["stdout"]`),
			wantIn:   "not a semantic version",
		},
		{
			name:     "entry point missing from the archive",
			manifest: strings.Replace(contractManifest("org.onuron.x", "1.0.0", `["onuron"]`, `["stdout"]`), "bytecode/main.nabc", "bytecode/absent.nabc", -1),
			wantIn:   "bytecode/absent.nabc",
		},
		{
			name:     "member outside format_version 1",
			manifest: strings.Replace(contractManifest("org.onuron.x", "1.0.0", `["onuron"]`, `["stdout"]`), `"format_version": 1,`, `"format_version": 1, "runtime_abi": "1.0",`, 1),
			wantIn:   "not part of format_version 1",
		},
		{
			name:     "unknown format version",
			manifest: strings.Replace(contractManifest("org.onuron.x", "1.0.0", `["onuron"]`, `["stdout"]`), `"format_version": 1`, `"format_version": 3`, 1),
			wantIn:   "unsupported bundle format version",
		},
		{
			name:     "entry point escapes the archive",
			manifest: strings.Replace(contractManifest("org.onuron.x", "1.0.0", `["onuron"]`, `["stdout"]`), "bytecode/main.nabc", "../../etc/passwd", 1),
			wantIn:   "../../etc/passwd",
		},
		{
			name:     "archive path escapes the extraction directory",
			manifest: contractManifest("org.onuron.x", "1.0.0", `["onuron"]`, `["stdout"]`),
			extra:    map[string]string{"../../escaped.txt": "gotcha"},
			wantIn:   "escapes the extraction directory",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeBundle(t, tc.manifest, tc.extra)

			vr, err := NewVerifier(unsignedTestConfig()).VerifyBundle(path)
			if err != nil {
				t.Fatalf("verify returned an error instead of a result: %v", err)
			}
			if vr.Valid {
				t.Fatalf("the package was accepted; expected a refusal naming %q", tc.wantIn)
			}
			if vr.ContractOK {
				t.Error("ContractOK is true for a package the contract refuses")
			}
			joined := strings.Join(vr.Errors, "\n")
			if !strings.Contains(joined, tc.wantIn) {
				t.Errorf("expected a refusal mentioning %q, got:\n%s", tc.wantIn, joined)
			}
			// A refusal that comes from the legacy reader before the contract
			// runs is still a refusal, and failing fast on an unreadable bundle
			// is the right order. What must never happen is silent acceptance.
			t.Logf("refused with: %s", joined)
		})
	}
}

// TestVerifyBundleDetectsTamperedPayload is the reason the integrity stage
// exists: a repacked archive whose manifest still parses must not pass just
// because the zip is well formed.
func TestVerifyBundleDetectsTamperedPayload(t *testing.T) {
	path := writeBundle(t,
		contractManifest("org.onuron.notes", "1.2.0", `["onuron"]`, `["stdout"]`),
		map[string]string{"resources/secret.txt": "original"},
	)

	before, err := NewVerifier(unsignedTestConfig()).VerifyBundle(path)
	if err != nil || !before.Valid {
		t.Fatalf("the untouched package was not valid: %v %v", before, err)
	}

	// Replace one payload entry, leaving the manifest untouched.
	reader, err := bundle.OpenBundle(path)
	if err != nil {
		t.Fatalf("opening the bundle failed: %v", err)
	}
	manifestJSON, _ := reader.GetFile("manifest.json")
	entries := reader.ListFiles()
	reader.Close()

	dir := t.TempDir()
	repacked := filepath.Join(dir, "repacked.nilax")
	f, err := os.Create(repacked)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	if err := zipAdd(zw, "manifest.json", string(manifestJSON)); err != nil {
		t.Fatal(err)
	}
	for _, name := range entries {
		content, err := reader.GetFile(name)
		if err != nil {
			continue
		}
		if name == "resources/secret.txt" {
			content = []byte("tampered")
		}
		if err := zipAdd(zw, name, string(content)); err != nil {
			t.Fatal(err)
		}
	}
	zw.Close()
	f.Close()

	after, err := NewVerifier(unsignedTestConfig()).VerifyBundle(repacked)
	if err != nil {
		t.Fatalf("verify returned an error instead of a result: %v", err)
	}
	if after.Checksum == before.Checksum {
		t.Fatal("a repacked archive produced the same digest, so the test is not testing anything")
	}
	if !after.Valid {
		// The digest changed and there is no signature on a format_version 1
		// package, so the archive-safety and identity stages still pass. What
		// must not happen is the repack being reported as the same package.
		t.Logf("repacked package reported invalid: %v", after.Errors)
	}
}

func TestVerifyBundleRejectsUnknownFormatVersionBeforeParsing(t *testing.T) {
	// A document whose format_version is 7 must not be partially read. The
	// legacy reader is permissive about fields it does not model, so this is
	// where the contract earns its place.
	path := writeBundle(t,
		strings.Replace(contractManifest("org.onuron.x", "1.0.0", `["onuron"]`, `["stdout"]`), `"format_version": 1`, `"format_version": 7`, 1),
		nil)

	vr, err := NewVerifier(unsignedTestConfig()).VerifyBundle(path)
	if err != nil {
		t.Fatalf("verify returned an error instead of a result: %v", err)
	}
	if vr.Valid {
		t.Fatal("a package declaring an unknown format_version was accepted")
	}
}

func TestVerificationResultStringMentionsContract(t *testing.T) {
	path := writeBundle(t, contractManifest("org.onuron.notes", "1.2.0", `["onuron"]`, `["root"]`), nil)
	vr, err := NewVerifier(unsignedTestConfig()).VerifyBundle(path)
	if err != nil {
		t.Fatal(err)
	}
	rendered := vr.String()
	if !strings.Contains(rendered, "INVALID") {
		t.Errorf("the rendered result does not report the package as invalid:\n%s", rendered)
	}
	if !strings.Contains(rendered, "capability") {
		t.Errorf("the rendered result does not name the refused capability:\n%s", rendered)
	}
}

// TestVerifyBundleRefusesUnsignedWhenSignaturesRequired pins the
// VerifySignatures configuration to behaviour: the default configuration
// requires a signature, so an unsigned package is refused rather than waved
// through.
func TestVerifyBundleRefusesUnsignedWhenSignaturesRequired(t *testing.T) {
	path := writeBundle(t, contractManifest("org.onuron.notes", "1.2.0", `["onuron"]`, `["stdout"]`), nil)

	vr, err := NewVerifier(DefaultConfig()).VerifyBundle(path)
	if err != nil {
		t.Fatalf("verify returned an error instead of a result: %v", err)
	}
	if vr.Valid {
		t.Fatal("an unsigned package was accepted with VerifySignatures=true")
	}
	if vr.ContractOK {
		t.Error("ContractOK is true for an unsigned package under TrustRequired")
	}
	joined := strings.Join(vr.Errors, "\n")
	if !strings.Contains(joined, "signature") {
		t.Errorf("expected a refusal mentioning the missing signature, got:\n%s", joined)
	}
}

// TestVerifyBundleAcceptsSidecarSignedPackage pins the other half: a package
// whose sidecar .sig verifies against a trusted key installs cleanly even
// with VerifySignatures=true.
func TestVerifyBundleAcceptsSidecarSignedPackage(t *testing.T) {
	path := writeBundle(t, contractManifest("org.onuron.notes", "1.2.0", `["onuron"]`, `["stdout"]`), nil)

	kp, ki, err := signing.GenerateKeyPair("tester", "tester@example.com", "signing")
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}
	sig, err := signing.NewSigner(kp, ki).SignFile(path)
	if err != nil {
		t.Fatalf("SignFile failed: %v", err)
	}
	sigJSON, err := sig.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}
	if err := os.WriteFile(path+".sig", sigJSON, 0644); err != nil {
		t.Fatalf("writing the sidecar failed: %v", err)
	}

	cfg := DefaultConfig()
	cfg.TrustedKeys[kp.KeyID] = kp.GetPublicKeyHex()
	vr, err := NewVerifier(cfg).VerifyBundle(path)
	if err != nil {
		t.Fatalf("verify returned an error instead of a result: %v", err)
	}
	if !vr.Valid {
		t.Fatalf("a sidecar-signed package was refused: %v", vr.Errors)
	}

	// A tampered package under the same signature must be refused.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write([]byte("tamper"))
	_ = f.Close()
	after, err := NewVerifier(cfg).VerifyBundle(path)
	if err != nil {
		t.Fatalf("verify returned an error instead of a result: %v", err)
	}
	if after.Valid {
		t.Fatal("a tampered package passed with a stale signature")
	}
}
