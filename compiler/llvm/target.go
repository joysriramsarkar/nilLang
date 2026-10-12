package llvm

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
)

// OS is an operating system a target can build for.
type OS string

const (
	OSLinux   OS = "linux"
	OSWindows OS = "windows"
	OSMacOS   OS = "macos"
	OSAndroid OS = "android"
	OSIOS     OS = "ios"
	OSWASM    OS = "wasm"
	// OSOnuron is the platform this compiler is written for. It is a Linux ABI,
	// and naming it separately keeps a Nilang-specific build from silently
	// claiming to be a generic Linux one — which matters when the two diverge.
	OSOnuron OS = "onuron"
)

// Arch is a processor architecture a target can build for.
type Arch string

const (
	ArchX86_64  Arch = "x86_64"
	ArchAArch64 Arch = "aarch64"
	ArchWASM32  Arch = "wasm32"
	ArchARM64   Arch = "arm64"
	ArchX86     Arch = "x86"
)

// PointerSize is the width of a pointer in bytes.
//
// It is a field rather than something derived, because the one target where it
// cannot be derived is 32-bit: nothing else in the target list is 32-bit today,
// and a target added later should not have to re-derive what it should have said.
type PointerSize int

const (
	Pointer32 PointerSize = 4
	Pointer64 PointerSize = 8
)

// Target is everything the emitter needs to know to write IR for one destination.
//
// # Why a struct and not a name
//
// A name is what a developer writes; the rest is what the emitter reads. Resolving
// one to the other once, here, means every function in the package takes a target
// that is complete — there is no second lookup table somewhere that could disagree
// about what `aarch64-android` means.
type Target struct {
	// ID is the name a developer writes: `aarch64-android`.
	ID string

	OS   OS
	Arch Arch

	// Triple is the LLVM target triple.
	Triple string

	// DataLayout is LLVM's data layout string.
	//
	// It is written out rather than composed from the other fields because LLVM
	// does not derive it, and a layout that is *nearly* right produces code that
	// mis-strides an array rather than failing to assemble.
	DataLayout string

	// PointerSize is 4 or 8 bytes.
	PointerSize PointerSize

	// ObjectFormat is the file the assembler writes: `elf`, `macho`, `coff`,
	// `wasm`.
	ObjectFormat string

	// ExecutableSuffix is what a linked binary ends with on this platform.
	ExecutableSuffix string

	// SharedLibrarySuffix and StaticLibrarySuffix are what a library ends with.
	SharedLibrarySuffix string
	StaticLibrarySuffix string
	ExecutableExtension string
}

// Features are the CPU features a target enables in its module header.
//
// The empty string is meaningful: a module with no features assumes the baseline
// the target triple names, which is what makes the emitted IR portable across the
// machines of one architecture.
func (t Target) Features() string {
	if t.Arch == ArchWASM32 {
		return ""
	}
	return "target-features=" + t.cpuFeatures()
}

func (t Target) cpuFeatures() string {
	switch t.Arch {
	case ArchX86_64, ArchX86:
		return "+sse2"
	case ArchAArch64, ArchARM64:
		return "+neon"
	}
	return ""
}

// PointerType is the LLVM type of a pointer on this target.
func (t Target) PointerType() string {
	switch t.PointerSize {
	case Pointer32:
		return "i32"
	default:
		return "i64"
	}
}

// IsNative reports whether a target is the one this binary is running on.
//
// "Native" is what decides whether a build can skip a cross-compilation step, so
// it is asked often and answered from the build-time values rather than by
// running a compiler. A wrong answer here produces a binary for the wrong
// machine, which is the failure a build system must never have silently.
func (t Target) IsNative() bool {
	return t.Triple == hostTriple
}

// targets is every target the compiler knows, keyed by the name a developer types.
//
// It is a map rather than a slice so a lookup cannot silently return the wrong
// target: an unknown name is not "the first one that is not nil" but an error.
// TargetHost is the target for the machine this binary runs on.
//
// It is registered rather than synthesised because a build that says it is
// producing `x86_64-windows` and then reports a different target is confusing in a
// way that is hard to diagnose later — and because the host is by far the most
// common build target, so it is the one whose absence a developer notices first.
//
// It is added to the table during initialisation, once the runtime's own
// architecture is known.
var TargetHost Target

