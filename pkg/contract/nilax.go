package contract

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Manifest is the format-independent view of a .nilax manifest.
//
// Two formats are described by the contracts — format_version 1 as implemented,
// format_version 2 as proposed — and they disagree about names. A verifier that
// had to know both shapes would implement both shapes and drift between them, so
// this view has the union's fields under the v2 names, and the reader fills them
// from whichever spelling the document used. Raw keeps the original bytes for
// the parts a signature covers.
type Manifest struct {
	// FormatVersion is the manifest format. Only 1 and 2 have schemas.
	FormatVersion int
	// AppID is app_name in v1 and app_id in v2.
	AppID string
	// Version is app_version in v1 and version in v2.
	Version string
	// Targets is always a list. v2 declares a single target; a v1 package may
	// declare several, and validating several against one device is the
	// installer's problem, stated explicitly in StageCompatibility.
	Targets []string
	// Capabilities is permissions in v1 and capabilities in v2.
	Capabilities []string
	// EntryPoint is entry_bytecode in v1 and payload.entrypoint in v2.
	EntryPoint string
	// CompilerVersion is the flat v1 field or compiler.version in v2.
	CompilerVersion string
	// RuntimeABI is empty for v1, which has no such field.
	RuntimeABI string
	// PayloadType and Backends are v2 only.
	PayloadType string
	Backends    []string
	// SourceRevision is v2 only.
	SourceRevision string
	// PayloadChecksums is v2 only. Its absence in v1 is why stage 3 has
	// nothing to check for a v1 package beyond the whole-archive digest.
	PayloadChecksums map[string]string
	// SignatureHex is the bare v1 signature string.
	SignatureHex string

	// Raw is the manifest exactly as read, for canonicalisation.
	Raw map[string]json.RawMessage
	// Present is the set of top-level member names, so that an unknown member
	// is distinguishable from a known member that is absent.
	Present map[string]bool
}

