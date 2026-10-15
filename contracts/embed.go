// Package contracts embeds the Onuron ecosystem contract bundle.
//
// The bundle is the machine-readable half of the inter-repo contracts. The
// files in this directory are the single editable copy: consumers import them or
// copy them against a pinned reference and verify the checksums in bundle.json,
// rather than maintaining a second definition that drifts.
//
// See contracts/README.md for the change policy and contracts/nilax/MIGRATION.md
// for the manifest version policy.
package contracts

import "embed"

// Files is the embedded contract bundle.
//
// The file list is explicit rather than a directory wildcard so that adding a
// document to contracts/ is a deliberate act that also updates this list — the
// alternative is a file that ships to every consumer without being in the
// checksummed index.
//
//go:embed bundle.json
//go:embed capabilities/vocabulary.json
//go:embed capabilities/requirements.json
//go:embed nilax/manifest-v1.schema.json
//go:embed nilax/manifest-v2.schema.json
//go:embed abi/runtime-abi.json
//go:embed signing/signing.json
//go:embed compatibility/matrix.json
//go:embed testdata/signing/v1-vectors.json
var Files embed.FS

// Paths lists every embedded contract file, in the order they appear in
// bundle.json. Generated files are included: a consumer that verifies checksums
// verifies the whole bundle, not the hand-written subset.
var Paths = []string{
	"capabilities/vocabulary.json",
	"capabilities/requirements.json",
	"nilax/manifest-v1.schema.json",
	"nilax/manifest-v2.schema.json",
	"abi/runtime-abi.json",
	"signing/signing.json",
	"compatibility/matrix.json",
	"testdata/signing/v1-vectors.json",
}

// Read returns the contents of an embedded contract file.
func Read(path string) ([]byte, error) {
	return Files.ReadFile(path)
}

// MustRead returns the contents of an embedded contract file, panicking on
// failure.
//
// Used only where a missing embedded file is a build error rather than a runtime
// condition, which is every call site here: the files are embedded at compile
// time, so absence is a broken checkout.
func MustRead(path string) []byte {
	data, err := Files.ReadFile(path)
	if err != nil {
		panic("contracts: embedded file missing: " + path)
	}
	return data
}