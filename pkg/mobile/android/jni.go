// Package android embeds the Nilang JNI bridge and APK size reporting
// (L8.3). The Kotlin activity template is a Go string asset so that
// `nil build android` can scaffold a project without shipping extra files.
package android

import (
	"archive/zip"
	"fmt"
	"os"
	"sort"
	"strings"
)

// NilangActivityKt is the Kotlin embedder template. It loads
// `libnilang_core.so`, runs the bundled `.nilax` on a worker thread and
// forwards lifecycle events through the `nil_runtime.h` C ABI.
const NilangActivityKt = `package dev.nilang.app

import android.app.Activity
import android.os.Bundle

// NilangActivity: minimal embedder for a Nilang bundle (L8.3).
// The APK packages assets/app.nilax plus lib/<abi>/libnilang_core.so.
// All runtime calls go through the nil_runtime.h C ABI via JNI.
class NilangActivity : Activity() {

    companion object {
        init {
            System.loadLibrary("nilang_core")
        }
    }

    private external fun nativeInit(): Int
    private external fun nativeRunBundle(assetPath: String): Int
    private external fun nativeDispatchEvent(kind: String, payload: String): Int
    private external fun nativeQuit()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        if (nativeInit() != 0) { finish(); return }
        Thread {
            val rc = nativeRunBundle("app.nilax")
            if (rc != 0) runOnUiThread { finish() }
        }.start()
    }

    override fun onResume() {
        super.onResume()
        nativeDispatchEvent("resume", "{}")
    }

    override fun onPause() {
        nativeDispatchEvent("pause", "{}")
        super.onPause()
    }

    override fun onDestroy() {
        nativeDispatchEvent("destroy", "{}")
        nativeQuit()
        super.onDestroy()
    }

    override fun onLowMemory() {
        super.onLowMemory()
        nativeDispatchEvent("lowMemory", "{}")
    }
}
`

// NilBridgeC is the JNI stub compiled into the app's native layer. Each
// function forwards to the matching nil_runtime.h entry point; the core
// library itself stays ABI-stable behind it.
const NilBridgeC = `#include <jni.h>
#include "nil_runtime.h"

extern int32_t nil_init(void);

JNIEXPORT jint JNICALL
Java_dev_nilang_app_NilangActivity_nativeInit(JNIEnv *env, jobject thiz) {
    nil_ffi_abi_v1();
    return nil_init();
}
`

// CoreLibPath returns the in-APK path for the native core per ABI.
func CoreLibPath(abi string) string {
	return "lib/" + abi + "/libnilang_core.so"
}

// SupportedABIs lists the ABIs packaged for release builds.
func SupportedABIs() []string { return []string{"arm64-v8a", "x86_64"} }

// SizeEntry is one APK section's contribution to install size.
type SizeEntry struct {
	Section string
	Bytes   int64
}

// SizeReport breaks an APK into sections (native libs, bundle, dex, rest)
// so `nil build android --size` can show where the <10MB budget goes.
type SizeReport struct {
	Total   int64
	Entries []SizeEntry
}

// ReportSize reads an APK (zip) and attributes every entry to a section.
func ReportSize(apkPath string) (*SizeReport, error) {
	zr, err := zip.OpenReader(apkPath)
	if err != nil {
		return nil, fmt.Errorf("open apk: %w", err)
	}
	defer zr.Close()

	rep := &SizeReport{}
	bySection := map[string]int64{}
	for _, f := range zr.File {
		size := int64(f.UncompressedSize64)
		if size == 0 {
			size = int64(f.CompressedSize64)
		}
		var section string
		switch {
		case strings.HasPrefix(f.Name, "lib/"):
			section = "native-libs"
		case strings.HasPrefix(f.Name, "assets/"):
			section = "nilax-bundle"
		case strings.HasSuffix(f.Name, ".dex"):
			section = "dex"
		case strings.HasPrefix(f.Name, "res/") || strings.HasPrefix(f.Name, "resources."):
			section = "resources"
		default:
			section = "other"
		}
		bySection[section] += size
		rep.Total += size
	}
	for section, n := range bySection {
		rep.Entries = append(rep.Entries, SizeEntry{Section: section, Bytes: n})
	}
	sort.Slice(rep.Entries, func(i, j int) bool { return rep.Entries[i].Section < rep.Entries[j].Section })
	return rep, nil
}

// UnderBudget reports whether the APK meets the <10MB store target (M4).
func (r *SizeReport) UnderBudget() bool { return r.Total < 10*1024*1024 }

// String renders the report the way `nil build android --size` prints it.
func (r *SizeReport) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "APK size: %.2f MB\n", float64(r.Total)/(1024*1024))
	for _, e := range r.Entries {
		fmt.Fprintf(&b, "  %-13s %8.2f MB\n", e.Section, float64(e.Bytes)/(1024*1024))
	}
	if r.UnderBudget() {
		b.WriteString("budget: OK (<10MB)\n")
	} else {
		b.WriteString("budget: OVER 10MB — strip debug symbols or split ABIs\n")
	}
	return b.String()
}

// StatSize is a convenience for tests and CLI output.
func StatSize(path string) (int64, error) {
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}
