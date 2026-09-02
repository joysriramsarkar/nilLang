package signing

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// KeyPair represents an Ed25519 key pair
type KeyPair struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
	KeyID      string
	CreatedAt  time.Time
	Algorithm  string
}

// KeyInfo holds metadata about a key
type KeyInfo struct {
	KeyID       string    `json:"key_id"`
	Algorithm   string    `json:"algorithm"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	Owner       string    `json:"owner"`
	Email       string    `json:"email,omitempty"`
	Purpose     string    `json:"purpose"` // "signing", "encryption", "both"
	Revoked     bool      `json:"revoked"`
	Fingerprint string    `json:"fingerprint"`
}

// GenerateKeyPair generates a new Ed25519 key pair
func GenerateKeyPair(owner, email, purpose string) (*KeyPair, *KeyInfo, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Generate key ID from public key hash
	hash := sha256.Sum256(pub)
	keyID := hex.EncodeToString(hash[:16])

	// Generate fingerprint
	fingerprint := formatFingerprint(hash[:])

	keyPair := &KeyPair{
		PublicKey:  pub,
		PrivateKey: priv,
		KeyID:      keyID,
		CreatedAt:  time.Now(),
		Algorithm:  "Ed25519",
	}

	keyInfo := &KeyInfo{
		KeyID:       keyID,
		Algorithm:   "Ed25519",
		CreatedAt:   time.Now(),
		Owner:       owner,
		Email:       email,
		Purpose:     purpose,
		Revoked:     false,
		Fingerprint: fingerprint,
	}

	return keyPair, keyInfo, nil
}

// Sign signs data using the private key
func (kp *KeyPair) Sign(data []byte) ([]byte, error) {
	if kp.PrivateKey == nil {
		return nil, fmt.Errorf("private key not available")
	}
	return ed25519.Sign(kp.PrivateKey, data), nil
}

// Verify verifies a signature using the public key
func (kp *KeyPair) Verify(data, signature []byte) bool {
	return ed25519.Verify(kp.PublicKey, data, signature)
}

// GetPublicKeyHex returns the public key as hex string
func (kp *KeyPair) GetPublicKeyHex() string {
	return hex.EncodeToString(kp.PublicKey)
}

// GetPrivateKeyHex returns the private key as hex string (use with caution!)
func (kp *KeyPair) GetPrivateKeyHex() string {
	return hex.EncodeToString(kp.PrivateKey)
}

// LoadKeyPairFromHex loads a key pair from hex strings
func LoadKeyPairFromHex(pubHex, privHex string) (*KeyPair, error) {
	pubBytes, err := hex.DecodeString(pubHex)
	if err != nil {
		return nil, fmt.Errorf("invalid public key hex: %w", err)
	}

	privBytes, err := hex.DecodeString(privHex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key hex: %w", err)
	}

	if len(pubBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: %d", len(pubBytes))
	}

	if len(privBytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: %d", len(privBytes))
	}

	hash := sha256.Sum256(pubBytes)
	keyID := hex.EncodeToString(hash[:16])

	return &KeyPair{
		PublicKey:  ed25519.PublicKey(pubBytes),
		PrivateKey: ed25519.PrivateKey(privBytes),
		KeyID:      keyID,
		Algorithm:  "Ed25519",
	}, nil
}

func formatFingerprint(hash []byte) string {
	hexStr := hex.EncodeToString(hash[:20])
	result := ""
	for i := 0; i < len(hexStr); i += 4 {
		if i > 0 {
			result += ":"
		}
		end := i + 4
		if end > len(hexStr) {
			end = len(hexStr)
		}
		result += hexStr[i:end]
	}
	return result
}