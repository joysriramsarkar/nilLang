package toolchain

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/joysriramsarkar/nilLang/compiler/llvm"
)

// ErrNoToolchain is returned when no LLVM installation can be found.
//
// It is a sentinel rather than a formatted error so a caller can ask *whether* the
// problem was a missing toolchain — which has a fix, and a different fix depending
// on the platform — separately from reading the message that says which.
var ErrNoToolchain = errors.New("no LLVM toolchain found")

// Tool is one external program the build shells out to.
type Tool struct {
	// Name is the executable's file name, without a directory.
	Name string

	// Path is where it was found, or "" when it was not.
	Path string
}

// Available reports whether the tool was found.
func (t Tool) Available() bool { return t.Path != "" }

// Version is the tool's `--version` output, or "" when it could not be run.
//
// It is read once and kept: a build that asks `clang` for its version on every
// invocation is a build that spends most of its time waiting for a subprocess.
func (t Tool) Version() string {
	if t.Path == "" {
		return ""
	}
	if cached, known := toolVersions.Load(t.Path); known {
		return cached.(string)
	}

	ctx, cancel := context.WithTimeout(context.Background(), versionTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, t.Path, "--version").Output()
	if err != nil {
		toolVersions.Store(t.Path, "")
		return ""
	}

	// Only the first line is kept. `clang --version` is four lines long and the
	// second one names a specific build; a build log quoting all of it hides the
	// one fact that matters — which toolchain answered.
	version := firstLine(string(out))
	toolVersions.Store(t.Path, version)
	return version
}

// Toolchain is a resolved set of external tools for one target.
type Toolchain struct {
	Target llvm.Target

	// LLVMConfig finds the installation's version, layout and binaries. It is
	// optional: a complete LLVM *release* ships it, and the tool set here does not
	// need one.
	LLVMConfig Tool

	// LLC assembles textual IR to an object file. It is optional — see Clang.
	LLC Tool

	// Clang compiles, assembles and links.
	//
	// # Why this is the tool that matters and `llc` is only a fallback
	//
	// `clang` reads `.ll` directly and goes all the way to an executable in one
	// step. `llc` goes from `.ll` to an object file and then stops, so a build
	// using only `llc` needs a second linker invocation and a hand-written list of
	// the C runtime's objects.
	//
	// The distinction is not academic. Some LLVM distributions — including a
	// current official Windows installer — ship `clang` and its LLVM libraries
	// without shipping the `llc` binary, because it belongs to the compiler
	// development package rather than the end-user one. Requiring `llc` would
	// report "no toolchain" on machines that have a perfectly good compiler.
	Clang Tool

	// CC and GCC are C compilers. They build the runtime library, and they are
	// frequently the tool that *links*, because a distribution that ships LLVM for
	// IR does not always ship a C runtime for the linker to use.
	//
	// They are separate fields rather than one "C compiler" so that the message
	// printed on a machine missing everything can say which names were looked for
	// and which of them were present. See capability.go for why being installed is
	// not the same as being able to compile C.
	CC  Tool
	GCC Tool
}

// Tools is every tool the toolchain knows about, in the order a person reading
// the output would want them.
//
// `clang` is first because it is the one that decides whether the build can
// happen at all. Listing `llc` first would imply it is required, which on a
// distribution that omits it is exactly backwards.
func (tc Toolchain) Tools() []Tool {
	return []Tool{tc.Clang, tc.LLC, tc.GCC, tc.CC, tc.LLVMConfig}
}

// Ready reports whether the toolchain can build a binary for this target.
//
// Clang alone is enough, and its absence alone is disqualifying. `llc` is
// consulted only for builds that want the object files without the link.
func (tc Toolchain) Ready() bool {
	return tc.Clang.Available()
}

// LLCAvailable reports whether the standalone assembler was found.
func (tc Toolchain) LLCAvailable() bool { return tc.LLC.Available() }

// ClangAvailable reports whether the compiler/linker was found.
func (tc Toolchain) ClangAvailable() bool { return tc.Clang.Available() }

