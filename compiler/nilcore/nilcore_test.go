package nilcore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/llvm"
	"github.com/joysriramsarkar/nilLang/compiler/toolchain"
)

// hostToolchain is this machine's target and its tools.
//
// `llvm.TargetHost` rather than a lookup by triple: `LookupTarget` is keyed by ID
// and `HostTriple` is a triple, so the lookup finds nothing and a caller that then
// falls back to "the first target" gets aarch64-android on a Windows machine. A
// test that builds for the wrong architecture and passes is worse than no test.
func hostToolchain(t *testing.T) toolchain.Toolchain {
	t.Helper()
	target := llvm.TargetHost
	if target.ID == "" {
		t.Fatal("no host target is registered")
	}
	return toolchain.DiscoverToolchain(target)
}

// TestTheSourcesAreAllPresent is the first check, and it exists because a missing
// source would otherwise show up as a compiler error about a file nobody edited.
//
// # Why there is no count here
//
// Because there was one, and it said "one more than `sources` holds" — which was
// true while the FFI was a single file and stopped being true the moment it became
// two. The failure was real; what it taught was nothing, because the only way to make
// it pass was to change a number.
//
// That is the same shape as the stale conformance skip L5.7 found (ADR-0089): a check
// that fails on a change without saying what the change broke trains people to edit
// the check. So this one asks the question that is actually at issue — does every
// declared source exist, is each in a directory that is supposed to hold it, and is
// every FFI source reached — and adding a file now passes *because it was checked*,
// not in spite of it.
func TestTheSourcesAreAllPresent(t *testing.T) {
	paths := Sources()
	if len(paths) == 0 {
		t.Fatal("the runtime declares no sources")
	}

	seen := map[string]bool{}
	for _, path := range paths {
		if seen[path] {
			t.Errorf("a runtime source is built twice: %s", path)
		}
		seen[path] = true

		if _, err := os.Stat(path); err != nil {
			t.Errorf("runtime source is missing: %s (%v)", path, err)
		}
	}

	// Every declared name must actually be reached. A name in `sources` that no loop
	// over `allSources` picks up would build a runtime one file short of itself, and
	// the compiler error would name a function rather than a file.
	declared := map[string]bool{}
	for _, name := range allSources() {
		declared[name] = true
	}
	for _, name := range ffiSources {
		if !declared[name] {
			t.Errorf("the FFI declares %q but no build reaches it", name)
		}
	}

	for _, header := range []string{"nil.h", "nil_internal.h"} {
		path := filepath.Join(IncludeDir(), header)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("runtime header is missing: %s (%v)", path, err)
		}
	}

	// The FFI's own header, which generated code includes. It is checked separately
	// because a missing `nilffi.h` fails only in a module that uses `extern`, and
	// the first anyone would look is the runtime's own headers.
	if _, err := os.Stat(filepath.Join(FFIDir(), "nilffi.h")); err != nil {
		t.Errorf("FFI header is missing: %v", err)
	}
}

// TestTheFingerprintChangesWhenASourceChanges is the property the build cache
// rests on.
//
// Without it, editing `gc.c` would keep reusing objects compiled from the previous
// version and the test suite would pass against a library that was never rebuilt.
func TestTheFingerprintChangesWhenASourceChanges(t *testing.T) {
	tc := hostToolchain(t)
	cc, ok := tc.CCompiler()
	if !ok {
		t.Skip("no C compiler here")
	}

	base, err := Fingerprint(cc, "x86_64-windows", nil)
	if err != nil {
		t.Fatal(err)
	}

	// The same inputs must give the same key. A fingerprint that varied between
	// calls would make every build a miss.
	again, err := Fingerprint(cc, "x86_64-windows", nil)
	if err != nil {
		t.Fatal(err)
	}
	if base != again {
		t.Errorf("the fingerprint is not stable: %s then %s", base[:12], again[:12])
	}

	// A different target must give a different key. Sharing one would make a
	// cross-build link the host's runtime.
	crossed, err := Fingerprint(cc, "aarch64-apple-darwin", nil)
	if err != nil {
		t.Fatal(err)
	}
	if base == crossed {
		t.Error("the fingerprint ignores the target, so a cross-build reuses the host's runtime")
	}

	// A different compiler must give a different key.
	other, _ := Fingerprint(toolchain.Tool{Name: "clang", Path: cc.Path + "x"},
		"x86_64-windows", nil)
	if base == other {
		t.Error("the fingerprint ignores the compiler")
	}
}

