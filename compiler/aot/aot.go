// Package aot turns emitted LLVM IR into a runnable executable.
//
// # Where this sits in the pipeline
//
//	nilang source → checker → HIR → MIR → LLVM IR (.ll) → object → executable
//	                                                    ↑
//	                                          this package
//
// The last two steps are the ones that need a toolchain, and they are separate
// from every step before them for a reason: emitting IR needs nothing installed at
// all. A machine with no LLVM whatsoever can still produce a `.ll`, read it, and
// see exactly what the program compiles to.
//
// # Why an assembler *and* a linker driver
//
// Because they are different jobs with different requirements, and on a real
// machine at least one of them is somewhere other than where you would guess.
//
// Assembling `.ll` to an object file needs only LLVM — no C library, no headers,
// no startup files. Linking that object into an executable needs a C runtime to
// pull in, which an LLVM installation does not include. A machine can therefore
// have a perfectly good `clang` that assembles and cannot link, and a `gcc` that
// does both. The toolchain package discovers which is which by trying; this
// package just asks.
//
// # Why one invocation rather than two tools and a script
//
// Because the intermediate objects are the only thing either side agrees about,
// and because a user who installed Nilang has neither the source tree nor any idea
// which `.a` to pass to `ld`. The runtime is built here, from source, keyed on its
// contents — see the nilcore package.
package aot

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/llvm"
	"github.com/joysriramsarkar/nilLang/compiler/nilcore"
	"github.com/joysriramsarkar/nilLang/compiler/toolchain"
)

// Artifact is a compiled program: where it is and what went into it.
type Artifact struct {
	// Path is the executable.
	Path string

	// Object is the intermediate object file, kept so a caller can link it
	// somewhere else or inspect it. It is not deleted: the cost of keeping it is
	// a few hundred kilobytes and the cost of regenerating it is a compiler run.
	Object string

	// Module is the textual IR the object was made from, when the caller asked to
	// keep it.
	Module string

	// RuntimeFingerprint identifies the runtime the program was linked against.
	//
	// It is here rather than only in a log because it is the answer to "is this
	// binary the one I tested?" — a binary linked against a different runtime is a
	// different program, and nothing about it says so.
	RuntimeFingerprint string
}

