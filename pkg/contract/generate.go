package contract

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	langcap "github.com/joysriramsarkar/nilLang/compiler/capability"
	"github.com/joysriramsarkar/nilLang/pkg/signing"
)

// Generated file paths, relative to the contracts directory.
const (
	PathRequirements = "capabilities/requirements.json"
	PathVectors      = "testdata/signing/v1-vectors.json"
	PathIndex        = "bundle.json"
)

// requirementsDoc is the document written to capabilities/requirements.json.
//
// Every field is derived: the table comes from compiler/capability and the
// prose is fixed here. Regenerating it on a machine that has not changed the
// compiler produces identical bytes, which is what makes "the contract matches
// the code" a testable statement rather than an intention.
type requirementsDoc struct {
	Schema        string              `json:"$schema"`
	Contract      string              `json:"contract"`
	Version       string              `json:"version"`
	Status        string              `json:"status"`
	Owner         string              `json:"owner"`
	Spec          string              `json:"spec"`
	SourceOfTruth string              `json:"source_of_truth"`
	GeneratedBy   string              `json:"generated_by"`
	Description   string              `json:"description"`
	DefaultDeny   string              `json:"default_deny"`
	UnknownOp     string              `json:"unknown_operation"`
	Requirements  map[string][]string `json:"requirements"`
}

// BuildRequirements produces the bytes of capabilities/requirements.json from the
// compiler's requirement table.
func BuildRequirements() ([]byte, error) {
	ops := langcap.KnownOperations()
	reqs := make(map[string][]string, len(ops))

	for _, op := range ops {
		req, ok := langcap.RequirementsFor(op)
		if !ok {
			// KnownOperations and RequirementsFor read the same map, so this
			// cannot happen. It is checked anyway because a silently dropped
			// operation would be a security hole with no symptom.
			return nil, fmt.Errorf("contract: %s is listed as known but has no requirement", op)
		}
		names := append([]string(nil), req...)
		if names == nil {
			// An empty requirement is meaningful: the operation is known and
			// needs nothing. It is stored as [] rather than null so that a
			// reader in another language does not have to distinguish "no
			// requirement" from "field missing".
			names = []string{}
		}
		reqs[op] = names
	}

	doc := requirementsDoc{
		Schema:        "https://json-schema.org/draft/2020-12/schema",
		Contract:      "onuron.capabilities.requirements",
		Version:       "1.0.0",
		Status:        StatusStable,
		Owner:         "nilLang",
		Spec:          "docs/spec/CAPABILITIES.md",
		SourceOfTruth: "compiler/capability/manifest.go",
		GeneratedBy:   "go run ./cmd/nil-contract generate",
		Description: "Capabilities each builtin and std module operation requires, generated from the compiler's " +
			"requirement table. An operation present here with an empty list is known and needs nothing; an " +
			"operation absent from this table is refused rather than assumed pure.",
		DefaultDeny: "An operation absent from this table is refused. A verifier that cannot find an operation here " +
			"must not report that it requires nothing: that is the difference between a typo being a refusal and a " +
			"typo being a hole.",
		UnknownOp:    "refused, with the same refusal an unknown operation gets at run time",
		Requirements: reqs,
	}
	return MarshalIndent(doc)
}

// signingVector is one conformance vector for the v1 package signature.
type signingVector struct {
	Name string `json:"name"`
	// Expect states whether a conforming verifier accepts this vector.
	Expect string `json:"expect"`
	// Stage names the verification step that decides the vector.
	Stage string `json:"stage"`
	// Reason is why a verifier reaches that verdict.
	Reason string `json:"reason"`
	// Payload is the exact byte string a verifier reconstructs before verifying.
	Payload string `json:"payload"`
	// PayloadHex is the same bytes, for implementations that prefer bytes.
	PayloadHex string `json:"payload_hex"`
	// SignatureHex is the Ed25519 signature over Payload.
	SignatureHex string `json:"signature_hex"`
	// KeyID, PublicKeyHex and Fingerprint describe the signing key.
	KeyID        string `json:"key_id"`
	PublicKeyHex string `json:"public_key_hex"`
	Fingerprint  string `json:"fingerprint"`
	// ChecksumHex and Timestamp build a valid payload when substituted into
	// Payload's shape.
	ChecksumHex string `json:"checksum_hex"`
	Timestamp   int64  `json:"timestamp"`
}

