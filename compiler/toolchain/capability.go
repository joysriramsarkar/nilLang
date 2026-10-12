package toolchain

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Capability is one thing a tool can actually do.
//
// # Why a capability and not a boolean per tool
//
// "Is clang installed" and "can clang build a program" stopped being the same
// question the moment a build needed three operations instead of one. A current
// official LLVM installer for Windows ships `clang` and its LLVM libraries but
// neither the MSVC SDK headers nor `libcmt.lib`, so that same `clang`:
//
//   - reads textual IR and writes an object file      — yes
//   - compiles a C source file                        — no (`stdio.h` not found)
//   - links objects into an executable                 — no (`libcmt.lib` not found)
//
// Reporting "LLVM found" and then failing at the second step would be true and
// useless. Each operation is therefore discovered by trying it, once, and the
// answer is cached.
//
// # Why probe instead of infer
//
// The obvious inference — a C compiler must compile C — is false in exactly the
// case that matters, because the failure is a *missing SDK*, not a wrong
// compiler. There is no flag, no version and no path that says "this one lacks
// headers"; the only honest answer comes from running it.
type Capability string

const (
	// CapAssemble turns a textual `.ll` module into an object file.
	CapAssemble Capability = "read LLVM IR"

	// CapCompileC turns a `.c` source file into an object file. It is needed only
	// to build the runtime library, never to compile a Nilang program.
	CapCompileC Capability = "compile C"

	// CapLink turns object files into an executable.
	CapLink Capability = "link an executable"
)

// cProbeSource includes one standard header and uses it.
//
// The `#include` is the point: a compiler without a C library's headers fails
// here, which is exactly the failure a real build would hit. A probe that only
// defined `main` would pass on such a compiler and send the user to a later,
// more confusing failure.
const cProbeSource = `#include <stdio.h>
int main(void) { printf("ok"); return 0; }
`

// llProbeSource is a minimal but *real* module.
//
// It has a function with a parameter, an arithmetic instruction, a call and a
// terminator, because a probe that only checks "clang exists and parses an empty
// file" would pass on a build where `clang` is a wrapper script or a different
// tool entirely.
const llProbeSource = `define i64 @nil_probe(i64 %a, i64 %b) {
entry:
  %sum = add i64 %a, %b
  ret i64 %sum
}
`

// capabilityCache memoises one probe per (tool, capability).
//
// Process-wide rather than per-discovery because a build resolves the toolchain
// once and then asks about several operations; probing each time would run the
// C compiler two or three times for a single build.
var capabilityCache sync.Map // string -> bool

// Can reports whether a tool performs a capability on this machine.
//
// An absent tool can do nothing. The cache is keyed by the tool's full path, so
// two tools with the same name in different directories are answered separately —
// which matters on a machine with both an MSVC-targeting and a MinGW-targeting
// gcc on `PATH`.
func (t Tool) Can(capability Capability) bool {
	if !t.Available() {
		return false
	}

	key := t.Path + "|" + string(capability)
	if cached, seen := capabilityCache.Load(key); seen {
		return cached.(bool)
	}

	result := probe(t, capability)
	capabilityCache.Store(key, result)
	return result
}

// probe runs one tool once and reports whether it worked.
//
// The failure output is discarded deliberately. A capability probe that stored
// the error would need somewhere to put it — the caller asked a yes/no question,
// and the interesting failure is the *real* build's, which will carry the message
// with the right file names in it.
func probe(tool Tool, capability Capability) bool {
	dir, err := os.MkdirTemp("", "nilcap")
	if err != nil {
		return false
	}
	defer os.RemoveAll(dir)

	switch capability {
	case CapCompileC:
		source := filepath.Join(dir, "probe.c")
		if err := os.WriteFile(source, []byte(cProbeSource), 0o644); err != nil {
			return false
		}
		_, err := RunTool(tool, source, "-o", filepath.Join(dir, "probe.exe"))
		return err == nil

	case CapAssemble:
		source := filepath.Join(dir, "probe.ll")
		if err := os.WriteFile(source, []byte(llProbeSource), 0o644); err != nil {
			return false
		}
		_, err := RunTool(tool, "-c", "-x", "ir", source, "-o", filepath.Join(dir, "probe.o"))
		return err == nil

	case CapLink:
		// Linking is probed by linking: the two objects are the same ones the
		// probe would otherwise have to synthesise, and a link that succeeds
		// against an empty module has still not proved there is a C runtime to
		// pull in — which is the part that is actually missing on a machine
		// without the Windows SDK.
		source := filepath.Join(dir, "probe.c")
		if err := os.WriteFile(source, []byte(cProbeSource), 0o644); err != nil {
			return false
		}
		_, err := RunTool(tool, source, "-o", filepath.Join(dir, "probe.exe"))
		return err == nil
	}
	return false
}

