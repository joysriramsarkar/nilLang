// Package nilcore builds and locates the Nilang runtime library.
//
// # What this package is for
//
// A Nilang program compiled ahead of time is machine code. It has no interpreter,
// no type tags it can inspect on its own, and no way to ask "what is this value?"
// without a function call. Everything it needs at run time — the representation of
// a value, the garbage collector, strings, lists, maps, printing — lives in a small
// C library, and this package builds it and hands the linker the result.
//
// The library is C rather than Go for three reasons, each of which is a constraint
// rather than a preference:
//
//   - a Go runtime cannot be linked into a C program, so a Go library here would
//     mean the AOT backend could only produce Go binaries;
//   - the collector needs to control every allocation, and a host runtime that
//     also allocates makes that control unprovable;
//   - C is the one language every LLVM target already has a compiler for, so a
//     build for iOS, Android or a microcontroller needs nothing installed beyond
//     the toolchain Nilang already requires.
//
// # Why the build is not checked in
//
// A prebuilt .a would be built by one compiler for one platform and would be
// linked into programs targeting another. The four source files compile in
// under a second, so building them on demand costs less than the confusion a
// stale archive causes.
package nilcore

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/toolchain"
)

// libraryName is the base name of the built runtime library.
const libraryName = "nilcore"

// sources are the files of the runtime, in a fixed order.
//
// The order matters for two reasons: the build is cached on their contents, and a
// file list built from a directory walk would depend on the filesystem's sort
// order. A cache key that changes when the same sources are rebuilt is a cache
// that never hits.
//
// `ffi.c` is here rather than in a library of its own because it is part of the
// runtime: the conversions at the C boundary are what a Nilang value is, seen from
// the other side, and splitting them off would make "what does the runtime do"
// have two answers.
var sources = []string{
	"gc.c",
	"value.c",
	"collections.c",
	"output.c",
}

// ffiSources is the FFI's half of the runtime, which lives beside the core rather than
// inside it.
//
// They are separate from `sources` because they have their own include directory:
// these files include `nilffi.h`, and generated code includes that header too, while
// nothing in `runtime/core` should be able to reach it. Keeping the two directories
// apart is what makes that separation real rather than a convention.
//
// A list rather than one name, because "the FFI" is now more than one translation
// unit. `ffi.c` holds the conversions; `channel.c` holds the message codec and the
// platform channel, which is a different concern with a different dependency — it
// needs locks and it builds values — and folding it into `ffi.c` would make a file
// about crossing a boundary also be a file about waiting for one.
var ffiSources = []string{"ffi.c", "channel.c"}

// isFFISource reports whether a source name belongs to the FFI half.
func isFFISource(name string) bool {
	for _, candidate := range ffiSources {
		if candidate == name {
			return true
		}
	}
	return false
}

// FFIDir is where `nilffi.h` lives.
func FFIDir() string {
	return filepath.Join(filepath.Dir(SourceDir()), "ffi", "include")
}

// FFISourcePath is the full path of one of the FFI's source files.
func FFISourcePath(name string) string {
	return filepath.Join(filepath.Dir(SourceDir()), "ffi", "src", name)
}

// SourceDir is where the runtime's C sources and its public header live.
//
// It is located relative to this file rather than to the working directory,
// because `nil` is run from every directory a project has and a build that worked
// from the repository root and not from `src/` would be unusable.
func SourceDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	// .../compiler/nilcore/nilcore.go -> .../runtime/core
	return filepath.Join(filepath.Dir(file), "..", "..", "runtime", "core")
}

// IncludeDir is where `nil.h` lives.
func IncludeDir() string { return filepath.Join(SourceDir(), "include") }

// SourcePath is the full path of one runtime source file.
func SourcePath(name string) string { return filepath.Join(SourceDir(), "src", name) }

// Sources returns every source file, absolute.
//
// It reads the fixed list rather than the directory, so a stray `.c` file someone
// left in the tree is not silently compiled into the runtime — which is how a
// debug helper in the runtime directory ends up in a shipped library.
func Sources() []string {
	paths := make([]string, 0, len(allSources()))
	for _, name := range allSources() {
		if isFFISource(name) {
			paths = append(paths, FFISourcePath(name))
			continue
		}
		paths = append(paths, SourcePath(name))
	}
	// The FFI last, and always: its object is linked after the rest, so a
	// conversion that names a runtime function resolves to the runtime's and not
	// to a copy a future edit might introduce.
	return paths
}

