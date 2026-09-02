package signing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Certificate represents a signing certificate
type Certificate struct {
	Version     int       `json:"version"`
	SerialNumber string   `json:"serial_number"`
	Issuer      string    `json:"issuer"`
	Subject     string    `json:"subject"`
	PublicKey   string    `json:"public_key"`
	KeyID       string    `json:"key_id"`
	Algorithm   string    `json:"algorithm"`
	ValidFrom   time.Time `json:"valid_from"`
	ValidTo     time.Time `json:"valid_to"`
	Signature   string    `json:"signature"`
	Extensions  map[string]string `json:"extensions,omitempty"`
	IsCA        bool      `json:"is_ca"`
}

// CertificateChain represents a chain of certificates
type CertificateChain struct {
	Certificates []*Certificate
	RootCA       *Certificate
}

// CertificateAuthority manages certificates
type CertificateAuthority struct {
	keyPair *KeyPair
	keyInfo *KeyInfo
	name    string
}

// NewCertificateAuthority creates a new CA
func NewCertificateAuthority(name string) (*CertificateAuthority, error) {
	keyPair, keyInfo, err := GenerateKeyPair(name, "", "signing")
	if err != nil {
		return nil, err
	}

	return &CertificateAuthority{
		keyPair: keyPair,
		keyInfo: keyInfo,
		name:    name,
	}, nil
}

// IssueCertificate issues a new certificate for a key
func (ca *CertificateAuthority) IssueCertificate(
	subject string,
	publicKeyHex string,
	keyID string,
	validDays int,
	isCA bool,
) (*Certificate, error) {
	validFrom := time.Now()
	validTo := validFrom.AddDate(0, 0, validDays)

	// Generate serial number
	serialHash := sha256.Sum256(fmt.Appendf(nil, "%s-%s-%d", subject, keyID, time.Now().UnixNano()))
	serialNumber := hex.EncodeToString(serialHash[:16])

	cert := &Certificate{
		Version:      1,
		SerialNumber: serialNumber,
		Issuer:       ca.name,
		Subject:      subject,
		PublicKey:    publicKeyHex,
		KeyID:        keyID,
		Algorithm:    "Ed25519",
		ValidFrom:    validFrom,
		ValidTo:      validTo,
		IsCA:         isCA,
		Extensions:   make(map[string]string),
	}

	// Sign the certificate
	if err := ca.signCertificate(cert); err != nil {
		return nil, fmt.Errorf("failed to sign certificate: %w", err)
	}

	return cert, nil
}

func (ca *CertificateAuthority) signCertificate(cert *Certificate) error {
	// Create certificate payload (without signature)
	payload := CertificatePayload{
		SerialNumber: cert.SerialNumber,
		Issuer:       cert.Issuer,
		Subject:      cert.Subject,
		PublicKey:    cert.PublicKey,
		KeyID:        cert.KeyID,
		Algorithm:    cert.Algorithm,
		ValidFrom:    cert.ValidFrom.Unix(),
		ValidTo:      cert.ValidTo.Unix(),
		IsCA:         cert.IsCA,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	signature, err := ca.keyPair.Sign(payloadBytes)
	if err != nil {
		return err
	}

	cert.Signature = hex.EncodeToString(signature)
	return nil
}

// VerifyCertificate verifies a certificate's signature
func (ca *CertificateAuthority) VerifyCertificate(cert *Certificate) error {
	// Check validity period
	now := time.Now()
	if now.Before(cert.ValidFrom) {
		return fmt.Errorf("certificate not yet valid")
	}
	if now.After(cert.ValidTo) {
		return fmt.Errorf("certificate expired")
	}

	// Reconstruct payload
	payload := CertificatePayload{
		SerialNumber: cert.SerialNumber,
		Issuer:       cert.Issuer,
		Subject:      cert.Subject,
		PublicKey:    cert.PublicKey,
		KeyID:        cert.KeyID,
		Algorithm:    cert.Algorithm,
		ValidFrom:    cert.ValidFrom.Unix(),
		ValidTo:      cert.ValidTo.Unix(),
		IsCA:         cert.IsCA,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Decode signature
	sigBytes, err := hex.DecodeString(cert.Signature)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	// Verify with CA's public key
	if !ca.keyPair.Verify(payloadBytes, sigBytes) {
		return fmt.Errorf("certificate signature verification failed")
	}

	return nil
}

// CertificatePayload is the data that gets signed in a certificate
type CertificatePayload struct {
	SerialNumber string `json:"serial_number"`
	Issuer       string `json:"issuer"`
	Subject      string `json:"subject"`
	PublicKey    string `json:"public_key"`
	KeyID        string `json:"key_id"`
	Algorithm    string `json:"algorithm"`
	ValidFrom    int64  `json:"valid_from"`
	ValidTo      int64  `json:"valid_to"`
	IsCA         bool   `json:"is_ca"`
}

// VerifyChain verifies a certificate chain
func VerifyChain(chain *CertificateChain, rootPubKey string) error {
	if len(chain.Certificates) == 0 {
		return fmt.Errorf("empty certificate chain")
	}

	// Verify each certificate in the chain
	for i, cert := range chain.Certificates {
		if i == 0 {
			// First cert is signed by root CA
			if err := verifyWithPublicKey(cert, rootPubKey); err != nil {
				return fmt.Errorf("certificate %d verification failed: %w", i, err)
			}
		} else {
			// Subsequent certs are signed by previous cert
			prevCert := chain.Certificates[i-1]
			if err := verifyWithPublicKey(cert, prevCert.PublicKey); err != nil {
				return fmt.Errorf("certificate %d verification failed: %w", i, err)
			}
		}
	}

	return nil
}

func verifyWithPublicKey(cert *Certificate, pubKeyHex string) error {
	pubKeyBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return err
	}

	payload := CertificatePayload{
		SerialNumber: cert.SerialNumber,
		Issuer:       cert.Issuer,
		Subject:      cert.Subject,
		PublicKey:    cert.PublicKey,
		KeyID:        cert.KeyID,
		Algorithm:    cert.Algorithm,
		ValidFrom:    cert.ValidFrom.Unix(),
		ValidTo:      cert.ValidTo.Unix(),
		IsCA:         cert.IsCA,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	sigBytes, err := hex.DecodeString(cert.Signature)
	if err != nil {
		return err
	}

	keyPair := &KeyPair{PublicKey: pubKeyBytes}
	if !keyPair.Verify(payloadBytes, sigBytes) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}