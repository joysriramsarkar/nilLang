package capability

import (
	"fmt"
	"strings"
)

// Type represents a system or hardware capability
type Type string

const (
	CapFilesystem Type = "Filesystem"
	CapNetwork    Type = "Network"
	CapCamera     Type = "Camera"
	CapGPS        Type = "GPS"
	CapBluetooth  Type = "Bluetooth"
	CapGPU        Type = "GPU"
	CapDatabase   Type = "Database"
	CapProcess    Type = "Process"
	CapCrypto     Type = "Crypto"
	CapAI         Type = "AI"
	CapSensors    Type = "Sensors"
	CapAudio      Type = "Audio"
	CapPrinter    Type = "Printer"
	CapScanner    Type = "Scanner"
)

// AllCapabilities lists all valid capability types
var AllCapabilities = []Type{
	CapFilesystem,
	CapNetwork,
	CapCamera,
	CapGPS,
	CapBluetooth,
	CapGPU,
	CapDatabase,
	CapProcess,
	CapCrypto,
	CapAI,
	CapSensors,
	CapAudio,
	CapPrinter,
	CapScanner,
}

// Inline capabilities are the process-local names that appear next to the code
// that needs them in a nil.json manifest (see compiler/capability/names.go and
// compiler/capability/manifest.go). They are not profile-gated hardware or
// system resources: every profile can print to stdout, read the clock, or
// access the environment.
//
// Two vocabularies describe capabilities in this codebase and they must
// interoperate. This package uses capitalised resource names such as
// "Filesystem"; compiler/capability uses lowercase names such as "stdout". A
// manifest written by hand, or produced by `nil init`, may use either, so both
// must be accepted by the build-time profile check.
var inlineCapabilities = []Type{
	"core", "stdout", "stderr", "stdin", "clock", "environment",
	"random", "clipboard", "device", "notifications", "ui", "async",
}

// resourceAliases maps spellings that differ between the two vocabularies onto
// this package's canonical resource type. The compiler vocabulary calls GPS
// "location"; accept both.
var resourceAliases = map[string]Type{
	"location": CapGPS,
	"gps":      CapGPS,
}

// normalizeName lowercases a capability name and drops '-', '_' and spaces so
// that "File-System", "file_system" and "Filesystem" collapse to one spelling.
func normalizeName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

// inlineCapability returns the canonical inline capability for name, if it is
// one. Matching ignores case and separators.
func inlineCapability(name string) (Type, bool) {
	norm := normalizeName(name)
	for _, c := range inlineCapabilities {
		if string(c) == norm {
			return c, true
		}
	}
	return "", false
}

// IsInline reports whether name is a process-local inline capability rather
// than a profile-gated resource. Exposed so callers can present the two
// vocabularies consistently.
func IsInline(name string) bool {
	_, ok := inlineCapability(name)
	return ok
}

// PermissionLevel defines access permission level for a capability
type PermissionLevel string

const (
	PermAllowed    PermissionLevel = "ALLOWED"
	PermDenied     PermissionLevel = "DENIED"
	PermRestricted PermissionLevel = "RESTRICTED" // Subject to user prompt or sandbox
)

// Descriptor contains capability metadata and runtime permissions
type Descriptor struct {
	Type        Type            `json:"type"`
	Description string          `json:"description"`
	Permission  PermissionLevel `json:"permission"`
	Reason      string          `json:"reason,omitempty"`
}

// CapabilitySet is a set of capabilities requested or granted
type CapabilitySet map[Type]Descriptor

// NewCapabilitySet creates a new capability set
func NewCapabilitySet(caps ...Type) CapabilitySet {
	set := make(CapabilitySet)
	for _, c := range caps {
		set[c] = Descriptor{
			Type:       c,
			Permission: PermAllowed,
		}
	}
	return set
}

