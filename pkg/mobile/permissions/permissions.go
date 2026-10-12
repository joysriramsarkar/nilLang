// Package permissions models runtime permissions (L8.7): declaration in
// code, the request flow at runtime, and Android manifest entries.
package permissions

import (
	"fmt"
	"sort"
	"strings"
)

// Permission is a device capability the app may request.
type Permission string

const (
	Camera      Permission = "camera"
	Location    Permission = "location"
	Microphone  Permission = "microphone"
	Storage     Permission = "storage"
	Contacts    Permission = "contacts"
	Notify      Permission = "notification"
	Biometric   Permission = "biometric"
	ClipboardRO Permission = "clipboard.read"
)

// Status is the runtime grant state of one permission.
type Status int

const (
	StatusUnknown Status = iota
	StatusGranted
	StatusDenied
	StatusPermanentlyDenied
)

// androidManifest maps each permission to its Android manifest entries.
var androidManifest = map[Permission][]string{
	Camera:     {"android.permission.CAMERA"},
	Location:   {"android.permission.ACCESS_FINE_LOCATION", "android.permission.ACCESS_COARSE_LOCATION"},
	Microphone: {"android.permission.RECORD_AUDIO"},
	Storage:    {"android.permission.READ_EXTERNAL_STORAGE"},
	Contacts:   {"android.permission.READ_CONTACTS"},
	Notify:     {"android.permission.POST_NOTIFICATIONS"},
	Biometric:  {"android.permission.USE_BIOMETRIC"},
}

// Manager tracks declared permissions and their runtime states.
type Manager struct {
	declared map[Permission]bool
	states   map[Permission]Status
}

// New returns an empty manager.
func New() *Manager {
	return &Manager{declared: map[Permission]bool{}, states: map[Permission]Status{}}
}

// Declare records that the app uses these permissions. Undeclared requests
// fail: a request for something the manifest never names is a programming
// error, not a user decision.
func (m *Manager) Declare(perms ...Permission) {
	for _, p := range perms {
		m.declared[p] = true
		if _, ok := m.states[p]; !ok {
			m.states[p] = StatusUnknown
		}
	}
}

// Declared returns the declared set in sorted order.
func (m *Manager) Declared() []Permission {
	out := make([]Permission, 0, len(m.declared))
	for p := range m.declared {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Request records the user's answer for a declared permission.
func (m *Manager) Request(p Permission, granted bool) error {
	if !m.declared[p] {
		return fmt.Errorf("permissions: %q was never declared", p)
	}
	if granted {
		m.states[p] = StatusGranted
	} else {
		m.states[p] = StatusDenied
	}
	return nil
}

// MarkPermanentlyDenied records a "don't ask again" denial.
func (m *Manager) MarkPermanentlyDenied(p Permission) error {
	if !m.declared[p] {
		return fmt.Errorf("permissions: %q was never declared", p)
	}
	m.states[p] = StatusPermanentlyDenied
	return nil
}

// Check returns the current status (Unknown when never requested).
func (m *Manager) Check(p Permission) Status { return m.states[p] }

// ManifestEntries renders <uses-permission> lines for the declared set.
func (m *Manager) ManifestEntries() []string {
	var out []string
	for _, p := range m.Declared() {
		for _, entry := range androidManifest[p] {
			out = append(out, fmt.Sprintf(`<uses-permission android:name=%q />`, entry))
		}
	}
	sort.Strings(out)
	return out
}

// ManifestXML renders a minimal manifest snippet for tests/CI inspection.
func (m *Manager) ManifestXML() string {
	return "<manifest>\n  " + strings.Join(m.ManifestEntries(), "\n  ") + "\n</manifest>\n"
}
