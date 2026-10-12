package device

import (
	"testing"
)

func stubAPIs() *APIs {
	s := &Stub{Fix: Location{Latitude: 23.7, Longitude: 90.4}, Authed: true}
	return &APIs{Camera: s, Locator: s, Notifier: s, Sensor: s, Biometric: s, Clipboard: s}
}

func TestStubRoundTrip(t *testing.T) {
	a := stubAPIs()
	stub := a.Camera.(*Stub)

	photo, err := a.Camera.Capture()
	if err != nil || len(photo) == 0 {
		t.Fatalf("capture failed: %v", err)
	}
	loc, err := a.Locator.LastLocation()
	if err != nil || loc.Latitude != 23.7 {
		t.Fatalf("location failed: %v %+v", err, loc)
	}
	if err := a.Notifier.Notify("hi", "there"); err != nil {
		t.Fatalf("notify failed: %v", err)
	}
	if stub.Notification != "hi: there" {
		t.Errorf("notification not recorded: %q", stub.Notification)
	}
	sample, err := a.Sensor.Sample("accelerometer")
	if err != nil || len(sample) != 3 {
		t.Fatalf("sample failed: %v %v", err, sample)
	}
	ok, err := a.Biometric.Authenticate("unlock")
	if err != nil || !ok {
		t.Fatalf("auth failed: %v %v", err, ok)
	}
	if err := a.Clipboard.WriteText("copy me"); err != nil {
		t.Fatal(err)
	}
	text, err := a.Clipboard.ReadText()
	if err != nil || text != "copy me" {
		t.Fatalf("clipboard failed: %v %q", err, text)
	}
}

func TestStubFailure(t *testing.T) {
	s := &Stub{Fail: true}
	a := &APIs{Camera: s, Locator: s, Notifier: s, Sensor: s, Biometric: s, Clipboard: s}
	if _, err := a.Camera.Capture(); err == nil {
		t.Error("failing stub capture must error")
	}
	if _, err := a.Locator.LastLocation(); err == nil {
		t.Error("failing stub location must error")
	}
	if err := a.Notifier.Notify("t", "b"); err == nil {
		t.Error("failing stub notify must error")
	}
	if _, err := a.Sensor.Sample("gyro"); err == nil {
		t.Error("failing stub sensor must error")
	}
	if _, err := a.Biometric.Authenticate("r"); err == nil {
		t.Error("failing stub auth must error")
	}
	if err := a.Clipboard.WriteText("x"); err == nil {
		t.Error("failing stub clipboard must error")
	}
}

func TestManifestEntries(t *testing.T) {
	a := stubAPIs()
	entries := a.ManifestEntries()
	want := []string{
		"android.permission.ACCESS_FINE_LOCATION",
		"android.permission.CAMERA",
		"android.permission.POST_NOTIFICATIONS",
		"android.permission.USE_BIOMETRIC",
	}
	if len(entries) != len(want) {
		t.Fatalf("expected %d entries, got %v", len(want), entries)
	}
	for i := range want {
		if entries[i] != want[i] {
			t.Fatalf("entries mismatch: got %v want %v", entries, want)
		}
	}
	// No APIs wired: no permissions needed.
	empty := (&APIs{}).ManifestEntries()
	if len(empty) != 0 {
		t.Errorf("empty APIs should need no permissions, got %v", empty)
	}
}
