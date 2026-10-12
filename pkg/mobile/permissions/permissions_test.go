package permissions

import (
	"strings"
	"testing"
)

func TestDeclareRequestFlow(t *testing.T) {
	m := New()
	m.Declare(Camera, Location)
	if got := m.Check(Camera); got != StatusUnknown {
		t.Errorf("before request status should be unknown, got %d", got)
	}
	if err := m.Request(Camera, true); err != nil {
		t.Fatal(err)
	}
	if got := m.Check(Camera); got != StatusGranted {
		t.Errorf("expected granted, got %d", got)
	}
	if err := m.Request(Location, false); err != nil {
		t.Fatal(err)
	}
	if got := m.Check(Location); got != StatusDenied {
		t.Errorf("expected denied, got %d", got)
	}
	if err := m.MarkPermanentlyDenied(Location); err != nil {
		t.Fatal(err)
	}
	if got := m.Check(Location); got != StatusPermanentlyDenied {
		t.Errorf("expected permanently denied, got %d", got)
	}
}

func TestUndeclaredRequestFails(t *testing.T) {
	m := New()
	m.Declare(Camera)
	if err := m.Request(Microphone, true); err == nil {
		t.Error("requesting an undeclared permission must fail")
	}
	if err := m.MarkPermanentlyDenied(Microphone); err == nil {
		t.Error("marking an undeclared permission must fail")
	}
}

func TestManifestEntries(t *testing.T) {
	m := New()
	m.Declare(Camera, Location)
	entries := m.ManifestEntries()
	joined := strings.Join(entries, "\n")
	for _, want := range []string{"android.permission.CAMERA", "android.permission.ACCESS_FINE_LOCATION", "android.permission.ACCESS_COARSE_LOCATION"} {
		if !strings.Contains(joined, want) {
			t.Errorf("manifest entries missing %q:\n%s", want, joined)
		}
	}
	if !strings.Contains(m.ManifestXML(), "<manifest>") {
		t.Error("ManifestXML must render a manifest element")
	}
}