// Describe is the message shown when the toolchain is incomplete.
//
// It is a method rather than a package function because the advice differs by
// platform, and a build system that says "install LLVM" without saying *how* has
// moved the problem rather than solved it.
func (tc Toolchain) Describe() string {
	if tc.Ready() {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "cannot build for %s: no LLVM toolchain.\n\n", tc.Target.ID)
	fmt.Fprintf(&b, "  found: %s\n\n", describeFound(tc))

	b.WriteString("Nilang compiles to LLVM IR and uses the system toolchain to turn\n")
	b.WriteString("that into a binary. There is no bundled compiler, because a bundled\n")
	b.WriteString("LLVM would pin the compiler to one version of one C++ API on one\n")
	b.WriteString("platform.\n\n")

	b.WriteString("Install one of:\n")
	switch runtime.GOOS {
	case "windows":
		b.WriteString("  winget install LLVM.LLVM\n")
		b.WriteString("  or download from https://releases.llvm.org/ and add its bin\\\n")
		b.WriteString("     directory to PATH\n")
	case "darwin":
		b.WriteString("  brew install llvm\n")
		b.WriteString("  then: export PATH=\"$(brew --prefix llvm)/bin:$PATH\"\n")
	default:
		b.WriteString("  apt install llvm clang      # Debian, Ubuntu\n")
		b.WriteString("  dnf install llvm clang      # Fedora\n")
		b.WriteString("  pacman -S llvm clang        # Arch\n")
	}

	fmt.Fprintf(&b, "\nThen confirm with:  nil build --print-toolchain\n")
	return b.String()
}

func describeFound(tc Toolchain) string {
	var parts []string
	for _, tool := range tc.Tools() {
		if tool.Available() {
			parts = append(parts, tool.Name+" ("+tool.Path+")")
		}
	}
	if len(parts) == 0 {
		return "nothing"
	}
	return strings.Join(parts, ", ")
}

// CapabilityReport is the machine-readable answer to "what can this toolchain do".
//
// It exists so `--print-toolchain` and the tests read the same discovery the
// build does. A report printed from `Tool.Available()` would say "clang 22.1.8"
// on a machine where clang cannot compile a C file, and a reader who believed it
// would be misled about the very thing the report exists to tell them.
type CapabilityReport struct {
	Tool      string
	Path      string
	Version   string
	Installed bool
	ReadsIR   bool
	CompilesC bool
	Links     bool
}

// Capability reports what every discovered tool can actually do here.
func (tc Toolchain) Capabilities() []CapabilityReport {
	reports := make([]CapabilityReport, 0, len(tc.Tools()))
	for _, tool := range tc.Tools() {
		// A tool with no name is one discovery never looked for. `llvm-config` is
		// only consulted when something else found it, so on a machine without one
		// it is genuinely absent rather than "looked for and missing" — and a row
		// for it would say we searched for something we never tried.
		if tool.Name == "" {
			continue
		}
		reports = append(reports, CapabilityReport{
			Tool:      tool.Name,
			Path:      tool.Path,
			Version:   tool.Version(),
			Installed: tool.Available(),
			ReadsIR:   tool.Can(CapAssemble),
			CompilesC: tool.Can(CapCompileC),
			Links:     tool.Can(CapLink),
		})
	}
	return reports
}

// llvmConfigDirs are the places `llvm-config` lives when it is installed but not
// on `PATH`.
//
// They are searched because the common installation story on Linux and macOS is
// "installed, but the user never edited their shell profile" — and a build system
// that only looks at `PATH` reports "install LLVM" to someone who already did.
func llvmConfigDirs() []string {
	if runtime.GOOS == "windows" {
		return []string{
			`C:\Program Files\LLVM\bin`,
			`C:\Program Files (x86)\LLVM\bin`,
			`C:\LLVM\bin`,
		}
	}

	dirs := []string{
		"/usr/lib/llvm-19/bin",
		"/usr/lib/llvm-18/bin",
		"/usr/lib/llvm-17/bin",
		"/usr/lib/llvm-16/bin",
		"/usr/lib/llvm-15/bin",
		"/usr/lib/llvm-14/bin",
		"/usr/local/opt/llvm/bin", // Homebrew on Apple Silicon
		"/opt/homebrew/opt/llvm/bin",
		"/usr/local/bin",
		"/usr/bin",
	}
	return dirs
}

