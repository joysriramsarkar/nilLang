package contract

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

type memTrustStore struct {
	keys map[string]TrustedKey
}

func (m memTrustStore) Key(keyID string) (TrustedKey, bool) {
	tk, ok := m.keys[keyID]
	return tk, ok
}

// signedV2Doc returns a goodV2 manifest with a signature record whose
// signature field is the given hex, so tests can try garbage, tampered and
// genuinely signed variants.
func signedV2Doc(t *testing.T, keyID, sigHex string) []byte {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(goodV2), &raw); err != nil {
		t.Fatalf("parsing goodV2: %v", err)
	}
	rec, err := json.Marshal(map[string]interface{}{
		"algorithm":     "Ed25519",
		"key_id":        keyID,
		"signed_fields": v2SignatureRequiredMembers,
		"signature":     sigHex,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw["signature"] = rec
	out, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func trustOpts(keyID, pubHex string) ValidateOptions {
	return ValidateOptions{
		Archive: FileArchive{
			{Path: "manifest.json", Size: 10},
			{Path: "bytecode/main.nabc", Size: 10},
		},
		Trust: TrustRequired,
		TrustStore: memTrustStore{keys: map[string]TrustedKey{
			keyID: {KeyID: keyID, PublicKeyHex: pubHex, Algorithm: "Ed25519"},
		}},
	}
}

func TestStageTrustVerifiesAV2Signature(t *testing.T) {
	set := mustLoad(t)

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(pub)
	keyID := hex.EncodeToString(sum[:16])
	pubHex := hex.EncodeToString(pub)

	// Sign exactly the canonical bytes the verifier will rebuild.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(goodV2), &raw); err != nil {
		t.Fatal(err)
	}
	canonical, err := canonicalSignedBytes(raw, v2SignatureRequiredMembers)
	if err != nil {
		t.Fatalf("canonicalising: %v", err)
	}
	sig := ed25519.Sign(priv, canonical)

	doc := signedV2Doc(t, keyID, hex.EncodeToString(sig))
	_, vs := set.ValidateJSON(doc, trustOpts(keyID, pubHex))
	if vs.HasStage(StageTrust) {
		t.Fatalf("a genuinely signed v2 manifest was refused:\n%v", vs.OrNil())
	}

	// The same signature over a tampered manifest must be refused.
	var tampered map[string]json.RawMessage
	if err := json.Unmarshal(doc, &tampered); err != nil {
		t.Fatal(err)
	}
	tampered["version"] = json.RawMessage(`"9.9.9"`)
	trebled, _ := json.Marshal(tampered)
	_, vs = set.ValidateJSON(trebled, trustOpts(keyID, pubHex))
	if !vs.HasStage(StageTrust) {
		t.Error("a tampered v2 manifest passed the trust stage")
	}
}

func TestStageTrustRefusesForgedV2Signatures(t *testing.T) {
	set := mustLoad(t)

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(pub)
	keyID := hex.EncodeToString(sum[:16])

	for _, tc := range []struct {
		name   string
		sigHex string
	}{
		{"arbitrary 128 hex chars are not a signature", strings.Repeat("ab", 64)},
		{"uppercase hex is not the specified encoding", strings.ToUpper(strings.Repeat("ab", 64))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := signedV2Doc(t, keyID, tc.sigHex)
			_, vs := set.ValidateJSON(doc, trustOpts(keyID, hex.EncodeToString(pub)))
			if !vs.HasStage(StageTrust) {
				t.Error("a forged signature passed the trust stage")
			}
		})
	}
}

func TestStageTrustRefusesUnknownKeyAndAlgorithm(t *testing.T) {
	set := mustLoad(t)

	// Unknown key id, no store entry.
	doc := signedV2Doc(t, "ffffffffffffffffffffffffffffffff", strings.Repeat("ab", 64))
	_, vs := set.ValidateJSON(doc, ValidateOptions{
		Archive:    FileArchive{{Path: "manifest.json", Size: 10}},
		Trust:      TrustRequired,
		TrustStore: memTrustStore{keys: map[string]TrustedKey{}},
	})
	if !vs.HasStage(StageTrust) {
		t.Error("an unknown signing key passed the trust stage")
	}

	// Unknown algorithm is refused, never skipped.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(goodV2), &raw); err != nil {
		t.Fatal(err)
	}
	rec, _ := json.Marshal(map[string]interface{}{
		"algorithm":     "RSA",
		"key_id":        "ffffffffffffffffffffffffffffffff",
		"signed_fields": v2SignatureRequiredMembers,
		"signature":     strings.Repeat("ab", 64),
	})
	raw["signature"] = rec
	withAlg, _ := json.Marshal(raw)
	_, vs = set.ValidateJSON(withAlg, ValidateOptions{
		Archive: FileArchive{{Path: "manifest.json", Size: 10}},
		Trust:   TrustRequired,
		TrustStore: memTrustStore{keys: map[string]TrustedKey{
			"ffffffffffffffffffffffffffffffff": {KeyID: "ffffffffffffffffffffffffffffffff", PublicKeyHex: strings.Repeat("cd", 32)},
		}},
	})
	if !vs.HasStage(StageTrust) {
		t.Error("an unknown signature algorithm passed the trust stage")
	}
}
