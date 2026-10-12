// Package device exposes device APIs to Nilang mobile apps (L8.8):
// camera, location, notification, sensor, biometric and clipboard behind Go
// interfaces, with a stub implementation for host-less tests and the Android
// manifest entries each API needs.
package device

import (
	"errors"
	"fmt"
	"sort"
)

// Location is a GPS fix.
type Location struct {
	Latitude  float64
	Longitude float64
	AccuracyM float64
}

// Camera captures a single JPEG frame.
type Camera interface {
	Capture() ([]byte, error)
}

// Locator returns the last known fix.
type Locator interface {
	LastLocation() (Location, error)
}

// Notifier posts a local notification.
type Notifier interface {
	Notify(title, body string) error
}

// SensorReader samples a named sensor ("accelerometer", "gyro", ...).
type SensorReader interface {
	Sample(sensor string) ([]float64, error)
}

// BiometricAuth authenticates the user.
type BiometricAuth interface {
	Authenticate(reason string) (bool, error)
}

// Clipboard reads and writes text.
type Clipboard interface {
	ReadText() (string, error)
	WriteText(s string) error
}

// APIs bundles every device API a screen can use.
type APIs struct {
	Camera    Camera
	Locator   Locator
	Notifier  Notifier
	Sensor    SensorReader
	Biometric BiometricAuth
	Clipboard Clipboard
}

// ManifestEntries returns the Android permissions the wired APIs need.
func (a *APIs) ManifestEntries() []string {
	var out []string
	if a.Camera != nil {
		out = append(out, "android.permission.CAMERA")
	}
	if a.Locator != nil {
		out = append(out, "android.permission.ACCESS_FINE_LOCATION")
	}
	if a.Notifier != nil {
		out = append(out, "android.permission.POST_NOTIFICATIONS")
	}
	if a.Biometric != nil {
		out = append(out, "android.permission.USE_BIOMETRIC")
	}
	sort.Strings(out)
	return out
}

// Stub is a host-less implementation for tests and desktop preview: every
// call succeeds with canned data except when Fail is set.
type Stub struct {
	Fail         bool
	Photo        []byte
	Fix          Location
	Notification string
	Samples      map[string][]float64
	Authed       bool
	Text         string
}

func (s *Stub) Capture() ([]byte, error) {
	if s.Fail {
		return nil, errors.New("device: camera unavailable in stub")
	}
	if s.Photo != nil {
		return s.Photo, nil
	}
	return []byte("JPEG-STUB"), nil
}

func (s *Stub) LastLocation() (Location, error) {
	if s.Fail {
		return Location{}, errors.New("device: location unavailable in stub")
	}
	return s.Fix, nil
}

func (s *Stub) Notify(title, body string) error {
	if s.Fail {
		return errors.New("device: notification failed in stub")
	}
	s.Notification = fmt.Sprintf("%s: %s", title, body)
	return nil
}

func (s *Stub) Sample(sensor string) ([]float64, error) {
	if s.Fail {
		return nil, errors.New("device: sensor unavailable in stub")
	}
	if v, ok := s.Samples[sensor]; ok {
		return v, nil
	}
	return []float64{0, 0, 0}, nil
}

func (s *Stub) Authenticate(reason string) (bool, error) {
	if s.Fail {
		return false, errors.New("device: biometric unavailable in stub")
	}
	_ = reason
	return s.Authed, nil
}

func (s *Stub) ReadText() (string, error) {
	if s.Fail {
		return "", errors.New("device: clipboard unavailable in stub")
	}
	return s.Text, nil
}

func (s *Stub) WriteText(text string) error {
	if s.Fail {
		return errors.New("device: clipboard unavailable in stub")
	}
	s.Text = text
	return nil
}