// ParseCapability converts a string to a valid Type.
//
// Both capability vocabularies are accepted: the capitalised resource names in
// AllCapabilities ("Filesystem", "Network") and the lowercase inline names used
// inline in a manifest ("stdout", "clock"). Matching ignores case and treats
// '-', '_' and spaces as equivalent.
func ParseCapability(s string) (Type, error) {
	norm := strings.TrimSpace(s)
	for _, c := range AllCapabilities {
		if strings.EqualFold(string(c), norm) {
			return c, nil
		}
	}
	if c, ok := resourceAliases[normalizeName(norm)]; ok {
		return c, nil
	}
	if c, ok := inlineCapability(norm); ok {
		return c, nil
	}
	return "", fmt.Errorf("unknown capability: %q (valid: %v, inline: %v)", s, AllCapabilities, inlineCapabilities)
}

// Has checks if a capability is present and allowed
func (cs CapabilitySet) Has(c Type) bool {
	desc, ok := cs[c]
	return ok && desc.Permission != PermDenied
}

// List returns sorted capability types as strings
func (cs CapabilitySet) List() []string {
	res := make([]string, 0, len(cs))
	for _, c := range AllCapabilities {
		if _, ok := cs[c]; ok {
			res = append(res, string(c))
		}
	}
	return res
}

// CapabilityMatrix maps profiles to their allowed capabilities
var CapabilityMatrix = map[string]map[Type]PermissionLevel{
	"web": {
		CapNetwork:    PermAllowed,
		CapCrypto:     PermAllowed,
		CapAI:         PermAllowed,
		CapAudio:      PermRestricted,
		CapCamera:     PermRestricted,
		CapGPS:        PermRestricted,
		CapGPU:        PermRestricted, // WebGL / WebGPU
		CapFilesystem: PermDenied,     // Direct filesystem denied, sandboxed origin only
		CapBluetooth:  PermDenied,
		CapDatabase:   PermRestricted, // IndexedDB only
		CapProcess:    PermDenied,     // Raw OS processes strictly denied in browser
		CapSensors:    PermRestricted,
	},
	"mobile": {
		CapNetwork:    PermAllowed,
		CapCrypto:     PermAllowed,
		CapAI:         PermAllowed,
		CapAudio:      PermAllowed,
		CapCamera:     PermRestricted, // User permission prompt
		CapGPS:        PermRestricted, // User permission prompt
		CapBluetooth:  PermRestricted, // User permission prompt
		CapGPU:        PermAllowed,
		CapFilesystem: PermRestricted, // App sandbox only
		CapDatabase:   PermAllowed,    // SQLite
		CapSensors:    PermAllowed,
		CapScanner:    PermRestricted, // Camera-based barcode scanning
		CapPrinter:    PermRestricted, // Bluetooth / thermal receipt printers
		CapProcess:    PermDenied,     // Spawning arbitrary child processes restricted
	},
	"server": {
		CapNetwork:    PermAllowed,
		CapCrypto:     PermAllowed,
		CapAI:         PermAllowed,
		CapDatabase:   PermAllowed,
		CapFilesystem: PermAllowed,
		CapProcess:    PermAllowed,
		CapGPU:        PermAllowed,
		CapAudio:      PermDenied, // Headless server
		CapCamera:     PermDenied,
		CapGPS:        PermDenied,
		CapBluetooth:  PermDenied,
		CapSensors:    PermDenied,
	},
	"data": {
		CapNetwork:    PermAllowed,
		CapFilesystem: PermAllowed,
		CapDatabase:   PermAllowed,
		CapGPU:        PermAllowed, // Tensor acceleration
		CapAI:         PermAllowed,
		CapCrypto:     PermAllowed,
		CapProcess:    PermAllowed,
		CapAudio:      PermDenied,
		CapCamera:     PermDenied,
		CapGPS:        PermDenied,
		CapBluetooth:  PermDenied,
		CapSensors:    PermDenied,
	},
	"game": {
		CapGPU:        PermAllowed,
		CapAudio:      PermAllowed,
		CapFilesystem: PermAllowed,
		CapNetwork:    PermAllowed,
		CapProcess:    PermAllowed,
		CapSensors:    PermAllowed,
		CapAI:         PermAllowed,
		CapCrypto:     PermAllowed,
		CapCamera:     PermRestricted,
		CapGPS:        PermRestricted,
		CapBluetooth:  PermRestricted,
		CapDatabase:   PermRestricted,
	},
	"os": {
		// Onuron OS / Native Linux profile: all capabilities available
		CapNetwork:    PermAllowed,
		CapCrypto:     PermAllowed,
		CapAI:         PermAllowed,
		CapAudio:      PermAllowed,
		CapCamera:     PermAllowed,
		CapGPS:        PermAllowed,
		CapBluetooth:  PermAllowed,
		CapGPU:        PermAllowed,
		CapFilesystem: PermAllowed,
		CapDatabase:   PermAllowed,
		CapProcess:    PermAllowed,
		CapSensors:    PermAllowed,
		CapPrinter:    PermAllowed, // Receipt / label printers (POS hardware)
		CapScanner:    PermAllowed, // Barcode / QR scanners (POS hardware)
	},
	"embedded": {
		CapSensors:    PermAllowed,
		CapBluetooth:  PermRestricted,
		CapAudio:      PermRestricted,
		CapCrypto:     PermAllowed,
		CapFilesystem: PermRestricted,
		CapNetwork:    PermRestricted,
		CapGPU:        PermDenied,
		CapDatabase:   PermDenied,
		CapProcess:    PermDenied,
		CapCamera:     PermDenied,
		CapGPS:        PermDenied,
		CapAI:         PermDenied,
	},
}

