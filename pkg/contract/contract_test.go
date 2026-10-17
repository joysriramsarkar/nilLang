package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goodV1 is a manifest in the shape `nil build` produces today.
//
// It is written out here rather than read from a built package so that a change
// to the builder shows up as a failing test rather than silently updating the
// fixture the contract is tested against.
const goodV1 = `{
  "format_version": 1,
  "app_name": "org.onuron.notes",
  "app_version": "1.2.0",
  "author": "Onuron Team",
  "description": "Notes",
  "entry_bytecode": "bytecode/main.nabc",
  "targets": ["onuron"],
  "created_at": "2026-10-14T06:39:06Z",
  "compiler_version": "0.1.0",
  "permissions": ["stdout"],
  "metadata": {"category": "productivity"}
}`

const goodV2 = `{
  "format_version": 2,
  "app_id": "org.onuron.notes",
  "version": "1.2.0",
  "payload": {"type": "bytecode", "entrypoint": "bytecode/main.nabc", "backends": ["vm"]},
  "source_revision": "sha256:0000000000000000000000000000000000000000000000000000000000000000",
  "compiler": {"name": "NilLang", "version": "0.1.0"},
  "runtime_abi": "1.0",
  "target": "onuron",
  "capabilities": ["stdout"],
  "dependencies": [],
  "payload_checksums": {"bytecode/main.nabc": "sha256:1111111111111111111111111111111111111111111111111111111111111111"}
}`

func mustLoad(t *testing.T) *Set {
	t.Helper()
	set, err := Load()
	if err != nil {
		t.Fatalf("loading the contract bundle failed: %v", err)
	}
	return set
}

func TestBundleLoadsAndVerifies(t *testing.T) {
	set := mustLoad(t)

	if set.Index().Version != BundleVersion {
		t.Errorf("bundle version is %q, want %q", set.Index().Version, BundleVersion)
	}
	if set.Index().Owner != "nilLang" {
		t.Errorf("bundle owner is %q, want nilLang", set.Index().Owner)
	}
	if len(set.Index().Files) == 0 {
		t.Fatal("bundle index lists no files")
	}
	for _, entry := range set.Index().Files {
		if entry.SHA256 == "" || len(entry.SHA256) != 64 {
			t.Errorf("%s has no usable checksum", entry.Path)
		}
		switch entry.Status {
		case StatusStable, StatusProposed, StatusDraft:
		default:
			t.Errorf("%s declares status %q, which is not recognised", entry.Path, entry.Status)
		}
	}
}

// TestRequirementsMatchSource is the drift gate.
//
// requirements.json is a projection of compiler/capability/manifest.go. If the
// compiler gains an operation and nobody regenerates, every verifier in every
// other language keeps answering from the old table — which for a capability
// table means it answers "this operation needs nothing".
func TestRequirementsMatchSource(t *testing.T) {
	built, err := BuildRequirements()
	if err != nil {
		t.Fatalf("building the requirement table failed: %v", err)
	}
	onDisk, err := mustLoad(t).File(PathRequirements)
	if err != nil {
		t.Fatalf("reading %s failed: %v", PathRequirements, err)
	}
	if string(built) != string(onDisk) {
		t.Fatalf("%s is out of date with compiler/capability; run `go run ./cmd/nil-contract generate`", PathRequirements)
	}
}

func TestRequirementsCoverEveryKnownOperation(t *testing.T) {
	set := mustLoad(t)
	table := set.Requirements()

	for _, op := range table.Operations() {
		if _, known := table.RequirementsFor(op); !known {
			t.Errorf("operation %q is listed but RequirementsFor says it is unknown", op)
		}
	}

	// An operation the table does not know must be refused, not waved through.
	for _, op := range []string{"definitelyNotAnOperation", "writeFile ", "std.http.getv2"} {
		if _, known := table.RequirementsFor(op); known {
			t.Errorf("unknown operation %q was reported as known", op)
		}
	}
}