// Library is a built runtime library, ready to be linked.
type Library struct {
	// Objects are the compiled object files, in a stable order.
	//
	// Objects rather than an archive: an archive's contents depend on the order
	// the archiver was given its inputs, and two builds of the same sources can
	// produce different bytes. The linker does not care, and a hash that changes
	// on every build is a hash nothing can rely on.
	Objects []string

	// Archive is the `.a` produced from Objects, if one was made.
	Archive string

	// Directory holds every intermediate file, for a caller that wants to keep it.
	Directory string

	// Fingerprint is the hash of the sources and the build settings.
	//
	// It is the cache key. Two builds with the same fingerprint compiled the same
	// code with the same compiler flags, so the objects can be reused — and a
	// changed header changes it, which is the case that matters: editing `nil.h`
	// without touching a `.c` file has to invalidate the build.
	Fingerprint string
}

// Fingerprint is the identity of a build: the sources, the header, and the flags.
//
// # Why the header is in it
//
// The object files depend on the header's contents, not only on the `.c` files.
// A cache keyed on the `.c` files alone would reuse an object compiled against an
// older `nil.h` after the struct layout changed, and the result would be a
// miscompile with no diagnostic at all.
func Fingerprint(cc toolchain.Tool, target string, extraFlags []string) (string, error) {
	hash := sha256.New()

	files := append(Sources(),
		filepath.Join(IncludeDir(), "nil.h"),
		filepath.Join(IncludeDir(), "nil_internal.h"),
		filepath.Join(FFIDir(), "nilffi.h"))
	sort.Strings(files)

	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("cannot read runtime source %s: %w", filepath.Base(path), err)
		}
		fmt.Fprintf(hash, "file %s\n", filepath.Base(path))
		hash.Write(content)
	}

	// The compiler's identity is in the key because the same sources compiled by
	// two compilers are two builds. Including the version rather than the path
	// means a reinstall that changes the path does not invalidate the cache, while
	// an upgrade that changes the version does.
	fmt.Fprintf(hash, "cc %s %s\n", cc.Path, cc.Version())
	fmt.Fprintf(hash, "target %s\n", target)
	fmt.Fprintf(hash, "host %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(hash, "flags %s\n", strings.Join(extraFlags, " "))

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Build compiles the runtime library into `dir`.
//
// # Why it compiles every time the fingerprint changes and not otherwise
//
// A cache is only worth having if a build is repeatable, and a build is not
// repeatable if the cache key is not exactly what determines the output. Rather
// than a timestamp — which changes when a file is touched without being edited,
// and so rebuilds needlessly — the key is the content of everything that reaches
// the compiler.
//
// # The flags
//
// `-O2` rather than `-O0`: the runtime is on the hot path of every arithmetic
// operation a compiled program performs, and an unoptimised collector shows up in
// every benchmark the project will ever run.
//
// `-fno-strict-aliasing`: the collector casts between `NilObject *` and the
// concrete struct types. The header is laid out so every cast is to the same
// address and the first member, which is what makes it correct — but a compiler
// that assumes two pointer types cannot alias will reorder a write across a
// collection. The cost is a few percent; the alternative is a bug that appears
// only at -O2 and only sometimes.
//
// No `-march=native` and no `-ffast-math`: the library is built on the machine
// that runs it and handed to a program compiled for a *target*, so a flag that
// tunes for this CPU would produce a binary that faults on another.
func Build(dir string, tc toolchain.Toolchain, extraFlags []string) (*Library, error) {
	cc, ok := tc.CCompiler()
	if !ok {
		return nil, fmt.Errorf("%w: no C compiler found; lib%s is written in C",
			toolchain.ErrNoToolchain, libraryName)
	}

	fingerprint, err := Fingerprint(cc, tc.Target.ID, extraFlags)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create the runtime build directory: %w", err)
	}

	// The build directory is keyed by the fingerprint, so two targets — or two
	// compiler versions — never share an object file. A single shared directory
	// would make a cross-build silently link the host's runtime.
	buildDir := filepath.Join(dir, fingerprint[:16])

	library := &Library{
		Directory:   buildDir,
		Fingerprint: fingerprint,
	}

	if cached, err := readMarker(buildDir); err == nil && cached == fingerprint {
		// The same list the build path produces, or the two disagree and a cached
		// build links a different set of objects than a fresh one.
		//
		// They did disagree: the check walked `sources`, which does not hold the
		// FFI's file, so a second build of an unchanged program linked without it.
		// Every `extern` call then failed to link, and only on the *second* build —
		// the worst possible moment, because the first one had worked.
		objects, complete := cachedObjects(buildDir)
		if complete {
			library.Objects = objects
			return library, nil
		}
	}

	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create the runtime build directory: %w", err)
	}

	flags := append([]string{
		"-c",
		"-O2",
		"-fno-strict-aliasing",
		"-std=c11",
		"-Wall",
		"-Wextra",
		"-Wno-unused-parameter",
		"-fPIC",
		"-I", IncludeDir(),
		"-I", FFIDir(),
	}, extraFlags...)

	for _, name := range allSources() {
		source := SourcePath(name)
		if isFFISource(name) {
			source = FFISourcePath(name)
		}
		object := filepath.Join(buildDir, objectName(name))

		args := append(append([]string{}, flags...), source, "-o", object)
		if output, err := toolchain.RunTool(cc, args...); err != nil {
			return nil, fmt.Errorf("building %s: %w", name, err)
		} else if warnings := significant(output, cc.Name); warnings != "" {
			// Warnings from the runtime are build output the user asked for by
			// building it, and a runtime that compiles with warnings is a runtime
			// whose next warning will be missed.
			fmt.Fprintf(os.Stderr, "nilcore: %s\n%s\n", name, warnings)
		}
		library.Objects = append(library.Objects, object)
	}

	if err := writeMarker(buildDir, fingerprint); err != nil {
		return nil, err
	}
	return library, nil
}

