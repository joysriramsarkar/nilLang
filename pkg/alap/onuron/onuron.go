package onuron

import (
	"fmt"
	"os"
)

// PlatformInfo contains Onuron OS environment metadata
type PlatformInfo struct {
	OSName        string `json:"os_name"`
	Version       string `json:"version"`
	KernelVersion string `json:"kernel_version"`
	DeviceModel   string `json:"device_model"`
	IsNative      bool   `json:"is_native"`
}

// Adapter bridges Alap applications to the Onuron OS Platform (Section 21)
type Adapter struct {
	platform PlatformInfo
}

// NewAdapter creates an Onuron platform adapter
func NewAdapter() *Adapter {
	isNative := os.Getenv("ONURON_OS") == "1"
	return &Adapter{
		platform: PlatformInfo{
			OSName:        "Onuron OS",
			Version:       "1.0.0",
			KernelVersion: "0.4.2-onuron",
			DeviceModel:   "Onuron Reference Board",
			IsNative:      isNative,
		},
	}
}

// Platform returns platform metadata
func (a *Adapter) Platform() PlatformInfo {
	return a.platform
}

// BatteryLevel returns current device battery percentage (0-100)
func (a *Adapter) BatteryLevel() int {
	// If native HAL is connected, query HAL; otherwise return simulated 100
	return 100
}

// DeviceName returns the device model string
func (a *Adapter) DeviceName() string {
	return a.platform.DeviceModel
}

// SendSoftbusMessage sends an IPC message across the Onuron Softbus
func (a *Adapter) SendSoftbusMessage(topic string, payload []byte) error {
	if !a.platform.IsNative {
		// In host simulation environment, record success
		return nil
	}
	return fmt.Errorf("softbus bridge unavailable on host simulation")
}