// FindToolchain locates the tools for a target.
//
// # Why the result is cached
//
// A build invokes this per target per build, and a miss costs a filesystem walk
// across every version directory above. The cache is keyed by the search path, so
// a developer who installs LLVM and rebuilds in the same terminal gets the new
// answer — which matters because the common reaction to "install LLVM" is to
// install it and try again immediately.
var (
	toolchainCache sync.Map // string (target ID) -> Toolchain
	toolVersions   sync.Map // string (tool path) -> string
)

// DiscoverToolchain finds the tools needed to build for a target.
//
// A missing tool is not an error here. The caller decides whether a missing tool
// matters, because it does not always: emitting IR needs nothing installed at all,
// and a build that only wants to look at the IR should work on a machine with no
// LLVM whatsoever.
func DiscoverToolchain(target llvm.Target) Toolchain {
	if cached, seen := toolchainCache.Load(target.ID); seen {
		return cached.(Toolchain)
	}

	tc := Toolchain{
		Target:     target,
		Clang:      findTool("clang"),
		LLC:        findTool("llc"),
		CC:         findTool("cc"),
		GCC:        findTool("gcc"),
		LLVMConfig: findTool("llvm-config"),
	}

	// `llvm-config` is the authority on an installation's layout. If it is
	// present, the tools are beside it — and that is how a versioned Linux
	// install is found at all, where the binaries have no unversioned symlink on
	// `PATH`.
	if cfg := findTool("llvm-config"); cfg.Available() {
		tc.LLVMConfig = cfg
		dir := filepath.Dir(cfg.Path)
		if llc, ok := lookBeside(dir, "llc"); ok {
			tc.LLC = Tool{Name: "llc", Path: llc}
		}
		if clang, ok := lookBeside(dir, "clang"); ok {
			tc.Clang = Tool{Name: "clang", Path: clang}
		}
	}

	toolchainCache.Store(target.ID, tc)
	return tc
}

// findTool looks on `PATH` first, then in the known installation directories.
//
// `PATH` comes first for a reason: a developer who has put a specific version
// there meant to use it, and a version in `/usr/lib/llvm-14/bin` does not get to
// override that because it was found first in a list.
func findTool(name string) Tool {
	if path, err := exec.LookPath(name); err == nil {
		return Tool{Name: name, Path: path}
	}
	for _, dir := range llvmConfigDirs() {
		if found, ok := lookBeside(dir, name); ok {
			return Tool{Name: name, Path: found}
		}
	}
	return Tool{Name: name}
}

// executableName is the platform's file name for a tool.
//
// Without the extension Windows will not run the file, and the search would report
// "no LLVM found" on a machine that has LLVM installed — the most confusing
// possible answer, because it is false.
func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func lookBeside(dir, name string) (string, bool) {
	candidate := filepath.Join(dir, executableName(name))
	info, err := os.Stat(candidate)
	if err != nil || info.IsDir() {
		return "", false
	}
	return candidate, true
}

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return strings.TrimSpace(s)
}

// RunTool executes a tool with arguments, returning its combined output on failure.
//
// The output is included in the error because a compiler's error message is the
// only thing that says *why*, and an error that says "llc failed" makes the reader
// go and run the command themselves to find out what they would have been told.
func RunTool(tool Tool, args ...string) (string, error) {
	if !tool.Available() {
		return "", fmt.Errorf("%w: %s is not installed", ErrNoToolchain, tool.Name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, tool.Path, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s %s: %w\n%s",
			filepath.Base(tool.Path), strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}