func TestSigningVectorsAreDeterministic(t *testing.T) {
	first, err := BuildSigningVectors()
	if err != nil {
		t.Fatalf("building signing vectors failed: %v", err)
	}
	second, err := BuildSigningVectors()
	if err != nil {
		t.Fatalf("rebuilding signing vectors failed: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("signing vectors are not reproducible; a consumer cannot pin them")
	}

	onDisk, err := mustLoad(t).File(PathVectors)
	if err != nil {
		t.Fatalf("reading %s failed: %v", PathVectors, err)
	}
	if string(onDisk) != string(first) {
		t.Fatalf("%s is out of date with pkg/signing; run `go run ./cmd/nil-contract generate`", PathVectors)
	}
}

// TestManifestSchemaAgreesWithCode keeps the published JSON Schema and the
// hand-written validator from describing different formats.
//
// There is no schema engine in this repository, so the two lists could drift
// with nothing noticing: a consumer in Rust validates against the schema while
// the Go validator accepts something the schema rejects.
func TestManifestSchemaAgreesWithCode(t *testing.T) {
	set := mustLoad(t)

	for version := 1; version <= 2; version++ {
		data, err := set.ManifestSchema(version)
		if err != nil {
			t.Fatalf("no schema for format_version %d: %v", version, err)
		}

		var schema struct {
			Properties           map[string]json.RawMessage `json:"properties"`
			Required             []string                   `json:"required"`
			AdditionalProperties any                        `json:"additionalProperties"`
		}
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Fatalf("format_version %d schema is not valid JSON: %v", version, err)
		}

		if allowed, isBool := schema.AdditionalProperties.(bool); !isBool || allowed {
			t.Errorf("format_version %d schema must set additionalProperties to false, got %v", version, schema.AdditionalProperties)
		}

		inCode := manifestMembers[version]
		for name := range inCode {
			if _, ok := schema.Properties[name]; !ok {
				t.Errorf("format_version %d: validator accepts %q but the schema does not define it", version, name)
			}
		}
		for name := range schema.Properties {
			if !inCode[name] {
				t.Errorf("format_version %d: schema defines %q but the validator rejects it as unknown", version, name)
			}
		}

		want := append([]string(nil), requiredMembers[version]...)
		got := append([]string(nil), schema.Required...)
		if len(want) != len(got) {
			t.Errorf("format_version %d required members differ: code has %d, schema has %d", version, len(want), len(got))
			continue
		}
		required := setOf(got...)
		for _, name := range want {
			if !required[name] {
				t.Errorf("format_version %d: code requires %q but the schema does not", version, name)
			}
		}
	}
}

func TestUnknownFormatVersionIsRefused(t *testing.T) {
	set := mustLoad(t)

	for _, version := range []string{"3", "0", "-1", "99"} {
		doc := strings.Replace(goodV1, `"format_version": 1`, `"format_version": `+version, 1)
		_, err := set.ParseManifest([]byte(doc))
		if err == nil {
			t.Errorf("format_version %s was accepted; an unknown format must be refused, not partially read", version)
		}
	}

	if _, err := set.ManifestSchema(3); err == nil {
		t.Error("ManifestSchema(3) returned a schema for a format that does not have one")
	}
}

func TestValidManifestsPass(t *testing.T) {
	set := mustLoad(t)
	entries := FileArchive{
		{Path: "manifest.json", Size: 100},
		{Path: "bytecode/main.nabc", Size: 984, SHA256: strings.Repeat("1", 64)},
	}

	cases := []struct {
		name string
		doc  string
		opts ValidateOptions
	}{
		{
			name: "v1 store package",
			doc:  goodV1,
			opts: ValidateOptions{Archive: FileArchive{entries[0], entries[1]}},
		},
		{
			name: "v2 package",
			doc:  goodV2,
			opts: ValidateOptions{Archive: FileArchive{entries[0], entries[1]}},
		},
		{
			name: "v1 with capability aliases spelled the old way",
			doc:  strings.Replace(goodV1, `["stdout"]`, `["Filesystem", "GPS"]`, 1),
			opts: ValidateOptions{Archive: FileArchive{entries[0], entries[1]}},
		},
		{
			name: "development target is allowed when asked for",
			doc:  strings.Replace(goodV1, `["onuron"]`, `["wasm"]`, 1),
			opts: ValidateOptions{Archive: FileArchive{entries[0], entries[1]}, AllowExperimentalTargets: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, vs := set.ValidateJSON([]byte(tc.doc), tc.opts)
			if m == nil {
				t.Fatalf("manifest was not parsed: %v", vs.OrNil())
			}
			if err := vs.OrNil(); err != nil {
				t.Errorf("a valid manifest was refused:\n%v", err)
			}
		})
	}
}