// significant filters compiler output down to what a reader should act on.
//
// The build is quiet on success, so this only ever produces output when something
// is wrong. `-Wno-unused-parameter` keeps the noise down at the source; this is the
// second half of the same idea.
func significant(output, toolName string) string {
	var kept []string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.Contains(trimmed, "Woverride-module") {
			continue
		}
		kept = append(kept, trimmed)
	}
	if len(kept) == 0 {
		return ""
	}
	return toolName + ": " + strings.Join(kept, "\n")
}

// objectName is one source file's object file name.
func objectName(source string) string {
	return strings.TrimSuffix(filepath.Base(source), ".c") + ".o"
}

// allSources is every source the runtime is built from, in the order they are
// compiled.
//
// One list, used by both the build path and the cache check. Two lists would be
// two answers to "what does this library contain", and they would drift the first
// time a source was added to one of them — which is exactly what happened when the
// FFI's file was added to the build and not to the check.
func allSources() []string {
	return append(append([]string(nil), sources...), ffiSources...)
}

// cachedObjects is the object list for a build that is already on disk, and
// whether every one of them is actually there.
//
// "Actually there" rather than "should be": a build directory can be half-written
// if a previous build was interrupted, and linking a library with a missing member
// fails with a message about an undefined symbol rather than about the interrupted
// build that caused it.
func cachedObjects(buildDir string) ([]string, bool) {
	objects := make([]string, 0, len(allSources()))
	for _, name := range allSources() {
		path := filepath.Join(buildDir, objectName(name))
		if _, err := os.Stat(path); err != nil {
			return nil, false
		}
		objects = append(objects, path)
	}
	return objects, true
}

// readMarker reports the fingerprint a previous build left behind.
func readMarker(dir string) (string, error) {
	content, err := os.ReadFile(filepath.Join(dir, ".fingerprint"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

// writeMarker records the fingerprint a build produced.
func writeMarker(dir, fingerprint string) error {
	return os.WriteFile(filepath.Join(dir, ".fingerprint"), []byte(fingerprint), 0o644)
}

// Describe is the one-line summary used in error messages and in
// `--print-toolchain`.
//
// It names the C compiler and says where the sources are, because the two
// questions a reader has are always "which compiler built this" and "can I change
// the code".
func Describe(library *Library) string {
	if library == nil {
		return libraryName + " (not built)"
	}
	return fmt.Sprintf("%s: %d objects, %s", libraryName, len(library.Objects),
		library.Fingerprint[:12])
}
