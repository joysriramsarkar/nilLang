package signing

import (
	"bytes"
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

func TestKeyStoreRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	ksPath := filepath.Join(tempDir, "keystore.json")

	keyPair, keyInfo, err := GenerateKeyPair("Developer", "dev@onuron.org", "signing")
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	ks, err := NewKeyStore(ksPath, "correct-horse-battery")
	if err != nil {
		t.Fatalf("failed to create keystore: %v", err)
	}
	if err := ks.AddKey(keyPair, keyInfo); err != nil {
		t.Fatalf("failed to add key: %v", err)
	}

	// Reopen with the same password — private key must decrypt byte-for-byte
	ks2, err := NewKeyStore(ksPath, "correct-horse-battery")
	if err != nil {
		t.Fatalf("failed to reopen keystore: %v", err)
	}
	got, gotInfo, err := ks2.GetKey(keyPair.KeyID)
	if err != nil {
		t.Fatalf("failed to get key with correct password: %v", err)
	}
	if !bytes.Equal(got.PrivateKey, keyPair.PrivateKey) {
		t.Error("decrypted private key does not match original")
	}
	if !bytes.Equal(got.PublicKey, keyPair.PublicKey) {
		t.Error("decrypted public key does not match original")
	}
	if gotInfo.Owner != keyInfo.Owner {
		t.Errorf("wrong owner: %s", gotInfo.Owner)
	}

	// Public key must be retrievable without a password
	pubHex, err := ks2.GetPublicKey(keyPair.KeyID)
	if err != nil {
		t.Fatalf("failed to get public key: %v", err)
	}
	if pubHex != keyPair.GetPublicKeyHex() {
		t.Error("stored public key does not match")
	}
}

func TestKeyStoreWrongPassword(t *testing.T) {
	tempDir := t.TempDir()
	ksPath := filepath.Join(tempDir, "keystore.json")

	keyPair, keyInfo, err := GenerateKeyPair("Developer", "dev@onuron.org", "signing")
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	ks, err := NewKeyStore(ksPath, "password-one")
	if err != nil {
		t.Fatalf("failed to create keystore: %v", err)
	}
	if err := ks.AddKey(keyPair, keyInfo); err != nil {
		t.Fatalf("failed to add key: %v", err)
	}

	// A different password must fail decryption
	ks2, err := NewKeyStore(ksPath, "password-two")
	if err != nil {
		t.Fatalf("failed to reopen keystore: %v", err)
	}
	if _, _, err := ks2.GetKey(keyPair.KeyID); err == nil {
		t.Fatal("expected decryption failure with wrong password, got nil")
	}
}

func TestSignVerifyRoundTripViaKeystore(t *testing.T) {
	tempDir := t.TempDir()
	ksPath := filepath.Join(tempDir, "keystore.json")

	keyPair, keyInfo, err := GenerateKeyPair("Developer", "dev@onuron.org", "signing")
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	ks, err := NewKeyStore(ksPath, "signing-password")
	if err != nil {
		t.Fatalf("failed to create keystore: %v", err)
	}
	if err := ks.AddKey(keyPair, keyInfo); err != nil {
		t.Fatalf("failed to add key: %v", err)
	}

	ks2, err := NewKeyStore(ksPath, "signing-password")
	if err != nil {
		t.Fatalf("failed to reopen keystore: %v", err)
	}
	loadedKey, loadedInfo, err := ks2.GetKey(keyPair.KeyID)
	if err != nil {
		t.Fatalf("failed to load key: %v", err)
	}

	dummyFile := filepath.Join(tempDir, "app.nilax")
	if err := os.WriteFile(dummyFile, []byte("test bundle content"), 0644); err != nil {
		t.Fatal(err)
	}

	signer := NewSigner(loadedKey, loadedInfo)
	sig, err := signer.SignFile(dummyFile)
	if err != nil {
		t.Fatalf("failed to sign file: %v", err)
	}

	pubHex, err := ks2.GetPublicKey(keyPair.KeyID)
	if err != nil {
		t.Fatalf("failed to get public key: %v", err)
	}
	if err := VerifySignature(pubHex, sig); err != nil {
		t.Fatalf("signature verification failed: %v", err)
	}

	// Tampered signature must fail
	tampered := *sig
	tampered.Checksum = "0000" + sig.Checksum[4:]
	if err := VerifySignature(pubHex, &tampered); err == nil {
		t.Fatal("expected verification failure for tampered signature, got nil")
	}
}
