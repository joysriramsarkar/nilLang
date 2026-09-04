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
}
