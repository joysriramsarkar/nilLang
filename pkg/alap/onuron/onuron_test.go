package onuron

import (
	"testing"
)

func TestOnuronAdapter(t *testing.T) {
	adapter := NewAdapter()
	info := adapter.Platform()

	if info.OSName != "Onuron OS" {
		t.Errorf("expected OSName 'Onuron OS', got %s", info.OSName)
	}

	if adapter.BatteryLevel() <= 0 {
		t.Errorf("expected positive battery level")
	}

	if len(adapter.DeviceName()) == 0 {
		t.Errorf("expected non-empty device name")
	}

	if err := adapter.SendSoftbusMessage("test.topic", []byte("hello")); err != nil {
		t.Errorf("unexpected error in simulated softbus message: %v", err)
	}
}