func TestInvalidManifestsAreRefused(t *testing.T) {
	set := mustLoad(t)
	entries := FileArchive{
		{Path: "manifest.json", Size: 100},
		{Path: "bytecode/main.nabc", Size: 984, SHA256: strings.Repeat("1", 64)},
	}
	base := ValidateOptions{Archive: entries}

	cases := []struct {
		name  string
		doc   string
		stage int
	}{
		{
			name:  "unknown manifest member",
			doc:   strings.Replace(goodV1, `"format_version": 1,`, `"format_version": 1, "runtime_abi": "1.0",`, 1),
			stage: StageIdentity,
		},
		{
			name:  "missing required member",
			doc:   strings.Replace(goodV1, `"app_name": "org.onuron.notes",`, ``, 1),
			stage: StageIdentity,
		},
		{
			name:  "version is not semver",
			doc:   strings.Replace(goodV1, `"app_version": "1.2.0"`, `"app_version": "1.2"`, 1),
			stage: StageIdentity,
		},
		{
			name:  "app name with characters a v1 name cannot carry",
			doc:   strings.Replace(goodV1, `"app_name": "org.onuron.notes"`, `"app_name": "notes app/../x"`, 1),
			stage: StageIdentity,
		},
		{
			name:  "v2 app id that is not reverse dns",
			doc:   strings.Replace(goodV2, `"app_id": "org.onuron.notes"`, `"app_id": "notes"`, 1),
			stage: StageIdentity,
		},
		{
			name:  "no targets",
			doc:   strings.Replace(goodV1, `"targets": ["onuron"],`, ``, 1),
			stage: StageIdentity,
		},
		{
			name:  "capability outside the vocabulary",
			doc:   strings.Replace(goodV1, `["stdout"]`, `["root"]`, 1),
			stage: StageCapability,
		},
		{
			name:  "planned sub-capability",
			doc:   strings.Replace(goodV1, `["stdout"]`, `["filesystem.read"]`, 1),
			stage: StageCapability,
		},
		{
			name:  "experimental target is not publishable",
			doc:   strings.Replace(goodV1, `["onuron"]`, `["android"]`, 1),
			stage: StageCompatibility,
		},
		{
			name:  "target nobody declared",
			doc:   strings.Replace(goodV1, `["onuron"]`, `["commodore64"]`, 1),
			stage: StageCompatibility,
		},
		{
			name:  "entry point is not in the archive",
			doc:   strings.Replace(goodV1, `"bytecode/main.nabc"`, `"bytecode/missing.nabc"`, 1),
			stage: StageInstall,
		},
		{
			name:  "entry point escapes the archive",
			doc:   strings.Replace(goodV1, `"bytecode/main.nabc"`, `"../../etc/passwd"`, 1),
			stage: StageIdentity,
		},
		{
			name:  "signature is not a signature",
			doc:   strings.Replace(goodV1, `"metadata":`, `"signature": "not-hex", "metadata":`, 1),
			stage: StageTrust,
		},
		{
			name:  "v2 without payload digests",
			doc:   strings.Replace(goodV2, `"payload_checksums": {"bytecode/main.nabc": "sha256:1111111111111111111111111111111111111111111111111111111111111111"}`, `"payload_checksums": {}`, 1),
			stage: StageIntegrity,
		},
		{
			name:  "v2 digest that does not match the payload",
			doc:   strings.Replace(goodV2, strings.Repeat("1", 64), strings.Repeat("2", 64), 1),
			stage: StageIntegrity,
		},
		{
			name:  "v2 requiring an ABI this runtime does not provide",
			doc:   goodV2,
			stage: StageCompatibility,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := base
			if strings.Contains(tc.name, "ABI") {
				opts.RuntimeABI = "0.9"
			}
			_, vs := set.ValidateJSON([]byte(tc.doc), opts)
			if !vs.HasStage(tc.stage) {
				t.Fatalf("expected a stage %d (%s) violation, got:\n%v", tc.stage, StageName(tc.stage), vs.OrNil())
			}
		})
	}
}

