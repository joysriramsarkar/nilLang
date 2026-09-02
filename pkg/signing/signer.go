package signing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// PackageSignature represents a signed package
type PackageSignature struct {
	PackageName    string    `json:"package_name"`
	PackageVersion string    `json:"package_version"`
	Checksum       string    `json:"checksum"`       // SHA-256 of the package file
	Signature      string    `json:"signature"`      // Ed25519 signature (hex)
	SignerKeyID    string    `json:"signer_key_id"`
	SignerName     string    `json:"signer_name"`
	Algorithm      string    `json:"algorithm"`
	SignedAt       time.Time `json:"signed_at"`
	Certificate    *Certificate `json:"certificate,omitempty"`
}

// Signer handles package signing
type Signer struct {
	keyPair *KeyPair
	keyInfo *KeyInfo
}

// NewSigner creates a new package signer
func NewSigner(keyPair *KeyPair, keyInfo *KeyInfo) *Signer {
	return &Signer{
		keyPair: keyPair,
		keyInfo: keyInfo,
	}
}

// SignFile signs a .nilax package file
func (s *Signer) SignFile(filePath string) (*PackageSignature, error) {
	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Calculate checksum
	checksum := sha256.Sum256(data)
	checksumHex := hex.EncodeToString(checksum[:])

	// Create signing payload
	payload := SigningPayload{
		Checksum:  checksumHex,
		Timestamp: time.Now().Unix(),
		KeyID:     s.keyPair.KeyID,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Sign the payload
	signature, err := s.keyPair.Sign(payloadBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to sign: %w", err)
	}

	return &PackageSignature{
		Checksum:    checksumHex,
		Signature:   hex.EncodeToString(signature),
		SignerKeyID: s.keyPair.KeyID,
		SignerName:  s.keyInfo.Owner,
		Algorithm:   "Ed25519",
		SignedAt:    time.Now(),
	}, nil
}

// SignChecksum signs a pre-calculated checksum
func (s *Signer) SignChecksum(checksum string, packageName, version string) (*PackageSignature, error) {
	payload := SigningPayload{
		Checksum:  checksum,
		Timestamp: time.Now().Unix(),
		KeyID:     s.keyPair.KeyID,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	signature, err := s.keyPair.Sign(payloadBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to sign: %w", err)
	}

	return &PackageSignature{
		PackageName:    packageName,
		PackageVersion: version,
		Checksum:       checksum,
		Signature:      hex.EncodeToString(signature),
		SignerKeyID:    s.keyPair.KeyID,
		SignerName:     s.keyInfo.Owner,
		Algorithm:      "Ed25519",
		SignedAt:       time.Now(),
	}, nil
}

// SigningPayload is the data that gets signed
type SigningPayload struct {
	Checksum  string `json:"checksum"`
	Timestamp int64  `json:"timestamp"`
	KeyID     string `json:"key_id"`
}

// VerifySignature verifies a package signature
func VerifySignature(pubKeyHex string, sig *PackageSignature) error {
	// Load public key
	pubKeyBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}

	// Decode signature
	sigBytes, err := hex.DecodeString(sig.Signature)
	if err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}

	// Reconstruct payload
	payload := SigningPayload{
		Checksum:  sig.Checksum,
		Timestamp: sig.SignedAt.Unix(),
		KeyID:     sig.SignerKeyID,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Verify
	keyPair := &KeyPair{
		PublicKey: pubKeyBytes,
	}

	if !keyPair.Verify(payloadBytes, sigBytes) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// VerifyFileChecksum verifies that a file matches its signed checksum
func VerifyFileChecksum(filePath string, expectedChecksum string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	checksum := sha256.Sum256(data)
	actualChecksum := hex.EncodeToString(checksum[:])

	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s",
			expectedChecksum, actualChecksum)
	}

	return nil
}

func (ps *PackageSignature) ToJSON() ([]byte, error) {
	return json.MarshalIndent(ps, "", "  ")
}

func SignatureFromJSON(data []byte) (*PackageSignature, error) {
	var sig PackageSignature
	if err := json.Unmarshal(data, &sig); err != nil {
		return nil, err
	}
	return &sig, nil
}