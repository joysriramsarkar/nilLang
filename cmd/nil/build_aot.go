package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/aot"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/llvm"
	"github.com/joysriramsarkar/nilLang/compiler/mir"
	"github.com/joysriramsarkar/nilLang/compiler/nilcore"
	"github.com/joysriramsarkar/nilLang/compiler/toolchain"
	"github.com/joysriramsarkar/nilLang/pkg/config"
)

// buildAOT compiles a program to a native executable.
//
// # Why this exists at all
//
// The AOT backend produced object files and links them — the linker was written,
// tested, and correct, and nothing in `nil` called it. A user with
// `extern "C" fn sin(x: Float) -> Float;` in their program had a declaration the
// language accepted and no way to build a program that used it.
//
// A backend nobody can reach is not a backend. It is a library.
//
// # Why it is a flag on `build` and not a separate command
//
// Building to a native executable is building. A separate word for it would mean
// two commands with two sets of options for the same pipeline, and the one that is
// less used is the one that stops working.
func buildAOT(projectDir string, outPath string, keepModule bool) string {
	cfg, err := config.LoadConfig(projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ nil.json পড়া যায়নি: %s\n", err)
		return ""
	}

	entry := filepath.Join(projectDir, cfg.Entry)
	source, err := os.ReadFile(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ এন্ট্রি পড়া যায়নি: %s\n", err)
		return ""
	}

	res := frontend.Check(string(source), frontend.Options{Filename: cfg.Entry})
	if !res.OK {
		fmt.Fprintln(os.Stderr, "❌ টাইপ ত্রুটি:")
		for _, d := range res.Diagnostics {
			fmt.Fprintf(os.Stderr, "   %s\n", d.String())
		}
		return ""
	}

	optimized := hir.NewOptimizer().Optimize(hir.NewLowerer().LowerProgram(res.Lowered))
	mirProg := mir.NewLowerer().LowerHIR(optimized)
	mir.NewOptimizer().OptimizeProgram(mirProg)

	module, err := llvm.NewEmitter(llvm.TargetHost).Emit(mirProg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ LLVM IR তৈরিতে সমস্যা: %s\n", err)
		return ""
	}

	tc := toolchain.DiscoverToolchain(llvm.TargetHost)

	// The libraries `nil.json` names, and only those.
	//
	// A declaration names a symbol; a library says where the symbol lives. The
	// declaration cannot say which — `sin` is in `libm` on Linux, in the C runtime
	// on Windows, and in `libSystem` on macOS — so the project states it and the
	// linker resolves it on the platform actually being built for.
	libraries := cfg.Build.Libraries

	if outPath == "" {
		outputDir := cfg.Build.OutputDir
		if outputDir == "" {
			outputDir = "build"
		}
		outPath = filepath.Join(projectDir, outputDir, cfg.Name+executableSuffix())
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "❌ বিল্ড ডিরেক্টরি তৈরি যায়নি: %s\n", err)
		return ""
	}

	objects, err := compileProjectC(projectDir, cfg, filepath.Dir(outPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ প্রজেক্টের C কম্পাইল ব্যর্থ: %s\n", err)
		return ""
	}

	artifact, err := aot.Build(module, llvm.TargetHost, tc, aot.Options{
		OutputPath:   outPath,
		RuntimeDir:   filepath.Join(projectDir, ".nilcore"),
		Libraries:    libraries,
		ExtraObjects: objects,
		CFlags:       cfg.Build.CFlags,
		KeepModule:   keepModule,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ নেটিভ বিল্ড ব্যর্থ: %s\n", err)
		return ""
	}

	fmt.Printf("বিল্ড সম্পন্ন: %s\n", artifact.Path)
	return artifact.Path
}

// compileProjectC builds the C a project wrote alongside its Nilang.
//
// # Why the compiler is found by capability
//
// `clang` on a machine with no Microsoft SDK can read LLVM IR and cannot compile
// C. Naming it directly would fail with a message about a missing SDK on a machine
// that has a perfectly good compiler under another name — which is not a bug in
// the project and reads like one. The toolchain already probes each tool for each
// operation, so this asks the same question the runtime's build asks.
//
// # Why the objects land beside the output
//
// Beside the output rather than in `.nilcore`, because they belong to the project
// and not to the runtime. A cache keyed on the runtime's fingerprint would rebuild
// them whenever the runtime changed, and one keyed on nothing would reuse an object
// compiled from a source that has since been edited.
//
// # Why `build/` and the runtime's headers are on the include path
//
// Because that is where `nil header` writes, and because the header it writes
// includes `nilffi.h`. A C file written against the generated header — the whole
// point of generating one, and the only way to write `total(const Named_t *)`
// correctly — starts with `#include "l57demo.h"` and got
//
//	l57demo.h: No such file or directory
//
// and then, once that was answered,
//
//	nilffi.h: No such file or directory
//
// from a compiler that was pointed at the right directory and told nothing else.
//
// The second one is the worse of the two, because the header promises to be
// includable and is not: `nilffi.h` is the file that defines `NilFFIString` and the
// whole contract, so there is no header that can be included without it.
//
// The project's own `c_flags` come *after* these and so win: a header the project
// ships under its own name should be the one it gets.
func compileProjectC(projectDir string, cfg *config.ProjectConfig, outputDir string) ([]string, error) {
	sources, err := projectCSources(projectDir, cfg.Build.CSources)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, nil
	}

	tc := toolchain.DiscoverToolchain(llvm.TargetHost)
	cc, ok := tc.CCompiler()
	if !ok {
		return nil, fmt.Errorf("no C compiler found; %s declares C sources but this machine has none",
			cfg.Name)
	}

	objects := make([]string, 0, len(sources))
	for _, source := range sources {
		// The object name is derived from the source path, so `src/mathx.c` and
		// `vendor/mathx.c` do not write over each other. The separator is folded to
		// `_` because a path separator is not a legal character in a filename on
		// every platform.
		name := strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(source)
		object := filepath.Join(outputDir, name+".o")

		args := []string{"-c", source, "-o", object, "-O2", "-fPIC"}
		args = append(args, projectCIncludes(outputDir)...)
		args = append(args, cfg.Build.CFlags...)
		if output, err := toolchain.RunTool(cc, args...); err != nil {
			return nil, fmt.Errorf("compiling %s: %s", source, strings.TrimSpace(firstLine(output, err)))
		}
		objects = append(objects, object)
	}
	return objects, nil
}