// Size is the executable's size in bytes, or zero when it cannot be read.
func (a Artifact) Size() int64 {
	info, err := os.Stat(a.Path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// Options controls one link.
type Options struct {
	// OutputPath is where the executable goes. Required.
	OutputPath string

	// KeepModule writes the textual IR next to the output as `<name>.ll`.
	//
	// On by default in tests and off otherwise: a `.ll` is the only artifact that
	// can be read without a disassembler, and a user debugging a wrong answer is
	// exactly the user who wants it.
	KeepModule bool

	// RuntimeDir is where the runtime's intermediate objects are cached.
	//
	// Empty means a directory beside the output. That keeps a build self-contained:
	// the cache is next to the thing it built, and deleting the build directory
	// deletes the cache with it rather than leaving it in a temporary directory
	// that nobody will find again.
	RuntimeDir string

	// ExtraFlags are passed to the assembler and the linker driver, unexamined.
	ExtraFlags []string

	// CFlags are passed to the C compiler that builds the runtime, unexamined.
	//
	// Separate from `ExtraFlags` because they go to different tools and are wanted
	// at different times. A `-I` in `ExtraFlags` would reach the assembler as well,
	// which is harmless until it is not; and a project's `-std=c11` for its own C
	// has no business being applied to the runtime, whose code is written to a
	// fixed standard.
	CFlags []string

	// Libraries are native libraries to link, as they would be written on a
	// compiler's command line: `m`, `pthread`, `ws2_32`.
	//
	// They come from `extern "C"` declarations in the program and the `libraries`
	// list in `nil.json`. Both are needed and neither is enough: a declaration
	// says a symbol is used and the list says where it lives, and a build that
	// inferred one from the other would either link every library a program
	// mentions or fail to link one it needs.
	//
	// The platform's own spelling wins. `-lws2_32` on Windows and `-lws2_32` on
	// Linux are different libraries, and a name that only exists on one of them
	// should fail there rather than silently resolve elsewhere.
	Libraries []string

	// ExtraObjects are already-compiled objects to link, given as paths.
	//
	// Separate from `Libraries` because they answer a different question. A
	// library is a *name* the linker searches for; an object is a file someone
	// already built. `extern "C"` needs both and conflates them: a declaration
	// says a symbol is used, a library says where it comes from, and a project
	// that builds its own C from source has the object before it has a library.
	//
	// They come before the runtime's objects, which come before the program's, so
	// a project can override a runtime function by defining it — the same order
	// every linker sees for a program and the libraries it uses.
	ExtraObjects []string
}

// Build assembles and links a program from emitted IR.
//
// # Why the IR is written to a file rather than piped
//
// Because `clang -x ir -` is not reliable across versions, and because a linker
// error message that names a file the reader can open is worth more than one that
// names stdin. The file is also the artifact worth keeping: it is the only part of
// the build a person can read.
func Build(ir string, target llvm.Target, tc toolchain.Toolchain, options Options) (*Artifact, error) {
	if strings.TrimSpace(ir) == "" {
		return nil, fmt.Errorf("cannot build an empty module")
	}
	if options.OutputPath == "" {
		return nil, fmt.Errorf("no output path was given")
	}

	assembler, ok := tc.Assembler()
	if !ok {
		return nil, fmt.Errorf("cannot assemble for %s: no tool here reads LLVM IR", target.ID)
	}
	linker, ok := tc.Linker()
	if !ok {
		return nil, fmt.Errorf("%s", tc.LinkAdvice())
	}

	directory := filepath.Dir(options.OutputPath)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create the output directory: %w", err)
	}

	// The output's extension is stripped before the intermediate names are derived
	// from it, so `app.exe` and `app` produce the same `app.ll` and `app.o`.
	base := strings.TrimSuffix(filepath.Base(options.OutputPath), executableSuffix())
	modulePath := filepath.Join(directory, base+".ll")
	objectPath := filepath.Join(directory, base+".o")

	// Written with os.WriteFile rather than through a redirect: a shell redirect on
	// Windows produces UTF-16 with a byte-order mark, and LLVM rejects such a module
	// with an error about the first line that says nothing about the encoding.
	if err := os.WriteFile(modulePath, []byte(ir), 0o644); err != nil {
		return nil, fmt.Errorf("cannot write the IR module: %w", err)
	}

	// Step 1: IR to an object file.
	//
	// `-x ir` is explicit rather than relying on the `.ll` suffix, because a build
	// that writes to a temporary file with no suffix — which is what a caching
	// layer does — would otherwise have clang guess from the file *contents* and
	// occasionally guess wrong.
	assembleArgs := append([]string{
		"-c", "-x", "ir",
		modulePath,
		"-o", objectPath,
	}, options.ExtraFlags...)
	if output, err := toolchain.RunTool(assembler, assembleArgs...); err != nil {
		return nil, fmt.Errorf("assembling the module:\n%s", clean(output, err))
	}

	// Step 2: the runtime, built from source.
	runtimeDir := options.RuntimeDir
	if runtimeDir == "" {
		runtimeDir = filepath.Join(directory, ".nilcore")
	}
	library, err := nilcore.Build(runtimeDir, tc, options.CFlags)
	if err != nil {
		return nil, err
	}

	// Step 3: link.
	//
	// The runtime's objects come *after* the program's, so that if a program
	// defined a symbol the library also defines, the program's wins and the
	// library's is ignored — the same order a linker sees for a normal program and
	// its libraries, and the one that makes a program's own `print` an override
	// rather than a duplicate-symbol error.
	linkArgs := append([]string{"-o", options.OutputPath, objectPath}, library.Objects...)
	linkArgs = append(linkArgs, options.ExtraObjects...)
	linkArgs = append(linkArgs, libraryFlags(effectiveLibraries(target.OS, options.Libraries))...)
	linkArgs = append(linkArgs, options.ExtraFlags...)
	if output, err := toolchain.RunTool(linker, linkArgs...); err != nil {
		return nil, fmt.Errorf("linking the program:\n%s", clean(output, err))
	}

	artifact := &Artifact{
		Path:               options.OutputPath,
		Object:             objectPath,
		RuntimeFingerprint: library.Fingerprint,
	}
	if options.KeepModule {
		artifact.Module = modulePath
	} else {
		// The `.ll` is kept when the caller asked and removed otherwise, but the
		// object is not: it is small, and a linker error about it can only be
		// reproduced with it in hand.
		_ = os.Remove(modulePath)
	}
	return artifact, nil
}

// executableSuffix is the platform's executable extension, including the dot.
func executableSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// clean trims a tool's output down to what is worth reading.
//
// # Why nothing is dropped from the front
//
// Because a tool's real message is not necessarily where its first "error" is.
// `gcc` links through a driver: the linker's complaint about an undefined symbol
// comes first, and the driver's own `collect2: error: ld returned 1 exit status`
// comes last. Keeping from the first "error" onwards — which is what this function
// did originally — throws away the only line that names the symbol, and what
// remains says a link failed without saying why.
//
// So: one known-noise warning goes, everything else stays.
func clean(output string, err error) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return err.Error()
	}

	lines := strings.Split(trimmed, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		// A warning about the module triple is emitted on every module and says
		// nothing about this one: the host triple overrides the module's, which is
		// expected, because the module was emitted for the host.
		if strings.Contains(line, "Woverride-module") {
			continue
		}
		kept = append(kept, strings.TrimRight(line, "\r"))
	}

	if len(kept) == 0 {
		return err.Error()
	}
	return strings.Join(kept, "\n")
}
