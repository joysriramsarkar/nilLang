package profile

import (
	"fmt"
	"strings"

	"github.com/joysriramsarkar/nilLang/pkg/capability"
)

// ID represents a NilLang runtime profile identifier
type ID string

const (
	ProfileCore     ID = "core"
	ProfileWeb      ID = "web"
	ProfileMobile   ID = "mobile"
	ProfileServer   ID = "server"
	ProfileData     ID = "data"
	ProfileOS       ID = "os"
	ProfileEmbedded ID = "embedded"
)

// Profile represents a NilLang execution target profile
type Profile struct {
	ID          ID                `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Target      string            `json:"target"` // wasm, native, arm, linux, onuron
	RuntimeAPIs []string          `json:"runtime_apis"`
	AllowedCaps []capability.Type `json:"allowed_caps"`
}

// Registry maps profile IDs to Profile metadata
var Registry = map[ID]Profile{
	ProfileCore: {
		ID:          ProfileCore,
		Name:        "NilLang Core",
		Description: "Minimal, strict language primitives and bytecode VM",
		Target:      "portable",
		RuntimeAPIs: []string{"core.math", "core.string", "core.array", "core.hash"},
		AllowedCaps: []capability.Type{capability.CapCrypto},
	},
	ProfileWeb: {
		ID:          ProfileWeb,
		Name:        "NilLang Web Profile",
		Description: "WebAssembly execution target with DOM bindings and Web APIs",
		Target:      "wasm",
		RuntimeAPIs: []string{"web.dom", "web.fetch", "web.websocket", "web.workers", "web.canvas"},
		AllowedCaps: []capability.Type{
			capability.CapNetwork,
			capability.CapCrypto,
			capability.CapAI,
			capability.CapAudio,
			capability.CapCamera,
			capability.CapGPS,
			capability.CapGPU,
		},
	},
	ProfileMobile: {
		ID:          ProfileMobile,
		Name:        "NilLang Mobile Profile",
		Description: "Native mobile execution with sensors, device APIs and graphics",
		Target:      "native-mobile",
		RuntimeAPIs: []string{"mobile.ui", "mobile.sensor", "mobile.camera", "mobile.storage", "mobile.gps"},
		AllowedCaps: []capability.Type{
			capability.CapNetwork,
			capability.CapCrypto,
			capability.CapAI,
			capability.CapAudio,
			capability.CapCamera,
			capability.CapGPS,
			capability.CapBluetooth,
			capability.CapGPU,
			capability.CapFilesystem,
			capability.CapDatabase,
			capability.CapSensors,
		},
	},
	ProfileServer: {
		ID:          ProfileServer,
		Name:        "NilLang Server Profile",
		Description: "High-performance server runtime with network sockets, processes, and DB drivers",
		Target:      "native-server",
		RuntimeAPIs: []string{"server.http", "server.tcp", "server.process", "server.db", "server.fs"},
		AllowedCaps: []capability.Type{
			capability.CapNetwork,
			capability.CapCrypto,
			capability.CapAI,
			capability.CapDatabase,
			capability.CapFilesystem,
			capability.CapProcess,
			capability.CapGPU,
		},
	},
	ProfileData: {
		ID:          ProfileData,
		Name:        "NilLang Data Profile",
		Description: "Numerical computing and data science pipelines with GPU acceleration",
		Target:      "native-data",
		RuntimeAPIs: []string{"data.tensor", "data.dataset", "data.math", "data.csv", "data.linear"},
		AllowedCaps: []capability.Type{
			capability.CapNetwork,
			capability.CapFilesystem,
			capability.CapDatabase,
			capability.CapGPU,
			capability.CapAI,
			capability.CapCrypto,
			capability.CapProcess,
		},
	},
	ProfileOS: {
		ID:          ProfileOS,
		Name:        "NilLang Onuron/OS Profile",
		Description: "Full OS runtime with direct HAL, device services, and system calls",
		Target:      "onuron-native",
		RuntimeAPIs: []string{"onuron.hal", "onuron.ipc", "onuron.device", "onuron.syscall", "onuron.softbus"},
		AllowedCaps: capability.AllCapabilities,
	},
	ProfileEmbedded: {
		ID:          ProfileEmbedded,
		Name:        "NilLang Embedded Profile",
		Description: "Bare-metal or micro-runtime with strict memory boundaries and GPIO",
		Target:      "bare-metal",
		RuntimeAPIs: []string{"embedded.gpio", "embedded.timer", "embedded.serial"},
		AllowedCaps: []capability.Type{
			capability.CapSensors,
			capability.CapBluetooth,
			capability.CapAudio,
			capability.CapCrypto,
			capability.CapFilesystem,
			capability.CapNetwork,
		},
	},
}

// Get returns the profile by ID or error
func Get(id string) (*Profile, error) {
	norm := ID(strings.ToLower(strings.TrimSpace(id)))
	if p, ok := Registry[norm]; ok {
		return &p, nil
	}
	return nil, fmt.Errorf("unknown profile %q (available: web, mobile, server, data, os, embedded, core)", id)
}

// ListAll returns all profiles
func ListAll() []Profile {
	list := make([]Profile, 0, len(Registry))
	for _, p := range Registry {
		list = append(list, p)
	}
	return list
}

// ValidateProfileCaps verifies requested capabilities against the specified profile
func ValidateProfileCaps(profileID string, caps []string) (*capability.VerificationResult, error) {
	p, err := Get(profileID)
	if err != nil {
		return nil, err
	}

	capTypes := make([]capability.Type, 0, len(caps))
	for _, c := range caps {
		ct, err := capability.ParseCapability(c)
		if err != nil {
			return nil, err
		}
		capTypes = append(capTypes, ct)
	}

	return capability.VerifyProfileCapabilities(string(p.ID), capTypes), nil
}
