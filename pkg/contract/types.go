package contract

import "fmt"

// ABI is contracts/abi/runtime-abi.json.
//
// Only the parts a verifier makes a decision from are typed. The prose fields
// are carried as strings because a validator reads them and a person reads them,
// and giving prose a struct invites a fake precision that the underlying
// specification does not have yet.
type ABI struct {
	Contract   string `json:"contract"`
	Version    string `json:"version"`
	Status     string `json:"status"`
	Owner      string `json:"owner"`
	Spec       string `json:"spec"`
	Language   ABILanguage `json:"language"`
	RuntimeABI ABIRuntime `json:"runtime_abi"`
	Targets    []ABITarget `json:"targets"`
	Backends   []ABIBackend `json:"backends"`
}

// ABILanguage records the language specification version a runtime implements.
type ABILanguage struct {
	Name           string           `json:"name"`
	SpecVersion    string           `json:"spec_version"`
	SpecFiles      []string         `json:"spec_files"`
	Freeze         ABIFreeze        `json:"freeze"`
	Compatibility  ABICompatibility `json:"compatibility_claim"`
}

// ABIFreeze records the active core freeze.
type ABIFreeze struct {
	Document string `json:"document"`
	Status   string `json:"status"`
	Meaning  string `json:"meaning"`
}

// ABICompatibility records what a compatibility claim is allowed to rest on.
type ABICompatibility struct {
	Rule          string   `json:"rule"`
	NotGuaranteed []string `json:"not_guaranteed"`
}

// ABIRuntime is the runtime ABI version and its release policy.
type ABIRuntime struct {
	Version           string `json:"version"`
	CompatibleRange   string `json:"compatible_range"`
	ReleasePolicy     string `json:"release_policy"`
	MismatchBehaviour string `json:"mismatch_behaviour"`
	DeclaredInManifest string `json:"declared_in_manifest"`
	Surface           map[string]any `json:"surface"`
}

// ABITarget is one build target.
type ABITarget struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
}

// ABIBackend is one execution backend.
type ABIBackend struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

// TargetStatus returns the declared status of a target and whether the target is
// known at all.
//
// The bool is what callers must branch on. A target the ABI does not list is not
// "probably fine": it is a target no owner has claimed, and treating it as
// supported is how a package ends up built for something that cannot run it.
func (a *ABI) TargetStatus(id string) (string, bool) {
	for _, t := range a.Targets {
		if t.ID == id {
			return t.Status, true
		}
	}
	return "", false
}

// BackendStatus returns the declared status of a backend and whether it is known.
func (a *ABI) BackendStatus(id string) (string, bool) {
	for _, b := range a.Backends {
		if b.ID == id {
			return b.Status, true
		}
	}
	return "", false
}

// SupportsTarget reports whether a target may be declared by a publishable
// package.
//
// A target listed as experimental is refused here rather than allowed with a
// warning: "works on my machine, no promises" is not a distribution target, and
// a package that carries one has told the user something the ecosystem cannot
// back up.
func (s *Set) SupportsTarget(id string) error {
	status, ok := s.abi.TargetStatus(id)
	if !ok {
		return fmt.Errorf("target %q is not declared in the runtime ABI", id)
	}
	if status == "experimental" {
		return fmt.Errorf("target %q is experimental and is not a publishable target", id)
	}
	return nil
}

// Signing is contracts/signing/signing.json.
type Signing struct {
	Contract   string `json:"contract"`
	Version    string `json:"version"`
	Status     string `json:"status"`
	Owner      string `json:"owner"`
	Spec       string `json:"spec"`
	Algorithms []SigningAlgorithm `json:"algorithms"`
	KeyIdentity SigningKeyIdentity `json:"key_identity"`
	PackageV1  SigningPackageV1     `json:"package_signature_v1"`
}

// SigningAlgorithm is one accepted signature algorithm.
type SigningAlgorithm struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Curve    string `json:"curve"`
	SignatureEncoding string `json:"signature_encoding"`
	PublicKeyEncoding string `json:"public_key_encoding"`
	Digest   string `json:"digest"`
}