// VerificationResult contains the result of checking capabilities against a profile
type VerificationResult struct {
	Valid       bool     `json:"valid"`
	Profile     string   `json:"profile"`
	Allowed     []string `json:"allowed"`
	Restricted  []string `json:"restricted"`
	Violations  []string `json:"violations"`
	Suggestions []string `json:"suggestions"`
}

// VerifyProfileCapabilities verifies requested capabilities against the target profile
func VerifyProfileCapabilities(profile string, requested []Type) *VerificationResult {
	normProfile := strings.ToLower(strings.TrimSpace(profile))
	if normProfile == "" {
		normProfile = "os"
	}

	matrix, exists := CapabilityMatrix[normProfile]
	if !exists {
		return &VerificationResult{
			Valid:      false,
			Profile:    profile,
			Violations: []string{fmt.Sprintf("unknown profile %q", profile)},
		}
	}

	res := &VerificationResult{
		Valid:       true,
		Profile:     normProfile,
		Allowed:     []string{},
		Restricted:  []string{},
		Violations:  []string{},
		Suggestions: []string{},
	}

	for _, capReq := range requested {
		// Inline process-local capabilities (stdout, clock, environment, ...)
		// are available on every profile; skip the resource matrix for them.
		if _, inline := inlineCapability(string(capReq)); inline {
			res.Allowed = append(res.Allowed, string(capReq))
			continue
		}
		perm, supported := matrix[capReq]
		if !supported || perm == PermDenied {
			res.Valid = false
			msg := fmt.Sprintf("Capability %s is DENIED on %s profile", capReq, normProfile)
			res.Violations = append(res.Violations, msg)

			// Provide actionable advice
			switch capReq {
			case CapFilesystem:
				if normProfile == "web" {
					res.Suggestions = append(res.Suggestions, "On web profile, use Alap Storage (IndexedDB/KV) instead of direct filesystem access")
				}
			case CapProcess:
				if normProfile == "web" || normProfile == "mobile" {
					res.Suggestions = append(res.Suggestions, fmt.Sprintf("Process execution is forbidden on %s. Use background tasks or server endpoints.", normProfile))
				}
			case CapCamera, CapGPS, CapBluetooth:
				if normProfile == "server" {
					res.Suggestions = append(res.Suggestions, fmt.Sprintf("Hardware sensor %s is not available on headless server profile.", capReq))
				}
			}
		} else if perm == PermRestricted {
			res.Restricted = append(res.Restricted, string(capReq))
		} else {
			res.Allowed = append(res.Allowed, string(capReq))
		}
	}

	return res
}
