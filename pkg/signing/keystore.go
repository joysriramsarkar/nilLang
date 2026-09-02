package signing

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// KeyStore manages encrypted key storage
type KeyStore struct {
	path     string
	password string
	keys     map[string]*StoredKey
}

// StoredKey represents an encrypted key in the keystore
type StoredKey struct {
	KeyID            string `json:"key_id"`
	EncryptedPrivKey string `json:"encrypted_private_key"`
	PublicKey        string `json:"public_key"`
	Algorithm        string `json:"algorithm"`
	Owner            string `json:"owner"`
	Purpose          string `json:"purpose"`
	Salt             string `json:"salt"`
	Nonce            string `json:"nonce"`
}

// NewKeyStore creates or opens a keystore
func NewKeyStore(path string, password string) (*KeyStore, error) {
	ks := &KeyStore{
		path:     path,
		password: password,
		keys:     make(map[string]*StoredKey),
	}

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create keystore directory: %w", err)
	}

	// Load existing keystore
	if err := ks.load(); err != nil {
		// If file doesn't exist, start fresh
		return ks, nil
	}

	return ks, nil
}

func (ks *KeyStore) load() error {
	data, err := os.ReadFile(ks.path)
	if err != nil {
		return err
	}

	var keys map[string]*StoredKey
	if err := json.Unmarshal(data, &keys); err != nil {
		return fmt.Errorf("corrupted keystore: %w", err)
	}

	ks.keys = keys
	return nil
}

func (ks *KeyStore) save() error {
	data, err := json.MarshalIndent(ks.keys, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ks.path, data, 0600)
}

// AddKey encrypts and stores a key pair
func (ks *KeyStore) AddKey(keyPair *KeyPair, keyInfo *KeyInfo) error {
	// Generate salt
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	// Derive encryption key from password + salt
	encKey := deriveKey(ks.password, salt)

	// Encrypt private key
	encryptedPriv, nonce, err := encrypt(encKey, keyPair.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt private key: %w", err)
	}

	storedKey := &StoredKey{
		KeyID:            keyPair.KeyID,
		EncryptedPrivKey: hex.EncodeToString(encryptedPriv),
		PublicKey:        keyPair.GetPublicKeyHex(),
		Algorithm:        keyPair.Algorithm,
		Owner:            keyInfo.Owner,
		Purpose:          keyInfo.Purpose,
		Salt:             hex.EncodeToString(salt),
		Nonce:            hex.EncodeToString(nonce),
	}

	ks.keys[keyPair.KeyID] = storedKey
	return ks.save()
}

// GetKey retrieves and decrypts a key pair
func (ks *KeyStore) GetKey(keyID string) (*KeyPair, *KeyInfo, error) {
	storedKey, exists := ks.keys[keyID]
	if !exists {
		return nil, nil, fmt.Errorf("key not found: %s", keyID)
	}

	// Decode salt
	salt, err := hex.DecodeString(storedKey.Salt)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid salt: %w", err)
	}

	// Derive decryption key
	decKey := deriveKey(ks.password, salt)

	// Decode encrypted private key
	encryptedPriv, err := hex.DecodeString(storedKey.EncryptedPrivKey)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid encrypted key: %w", err)
	}

	// Decode nonce
	nonce, err := hex.DecodeString(storedKey.Nonce)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid nonce: %w", err)
	}

	// Decrypt private key
	privKey, err := decrypt(decKey, encryptedPriv, nonce)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decrypt private key (wrong password?): %w", err)
	}

	// Decode public key
	pubKey, err := hex.DecodeString(storedKey.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid public key: %w", err)
	}

	keyPair := &KeyPair{
		PublicKey:  pubKey,
		PrivateKey: privKey,
		KeyID:      keyID,
		Algorithm:  storedKey.Algorithm,
	}

	keyInfo := &KeyInfo{
		KeyID:   keyID,
		Owner:   storedKey.Owner,
		Purpose: storedKey.Purpose,
	}

	return keyPair, keyInfo, nil
}

// ListKeys returns all key IDs in the keystore
func (ks *KeyStore) ListKeys() []*KeyInfo {
	result := make([]*KeyInfo, 0, len(ks.keys))
	for _, storedKey := range ks.keys {
		result = append(result, &KeyInfo{
			KeyID:     storedKey.KeyID,
			Owner:     storedKey.Owner,
			Purpose:   storedKey.Purpose,
			Algorithm: storedKey.Algorithm,
		})
	}
	return result
}

// DeleteKey removes a key from the keystore
func (ks *KeyStore) DeleteKey(keyID string) error {
	if _, exists := ks.keys[keyID]; !exists {
		return fmt.Errorf("key not found: %s", keyID)
	}
	delete(ks.keys, keyID)
	return ks.save()
}

// --- Crypto Helpers ---

func deriveKey(password string, salt []byte) []byte {
	// Simple key derivation (in production, use Argon2 or scrypt)
	combined := append([]byte(password), salt...)
	hash := sha256.Sum256(combined)
	return hash[:]
}

func encrypt(key, plaintext []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	ciphertext := aesGCM.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

func decrypt(key, ciphertext, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
