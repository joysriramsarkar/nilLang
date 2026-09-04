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

// ParseCapability converts a string to a valid Type
func ParseCapability(s string) (Type, error) {
	norm := strings.TrimSpace(s)
	for _, c := range AllCapabilities {
		if strings.EqualFold(string(c), norm) {
			return c, nil
		}
	}
	return "", fmt.Errorf("unknown capability: %q (valid: %v)", s, AllCapabilities)
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
		CapProcess:    PermDenied, // Spawning arbitrary child processes restricted
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
