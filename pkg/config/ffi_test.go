package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTheFFIFieldsSurviveARoundTrip is the smallest possible check, and the one
// that catches the most.
//
// A JSON tag that does not match the field name reads back as nothing. The
// project then builds without the library it declared, or without the C source it
// declared, and the failure appears as an undefined symbol from a linker rather
// than as a missing line in a file the person wrote.
func TestTheFFIFieldsSurviveARoundTrip(t *testing.T) {
	dir := t.TempDir()

	original := &ProjectConfig{
		Name:  "ffi",
		Entry: "src/main.nil",
		Build: BuildConfig{
			OutputDir: "build",
			Libraries: []string{"m", "pthread"},
			CSources:  []string{"c", "vendor/extra.c"},
			CFlags:    []string{"-std=c11", "-Ivendor/include"},
		},
	}
	if err := original.Save(dir); err != nil {
		t.Fatal(err)
	}

	read, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(read.Build.Libraries) != 2 {
		t.Errorf("libraries came back as %q", read.Build.Libraries)
	}
	if len(read.Build.CSources) != 2 {
		t.Errorf("c_sources came back as %q", read.Build.CSources)
	}
	if len(read.Build.CFlags) != 2 {
		t.Errorf("c_flags came back as %q", read.Build.CFlags)
	}
}

// TestABOMDoesNotStopAProjectFromLoading is a bug this suite found.
//
// Every tool on Windows writes a UTF-8 byte-order mark by default: PowerShell's
// `Out-File`, `Set-Content -Encoding utf8`, older Visual Studio, most editors'
// "save with encoding" menus. `encoding/json` stops at the first byte it cannot
// use, so a `nil.json` written with any of them was rejected with an error naming
// a character nobody deliberately typed.
//
// It is worth a test because it is invisible on the machine that writes the file
// with a Go tool and obvious on the one that does not, and because "rejected every
// project's config file" is the kind of failure a user hits once and then works
// around by not using the language.
func TestABOMDoesNotStopAProjectFromLoading(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nil.json")

	// The three bytes a BOM is, then ordinary JSON.
	content := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"name":"bom","entry":"src/main.nil","targets":["linux"]}`)...)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("a nil.json with a byte-order mark was rejected: %v", err)
	}
	if loaded.Name != "bom" {
		t.Errorf("the project loaded as %q, wanted \"bom\"", loaded.Name)
	}
}

// TestAProjectWithNoFFIFieldsStillLoads keeps the new fields optional.
//
// They are `omitempty` and default to nil, so every existing `nil.json` keeps
// working unchanged. A field that had to be written would make the FFI a
// prerequisite for every project rather than an opt-in.
func TestAProjectWithNoFFIFieldsStillLoads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nil.json")

	content := `{"name":"plain","entry":"src/main.nil","targets":["linux"]}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("loading failed: %v", err)
	}
	if len(loaded.Build.Libraries) != 0 || len(loaded.Build.CSources) != 0 {
		t.Errorf("a project with no FFI fields came back with some: %+v", loaded.Build)
	}
}
