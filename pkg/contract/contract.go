// Package contract loads and enforces the Onuron ecosystem contract bundle.
//
// The bundle lives in the top-level contracts/ directory of this repository and
// is embedded at compile time. This package is the Go reader for it, and the
// reference for what "reading it correctly" means: every value is checked
// against the bundle, and anything the bundle does not name is refused.
//
// # Default-deny
//
// A capability, target or format version that the bundle does not declare is
// refused rather than passed through. The asymmetry is deliberate. A false
// refusal is a bug someone reports; a false acceptance is a package running with
// authority nobody granted it, and nothing reports that.
//
// # Relationship to the legacy paths
//
// pkg/bundle and pkg/nilpkg predate the bundle and remain permissive about
// fields they do not model. They are the compatibility path. New verification
// goes through this package; nothing here changes how an existing build
// behaves until a caller opts in.
package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/joysriramsarkar/nilLang/contracts"
)

// Status values a contract file may declare.
const (
	// StatusStable means a validator enforces the file today.
	StatusStable = "stable"
	// StatusProposed means the contract text exists but nothing enforces it. A
	// verifier must not reject on a proposed contract, and must not advertise
	// support for it either.
	StatusProposed = "proposed"
	// StatusDraft means the document is under discussion and is not a contract.
	StatusDraft = "draft"
)

// Index is the bundle index: which files the bundle contains and what they hash
// to. A consumer that materialises a copy verifies these checksums before using
// it, which is what makes "pin to a version" a checkable statement.
type Index struct {
	Bundle  string       `json:"bundle"`
	Version string       `json:"version"`
	Owner   string       `json:"owner"`
	Files   []IndexEntry `json:"files"`
}

// IndexEntry is one file in the bundle index.
type IndexEntry struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
	// Generated marks a file that `nil-contract generate` owns. Editing it by
	// hand is a mistake the verify command will report.
	Generated bool `json:"generated,omitempty"`
}

// Set is a loaded, checksum-verified contract bundle.
//
// The typed documents are parsed once at load rather than per call. They are
// small, and a verifier validates a package at a point where a parse failure
// should be impossible — anything reaching that code already came through a
// successful Load.
type Set struct {
	index Index
	files map[string][]byte

	vocab   *Vocabulary
	reqs    *RequirementTable
	abi     *ABI
	signing *Signing
	compat  *Compatibility
}

// embedded caches the embedded bundle.
//
// The embedded copy is verified once, lazily. A build that never touches the
// contracts pays nothing; a build that does pays for the check once.
var (
	embeddedOnce sync.Once
	embeddedSet  *Set
	embeddedErr  error
)

// Load returns the embedded contract bundle, verifying every checksum in the
// index and parsing every document on first use.
//
// An error here means the repository's own bundle is inconsistent, which is a
// defect in this repository rather than a condition a caller can handle. Call
// sites treat it as fatal.
func Load() (*Set, error) {
	embeddedOnce.Do(func() {
		embeddedSet, embeddedErr = loadFS(contracts.Files.ReadFile, "bundle.json")
		if embeddedErr != nil {
			return
		}
		embeddedErr = embeddedSet.parse()
	})
	return embeddedSet, embeddedErr
}

// MustLoad returns the embedded bundle and panics on failure.
func MustLoad() *Set {
	set, err := Load()
	if err != nil {
		panic("contract: embedded bundle is inconsistent: " + err.Error())
	}
	return set
}

// Index returns the bundle index.
func (s *Set) Index() Index { return s.index }

// File returns the raw bytes of a bundle file.
func (s *Set) File(path string) ([]byte, error) {
	data, ok := s.files[path]
	if !ok {
		return nil, fmt.Errorf("contract: %q is not in the bundle", path)
	}
	return data, nil
}

// StatusOf returns the declared status of a bundle file.
func (s *Set) StatusOf(path string) (string, error) {
	for _, entry := range s.index.Files {
		if entry.Path == path {
			return entry.Status, nil
		}
	}
	return "", fmt.Errorf("contract: %q is not in the bundle", path)
}