// semverRE matches a semantic version with optional prerelease and build
// metadata. Written out rather than taken from a dependency because the rule is
// part of the published contract: a second implementation must reach the same
// verdict on "1.0" and "1.0.0-".
var semverRE = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-([0-9A-Za-z.-]+))?(\+([0-9A-Za-z.-]+))?$`)

// appIDRE matches a reverse-DNS application id, which is what format_version 2
// requires.
var appIDRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

// appNameV1RE matches a format_version 1 app_name.
//
// Looser than the v2 rule on purpose. v1 is the format already in the field and
// in the store — "roundtrip-pkg" and "notes" are valid v1 names and packages
// carrying them are already published. Tightening v1 would break every device
// that has one, which is the migration problem contracts/nilax/MIGRATION.md
// exists to sequence.
var appNameV1RE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// archivePathRE matches a safe, relative archive entry path.
//
// The point is not to ban odd characters but to ban anything that could escape
// the extraction directory or name a device: absolute paths, parent traversal,
// Windows drive letters and backslashes all fail here.
var archivePathRE = regexp.MustCompile(`^[A-Za-z0-9._][A-Za-z0-9._/-]*$`)

// UnsafePath reports whether an archive path could escape its extraction
// directory.
//
// The character class in archivePathRE permits '.' and '/', so "../.." is a
// legal string by that test alone. Traversal is therefore checked as a segment
// rather than as a substring: a file legitimately named "a..b" is fine, and
// "a/../b" is not, and only a segment comparison tells those apart.
func UnsafePath(p string) bool {
	switch {
	case p == "":
		return true
	case strings.HasPrefix(p, "/"):
		return true
	case strings.Contains(p, "\\"):
		return true
	case len(p) >= 2 && p[1] == ':':
		return true
	case !archivePathRE.MatchString(p):
		return true
	}
	for _, segment := range strings.Split(p, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

// archiveLimits bound what a validator will read from an untrusted archive.
//
// The limits exist so that "validate this package" cannot be turned into "read
// four gigabytes into memory". They are contract, not implementation detail:
// a consumer enforcing the contract needs the same numbers.
const (
	// MaxArchiveEntries is the largest entry count accepted.
	MaxArchiveEntries = 10000
	// MaxArchiveBytes is the largest total uncompressed payload accepted.
	MaxArchiveBytes = 256 << 20
	// MaxEntryBytes is the largest single uncompressed entry accepted.
	MaxEntryBytes = 64 << 20
)

// ArchiveEntry is one entry in a .nilax archive.
type ArchiveEntry struct {
	Path string
	Size int64
	// SHA256 is the entry digest when the caller computed it. Stage 3 checks it
	// only for entries the manifest declares, so a caller that has not hashed
	// anything still gets stages 1, 2, 4, 5 and 6.
	SHA256 string
}

// Archive is the subset of a .nilax archive that validation needs.
type Archive interface {
	Entries() ([]ArchiveEntry, error)
}

// FileArchive is an Archive backed by in-memory entries.
type FileArchive []ArchiveEntry

// Entries implements Archive.
func (f FileArchive) Entries() ([]ArchiveEntry, error) { return f, nil }

// ZipArchive reads a .nilax file's entry table.
//
// It reads the central directory only — names, sizes, CRCs — and never the entry
// bodies. That is enough for stages 1 and 2 and it means validating an archive
// cannot itself become the decompression bomb.
type ZipArchive struct{ File string }

// Entries implements Archive.
func (z ZipArchive) Entries() ([]ArchiveEntry, error) {
	r, err := zip.OpenReader(z.File)
	if err != nil {
		return nil, fmt.Errorf("contract: opening archive: %w", err)
	}
	defer r.Close()

	out := make([]ArchiveEntry, 0, len(r.File))
	for _, f := range r.File {
		out = append(out, ArchiveEntry{Path: f.Name, Size: int64(f.UncompressedSize64)})
	}
	return out, nil
}

// ReadZipEntry returns one entry's bytes, refusing an unsafe path.
//
// Callers reach this after stage 1 has passed; the check is repeated anyway
// because a second path to the filesystem should not depend on a first check
// having happened somewhere else.
func ReadZipEntry(zf *zip.File) ([]byte, error) {
	if UnsafePath(zf.Name) {
		return nil, fmt.Errorf("contract: unsafe archive path %q", zf.Name)
	}
	rc, err := zf.Open()
	if err != nil {
		return nil, fmt.Errorf("contract: opening %s: %w", zf.Name, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, MaxEntryBytes+1))
	if err != nil {
		return nil, fmt.Errorf("contract: reading %s: %w", zf.Name, err)
	}
	if int64(len(data)) > MaxEntryBytes {
		return nil, fmt.Errorf("contract: entry %s exceeds the %d byte limit", zf.Name, MaxEntryBytes)
	}
	return data, nil
}

// ParseManifest decodes a manifest document into the common view.
//
// It refuses a document whose format_version it does not know, rather than
// returning a partially populated view: a verifier that read the fields it
// recognised out of a future format would be making a compatibility decision
// nobody authorised it to make.
func (s *Set) ParseManifest(data []byte) (*Manifest, error) {
	var raw map[string]json.RawMessage
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("contract: manifest is not a JSON object: %w", err)
	}

	var formatVersion int
	if err := json.Unmarshal(raw["format_version"], &formatVersion); err != nil {
		return nil, fmt.Errorf("contract: manifest has no integer format_version")
	}

	switch formatVersion {
	case 1:
		return parseManifestV1(raw)
	case 2:
		return parseManifestV2(raw)
	default:
		return nil, fmt.Errorf("contract: unknown .nilax format_version %d (this validator understands 1 and 2)", formatVersion)
	}
}

// parseManifestV1 reads the format a `nil build` produces today.
func parseManifestV1(raw map[string]json.RawMessage) (*Manifest, error) {
	m := &Manifest{FormatVersion: 1, Raw: raw, Present: presentKeys(raw)}

	var doc struct {
		FormatVersion   int               `json:"format_version"`
		AppName         string            `json:"app_name"`
		AppVersion      string            `json:"app_version"`
		Author          string            `json:"author"`
		Description     string            `json:"description"`
		EntryBytecode   string            `json:"entry_bytecode"`
		Targets         []string          `json:"targets"`
		CreatedAt       string            `json:"created_at"`
		CompilerVersion string            `json:"compiler_version"`
		Dependencies    map[string]string `json:"dependencies"`
		Permissions     []string          `json:"permissions"`
		Metadata        map[string]string `json:"metadata"`
		Signature       string            `json:"signature"`
	}
	if err := json.Unmarshal(dataOf(raw), &doc); err != nil {
		return nil, fmt.Errorf("contract: reading format_version 1 manifest: %w", err)
	}

	m.AppID = doc.AppName
	m.Version = doc.AppVersion
	m.Targets = doc.Targets
	m.Capabilities = doc.Permissions
	m.EntryPoint = doc.EntryBytecode
	m.CompilerVersion = doc.CompilerVersion
	m.SignatureHex = doc.Signature
	return m, nil
}

// parseManifestV2 reads the proposed format.
func parseManifestV2(raw map[string]json.RawMessage) (*Manifest, error) {
	var doc struct {
		FormatVersion int    `json:"format_version"`
		AppID         string `json:"app_id"`
		Version       string `json:"version"`
		Payload       struct {
			Type       string   `json:"type"`
			Entrypoint string   `json:"entrypoint"`
			Backends   []string `json:"backends"`
		} `json:"payload"`
		SourceRevision string `json:"source_revision"`
		Compiler       struct {
			Name     string `json:"name"`
			Version  string `json:"version"`
			Revision string `json:"revision"`
		} `json:"compiler"`
		RuntimeABI       string            `json:"runtime_abi"`
		Target           string            `json:"target"`
		Capabilities     []string          `json:"capabilities"`
		Dependencies     []json.RawMessage `json:"dependencies"`
		PayloadChecksums map[string]string `json:"payload_checksums"`
		Signature        *struct {
			Algorithm    string   `json:"algorithm"`
			KeyID        string   `json:"key_id"`
			SignedFields []string `json:"signed_fields"`
			Signature    string   `json:"signature"`
		} `json:"signature"`
	}
	if err := json.Unmarshal(dataOf(raw), &doc); err != nil {
		return nil, fmt.Errorf("contract: reading format_version 2 manifest: %w", err)
	}

	m := &Manifest{
		FormatVersion:    2,
		AppID:            doc.AppID,
		Version:          doc.Version,
		EntryPoint:       doc.Payload.Entrypoint,
		PayloadType:      doc.Payload.Type,
		Backends:         doc.Payload.Backends,
		SourceRevision:   doc.SourceRevision,
		CompilerVersion:  doc.Compiler.Version,
		RuntimeABI:       doc.RuntimeABI,
		Capabilities:     doc.Capabilities,
		PayloadChecksums: doc.PayloadChecksums,
		Raw:              raw,
		Present:          presentKeys(raw),
	}
	if doc.Target != "" {
		m.Targets = []string{doc.Target}
	}
	if doc.Signature != nil && doc.Signature.Signature != "" {
		m.SignatureHex = doc.Signature.Signature
	}
	return m, nil
}

// dataOf re-serialises the raw map so that a typed struct can be filled from
// it. The original document's member names are what matter here; byte-exact
// preservation for signature purposes uses Raw and the canonicalisation rules in
// signing/signing.json.
func dataOf(raw map[string]json.RawMessage) []byte {
	data, err := json.Marshal(raw)
	if err != nil {
		return []byte("{}")
	}
	return data
}

func presentKeys(raw map[string]json.RawMessage) map[string]bool {
	out := make(map[string]bool, len(raw))
	for k := range raw {
		out[k] = true
	}
	return out
}

// TrustStore supplies the public keys a verifier trusts.
type TrustStore interface {
	// Key returns a trusted key by id. The bool reports whether the id is known.
	Key(keyID string) (TrustedKey, bool)
}

// TrustedKey is public verification material.
type TrustedKey struct {
	KeyID        string
	PublicKeyHex string
	Revoked      bool
	NotBefore    string
	NotAfter     string
	Algorithm    string
}

// RequireTrust controls how hard the trust stage pushes.
//
// It exists because the same verifier code serves two callers with genuinely
// different jobs: a developer running a package locally, who wants the loudest
// possible report, and a store ingest job, which must refuse anything unsigned.
// Defaulting that decision at the call site rather than in the code means a new
// caller cannot accidentally get the permissive one by omission.
type RequireTrust int

const (
	// TrustIfPresent verifies a signature when the package carries one and
	// accepts an unsigned package.
	TrustIfPresent RequireTrust = iota
	// TrustRequired refuses a package with no signature or an untrusted key.
	TrustRequired
)

// ValidateOptions are the inputs to a validation pass that the manifest itself
// does not carry.
type ValidateOptions struct {
	// Archive is optional. Without it, stage 1 is skipped and reported as such.
	Archive Archive
	// Trust selects the trust posture. The zero value is TrustIfPresent.
	Trust RequireTrust
	// TrustStore is required when Trust is TrustRequired or when the package
	// carries a signature.
	TrustStore TrustStore
	// RuntimeABI is the ABI the validating runtime implements. When empty the
	// compatibility stage checks targets and backends but not the ABI.
	RuntimeABI string
	// SupportedBackends is what the validating runtime can execute. Empty means
	// backend support is not checked, which a store ingest does and a device
	// install does not.
	SupportedBackends []string
	// AllowExperimentalTargets lifts the publishable-target restriction. Used
	// by development builds, never by a store.
	AllowExperimentalTargets bool
}

// Validate runs the seven stages and returns every violation it found.
//
// It returns violations rather than an error because a caller usually wants to
// report all of them: a package that is both unsigned and built for a target the
// device cannot run should say so in one pass, not one complaint at a time.
func (s *Set) Validate(m *Manifest, opts ValidateOptions) Violations {
	var vs Violations

	// Stage 1 — archive safety. First, because everything after it assumes the
	// entries it is about to name are real files inside the archive.
	var entries []ArchiveEntry
	haveArchive := opts.Archive != nil
	if haveArchive {
		var err error
		entries, err = opts.Archive.Entries()
		if err != nil {
			vs.Add(StageArchive, "archive", "", err.Error())
			haveArchive = false
		}
		if haveArchive {
			vs = append(vs, s.stageArchive(entries)...)
		}
	}

	// Stage 2 — identity.
	vs = append(vs, s.stageIdentity(m, haveArchive)...)

	// Stage 3 — integrity.
	vs = append(vs, s.stageIntegrity(m, entries, haveArchive)...)

	// Stage 4 — trust.
	vs = append(vs, s.stageTrust(m, opts)...)

	// Stage 5 — compatibility.
	vs = append(vs, s.stageCompatibility(m, opts)...)

	// Stage 6 — capabilities.
	vs = append(vs, s.CheckCapabilities(m.Capabilities)...)

	// Stage 7 — install.
	vs = append(vs, s.stageInstall(m, entries, haveArchive)...)

	return vs
}

// ValidateJSON parses and validates a manifest document in one call.
func (s *Set) ValidateJSON(data []byte, opts ValidateOptions) (*Manifest, Violations) {
	m, err := s.ParseManifest(data)
	if err != nil {
		return nil, Violations{{
			Stage:  StageIdentity,
			Field:  "format_version",
			Reason: err.Error(),
		}}
	}
	return m, s.Validate(m, opts)
}

// stageArchive checks path safety, duplicates and size limits.
func (s *Set) stageArchive(entries []ArchiveEntry) Violations {
	var vs Violations
	if len(entries) > MaxArchiveEntries {
		vs.Add(StageArchive, "archive", fmt.Sprint(len(entries)),
			fmt.Sprintf("archive holds %d entries, over the %d entry limit", len(entries), MaxArchiveEntries))
		return vs
	}

	seen := make(map[string]bool, len(entries))
	var total int64
	for _, e := range entries {
		switch {
		case e.Path == "":
			vs.Add(StageArchive, e.Path, e.Path, "empty archive entry name")
		case strings.HasPrefix(e.Path, "/"):
			vs.Add(StageArchive, e.Path, e.Path, "absolute path in archive")
		case strings.Contains(e.Path, "\\"):
			vs.Add(StageArchive, e.Path, e.Path, "backslash in archive path")
		case len(e.Path) >= 2 && e.Path[1] == ':':
			vs.Add(StageArchive, e.Path, e.Path, "drive-letter path in archive")
		case !archivePathRE.MatchString(e.Path):
			vs.Add(StageArchive, e.Path, e.Path, "archive path contains characters outside [A-Za-z0-9._/-]")
		case hasTraversal(e.Path):
			vs.Add(StageArchive, e.Path, e.Path, "archive path escapes the extraction directory")
		}
		if seen[e.Path] {
			vs.Add(StageArchive, e.Path, e.Path, "duplicate archive entry")
		}
		seen[e.Path] = true

		if e.Size > MaxEntryBytes {
			vs.Add(StageArchive, e.Path, fmt.Sprint(e.Size),
				fmt.Sprintf("entry exceeds the %d byte limit", MaxEntryBytes))
		}
		total += e.Size
	}
	if total > MaxArchiveBytes {
		vs.Add(StageArchive, "archive", fmt.Sprint(total),
			fmt.Sprintf("archive expands to %d bytes, over the %d byte limit", total, MaxArchiveBytes))
	}
	return vs
}

// hasTraversal reports whether a path contains a parent-directory segment.
func hasTraversal(p string) bool {
	for _, segment := range strings.Split(strings.ReplaceAll(p, "\\", "/"), "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

// stageIdentity checks the schema-versioned identity fields.
func (s *Set) stageIdentity(m *Manifest, haveArchive bool) Violations {
	var vs Violations

	// A validator that cannot read the schema for the version in front of it
	// cannot claim to have checked that version.
	if _, err := s.ManifestSchema(m.FormatVersion); err != nil {
		vs.Add(StageIdentity, "format_version", fmt.Sprint(m.FormatVersion), err.Error())
	}

	if m.AppID == "" {
		vs.Add(StageIdentity, "app_id", m.AppID, "missing application identity")
	} else if m.FormatVersion >= 2 && !appIDRE.MatchString(m.AppID) {
		vs.Add(StageIdentity, "app_id", m.AppID, "a format_version 2 application id must be a reverse-DNS name")
	} else if m.FormatVersion == 1 && !appNameV1RE.MatchString(m.AppID) {
		vs.Add(StageIdentity, "app_name", m.AppID, "an application name must start alphanumeric and contain only letters, digits, '.', '_' or '-'")
	}

	if m.Version == "" {
		vs.Add(StageIdentity, "version", m.Version, "missing version")
	} else if !semverRE.MatchString(m.Version) {
		vs.Add(StageIdentity, "version", m.Version, "version is not a semantic version")
	}

	if len(m.Targets) == 0 {
		vs.Add(StageIdentity, "target", "", "a package must declare at least one target")
	}

	if m.EntryPoint == "" {
		vs.Add(StageIdentity, "entrypoint", m.EntryPoint, "missing entry point")
	} else if UnsafePath(m.EntryPoint) {
		vs.Add(StageIdentity, "entrypoint", m.EntryPoint, "entry point is not a safe archive path")
	}

	// An unknown member in a manifest we sign is a mistake worth hearing about
	// rather than a field to ignore. A v1 reader that silently dropped
	// `runtime_abi` would report the package as compatible when it is not.
	for _, key := range sortedKeys(m.Present) {
		if !manifestMembers[m.FormatVersion][key] {
			vs.Add(StageIdentity, key, "", "member is not part of format_version "+fmt.Sprint(m.FormatVersion))
		}
	}

	for _, required := range requiredMembers[m.FormatVersion] {
		if !m.Present[required] {
			vs.Add(StageIdentity, required, "", "required member is missing")
		}
	}

	return vs
}

// stageIntegrity checks declared payload digests.
func (s *Set) stageIntegrity(m *Manifest, entries []ArchiveEntry, haveArchive bool) Violations {
	var vs Violations

	if len(m.PayloadChecksums) == 0 {
		if m.FormatVersion == 2 {
			vs.Add(StageIntegrity, "payload_checksums", "",
				"a format_version 2 package must declare a digest for every payload file")
		}
		// A v1 package has no per-file digests. That is a recorded gap, not a
		// refusal: v1 packages already signed keep installing.
		return vs
	}

	declared := make(map[string]bool, len(m.PayloadChecksums))
	for _, name := range sortedKeys(toBoolMap(m.PayloadChecksums)) {
		digest := m.PayloadChecksums[name]
		declared[name] = true

		if !strings.HasPrefix(digest, "sha256:") || len(digest) != len("sha256:")+64 {
			vs.Add(StageIntegrity, "payload_checksums."+name, digest,
				"digest must be sha256: followed by 64 lowercase hex characters")
			continue
		}
		if UnsafePath(name) {
			vs.Add(StageIntegrity, "payload_checksums."+name, name, "checksum key is not a safe archive path")
			continue
		}
		if !haveArchive {
			continue
		}
		if entry, ok := findEntry(entries, name); ok && entry.SHA256 != "" {
			got := "sha256:" + entry.SHA256
			if got != digest {
				vs.Add(StageIntegrity, "payload_checksums."+name, got,
					"payload does not match its declared digest: the file was modified after signing")
			}
		}
	}

	if haveArchive {
		// A payload file the manifest does not mention is unaccounted for. For
		// v2 that is a failure: the whole point of per-file digests is that
		// every payload file has one.
		for _, e := range entries {
			if e.Path == "manifest.json" || declared[e.Path] {
				continue
			}
			if strings.HasSuffix(e.Path, "/") {
				continue
			}
			vs.Add(StageIntegrity, e.Path, "", "archive entry has no digest in payload_checksums")
		}
	}

	return vs
}

// stageTrust checks signer, key and signature.
func (s *Set) stageTrust(m *Manifest, opts ValidateOptions) Violations {
	var vs Violations

	if m.SignatureHex == "" {
		if opts.Trust == TrustRequired {
			vs.Add(StageTrust, "signature", "", "package is unsigned and this verifier requires a trusted signature")
		}
		return vs
	}

	if len(m.SignatureHex) != 128 || !isLowerHex(m.SignatureHex) {
		vs.Add(StageTrust, "signature", m.SignatureHex,
			"an Ed25519 signature is 128 lowercase hex characters")
		return vs
	}
	sigBytes, err := hex.DecodeString(m.SignatureHex)
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		vs.Add(StageTrust, "signature", m.SignatureHex,
			"signature is not 64 bytes of Ed25519 signature")
		return vs
	}

	if opts.TrustStore == nil {
		vs.Add(StageTrust, "signature", "",
			"package carries a signature but no trust store was supplied, so it cannot be checked")
		return vs
	}

	// The key id lives in the signature record for v2; v1 carries a bare
	// signature with no key binding at all.
	keyID := m.keyID()
	if keyID == "" {
		if opts.Trust == TrustRequired {
			// A v1 embedded signature names no key and defines no signed
			// bytes, so there is nothing to verify it against. Presence
			// alone must never satisfy a required-trust decision.
			vs.Add(StageTrust, "signature", "",
				"format_version 1 embedded signature is unverifiable: it names no key; use a sidecar .sig or a format_version 2 signature")
		}
		// Under TrustIfPresent the legacy mark is reported nowhere and
		// passes: local development packages predate verifiable signatures.
		return vs
	}
	tk, ok := opts.TrustStore.Key(keyID)
	if !ok {
		vs.Add(StageTrust, "key_id", keyID, "signing key is not in the trust store")
		return vs
	}
	if tk.Revoked {
		vs.Add(StageTrust, "key_id", keyID, "signing key has been revoked")
		return vs
	}
	if m.FormatVersion == 2 {
		if err := verifyManifestSignatureV2(m, tk, sigBytes); err != nil {
			vs.Add(StageTrust, "signature", "", err.Error())
			return vs
		}
	}
	return vs
}

// v2SignatureRequiredMembers must all be covered by signature.signed_fields.
// A signer that leaves one out is asking the verifier to trust an uncovered
// claim, which the contract refuses.
var v2SignatureRequiredMembers = []string{
	"format_version", "app_id", "version", "payload", "source_revision",
	"compiler", "runtime_abi", "target", "capabilities", "dependencies",
	"payload_checksums",
}

// isLowerHex reports whether s is all lowercase hex digits.
func isLowerHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return len(s) > 0
}

// verifyManifestSignatureV2 checks a format_version 2 manifest signature per
// contracts/signing/signing.json manifest_signature_v2: canonical bytes are the
// manifest with object keys sorted, no insignificant whitespace, and the
// signature member removed, restricted to the listed signed_fields.
func verifyManifestSignatureV2(m *Manifest, tk TrustedKey, sig []byte) error {
	var doc struct {
		Algorithm    string   `json:"algorithm"`
		KeyID        string   `json:"key_id"`
		SignedFields []string `json:"signed_fields"`
		Signature    string   `json:"signature"`
	}
	rawSig, ok := m.Raw["signature"]
	if !ok {
		return fmt.Errorf("signature record is missing")
	}
	if err := json.Unmarshal(rawSig, &doc); err != nil {
		return fmt.Errorf("signature record is not readable: %v", err)
	}
	if doc.Algorithm != "" && doc.Algorithm != "Ed25519" {
		// Unknown algorithms are refused, never skipped.
		return fmt.Errorf("unsupported signature algorithm %q", doc.Algorithm)
	}
	if len(doc.SignedFields) == 0 {
		return fmt.Errorf("signature names no signed fields")
	}
	for _, f := range doc.SignedFields {
		if f == "signature" || f == "metadata" {
			return fmt.Errorf("signed_fields must not cover %q", f)
		}
	}
	covered := make(map[string]bool, len(doc.SignedFields))
	for _, f := range doc.SignedFields {
		covered[f] = true
	}
	for _, req := range v2SignatureRequiredMembers {
		if !covered[req] {
			return fmt.Errorf("signed_fields omits required member %q", req)
		}
	}

	// Recompute the key id when it has the derived shape (32 lowercase hex):
	// a swapped key id on a different key is refused as a tampered store.
	// Friendly aliases such as "onuron-team" predate derivation and skip this.
	if len(tk.KeyID) == 32 && isLowerHex(tk.KeyID) {
		pub, err := hex.DecodeString(tk.PublicKeyHex)
		if err != nil || len(pub) != ed25519.PublicKeySize {
			return fmt.Errorf("trust store holds a malformed public key for %q", tk.KeyID)
		}
		sum := sha256.Sum256(pub)
		if hex.EncodeToString(sum[:16]) != tk.KeyID {
			return fmt.Errorf("key id %q does not match the stored public key", tk.KeyID)
		}
	}

	pubBytes, err := hex.DecodeString(tk.PublicKeyHex)
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("trust store holds a malformed public key for %q", tk.KeyID)
	}

	canonical, err := canonicalSignedBytes(m.Raw, doc.SignedFields)
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(pubBytes), canonical, sig) {
		return fmt.Errorf("Ed25519 signature does not verify over the manifest's signed fields")
	}
	return nil
}

// canonicalSignedBytes serialises exactly the listed manifest members with
// keys sorted and no insignificant whitespace. encoding/json emits map keys in
// lexicographic byte order, which is the canonicalisation the contract pins.
func canonicalSignedBytes(raw map[string]json.RawMessage, fields []string) ([]byte, error) {
	picked := make(map[string]json.RawMessage, len(fields))
	for _, f := range fields {
		v, ok := raw[f]
		if !ok {
			return nil, fmt.Errorf("signed field %q is absent from the manifest", f)
		}
		var compacted bytes.Buffer
		if err := json.Compact(&compacted, v); err != nil {
			return nil, fmt.Errorf("signed field %q is not valid JSON", f)
		}
		picked[f] = json.RawMessage(append([]byte(nil), compacted.Bytes()...))
	}
	out, err := json.Marshal(picked)
	if err != nil {
		return nil, fmt.Errorf("canonicalising signed fields: %v", err)
	}
	return out, nil
}

// stageCompatibility checks ABI, target and backend support.
func (s *Set) stageCompatibility(m *Manifest, opts ValidateOptions) Violations {
	var vs Violations

	for _, target := range m.Targets {
		if opts.AllowExperimentalTargets {
			if _, ok := s.abi.TargetStatus(target); !ok {
				vs.Add(StageCompatibility, "target", target, "target is not declared in the runtime ABI")
			}
			continue
		}
		if err := s.SupportsTarget(target); err != nil {
			vs.Add(StageCompatibility, "target", target, err.Error())
		}
	}

	if len(opts.SupportedBackends) > 0 {
		supported := make(map[string]bool, len(opts.SupportedBackends))
		for _, b := range opts.SupportedBackends {
			supported[b] = true
		}
		for _, backend := range m.Backends {
			if !supported[backend] {
				vs.Add(StageCompatibility, "payload.backends", backend,
					"this runtime cannot execute that backend")
			}
		}
	}

	for _, backend := range m.Backends {
		if _, ok := s.abi.BackendStatus(backend); !ok {
			vs.Add(StageCompatibility, "payload.backends", backend,
				"backend is not declared in the runtime ABI")
		}
	}

	if m.RuntimeABI != "" && opts.RuntimeABI != "" {
		if !s.abiSatisfies(m.RuntimeABI, opts.RuntimeABI) {
			vs.Add(StageCompatibility, "runtime_abi", m.RuntimeABI,
				fmt.Sprintf("this runtime implements ABI %s, which is outside the package's requirement", opts.RuntimeABI))
		}
	}

	return vs
}

// abiSatisfies answers whether a runtime ABI satisfies a package's requirement,
// under the contract's stated policy: same major line, and no older than the
// package asks for.
//
// The comparison is written out rather than delegated to a semver library
// because it is a published rule and a second implementation must reach the same
// verdict on "1.10" and "1.9".
func (s *Set) abiSatisfies(required, provided string) bool {
	reqMajor, reqMinor, ok := splitABI(required)
	if !ok {
		return false
	}
	provMajor, provMinor, ok := splitABI(provided)
	if !ok {
		return false
	}
	compatible := s.abi.RuntimeABI.CompatibleRange
	wantMajor, wantMinor, ok := splitABI(strings.TrimPrefix(compatible, "^"))
	if !ok {
		return false
	}
	if reqMajor != wantMajor || provMajor != wantMajor {
		return false
	}
	_ = reqMinor
	if provMinor < wantMinor {
		return false
	}
	return provMinor >= 0
}

func splitABI(v string) (major, minor int, ok bool) {
	parts := strings.SplitN(strings.TrimSpace(v), ".", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	var ma, mi int
	if _, err := fmt.Sscanf(parts[0], "%d", &ma); err != nil {
		return 0, 0, false
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &mi); err != nil {
		return 0, 0, false
	}
	return ma, mi, true
}

// stageInstall checks what an installer would act on.
func (s *Set) stageInstall(m *Manifest, entries []ArchiveEntry, haveArchive bool) Violations {
	var vs Violations
	if !haveArchive {
		return vs
	}
	if m.EntryPoint == "" {
		return vs
	}
	if _, ok := findEntry(entries, m.EntryPoint); !ok {
		vs.Add(StageInstall, m.EntryPoint, "",
			"the declared entry point is not present in the archive")
	}
	return vs
}

// keyID returns the signing key id for either manifest format.
func (m *Manifest) keyID() string {
	if m.FormatVersion == 2 {
		var doc struct {
			Signature *struct {
				KeyID string `json:"key_id"`
			} `json:"signature"`
		}
		if err := json.Unmarshal(dataOf(m.Raw), &doc); err == nil && doc.Signature != nil {
			return doc.Signature.KeyID
		}
		return ""
	}
	return ""
}

func findEntry(entries []ArchiveEntry, name string) (ArchiveEntry, bool) {
	for _, e := range entries {
		if e.Path == name {
			return e, true
		}
	}
	return ArchiveEntry{}, false
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func toBoolMap(m map[string]string) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}

// manifestMembers lists the top-level members each format version defines.
//
// Kept here rather than read out of the JSON Schema because the validator must
// not depend on a schema engine it does not ship, and because a member list
// read at runtime is one more thing that can be edited without a code change.
// TestManifestSchemaAgreesWithCode compares this against the published schema.
var manifestMembers = map[int]map[string]bool{
	1: setOf(
		"format_version", "app_name", "app_version", "author", "description",
		"entry_bytecode", "targets", "created_at", "compiler_version",
		"dependencies", "permissions", "metadata", "signature",
	),
	2: setOf(
		"format_version", "app_id", "version", "payload", "source_revision",
		"compiler", "runtime_abi", "target", "capabilities", "dependencies",
		"payload_checksums", "signature", "metadata",
	),
}

// requiredMembers lists what each format version cannot do without.
var requiredMembers = map[int][]string{
	1: {"format_version", "app_name", "app_version", "entry_bytecode", "targets", "created_at", "compiler_version"},
	2: {"format_version", "app_id", "version", "payload", "source_revision", "compiler", "runtime_abi", "target", "capabilities", "dependencies", "payload_checksums"},
}

func setOf(names ...string) map[string]bool {
	out := make(map[string]bool, len(names))
	for _, n := range names {
		out[n] = true
	}
	return out
}

// OpenZipManifest reads manifest.json out of a .nilax file and parses it.
//
// It exists so that a verifier does not have to reimplement "which entry holds
// the manifest" and get the answer wrong in a way that happens to look like a
// tampered package.
func OpenZipManifest(path string) (*Manifest, []ArchiveEntry, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, nil, fmt.Errorf("contract: opening %s: %w", path, err)
	}
	defer r.Close()

	var entries []ArchiveEntry
	var manifest *Manifest
	for _, f := range r.File {
		entries = append(entries, ArchiveEntry{Path: f.Name, Size: int64(f.UncompressedSize64)})
		if f.Name != "manifest.json" {
			continue
		}
		data, err := ReadZipEntry(f)
		if err != nil {
			return nil, nil, err
		}
		set, err := Load()
		if err != nil {
			return nil, nil, err
		}
		manifest, err = set.ParseManifest(data)
		if err != nil {
			return nil, entries, err
		}
	}
	if manifest == nil {
		return nil, entries, fmt.Errorf("contract: %s has no manifest.json", path)
	}
	return manifest, entries, nil
}

// sha256Hex returns the SHA-256 of data as lowercase hex.
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// FileDigest returns the SHA-256 of a file, as lowercase hex.
func FileDigest(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}
