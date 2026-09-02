package softbus

import (
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	MDNSPort          = 5353
	MDNSAddress       = "224.0.0.251"
	ServiceType       = "_nilang-softbus._tcp.local."
	DiscoveryInterval = 5 * time.Second
)

// Discovery handles device discovery via mDNS-SD
type Discovery struct {
	mu         sync.RWMutex
	deviceID   string
	deviceName string
	deviceType string
	devices    map[string]*DeviceInfo
	listeners  []chan *DeviceInfo
	running    bool
	stopChan   chan struct{}
}

// NewDiscovery creates a new discovery service
func NewDiscovery(deviceID, deviceName, deviceType string) *Discovery {
	return &Discovery{
		deviceID:   deviceID,
		deviceName: deviceName,
		deviceType: deviceType,
		devices:    make(map[string]*DeviceInfo),
		listeners:  []chan *DeviceInfo{},
		stopChan:   make(chan struct{}),
	}
}

// Start starts the discovery service
func (d *Discovery) Start() error {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return fmt.Errorf("discovery already running")
	}
	d.running = true
	d.mu.Unlock()

	fmt.Printf("🔍 SoftBus Discovery started (Device: %s)\n", d.deviceName)

	// Start mDNS advertisement
	go d.advertise()

	// Start mDNS browsing
	go d.browse()

	return nil
}

// Stop stops the discovery service
func (d *Discovery) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return
	}

	d.running = false
	close(d.stopChan)
	fmt.Println("🔍 SoftBus Discovery stopped")
}

// GetDevices returns all discovered devices
func (d *Discovery) GetDevices() []*DeviceInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()

	devices := make([]*DeviceInfo, 0, len(d.devices))
	for _, device := range d.devices {
		devices = append(devices, device)
	}
	return devices
}

// GetDevice returns a specific device by ID
func (d *Discovery) GetDevice(deviceID string) *DeviceInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.devices[deviceID]
}

// OnDeviceFound registers a callback for when a new device is found
func (d *Discovery) OnDeviceFound(callback func(*DeviceInfo)) {
	ch := make(chan *DeviceInfo, 10)
	d.mu.Lock()
	d.listeners = append(d.listeners, ch)
	d.mu.Unlock()

	go func() {
		for device := range ch {
			callback(device)
		}
	}()
}

// advertise advertises this device via mDNS
func (d *Discovery) advertise() {
	ticker := time.NewTicker(DiscoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopChan:
			return
		case <-ticker.C:
			d.sendAdvertisement()
		}
	}
}

func (d *Discovery) sendAdvertisement() {
	// In production: send mDNS-SD TXT record
	// For now, simulate
	addr := &net.UDPAddr{
		IP:   net.ParseIP(MDNSAddress),
		Port: MDNSPort,
	}

	_ = addr // Would use this to send mDNS packet
}

// browse browses for other devices via mDNS
func (d *Discovery) browse() {
	ticker := time.NewTicker(DiscoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopChan:
			return
		case <-ticker.C:
			d.sendBrowseQuery()
		}
	}
}

func (d *Discovery) sendBrowseQuery() {
	// In production: send mDNS-SD query and listen for responses
	// For now, simulate
}

// AddDevice manually adds a device (for testing)
func (d *Discovery) AddDevice(device *DeviceInfo) {
	d.mu.Lock()
	d.devices[device.DeviceID] = device
	d.mu.Unlock()

	// Notify listeners
	for _, listener := range d.listeners {
		select {
		case listener <- device:
		default:
		}
	}
}

// RemoveDevice removes a device
func (d *Discovery) RemoveDevice(deviceID string) {
	d.mu.Lock()
	delete(d.devices, deviceID)
	d.mu.Unlock()
}

// CleanupStale removes devices not seen for a while
func (d *Discovery) CleanupStale(timeout time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now().UnixMilli()
	for id, device := range d.devices {
		if now-device.LastSeen > timeout.Milliseconds() {
			delete(d.devices, id)
		}
	}
}
