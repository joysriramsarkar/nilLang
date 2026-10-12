package android

import (
	"archive/zip"
	"os"
	"strings"
	"testing"
)

func TestActivityTemplateHasLifecycle(t *testing.T) {
	for _, want := range []string{
		"class NilangActivity",
		"System.loadLibrary(\"nilang_core\")",
		"nativeInit", "nativeRunBundle", "nativeDispatchEvent", "nativeQuit",
		"onCreate", "onResume", "onPause", "onDestroy", "onLowMemory",
	} {
		if !strings.Contains(NilangActivityKt, want) {
			t.Errorf("activity template missing %q", want)
		}
	}
	if !strings.Contains(NilBridgeC, "nil_ffi_abi_v1") {
		t.Error("JNI bridge must reference the nil_ffi_abi_v1 link guard")
	}
	if !strings.Contains(NilBridgeC, "nil_runtime.h") {
		t.Error("JNI bridge must include nil_runtime.h")
	}
}

func TestCoreLibPaths(t *testing.T) {
	if got := CoreLibPath("arm64-v8a"); got != "lib/arm64-v8a/libnilang_core.so" {
		t.Errorf("bad core lib path: %s", got)
	}
	if len(SupportedABIs()) != 2 {
		t.Errorf("expected 2 ABIs, got %v", SupportedABIs())
	}
}

func writeTestAPK(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	defer w.Close()
	for name, content := range entries {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSizeReportSections(t *testing.T) {
	apk := t.TempDir() + "/s.apk"
	writeTestAPK(t, apk, map[string]string{
		"lib/arm64-v8a/libnilang_core.so": "SODATA",
		"assets/app.nilax":                "BUNDLE",
		"classes.dex":                     "DEX",
		"res/layout/main.xml":             "RES",
		"AndroidManifest.xml":             "MANIFEST",
	})
	rep, err := ReportSize(apk)
	if err != nil {
		t.Fatalf("ReportSize failed: %v", err)
	}
	got := map[string]int64{}
	for _, e := range rep.Entries {
		got[e.Section] = e.Bytes
	}
	for _, section := range []string{"native-libs", "nilax-bundle", "dex", "resources", "other"} {
		if _, ok := got[section]; !ok {
			t.Errorf("section %q missing from report", section)
		}
	}
	var total int64
	for _, n := range got {
		total += n
	}
	if rep.Total != total {
		t.Errorf("total %d != sum of sections %d", rep.Total, total)
	}
	if !rep.UnderBudget() {
		t.Error("tiny test APK should be under the 10MB budget")
	}
	if !strings.Contains(rep.String(), "APK size") {
		t.Error("report string should contain APK size line")
	}
}
