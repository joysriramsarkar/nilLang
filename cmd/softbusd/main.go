package main

import (
	"flag"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/softbus"
)

func main() {
	// Parse flags
	deviceID := flag.String("id", "device-001", "Device ID")
	deviceName := flag.String("name", "Onuron Device", "Device name")
	deviceType := flag.String("type", "phone", "Device type (phone/tablet/desktop/watch)")
	port := flag.Int("port", 9000, "SoftBus port")
	version := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *version {
		fmt.Println("SoftBus Daemon v0.1.0")
		fmt.Println("Nilang Distributed Communication Protocol")
		os.Exit(0)
	}

	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║   🌐 SoftBus Daemon - Onuron OS          ║")
	fmt.Println("║   Distributed Device Communication       ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Println()

	// Initialize components
	fmt.Printf("📱 Device: %s (%s)\n", *deviceName, *deviceID)
	fmt.Printf("📡 Type: %s\n", *deviceType)
	fmt.Printf("🔌 Port: %d\n", *port)
	fmt.Println()

	// Step 1: Start discovery
	fmt.Print("   [1/4] Discovery service starting... ")
	discovery := softbus.NewDiscovery(*deviceID, *deviceName, *deviceType)
	if err := discovery.Start(); err != nil {
		fmt.Println("❌")
		fmt.Fprintf(os.Stderr, "❌ Discovery failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅")

	// Step 2: Start transport
	fmt.Print("   [2/4] Transport layer starting... ")
	transport := softbus.NewTransport(*deviceID, nil)
	if err := transport.Listen(*port); err != nil {
		fmt.Println("❌")
		fmt.Fprintf(os.Stderr, "❌ Transport failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅")

	// Step 3: Initialize session manager
	fmt.Print("   [3/4] Session manager starting... ")
	sessionMgr := softbus.NewSessionManager(*deviceID)
	_ = sessionMgr
	fmt.Println("✅")

	// Step 4: Initialize RPC server
	fmt.Print("   [4/4] RPC server starting... ")
	rpcServer := softbus.NewRPCServer()

	// Register custom RPC handlers
	rpcServer.Register("nilang.version", func(params json.RawMessage) (interface{}, error) {
		return map[string]string{
			"version":   "0.1.0",
			"compiler":  "nilc",
			"framework": "alap",
			"os":        "onuron",
		}, nil
	})

	rpcServer.Register("device.battery", func(params json.RawMessage) (interface{}, error) {
		return map[string]int{"level": 85, "charging": 0}, nil
	})

	rpcServer.Register("app.launch", func(params json.RawMessage) (interface{}, error) {
		return map[string]string{"status": "launched"}, nil
	})

	rpcServer.Register("screen.share", func(params json.RawMessage) (interface{}, error) {
		return map[string]string{"status": "sharing"}, nil
	})
	fmt.Println("✅")

	// Set up message handler
	transport.SetOnMessage(func(msg *softbus.Message) {
		fmt.Printf("📨 Message: %s from %s to %s\n", msg.Type, msg.SourceID, msg.DestID)

		switch msg.Type {
		case softbus.MsgRPC:
			var req softbus.RPCRequest
			if err := msg.GetPayload(&req); err == nil {
				resp := rpcServer.HandleRequest(&req)
				_ = resp // Would send response back
			}

		case softbus.MsgHeartbeat:
			// Handle heartbeat
			fmt.Printf("  💓 Heartbeat from %s\n", msg.SourceID)

		case softbus.MsgFileTransfer:
			fmt.Printf("  📁 File transfer request: %s\n", msg.Metadata["file_name"])
		}
	})

	// Register device found callback
	discovery.OnDeviceFound(func(device *softbus.DeviceInfo) {
		fmt.Printf("🔍 Device found: %s (%s) at %s:%d\n",
			device.DeviceName, device.DeviceType, device.IPAddress, device.Port)
	})

	// Cleanup on shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		fmt.Println("\n🛑 Shutting down SoftBus daemon...")
		discovery.Stop()
		transport.Stop()
		os.Exit(0)
	}()

	// Start heartbeat
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			heartbeat := &softbus.Heartbeat{
				DeviceID:  *deviceID,
				Timestamp: time.Now().UnixMilli(),
				Load:      0.1,
				Battery:   85,
			}
			_ = heartbeat // Would broadcast to connected devices
		}
	}()

	// Session cleanup
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			expired := sessionMgr.CleanupExpired()
			if expired > 0 {
				fmt.Printf("🧹 Cleaned up %d expired sessions\n", expired)
			}
		}
	}()

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════")
	fmt.Println("✅ SoftBus Daemon is running!")
	fmt.Printf("📡 Listening on port %d\n", *port)
	fmt.Println("🔍 Discovering nearby devices...")
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println("═══════════════════════════════════════════")

	// Keep running
	select {}
}