// AsseblerLike is the tool that reads `.ll`.
//
// # Why a separate lookup and not "use clang"
//
// Because on this machine `clang` exists and works, and `llc` does not exist at
// all — and on a Linux box with a versioned install the reverse can hold. Writing
// the lookup once here means the AOT linker asks a question about the job rather
// than about a tool's name.
func (tc Toolchain) Assembler() (Tool, bool) {
	if tc.Clang.Can(CapAssemble) {
		return tc.Clang, true
	}
	if tc.LLC.Can(CapAssemble) {
		return tc.LLC, true
	}
	return Tool{}, false
}

// CCompiler is the tool that builds the runtime library.
//
// This is not the same tool as the assembler, on this machine and on others. The
// runtime is C because a garbage collector needs control over allocation and
// because its data structures are simpler to write and to audit in a language
// with no garbage collector of its own — but it is still C, and the LLVM
// distribution that compiles IR is not always one that compiles C.
func (tc Toolchain) CCompiler() (Tool, bool) {
	candidates := []Tool{tc.CC, tc.GCC, tc.Clang}
	for _, candidate := range candidates {
		if candidate.Can(CapCompileC) {
			return candidate, true
		}
	}
	return Tool{}, false
}

// Linker is the driver that produces an executable from objects.
//
// # Why the order is clang first
//
// When `clang` can link, it is the right choice: it knows the target triple the
// module was emitted for, and it will pass it to the linker. Falling back to `gcc`
// when `clang` cannot link is not a compromise about quality — the objects are
// COFF either way, and the driver that does have a C runtime is the one that has
// to supply it.
func (tc Toolchain) Linker() (Tool, bool) {
	candidates := []Tool{tc.Clang, tc.GCC, tc.CC}
	for _, candidate := range candidates {
		if candidate.Can(CapLink) {
			return candidate, true
		}
	}
	return Tool{}, false
}

// LinkReady reports whether a build can produce a runnable binary.
//
// Separate from `Ready`, which asks whether IR can be turned into objects. The
// distinction is the whole reason this file exists, and collapsing them would
// produce a build that reports success having produced nothing that can run.
func (tc Toolchain) LinkReady() bool {
	_, ok := tc.Linker()
	return ok
}

// LinkAdvice is the message shown when IR compiles but nothing can be linked.
//
// It names the specific missing piece rather than repeating "install LLVM". The
// machine in question already *has* LLVM — telling someone to install it is the
// advice that makes them give up, because they did and it changed nothing.
func (tc Toolchain) LinkAdvice() string {
	if tc.LinkReady() {
		return ""
	}

	var b strings.Builder
	b.WriteString("cannot link an executable: no tool found that can link objects.\n\n")

	found := 0
	b.WriteString("  found:\n")
	for _, tool := range tc.Tools() {
		if !tool.Available() {
			continue
		}
		found++
		if tool.Can(CapLink) {
			b.WriteString("    " + tool.Path + "  links\n")
			continue
		}
		b.WriteString("    " + tool.Path + "  cannot link")
		if tool.Can(CapCompileC) {
			b.WriteString(" (compiles C, but no C runtime to link against)")
		} else {
			b.WriteString(" (no C library headers, and no C runtime)")
		}
		b.WriteString("\n")
	}
	if found == 0 {
		b.WriteString("    nothing\n")
	}

	b.WriteString("\nThis is a missing C runtime, not a missing compiler.\n")
	b.WriteString("An LLVM install on Windows ships no MSVC SDK, so the linker\n")
	b.WriteString("reports `lld-link: error: could not open 'libcmt.lib'` — it has\n")
	b.WriteString("nothing to link the objects against. Either:\n\n")
	b.WriteString("  - install the MSVC build tools (Visual Studio, or the\n")
	b.WriteString("    standalone Build Tools), or\n")
	b.WriteString("  - install a gcc that carries its own runtime (MinGW-w64 or\n")
	b.WriteString("    w64devkit) and put gcc.exe on PATH; nil will use it.\n\n")
	b.WriteString("Confirm with:  nil build --print-toolchain\n")
	return b.String()
}
