package harmonyos

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldGeneratesFiles(t *testing.T) {
	dir := t.TempDir()
	if err := Scaffold(dir, "HelloApp", "1.0.0"); err != nil {
		t.Fatalf("Scaffold failed: %v", err)
	}
	for _, rel := range []string{
		"entry/src/main/cpp/nilang_bridge.cpp",
		"entry/src/main/ets/EntryAbility.ets",
		"entry/module.json",
	} {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("generated file missing: %s", rel)
			continue
		}
		if len(data) == 0 {
			t.Errorf("generated file empty: %s", rel)
		}
	}
	bridge, _ := os.ReadFile(filepath.Join(dir, "entry", "src", "main", "cpp", "nilang_bridge.cpp"))
	if !strings.Contains(string(bridge), "nil_runtime.h") || !strings.Contains(string(bridge), "nil_ffi_abi_v1") {
		t.Error("NAPI bridge must include nil_runtime.h and the ABI guard")
	}
	manifest, _ := os.ReadFile(filepath.Join(dir, "entry", "module.json"))
	if !strings.Contains(string(manifest), "EntryAbility") || !strings.Contains(string(manifest), "1.0.0") {
		t.Error("module.json stub must name the ability and version")
	}
	entry, _ := os.ReadFile(filepath.Join(dir, "entry", "src", "main", "ets", "EntryAbility.ets"))
	if !strings.Contains(string(entry), "HelloApp") {
		t.Error("ArkTS entry must reference the app name")
	}
}