func TestArchiveSafety(t *testing.T) {
	set := mustLoad(t)

	cases := []struct {
		name    string
		entries FileArchive
	}{
		{"absolute path", FileArchive{{Path: "/etc/passwd"}}},
		{"parent traversal", FileArchive{{Path: "../outside.txt"}}},
		{"windows drive letter", FileArchive{{Path: `C:/windows/system32`}}},
		{"backslash", FileArchive{{Path: `dir\file.nabc`}}},
		{"duplicate entry", FileArchive{{Path: "a.nabc"}, {Path: "a.nabc"}}},
		{"entry over the size limit", FileArchive{{Path: "big.nabc", Size: MaxEntryBytes + 1}}},
		{"too many entries", FileArchive(makeEntries(MaxArchiveEntries + 1))},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := set.ParseManifest([]byte(goodV1))
			if err != nil {
				t.Fatalf("parsing the fixture manifest failed: %v", err)
			}
			vs := set.Validate(m, ValidateOptions{Archive: tc.entries})
			if !vs.HasStage(StageArchive) {
				t.Fatalf("expected an archive-safety violation, got:\n%v", vs.OrNil())
			}
		})
	}
}

func TestIntegrityIsCheckedBeforeTrust(t *testing.T) {
	// The contract states the order; a package that is both tampered and
	// untrusted should report both, and the integrity failure must be present
	// rather than swallowed by an early return on trust.
	set := mustLoad(t)
	doc := strings.Replace(goodV2, strings.Repeat("1", 64), strings.Repeat("9", 64), 1)

	_, vs := set.ValidateJSON([]byte(doc), ValidateOptions{
		Archive: FileArchive{
			{Path: "manifest.json", Size: 10},
			{Path: "bytecode/main.nabc", Size: 10, SHA256: strings.Repeat("1", 64)},
		},
		Trust: TrustRequired,
	})

	if !vs.HasStage(StageIntegrity) {
		t.Errorf("expected an integrity violation:\n%v", vs.OrNil())
	}
	if !vs.HasStage(StageTrust) {
		t.Errorf("expected a trust violation:\n%v", vs.OrNil())
	}
}

func TestCapabilityVocabularyIsDefaultDeny(t *testing.T) {
	set := mustLoad(t)

	for _, name := range []string{"root", "admin", "filesystem.read", "", "  ", "sudo"} {
		if _, err := set.Vocabulary().Canonicalize(name); err == nil {
			t.Errorf("capability %q was accepted; an unknown capability must be refused", name)
		}
	}

	for _, name := range []string{"stdout", "Filesystem", "file_system", "GPS", "File-System"} {
		canonical, err := set.Vocabulary().Canonicalize(name)
		if err != nil {
			t.Errorf("capability %q was refused: %v", name, err)
			continue
		}
		if canonical != "filesystem" && canonical != "stdout" && canonical != "location" {
			t.Errorf("capability %q normalised to %q, which is not the expected canonical name", name, canonical)
		}
	}
}

func TestRequireCapabilityRefusesUnknownOperations(t *testing.T) {
	table := mustLoad(t).Requirements()

	if err := table.RequireCapability("writeFile", []string{"filesystem"}); err != nil {
		t.Errorf("writeFile with a filesystem grant was refused: %v", err)
	}
	if err := table.RequireCapability("writeFile", []string{"Filesystem"}); err != nil {
		t.Errorf("writeFile with an aliased grant was refused: %v", err)
	}
	if err := table.RequireCapability("writeFile", nil); err == nil {
		t.Error("writeFile with no grant was allowed")
	}
	if err := table.RequireCapability("notAnOperation", []string{"filesystem", "network", "core"}); err == nil {
		t.Error("an unknown operation was allowed; a typo must be a refusal, not a hole")
	}
	// clear is known and needs nothing. This is the case that a
	// "unknown means pure" implementation gets wrong in the safe-looking
	// direction, which is why it has a vector of its own.
	if _, known := table.RequirementsFor("clear"); !known {
		t.Error("clear is not in the requirement table")
	}
	if err := table.RequireCapability("clear", nil); err != nil {
		t.Errorf("clear requires nothing and was refused: %v", err)
	}
}

