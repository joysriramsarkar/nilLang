// Package ios scaffolds the Nilang iOS embedder (L8.4): a Swift package
// that hosts the Nilang core behind an Objective-C bridge header, built for
// `arm64-apple-ios` and distributed as an XCFramework stub.
package ios

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TargetConfig is the LLVM-style target triple for iOS devices.
type TargetConfig struct {
	ID        string
	Triple    string
	Arch      string
	OSVersion string
}

// AppleSiliconTarget is the device target `nil build ios` compiles for.
var AppleSiliconTarget = TargetConfig{
	ID:        "arm64-apple-ios",
	Triple:    "arm64-apple-ios",
	Arch:      "arm64",
	OSVersion: "17.0",
}

// BridgeHeader is the Objective-C bridge exposing nil_runtime.h to Swift.
const BridgeHeader = `// Nilang-Bridging-Header.h — iOS embedder bridge (L8.4).
#import "nil_runtime.h"

int32_t NilangRunBundle(const uint8_t *data, int64_t len);
void NilangDispatchEvent(const char *kind, int64_t kindLen,
                         const char *payload, int64_t payloadLen);
`

// SwiftPackageManifest renders Package.swift for the generated app package.
func SwiftPackageManifest(appName string) string {
	return fmt.Sprintf(`// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: %[1]q,
    platforms: [.iOS(.v17)],
    targets: [
        .target(
            name: %[1]q,
            dependencies: ["NilangCore"],
            cSettings: [.headerSearchPath("Bridging")]
        ),
        .binaryTarget(
            name: "NilangCore",
            path: "NilangCore.xcframework"
        ),
    ]
)
`, appName)
}

// SwiftAppEntry renders the @main SwiftUI entry that boots the bundle.
func SwiftAppEntry(appName string) string {
	return fmt.Sprintf(`import SwiftUI

// %s — Nilang iOS host (L8.4). The Nilang bundle (app.nilax) ships in the
// app resources; all calls go through the nil_runtime.h C ABI.
@main
struct %sApp: App {
    init() {
        NilangBootstrap.run()
    }
    var body: some Scene {
        WindowGroup {
            NilangHostView()
        }
    }
}
`, appName, appName)
}

// XCFrameworkStub describes the XCFramework layout `nil build ios` produces.
// A real .xcframework needs Xcode; the stub records the layout so CI without
// macOS can still verify file placement (risk register: macOS unavailable).
type XCFrameworkStub struct {
	AppName string
	LibName string
}

// Files returns the relative paths the stub must contain.
func (x XCFrameworkStub) Files() []string {
	return []string{
		"NilangCore.xcframework/Info.plist",
		"NilangCore.xcframework/ios-arm64/libnilang_core.a",
		"NilangCore.xcframework/ios-arm64/Headers/nil_runtime.h",
		"Bridging/Nilang-Bridging-Header.h",
		"Package.swift",
		"Sources/" + x.AppName + "/App.swift",
	}
}

// Scaffold writes the Swift package scaffold into dir.
func Scaffold(dir, appName string) error {
	stub := XCFrameworkStub{AppName: appName, LibName: "libnilang_core.a"}
	files := map[string]string{
		"Package.swift":                                          SwiftPackageManifest(appName),
		"Bridging/Nilang-Bridging-Header.h":                      BridgeHeader,
		"Sources/" + appName + "/App.swift":                      SwiftAppEntry(appName),
		"NilangCore.xcframework/Info.plist":                      xcInfoPlist(appName),
		"NilangCore.xcframework/ios-arm64/Headers/nil_runtime.h": "// vendored nil_runtime.h — see runtime/core/include/nil_runtime.h\n",
	}
	// The static lib itself is a build artefact; record a placeholder note.
	files["NilangCore.xcframework/ios-arm64/libnilang_core.a.PLACEHOLDER"] = "built by: nil build ios (requires Xcode/macOS)\n"
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			return err
		}
	}
	_ = stub
	return nil
}

func xcInfoPlist(appName string) string {
	return strings.Join([]string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<plist version="1.0"><dict>`,
		`  <key>CFBundleName</key><string>` + appName + `</string>`,
		`  <key>XCFrameworkFormatVersion</key><string>1.0</string>`,
		`</dict></plist>`,
		``,
	}, "\n")
}