// TestTheFingerprintCoversTheHeader is the case that would miscompile silently.
//
// Changing `nil.h` changes a struct layout. Every `.c` file depends on it. A
// fingerprint over the `.c` files alone would rebuild nothing and link objects
// compiled against the old layout.
func TestTheFingerprintCoversTheHeader(t *testing.T) {
	tc := hostToolchain(t)
	cc, ok := tc.CCompiler()
	if !ok {
		t.Skip("no C compiler here")
	}

	before, err := Fingerprint(cc, "x", nil)
	if err != nil {
		t.Fatal(err)
	}

	header := filepath.Join(IncludeDir(), "nil.h")
	original, err := os.ReadFile(header)
	if err != nil {
		t.Fatal(err)
	}
	// The write is undone by a deferred restore, not by a t.Cleanup, so that a
	// failure inside this test cannot leave the header modified for the next run.
	defer func() {
		if err := os.WriteFile(header, original, 0o644); err != nil {
			t.Fatalf("could not restore %s: %v", header, err)
		}
	}()

	if err := os.WriteFile(header, append(original, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	after, err := Fingerprint(cc, "x", nil)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Error("the fingerprint ignores nil.h, so a struct change would reuse stale objects")
	}
}

// TestTheRuntimeBuildsHere compiles the real runtime.
//
// This is the test that would have caught every compiler error in the C files. It is
// skipped rather than failed when no C compiler exists, because the ability to run the
// AOT tests is not the same as the ability to build Nilang itself, and a developer
// working on the compiler should not be blocked by the absence of gcc.
//
// # Why the comparison is against the declared list rather than a number
//
// Because a number goes stale the moment a file is added, and fixing it teaches
// nothing — see the note on `TestTheSourcesAreAllPresent`. What matters is that every
// declared source produced an object, and that a build which quietly dropped one would
// fail. Comparing to `allSources()` says exactly that.
func TestTheRuntimeBuildsHere(t *testing.T) {
	tc := hostToolchain(t)
	cc, ok := tc.CCompiler()
	if !ok {
		t.Skip("no C compiler on this machine")
	}
	t.Logf("building with %s", cc.Path)

	library, err := Build(t.TempDir(), tc, nil)
	if err != nil {
		t.Fatalf("the runtime does not build: %v", err)
	}

	declared := allSources()
	if len(library.Objects) != len(declared) {
		t.Errorf("built %d objects for %d declared sources (%v)",
			len(library.Objects), len(declared), declared)
	}
	for _, object := range library.Objects {
		if _, err := os.Stat(object); err != nil {
			t.Errorf("object was reported but is not there: %s", object)
		}
	}
}

// TestTheBuildIsCachedByContent builds twice and expects the second to reuse the
// first.
//
// Without the cache, every `nil build --aot` would recompile four C files before
// doing the work the user asked for. The test checks reuse by modification time
// rather than by a flag, because a flag would be a second piece of state that
// could disagree with the files on disk.
func TestTheBuildIsCachedByContent(t *testing.T) {
	tc := hostToolchain(t)
	if _, ok := tc.CCompiler(); !ok {
		t.Skip("no C compiler on this machine")
	}

	dir := t.TempDir()
	first, err := Build(dir, tc, nil)
	if err != nil {
		t.Fatal(err)
	}
	firstTimes := modificationTimes(t, first.Objects)

	second, err := Build(dir, tc, nil)
	if err != nil {
		t.Fatal(err)
	}
	secondTimes := modificationTimes(t, second.Objects)

	for path, when := range firstTimes {
		if secondTimes[path] != when {
			t.Errorf("%s was rebuilt although nothing changed", filepath.Base(path))
		}
	}
	if first.Fingerprint != second.Fingerprint {
		t.Errorf("two builds of the same sources disagree on the fingerprint")
	}
}

// TestTwoBuildsOfTheSameSourcesAgree is the other half of the cache test.
//
// If the fingerprint were stable but the rebuild still happened, the cache would
// be doing nothing. Together the two tests say the cache works and is correct,
// which are different properties and both of which matter.
func TestTwoBuildsOfTheSameSourcesAgree(t *testing.T) {
	tc := hostToolchain(t)
	if _, ok := tc.CCompiler(); !ok {
		t.Skip("no C compiler on this machine")
	}

	first, err := Build(t.TempDir(), tc, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Build(t.TempDir(), tc, nil)
	if err != nil {
		t.Fatal(err)
	}

	if first.Directory == second.Directory {
		t.Error("two independent build directories were given the same path")
	}
	for i := range first.Objects {
		if filepath.Base(first.Objects[i]) != filepath.Base(second.Objects[i]) {
			t.Errorf("object %d is named differently between builds: %s and %s",
				i, first.Objects[i], second.Objects[i])
		}
	}
}

// TestAMissingSourceIsAnErrorRatherThanAnEmptyLibrary.
//
// A build that produced zero objects and reported success would link, and the
// link would fail with "undefined symbol nil_print" — an error that points at the
// linker rather than at the thing that is actually wrong.
func TestAMissingSourceIsAnErrorRatherThanAnEmptyLibrary(t *testing.T) {
	tc := hostToolchain(t)
	if _, ok := tc.CCompiler(); !ok {
		t.Skip("no C compiler on this machine")
	}

	// The fingerprint is computed from a fixed list, so the way to make a source
	// missing is to move it. The move is undone before the test returns.
	path := SourcePath(sources[0])
	saved := path + ".moved"
	if err := os.Rename(path, saved); err != nil {
		t.Skipf("cannot rename the runtime source: %v", err)
	}
	defer os.Rename(saved, path)

	if _, err := Build(t.TempDir(), tc, nil); err == nil {
		t.Fatal("building with a missing source reported success")
	} else if !strings.Contains(err.Error(), sources[0]) {
		t.Errorf("the error does not name the missing file: %v", err)
	}
}

// TestDescribeSaysSomethingUseful covers the message a build failure prints.
func TestDescribeSaysSomethingUseful(t *testing.T) {
	if got := Describe(nil); !strings.Contains(got, "not built") {
		t.Errorf("Describe(nil) = %q", got)
	}

	tc := hostToolchain(t)
	if _, ok := tc.CCompiler(); !ok {
		t.Skip("no C compiler on this machine")
	}
	library, err := Build(t.TempDir(), tc, nil)
	if err != nil {
		t.Fatal(err)
	}
	description := Describe(library)
	if !strings.Contains(description, libraryName) {
		t.Errorf("Describe does not name the library: %q", description)
	}
	// The count is checked against the library rather than written out, because
	// this test is about `Describe` reporting what it built — not about how many
	// files the runtime has. `TestTheSourcesAreAllPresent` is where that count
	// belongs.
	if !strings.Contains(description, fmt.Sprintf("%d objects", len(library.Objects))) {
		t.Errorf("Describe does not count the objects: %q", description)
	}
}

func modificationTimes(t *testing.T, paths []string) map[string]int64 {
	t.Helper()
	times := make(map[string]int64, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		times[path] = info.ModTime().UnixNano()
	}
	return times
}
