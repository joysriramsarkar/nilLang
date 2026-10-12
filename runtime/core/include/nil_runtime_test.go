package include

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestNilRuntimeHeaderSymbols checks L8.1: the embedder C ABI header exists
// and declares exactly the promised surface.
func TestNilRuntimeHeaderSymbols(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	headerPath := filepath.Join(filepath.Dir(thisFile), "nil_runtime.h")
	data, err := os.ReadFile(headerPath)
	if err != nil {
		t.Fatalf("nil_runtime.h missing: %v", err)
	}
	src := string(data)

	required := []string{
		"#ifndef NIL_RUNTIME_H",
		"nil_ffi_abi_v1",
		"nil_init",
		"nil_run_bundle",
		"nil_dispatch_event",
		"nil_set_callback",
		"nil_quit",
		"NilRuntimeString",
		"NilRuntimeBundle",
		"NilRuntimeEvent",
		"NilRuntimeCallback",
		"nil_runtime_string_len",
		"nil_runtime_string_equal",
		"NIL_RUNTIME_ABI_VERSION",
	}
	for _, sym := range required {
		if !strings.Contains(src, sym) {
			t.Errorf("nil_runtime.h missing required symbol %q", sym)
		}
	}
}
