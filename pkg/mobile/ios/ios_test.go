package ios

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTargetConfig(t *testing.T) {
	if AppleSiliconTarget.Triple != "arm64-apple-ios" {
		t.Errorf("wrong triple: %s", AppleSiliconTarget.Triple)
	}
	if AppleSiliconTarget.Arch != "arm64" {
		t.Errorf("wrong arch: %s", AppleSiliconTarget.Arch)
	}
}

func TestScaffoldGeneratesFiles(t *testing.T) {
	dir := t.TempDir()
	if err := Scaffold(dir, "HelloApp"); err != nil {
		t.Fatalf("Scaffold failed: %v", err)
	}
	stub := XCFrameworkStub{AppName: "HelloApp"}
	for _, rel := range stub.Files() {
		// The static lib is a build artefact; its placeholder stands in.
		check := rel
		if strings.HasSuffix(rel, ".a") {
			check = rel + ".PLACEHOLDER"
		}
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(check))); err != nil {
			t.Errorf("generated file missing: %s", rel)
		}
	}
	manifest, _ := os.ReadFile(filepath.Join(dir, "Package.swift"))
	if !strings.Contains(string(manifest), "HelloApp") || !strings.Contains(string(manifest), "NilangCore") {
		t.Error("Package.swift missing app name or NilangCore target")
	}
	bridge, _ := os.ReadFile(filepath.Join(dir, "Bridging", "Nilang-Bridging-Header.h"))
	if !strings.Contains(string(bridge), "nil_runtime.h") {
		t.Error("bridge header must import nil_runtime.h")
	}
	entry, _ := os.ReadFile(filepath.Join(dir, "Sources", "HelloApp", "App.swift"))
	if !strings.Contains(string(entry), "NilangHostView") {
		t.Error("Swift entry must reference NilangHostView")
	}
}
