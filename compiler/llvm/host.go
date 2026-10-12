package llvm

import "runtime"

// hostTriple is the LLVM triple of the machine this binary was built for.
//
// It is computed once from Go's build information rather than by running `clang`
// at start-up: a build system that shells out to find out where it is running
// cannot run at all on a machine without a compiler, which is exactly the machine
// whose compiler it would be looking for.
var hostTriple = computeHostTriple()

func computeHostTriple() string {
	vendor := "unknown"
	switch runtime.GOOS {
	case "windows":
		// LLVM's Windows triples name the environment rather than the OS:
		// `x86_64-pc-windows-msvc` is the triple every Windows toolchain
		// recognises, and `-pc-windows` alone is not one of them.
		return llvmArchName(runtime.GOARCH) + "-pc-windows-msvc"
	case "linux":
		vendor = "pc"
	case "darwin":
		return llvmArchName(runtime.GOARCH) + "-apple-macosx"
	case "android":
		return "aarch64-unknown-linux-android"
	}
	return llvmArchName(runtime.GOARCH) + "-" + vendor + "-" + runtime.GOOS
}

// llvmArchName maps Go's architecture names onto LLVM's.
//
// They are not the same. Go says `amd64` and `arm64`; LLVM says `x86_64` and
// `aarch64`. Writing Go's name into a triple produces one LLVM does not recognise,
// and the failure surfaces as an unhelpful assembler message rather than as the
// name mismatch it is.
func llvmArchName(goarch string) string {
	switch goarch {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	case "386":
		return "i386"
	}
	return goarch
}

func HostTriple() string { return hostTriple }

func hostPointerSize() PointerSize {
	if 32<<(^uint(0)>>63) == 32 {
		return Pointer32
	}
	return Pointer64
}

func hostObjectFormat() string {
	switch runtime.GOOS {
	case "windows":
		return "coff"
	case "darwin", "ios":
		return "macho"
	case "js", "wasip1":
		return "wasm"
	}
	return "elf"
}

// defaultDataLayout is LLVM's documented default for an architecture.
//
// It is used only for a platform with no registered target. The value matters:
// LLVM will not verify it, so a wrong layout produces code that mis-strides an
// array instead of failing to assemble.
func defaultDataLayout(arch string) string {
	switch arch {
	case "amd64":
		return "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-f80:128-n8:16:32:64-S128"
	case "arm64":
		return "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-n32:64-S128"
	}
	return "e-p:64:64-i64:64-n32:64-S128"
}