// WindowsMSVCTriple is the triple every Windows LLVM toolchain recognises.
//
// It is spelled out here rather than composed because the parts are not
// independent: `-pc-windows` is not a triple LLVM accepts, and `-pc-windows-msvc`
// is not derivable from `GOOS` without also knowing the ABI, which on Windows is
// the default and on some cross-compilation setups is not.
const WindowsMSVCTriple = "x86_64-pc-windows-msvc"

// init registers the host target with the table.
//
// It is registered rather than kept separate so that `--print-targets` lists it and
// `LookupTarget` resolves it by name. A host that is *not* in the table is a
// target a developer cannot ask the compiler to build for — strange, for the most
// common case of all.
func init() {
	TargetHost = register(computeTarget())
}

// computeTarget builds the host's Target from the runtime's own description.
//
// It is a function rather than a reference to `HostTarget` because Go does not order
// variable initialisers across files: `HostTarget` is populated by a variable
// initialiser, and a package-level `init` that reads it may run first. Reading it
// would then register a zero target under an empty name, and every later use of
// `HostTarget` would be a target with no triple.
func computeTarget() Target {
	for _, t := range targets {
		if t.Triple == hostTriple {
			return t
		}
	}
	// The host's platform has no row in the table — Go supports it, this compiler
	// does not list it yet. A synthesised entry keeps a build working rather than
	// refusing on an unlisted platform, and its ID says which platform it is.
	return Target{
		ID:           runtime.GOOS + "-" + runtime.GOARCH,
		OS:           OS(runtime.GOOS),
		Arch:         Arch(llvmArchName(runtime.GOARCH)),
		Triple:       hostTriple,
		DataLayout:   defaultDataLayout(runtime.GOARCH),
		PointerSize:  hostPointerSize(),
		ObjectFormat: hostObjectFormat(),
	}
}

// targets is every target the compiler knows, keyed by the name a developer types.
//
// It is a map rather than a slice so a lookup cannot silently return the wrong
// target: an unknown name is an error, not "the first one that happens to match".
var targets = map[string]Target{}

func register(t Target) Target {
	targets[t.ID] = t
	return t
}