// SigningKeyIdentity records how a key id is derived.
type SigningKeyIdentity struct {
	KeyID       SigningDerivedField `json:"key_id"`
	Fingerprint SigningDerivedField `json:"fingerprint"`
	Record      map[string]any      `json:"record"`
}

// SigningDerivedField is a derived key identifier.
type SigningDerivedField struct {
	Derivation string `json:"derivation"`
	Length     int    `json:"length"`
	ExampleShape string `json:"example_shape"`
	Recomputable bool  `json:"recomputable"`
	Purpose    string `json:"purpose"`
}

// SigningPackageV1 is the v1 package signature contract.
type SigningPackageV1 struct {
	Status          string          `json:"status"`
	AppliesTo       string          `json:"applies_to"`
	SignedPayload   SigningPayload  `json:"signed_payload"`
	VerificationOrder []SigningStep  `json:"verification_order"`
	OrderingRule    string          `json:"ordering_rule"`
}

// SigningPayload is the exact byte sequence a v1 signature covers.
type SigningPayload struct {
	Description     string                 `json:"description"`
	FieldsInOrder   []SigningPayloadField  `json:"fields_in_order"`
	CanonicalForm   string                 `json:"canonical_form"`
	MustNotInclude  []string               `json:"must_not_include"`
	WhyExcluded     string                 `json:"why_excluded"`
}

// SigningPayloadField is one field of the v1 signing payload.
type SigningPayloadField struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Encoding string `json:"encoding"`
}

// SigningStep is one step of signature verification.
type SigningStep struct {
	Step     int    `json:"step"`
	Action   string `json:"action"`
	OnFailure string `json:"on_failure"`
}

// AlgorithmSupported reports whether the contract accepts an algorithm id.
func (s *Set) AlgorithmSupported(id string) bool {
	for _, a := range s.signing.Algorithms {
		if a.ID == id && a.Status == StatusStable {
			return true
		}
	}
	return false
}

// Compatibility is contracts/compatibility/matrix.json.
type Compatibility struct {
	Contract  string `json:"contract"`
	Version   string `json:"version"`
	Status    string `json:"status"`
	Owner     string `json:"owner"`
	Spec      string `json:"spec"`
	Statuses  map[string]string `json:"statuses"`
	EvidencePolicy EvidencePolicy `json:"evidence_policy"`
	Backends  map[string]CompatibilityBackend `json:"backends"`
}

// EvidencePolicy is the rule that keeps the matrix honest.
type EvidencePolicy struct {
	Rule             string `json:"rule"`
	CIJob            string `json:"ci_job"`
	WhereEvidenceLives string `json:"where_evidence_lives"`
}

// CompatibilityBackend is one backend's feature support.
type CompatibilityBackend struct {
	Description string                     `json:"description"`
	Status      string                     `json:"status,omitempty"`
	Features    map[string]CompatibilityFeature `json:"features"`
}

// CompatibilityFeature is one feature cell.
type CompatibilityFeature struct {
	Status      string `json:"status"`
	Evidence    string `json:"evidence"`
	Limitations string `json:"limitations"`
}

// FeatureStatus returns a feature's status for a backend.
func (c *Compatibility) FeatureStatus(backend, feature string) (CompatibilityFeature, bool) {
	b, ok := c.Backends[backend]
	if !ok {
		return CompatibilityFeature{}, false
	}
	f, ok := b.Features[feature]
	return f, ok
}

// SupportedFeatures lists the features a backend supports at stable status,
// sorted. It is what a runtime advertises, and it is derived from the matrix
// rather than restated, so the advertisement cannot exceed the claim.
func (c *Compatibility) SupportedFeatures(backend string) []string {
	b, ok := c.Backends[backend]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(b.Features))
	for feature, f := range b.Features {
		if f.Status == StatusStable {
			out = append(out, feature)
		}
	}
	sortStrings(out)
	return out
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}