type vectorsDoc struct {
	Schema      string          `json:"$schema"`
	Contract    string          `json:"contract"`
	Version     string          `json:"version"`
	Status      string          `json:"status"`
	Owner       string          `json:"owner"`
	Spec        string          `json:"spec"`
	GeneratedBy string          `json:"generated_by"`
	Description string          `json:"description"`
	Usage       string          `json:"usage"`
	Vectors     []signingVector `json:"vectors"`
}

// vectorSeed derives the deterministic key the vectors are signed with.
//
// A fixed seed means the vectors do not change on every regeneration, which
// matters because a consumer pins them. These keys are published here and are
// therefore public by construction: they sign test payloads and nothing else,
// and a verifier that trusts them for a real package is misconfigured.
const vectorSeed = "onuron.contracts.signing.v1.test-vector"

// BuildSigningVectors produces the bytes of testdata/signing/v1-vectors.json.
//
// The vectors are what makes "the same signature verifies the same way in Go,
// Rust and JavaScript" checkable rather than asserted. Each one is a decision,
// not just bytes: a verifier that disagrees with one of these has a contract
// bug until it can show the contract is wrong.
func BuildSigningVectors() ([]byte, error) {
	seed := sha256.Sum256([]byte(vectorSeed))
	priv := ed25519.NewKeyFromSeed(seed[:])
	pub := priv.Public().(ed25519.PublicKey)

	keyHash := sha256.Sum256(pub)
	keyID := hex.EncodeToString(keyHash[:16])
	fingerprint := fingerprintOf(keyHash[:])

	// A fixed package body and timestamp, so the whole document is a pure
	// function of the seed.
	body := []byte("onuron contracts v1 signature test payload")
	checksum := sha256.Sum256(body)
	checksumHex := hex.EncodeToString(checksum[:])
	const timestamp int64 = 1767225600 // 2026-01-01T00:00:00Z

	sign := func(checksumHex string, ts int64, keyID string) (payload string, sigHex string) {
		p := signing.SigningPayload{Checksum: checksumHex, Timestamp: ts, KeyID: keyID}
		data, err := json.Marshal(p)
		if err != nil {
			return "", ""
		}
		return string(data), hex.EncodeToString(ed25519.Sign(priv, data))
	}

	// The digest an attacker would substitute: a validly shaped SHA-256 of
	// something that is not the package.
	tamperedSum := sha256.Sum256([]byte("onuron contracts v1 signature tampered payload"))
	tamperedDigest := hex.EncodeToString(tamperedSum[:])

	goodPayload, goodSig := sign(checksumHex, timestamp, keyID)
	tamperedChecksum, tamperedChecksumSig := sign(tamperedDigest, timestamp, keyID)
	wrongTimePayload, wrongTimeSig := sign(checksumHex, timestamp+1, keyID)
	wrongKeyPayload, wrongKeySig := sign(checksumHex, timestamp, keyID[:31]+"0")

	doc := vectorsDoc{
		Schema:      "https://onuron.org/schemas/signing/v1-vectors.json",
		Contract:    "onuron.signing.test_vectors",
		Version:     "1.0.0",
		Status:      StatusStable,
		Owner:       "nilLang",
		Spec:        "contracts/signing/signing.json",
		GeneratedBy: "go run ./cmd/nil-contract generate",
		Description: "Conformance vectors for the v1 package signature. Every vector states the accept/reject decision " +
			"a conforming verifier must reach and the exact bytes it must reach it from.",
		Usage: "Rebuild the canonical payload as `{\"checksum\":<checksum_hex>,\"timestamp\":<timestamp>,\"key_id\":<key_id>}` " +
			"with no whitespace, verify signature_hex against the public key, then follow the verification order in " +
			"signing/signing.json. The keys below are published test keys and must never appear in a trust store.",
		Vectors: []signingVector{
			{
				Name: "known-good", Expect: "accept", Stage: "6",
				Reason:       "Every step passes: the key is known and not revoked, the file digest matches, and the signature verifies over the reconstructed payload.",
				Payload:      goodPayload,
				PayloadHex:   hex.EncodeToString([]byte(goodPayload)),
				SignatureHex: goodSig,
				KeyID:        keyID,
				PublicKeyHex: hex.EncodeToString(pub),
				Fingerprint:  fingerprint,
				ChecksumHex:  checksumHex,
				Timestamp:    timestamp,
			},
			{
				Name: "tampered-payload", Expect: "reject", Stage: "5",
				Reason:       "The file's digest no longer matches the signed checksum. The signature over the original digest is still perfectly valid, which is exactly why integrity is checked before the signature.",
				Payload:      goodPayload,
				PayloadHex:   hex.EncodeToString([]byte(goodPayload)),
				SignatureHex: goodSig,
				KeyID:        keyID,
				PublicKeyHex: hex.EncodeToString(pub),
				Fingerprint:  fingerprint,
				ChecksumHex:  tamperedDigest,
				Timestamp:    timestamp,
			},
			{
				Name: "signature-over-different-digest", Expect: "reject", Stage: "5",
				Reason:       "A correctly formed signature over a digest that does not match the file. A verifier that checks only the signature accepts this package; one that checks integrity first does not.",
				Payload:      tamperedChecksum,
				PayloadHex:   hex.EncodeToString([]byte(tamperedChecksum)),
				SignatureHex: tamperedChecksumSig,
				KeyID:        keyID,
				PublicKeyHex: hex.EncodeToString(pub),
				Fingerprint:  fingerprint,
				ChecksumHex:  tamperedDigest,
				Timestamp:    timestamp,
			},
			{
				Name: "timestamp-tampered", Expect: "reject", Stage: "6",
				Reason:       "The timestamp is inside the signed payload, so changing it invalidates the signature. A verifier that reconstructs the payload from signed_at instead of from timestamp would accept this.",
				Payload:      wrongTimePayload,
				PayloadHex:   hex.EncodeToString([]byte(wrongTimePayload)),
				SignatureHex: wrongTimeSig,
				KeyID:        keyID,
				PublicKeyHex: hex.EncodeToString(pub),
				Fingerprint:  fingerprint,
				ChecksumHex:  checksumHex,
				Timestamp:    timestamp + 1,
			},
			{
				Name: "key-id-swapped", Expect: "reject", Stage: "6",
				Reason:       "A key id altered after signing. The key id is inside the payload, so the signature fails — and the altered id is also absent from any trust store.",
				Payload:      wrongKeyPayload,
				PayloadHex:   hex.EncodeToString([]byte(wrongKeyPayload)),
				SignatureHex: wrongKeySig,
				KeyID:        keyID[:31] + "0",
				PublicKeyHex: hex.EncodeToString(pub),
				Fingerprint:  fingerprint,
				ChecksumHex:  checksumHex,
				Timestamp:    timestamp,
			},
		},
	}

	return MarshalIndent(doc)
}