func TestExperimentalTargetsAreNotPublishable(t *testing.T) {
	set := mustLoad(t)

	if err := set.SupportsTarget("onuron"); err != nil {
		t.Errorf("onuron should be a publishable target: %v", err)
	}
	if err := set.SupportsTarget("android"); err == nil {
		t.Error("android is experimental and must not be a publishable target")
	}
	if err := set.SupportsTarget("commodore64"); err == nil {
		t.Error("an undeclared target must be refused")
	}
}

func TestCompatibilityMatrixHasNoUnsupportedClaims(t *testing.T) {
	compat := mustLoad(t).Compatibility()

	for backend, b := range compat.Backends {
		for feature, f := range b.Features {
			switch f.Status {
			case "stable", "partial", "experimental", "unsupported":
			default:
				t.Errorf("%s/%s has status %q, which is not one of the four declared statuses", backend, feature, f.Status)
			}
			if f.Status == "stable" && f.Evidence == "" {
				t.Errorf("%s/%s claims stable with no evidence path; the evidence policy requires one", backend, feature)
			}
		}
	}

	vm, ok := compat.FeatureStatus("vm", "capability_gate")
	if !ok {
		t.Fatal("the VM's capability_gate cell is missing from the matrix")
	}
	if vm.Status != "partial" {
		t.Errorf("vm/capability_gate is %q; the matrix must not claim more than the spec admits", vm.Status)
	}
	if !strings.Contains(vm.Limitations, "Indirect") {
		t.Error("vm/capability_gate must record that indirect calls are not gated")
	}
}

func TestVocabularyMatchesTheCompiler(t *testing.T) {
	set := mustLoad(t)

	// Every capability a builtin requires must exist in the published
	// vocabulary. A requirement naming something the vocabulary does not carry
	// is a grant that no verifier can validate.
	for _, op := range set.Requirements().Operations() {
		req, _ := set.Requirements().RequirementsFor(op)
		for _, name := range req {
			if _, err := set.Vocabulary().Canonicalize(name); err != nil {
				t.Errorf("operation %q requires %q, which is not in the canonical vocabulary", op, name)
			}
		}
	}
}

func TestBundleFilesAreValidJSONWithAStatus(t *testing.T) {
	set := mustLoad(t)

	for _, entry := range set.Index().Files {
		data, err := set.File(entry.Path)
		if err != nil {
			t.Fatalf("%s: %v", entry.Path, err)
		}
		var doc struct {
			Contract string `json:"contract"`
			Version  string `json:"version"`
			Status   string `json:"status"`
			Owner    string `json:"owner"`
		}
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Errorf("%s is not valid JSON: %v", entry.Path, err)
			continue
		}
		if doc.Contract == "" || doc.Version == "" || doc.Status == "" || doc.Owner == "" {
			t.Errorf("%s must declare contract, version, status and owner", entry.Path)
		}
	}
}

// TestBundleIsTheOnlyCopy guards against the failure mode the bundle exists to
// prevent: a second hand-maintained definition appearing somewhere in the tree.
func TestBundleIsTheOnlyCopy(t *testing.T) {
	root := repoRoot(t)

	var duplicates []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch filepath.Base(path) {
			case ".git", "node_modules", "dist", "bin", "build", "out":
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "contracts/") {
			return nil
		}
		switch filepath.Base(path) {
		case "vocabulary.json", "requirements.json", "runtime-abi.json", "signing.json":
			duplicates = append(duplicates, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the repository failed: %v", err)
	}
	if len(duplicates) > 0 {
		t.Errorf("these files duplicate a contract that lives in contracts/: %s\n"+
			"Consumers should import or pin the bundle, not keep a copy.", strings.Join(duplicates, ", "))
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("cannot determine the working directory: %v", err)
	}
	for d := dir; ; {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			t.Fatal("cannot find the repository root")
		}
		d = parent
	}
}

func makeEntries(n int) FileArchive {
	out := make(FileArchive, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, ArchiveEntry{Path: "f" + itoa(i) + ".nabc"})
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
