package signing

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateAndSign(t *testing.T) {
	keyPair, keyInfo, err := GenerateKeyPair("Developer", "dev@onuron.org", "signing")
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	tempDir, err := os.MkdirTemp("", "signing_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dummyFile := filepath.Join(tempDir, "app.nilax")
	err = os.WriteFile(dummyFile, []byte("test bundle content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	signer := NewSigner(keyPair, keyInfo)
	sig, err := signer.SignFile(dummyFile)
	if err != nil {
		t.Fatalf("failed to sign file: %v", err)
	}

	if sig.SignerName != "Developer" {
		t.Errorf("wrong signer name: %s", sig.SignerName)
	}

	err = VerifySignature(keyPair.GetPublicKeyHex(), sig)
	if err != nil {
		t.Fatalf("signature verification failed: %v", err)
	}

	err = VerifyFileChecksum(dummyFile, sig.Checksum)
	if err != nil {
		t.Fatalf("checksum verification failed: %v", err)
	}
}