// projectCIncludes is the `-I` list every project C source is compiled with.
//
// A function rather than a literal inside `compileProjectC` so that a test can ask
// the question directly: *is the header a project's C file includes reachable from
// here?* The question is worth asking because the answer was twice no — first `build/`
// was missing and then `nilffi.h` was — and both failures surfaced only as a compiler
// error a user has to decode, naming a file they never mentioned.
func projectCIncludes(outputDir string) []string {
	return []string{
		"-I", outputDir,
		"-I", nilcore.IncludeDir(),
		"-I", nilcore.FFIDir(),
	}
}

// projectCSources expands `c_sources` into a list of `.c` files.
//
// A directory means every `.c` file directly inside it, sorted. Sorted because a
// directory walk is not ordered, and an unordered list makes two builds of the same
// project differ for no reason a reader could see.
func projectCSources(projectDir string, entries []string) ([]string, error) {
	var out []string
	seen := map[string]bool{}

	add := func(path string) {
		absolute := filepath.Join(projectDir, filepath.FromSlash(path))
		if seen[absolute] {
			return
		}
		seen[absolute] = true
		out = append(out, absolute)
	}

	for _, entry := range entries {
		absolute := filepath.Join(projectDir, filepath.FromSlash(entry))
		info, err := os.Stat(absolute)
		if err != nil {
			return nil, fmt.Errorf("%s is declared in c_sources but is not there: %w", entry, err)
		}
		if !info.IsDir() {
			add(entry)
			continue
		}

		items, err := os.ReadDir(absolute)
		if err != nil {
			return nil, fmt.Errorf("cannot read %s: %w", entry, err)
		}
		var inside []string
		for _, item := range items {
			if item.IsDir() || !strings.EqualFold(filepath.Ext(item.Name()), ".c") {
				continue
			}
			inside = append(inside, filepath.ToSlash(filepath.Join(entry, item.Name())))
		}
		sort.Strings(inside)
		for _, path := range inside {
			add(path)
		}
	}
	return out, nil
}

// firstLine is the first line of a tool's output, which is where its real message
// is.
//
// gcc and clang put theirs first and follow it with a `collect2: error` line that
// says nothing new. Reporting only the first line is what keeps a build error
// readable — and taking everything before the first "error" is what keeps the
// useful part when the driver's message is not the first one.
func firstLine(output string, err error) string {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		return trimmed
	}
	if err != nil {
		return err.Error()
	}
	return "the tool failed without saying why"
}

// projectDirFrom returns the directory the command line names, or "".
//
// Read separately from the main argument loop because `--aot` is handled before
// that loop runs: a native build is one file and skips the per-target bundle path
// entirely. Without its own reader, `--dir` would be ignored by `build --aot` and
// the build would silently happen in the current directory — which succeeds, and
// writes its output somewhere the person did not ask for.
//
// The spellings are the ones the main loop accepts, so `--dir=x`, `--dir x` and
// `-dir x` all work here too.
func projectDirFrom(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		lower := strings.ToLower(arg)
		switch {
		case strings.HasPrefix(lower, "--dir="):
			return arg[len("--dir="):]
		case strings.HasPrefix(lower, "-dir="):
			return arg[len("-dir="):]
		case lower == "--dir" || lower == "-dir":
			if i+1 < len(args) {
				return args[i+1]
			}
		}
	}
	return ""
}

// executableSuffix is the platform's executable extension, including the dot.
func executableSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// hasAOTFlag reports whether the command line asks for a native build.
//
// Case-insensitively: the rest of `build` accepts `-target=` and `--target=`, and a
// flag that works in one spelling and not the other is a flag somebody types wrong
// once and then works around forever.
func hasAOTFlag(args []string) bool {
	for _, arg := range args {
		if strings.EqualFold(arg, "--aot") {
			return true
		}
	}
	return false
}

// aotOutputPath returns the value of `--out=`, or "" when none was given.
func aotOutputPath(args []string) string {
	for _, arg := range args {
		if len(arg) > len("--out=") && strings.EqualFold(arg[:len("--out=")], "--out=") {
			return arg[len("--out="):]
		}
	}
	return ""
}

// keepsModule reports whether `--keep-module` was asked for.
//
// The `.ll` is the only part of a native build a person can read without a
// disassembler, so keeping it is how somebody investigates a wrong answer.
func keepsModule(args []string) bool {
	for _, arg := range args {
		if strings.EqualFold(arg, "--keep-module") {
			return true
		}
	}
	return false
}