// loadFS reads a bundle from a filesystem and verifies its index.
//
// Verification is not optional and not skippable: a bundle whose recorded
// checksum disagrees with its bytes is exactly the state a supply-chain
// substitution produces, and reading it "for information" is how it reaches a
// decision point.
func loadFS(readFile func(string) ([]byte, error), indexPath string) (*Set, error) {
	indexBytes, err := readFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("contract: reading bundle index: %w", err)
	}

	var index Index
	if err := json.Unmarshal(indexBytes, &index); err != nil {
		return nil, fmt.Errorf("contract: parsing bundle index: %w", err)
	}
	if len(index.Files) == 0 {
		return nil, fmt.Errorf("contract: bundle index lists no files")
	}

	files := make(map[string][]byte, len(index.Files)+1)
	for _, entry := range index.Files {
		data, err := readFile(entry.Path)
		if err != nil {
			return nil, fmt.Errorf("contract: reading %s: %w", entry.Path, err)
		}
		sum := sha256.Sum256(data)
		got := hex.EncodeToString(sum[:])
		if got != entry.SHA256 {
			return nil, fmt.Errorf("contract: %s checksum mismatch: index says %s, file is %s (run `go run ./cmd/nil-contract generate`)",
				entry.Path, entry.SHA256, got)
		}
		if entry.Bytes != 0 && entry.Bytes != len(data) {
			return nil, fmt.Errorf("contract: %s size mismatch: index says %d, file is %d", entry.Path, entry.Bytes, len(data))
		}
		files[entry.Path] = data
	}
	files[indexPath] = indexBytes

	return &Set{index: index, files: files}, nil
}

// parse decodes every typed document in the bundle.
//
// A failure to parse a contract file fails the whole load. Half a contract
// bundle is worse than none: a validator that silently skips the file it could
// not read becomes a validator with a hole exactly where nobody is looking.
func (s *Set) parse() error {
	decode := func(path string, into any) error {
		data, err := s.File(path)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, into); err != nil {
			return fmt.Errorf("contract: parsing %s: %w", path, err)
		}
		return nil
	}

	if err := decode("capabilities/vocabulary.json", &s.vocab); err != nil {
		return err
	}
	if err := decode("capabilities/requirements.json", &s.reqs); err != nil {
		return err
	}
	if err := decode("abi/runtime-abi.json", &s.abi); err != nil {
		return err
	}
	if err := decode("signing/signing.json", &s.signing); err != nil {
		return err
	}
	if err := decode("compatibility/matrix.json", &s.compat); err != nil {
		return err
	}

	for _, doc := range []struct {
		status string
		name   string
	}{
		{s.vocab.Status, "capabilities/vocabulary.json"},
		{s.reqs.Status, "capabilities/requirements.json"},
		{s.abi.Status, "abi/runtime-abi.json"},
		{s.signing.Status, "signing/signing.json"},
		{s.compat.Status, "compatibility/matrix.json"},
	} {
		switch doc.status {
		case StatusStable, StatusProposed, StatusDraft:
		default:
			return fmt.Errorf("contract: %s declares status %q, which is not one of stable, proposed or draft", doc.name, doc.status)
		}
	}

	return nil
}

// Vocabulary returns the canonical capability vocabulary.
func (s *Set) Vocabulary() *Vocabulary { return s.vocab }

// Requirements returns the operation → capability requirement table.
func (s *Set) Requirements() *RequirementTable { return s.reqs }

// ABI returns the language and runtime ABI contract.
func (s *Set) ABI() *ABI { return s.abi }

// Signing returns the signing and trust contract.
func (s *Set) Signing() *Signing { return s.signing }

// Compatibility returns the backend × feature support matrix.
func (s *Set) Compatibility() *Compatibility { return s.compat }

// ManifestSchema returns the JSON Schema for a manifest format version.
//
// A version with no schema is an error rather than an empty result. There is no
// "unvalidated but accepted" mode: the compatibility policy in
// contracts/nilax/MIGRATION.md refuses an unknown format_version.
func (s *Set) ManifestSchema(formatVersion int) ([]byte, error) {
	path := fmt.Sprintf("nilax/manifest-v%d.schema.json", formatVersion)
	if _, ok := s.files[path]; !ok {
		return nil, fmt.Errorf("contract: no schema for .nilax format_version %d (this validator understands 1 and 2)", formatVersion)
	}
	return s.File(path)
}

// buildIndex produces the index for a directory of contract files.
//
// The index is derived rather than maintained, and it carries no timestamp: a
// bundle index that changed every regeneration would defeat the purpose of
// pinning to one.
func buildIndex(entries []IndexEntry) Index {
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return Index{
		Bundle:  "onuron.contracts",
		Version: BundleVersion,
		Owner:   "nilLang",
		Files:   entries,
	}
}

// BundleVersion is the version of the bundle as a whole.
//
// It moves when any member moves in a way a consumer must notice. An edit that
// only rewording leaves it alone.
const BundleVersion = "1.0.0"

// MarshalIndent is the single serialisation used for every generated contract
// file, so that regenerating one that has not changed produces identical bytes.
func MarshalIndent(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