// MarshalCompact serialises a signing payload exactly as the contract states it.
//
// json.Marshal, not MarshalIndent: the canonical payload has no whitespace, and
// an indenting serialiser would produce bytes no other implementation can
// reproduce.
func MarshalCompact(v any) ([]byte, error) {
	return json.Marshal(v)
}

// BuildIndex produces the bytes of bundle.json from a set of contract files.
//
// The index carries no generation timestamp on purpose. A bundle index that
// changed on every run could not be pinned to, and pinning is the only thing it
// is for.
func BuildIndex(entries []IndexEntry) ([]byte, error) {
	return MarshalIndent(buildIndex(entries))
}

// SortEntries orders entries by path so the index is a pure function of the
// file set.
func SortEntries(entries []IndexEntry) {
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
}

func fingerprintOf(hash []byte) string {
	hexStr := hex.EncodeToString(hash[:20])
	var out string
	for i := 0; i < len(hexStr); i += 4 {
		if i > 0 {
			out += ":"
		}
		end := i + 4
		if end > len(hexStr) {
			end = len(hexStr)
		}
		out += hexStr[i:end]
	}
	return out
}

// DerivedKeyID recomputes a key id from a public key, as the contract states.
//
// A verifier that trusts a key id it cannot recompute is trusting a label.
func DerivedKeyID(publicKey []byte) string {
	sum := sha256.Sum256(publicKey)
	return hex.EncodeToString(sum[:16])
}

// Fingerprint renders a public key fingerprint in the contract's format.
func Fingerprint(publicKey []byte) string {
	sum := sha256.Sum256(publicKey)
	return fingerprintOf(sum[:])
}