var (
	// TargetX8664Linux is the ordinary desktop Linux build.
	TargetX8664Linux = register(Target{
		ID: "x86_64-linux", OS: OSLinux, Arch: ArchX86_64,
		Triple:      "x86_64-unknown-linux-gnu",
		DataLayout:  "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-f80:128-n8:16:32:64-S128",
		PointerSize: Pointer64, ObjectFormat: "elf",
		ExecutableSuffix: "", SharedLibrarySuffix: ".so", StaticLibrarySuffix: ".a",
	})

	// TargetAArch64Linux is server and desktop ARM.
	TargetAArch64Linux = register(Target{
		ID: "aarch64-linux", OS: OSLinux, Arch: ArchAArch64,
		Triple:      "aarch64-unknown-linux-gnu",
		DataLayout:  "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n32:64-S128",
		PointerSize: Pointer64, ObjectFormat: "elf",
		ExecutableSuffix: "", SharedLibrarySuffix: ".so", StaticLibrarySuffix: ".a",
	})

	// TargetAArch64Android is the phone target. Its triple uses the Android ABI
	// rather than the Linux one: the two differ in how they pass structs and in
	// which C library is linked, so a triple copied from desktop Linux produces
	// a binary that links and then misbehaves.
	TargetAArch64Android = register(Target{
		ID: "aarch64-android", OS: OSAndroid, Arch: ArchAArch64,
		Triple:      "aarch64-unknown-linux-android",
		DataLayout:  "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n32:64-S128",
		PointerSize: Pointer64, ObjectFormat: "elf",
		ExecutableSuffix: "", SharedLibrarySuffix: ".so", StaticLibrarySuffix: ".a",
	})

	// TargetAArch64IOS is iOS. The triple pins the Apple OS version because the
	// ABI is not stable across it.
	TargetAArch64IOS = register(Target{
		ID: "aarch64-apple-ios", OS: OSIOS, Arch: ArchAArch64,
		Triple:      "arm64-apple-ios",
		DataLayout:  "e-m:o-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n32:64-S128",
		PointerSize: Pointer64, ObjectFormat: "macho",
		ExecutableSuffix: "", SharedLibrarySuffix: ".dylib", StaticLibrarySuffix: ".a",
	})

	// TargetX8664Windows is 64-bit Windows.
	TargetX8664Windows = register(Target{
		ID: "x86_64-windows", OS: OSWindows, Arch: ArchX86_64,
		Triple:      "x86_64-pc-windows-msvc",
		DataLayout:  "e-m:w-p270:32:32-p271:32:32-p272:64:64-i64:64-f80:128-n8:16:32:64-S128",
		PointerSize: Pointer64, ObjectFormat: "coff",
		ExecutableSuffix: ".exe", SharedLibrarySuffix: ".dll", StaticLibrarySuffix: ".lib",
	})

	// TargetWASM32 is the browser build.
	TargetWASM32 = register(Target{
		ID: "wasm32", OS: OSWASM, Arch: ArchWASM32,
		Triple:      "wasm32-unknown-unknown",
		DataLayout:  "e-m:e-p:32:32-p270:32:32-p271:32:32-p272:64:64-i64:64-n32:64-S128",
		PointerSize: Pointer32, ObjectFormat: "wasm",
		ExecutableSuffix: ".wasm", SharedLibrarySuffix: ".so", StaticLibrarySuffix: ".a",
	})

	// TargetAArch64MacOS is Apple Silicon macOS. Its triple is `arm64` while the
	// architecture field is `aarch64`, because that is what LLVM calls it — the
	// two names are the same architecture and using the wrong one produces a
	// target triple LLVM rejects.
	TargetAArch64MacOS = register(Target{
		ID: "aarch64-macos", OS: OSMacOS, Arch: ArchAArch64,
		Triple:      "arm64-apple-macosx",
		DataLayout:  "e-m:o-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n32:64-S128",
		PointerSize: Pointer64, ObjectFormat: "macho",
		ExecutableSuffix: "", SharedLibrarySuffix: ".dylib", StaticLibrarySuffix: ".a",
	})

	// TargetX8664Onuron is the platform this compiler ships for. It is a Linux
	// ABI today; naming it separately means a future divergence is a change to
	// one row here rather than a change to every build that says "linux".
	TargetX8664Onuron = register(Target{
		ID: "x86_64-linux-onuron", OS: OSOnuron, Arch: ArchX86_64,
		Triple:      "x86_64-unknown-linux-gnu",
		DataLayout:  "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-f80:128-n8:16:32:64-S128",
		PointerSize: Pointer64, ObjectFormat: "elf",
		ExecutableSuffix: "", SharedLibrarySuffix: ".so", StaticLibrarySuffix: ".a",
	})
)

// AllTargets returns every target, sorted by name.
//
// Sorted so that `nil build --print-targets` prints the same list every time. A
// list whose order depends on Go's map iteration is a list nobody can read twice
// the same way, and the sort key is the name a developer types.
func AllTargets() []Target {
	out := make([]Target, 0, len(targets))
	for _, t := range targets {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// LookupTarget resolves a target name.
//
// It returns an error rather than a zero Target, because a zero Target has a
// plausible triple that names nothing and would produce an `.ll` file that fails
// somewhere far from the mistake.
func LookupTarget(id string) (Target, error) {
	if t, ok := targets[id]; ok {
		return t, nil
	}

	known := make([]string, 0, len(targets))
	for name := range targets {
		known = append(known, name)
	}
	sort.Strings(known)

	return Target{}, fmt.Errorf("unknown target %q; known targets are: %s", id, strings.Join(known, ", "))
}

// IsKnownTarget reports whether a name resolves.
func IsKnownTarget(id string) bool {
	_, ok := targets[id]
	return ok
}
