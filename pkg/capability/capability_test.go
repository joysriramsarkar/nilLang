package capability

import (
	"testing"
)

func TestParseCapability(t *testing.T) {
	tests := []struct {
		input   string
		want    Type
		wantErr bool
	}{
		{"Network", CapNetwork, false},
		{"filesystem", CapFilesystem, false},
		{"GPU", CapGPU, false},
		// The inline vocabulary must be accepted too: a manifest that grants
		// "stdout" (needed by puts/println) is valid as far as this package
		// is concerned, whatever casing or separators are used.
		{"stdout", "stdout", false},
		{"STDOUT", "stdout", false},
		{"std_err", "stderr", false},
		{"clock", "clock", false},
		{"Environment", "environment", false},
		// pkg/capability calls location "GPS"; accept both spellings.
		{"location", CapGPS, false},
		{"gps", CapGPS, false},
		{"invalid_cap", "", true},
	}

	for _, tc := range tests {
		got, err := ParseCapability(tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseCapability(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			continue
		}
		if !tc.wantErr && got != tc.want {
			t.Errorf("ParseCapability(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseCapabilityRejectsBareInlineNamesWhenResourceIsWrong(t *testing.T) {
	// A resource name that is not in either vocabulary must still fail.
	if _, err := ParseCapability("definitely-not-a-capability"); err == nil {
		t.Errorf("expected an error for an unknown capability name")
	}
}

func TestVerifyProfileCapabilities(t *testing.T) {
	// Web profile cannot have Process or direct Filesystem
	webCaps := []Type{CapNetwork, CapCrypto, CapProcess}
	res := VerifyProfileCapabilities("web", webCaps)
	if res.Valid {
		t.Errorf("expected web profile with CapProcess to be invalid, got valid")
	}
	if len(res.Violations) == 0 {
		t.Errorf("expected violations for CapProcess on web")
	}

	// Server profile allows Network, Database, Filesystem
	serverCaps := []Type{CapNetwork, CapDatabase, CapFilesystem}
	resServer := VerifyProfileCapabilities("server", serverCaps)
	if !resServer.Valid {
		t.Errorf("expected server profile to be valid, got violations: %v", resServer.Violations)
	}

	// OS profile allows all
	osCaps := []Type{CapNetwork, CapCamera, CapFilesystem, CapProcess, CapGPU}
	resOS := VerifyProfileCapabilities("os", osCaps)
	if !resOS.Valid {
		t.Errorf("expected OS profile to allow all caps, got: %v", resOS.Violations)
	}

	// Inline process capabilities are allowed on every profile, including the
	// most restrictive ones — stdout printing is not a profile-gated resource.
	webInline := []Type{"stdout", "stderr", "stdin", "clock", "environment", "random"}
	resWebInline := VerifyProfileCapabilities("web", webInline)
	if !resWebInline.Valid {
		t.Errorf("expected web profile to allow inline capabilities, got: %v", resWebInline.Violations)
	}
	for _, want := range webInline {
		found := false
		for _, a := range resWebInline.Allowed {
			if a == string(want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected inline capability %q in Allowed, got %v", want, resWebInline.Allowed)
		}
	}
}
