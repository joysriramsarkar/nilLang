package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/joysriramsarkar/nilLang/compiler/diagnostics"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/llvm"
	"github.com/joysriramsarkar/nilLang/compiler/mir"
	"github.com/joysriramsarkar/nilLang/compiler/toolchain"
	"github.com/joysriramsarkar/nilLang/compiler/wasm"
	"github.com/joysriramsarkar/nilLang/pkg/bundle"
	"github.com/joysriramsarkar/nilLang/pkg/compiler"
	"github.com/joysriramsarkar/nilLang/pkg/config"
	"github.com/joysriramsarkar/nilLang/pkg/mobile/android"
)

type PlatformTarget struct {
	ID          string
	OS          string
	Arch        string
	Ext         string
	DisplayName string
	RunnerName  string
	FileSuffix  string
	IsMobile    bool
}

var allPlatforms = []PlatformTarget{
	{
		ID:          "windows",
		OS:          "windows",
		Arch:        "amd64",
		Ext:         ".exe",
		DisplayName: "Windows (x86_64 PE Binary)",
		RunnerName:  "nil-runner-windows-amd64.exe",
		FileSuffix:  "-windows-amd64.exe",
	},
	{
		ID:          "linux",
		OS:          "linux",
		Arch:        "amd64",
		Ext:         "",
		DisplayName: "Linux (x86_64 ELF Binary)",
		RunnerName:  "nil-runner-linux-amd64",
		FileSuffix:  "-linux-amd64",
	},
	{
		ID:          "darwin-arm64",
		OS:          "darwin",
		Arch:        "arm64",
		Ext:         "",
		DisplayName: "macOS Apple Silicon (ARM64)",
		RunnerName:  "nil-runner-darwin-arm64",
		FileSuffix:  "-darwin-arm64",
	},
	{
		ID:          "darwin-amd64",
		OS:          "darwin",
		Arch:        "amd64",
		Ext:         "",
		DisplayName: "macOS Intel (x86_64)",
		RunnerName:  "nil-runner-darwin-amd64",
		FileSuffix:  "-darwin-amd64",
	},
	{
		ID:          "onuron",
		OS:          "linux",
		Arch:        "amd64",
		Ext:         "",
		DisplayName: "Onuron OS (Native)",
		RunnerName:  "nil-runner-onuron-x86_64",
		FileSuffix:  "-onuron-x86_64",
	},
	{
		ID:          "android",
		OS:          "android",
		Arch:        "arm64",
		Ext:         ".apk",
		DisplayName: "Android (APK Package)",
		RunnerName:  "nil-runner-android-arm64",
		FileSuffix:  ".apk",
		IsMobile:    true,
	},
}

// printTargets lists every target the compiler can emit for.
//
// The list is sorted by the name a developer types, so it reads the same way every
// time — a target list whose order came from a Go map is a list nobody can find
// anything in twice.
func printTargets() {
	fmt.Println("Nilang build targets")
	fmt.Println()
	fmt.Printf("  %-22s %-10s %-10s %s\n", "TARGET", "OS", "ARCH", "TRIPLE")
	fmt.Printf("  %s\n", strings.Repeat("-", 74))

	for _, t := range llvm.AllTargets() {
		marker := ""
		if t.IsNative() {
			marker = "  (host)"
		}
		fmt.Printf("  %-22s %-10s %-10s %s%s\n", t.ID, t.OS, t.Arch, t.Triple, marker)
	}

	fmt.Println()
	fmt.Println("Choose one with:  nil build --target <name>")
	fmt.Println("Cross-compilation needs an LLVM toolchain; see --print-toolchain.")
}

// printToolchain reports which external tools the compiler found.
//
// It is the command to run *after* installing LLVM, and the command to run before
// filing a bug about a build that will not start. A missing tool is named here
// with the path that was searched, because "not found" without the path sends the
// reader looking in the wrong directory.
func printToolchain() {
	fmt.Println("Nilang LLVM toolchain")
	fmt.Println()

	tc := toolchain.DiscoverToolchain(llvm.TargetHost)
	fmt.Printf("  host target:  %s (%s)\n", llvm.TargetHost.ID, llvm.TargetHost.Triple)
	fmt.Println()

	// The columns are the *capabilities*, not a checkmark.
	//
	// A single ✔ next to a tool says "this program exists" and the reader takes it
	// to mean "this program can build my project". Those are different questions
	// with different answers on a real machine: an LLVM install on Windows ships a
	// clang that reads IR and cannot compile C or link, and a report that said ✔
	// once would send the user to find the problem somewhere other than where it
	// is.
	fmt.Printf("  %-10s  %-12s  %-6s  %-8s\n", "tool", "reads IR", "comp C", "links")
	for _, report := range tc.Capabilities() {
		if !report.Installed {
			// A row of dashes says "we looked and did not find it" while looking
			// like a row of answers. One word in the tool column says the same
			// thing and cannot be misread.
			fmt.Printf("  %-10s  not found\n", report.Tool)
			continue
		}
		fmt.Printf("  %-10s  %-12s  %-6s  %-8s\n",
			report.Tool, mark(report.ReadsIR), mark(report.CompilesC), mark(report.Links))
	}

	fmt.Println()
	fmt.Println("  paths:")
	for _, report := range tc.Capabilities() {
		if !report.Installed {
			continue
		}
		fmt.Printf("    %-10s %s\n", report.Tool, report.Path)
		// The version goes on its own line because clang's is longer than the
		// table is wide, and truncating it would leave out the part that
		// distinguishes one install from another.
		if report.Version != "" {
			fmt.Printf("                 %s\n", report.Version)
		}
	}

	fmt.Println()
	if !tc.Ready() {
		fmt.Print(tc.Describe())
		return
	}
	if !tc.LinkReady() {
		fmt.Print(tc.LinkAdvice())
		return
	}
	fmt.Println("Ready to build native binaries.")
	fmt.Println("Emitting LLVM IR needs no toolchain at all; see: nil build --emit-llvm")
}

// mark renders a capability as something a reader can scan down a column.
//
// Yes and no rather than ✔ and ✘: the report is read by people whose terminal
// renders the second pair as monochrome boxes, and a column of boxes answers
// nothing.
func mark(yes bool) string {
	if yes {
		return "yes"
	}
	return "no"
}

// requireToolchain returns the toolchain or exits with the message explaining how
// to fix what is missing.
//
// It exits rather than returning an error because every caller would otherwise
// have to decide whether to exit, print, or continue — and a build that continued
// without a compiler produces a confusing failure much further on instead of a
// clear one here.
func requireToolchain(target llvm.Target) toolchain.Toolchain {
	tc := toolchain.DiscoverToolchain(target)
	if !tc.Ready() {
		fmt.Fprint(os.Stderr, tc.Describe())
		os.Exit(1)
	}
	return tc
}

// emitLLVM compiles a source file to textual LLVM IR and prints it.
//
// It needs no toolchain. Emitting IR is what this compiler does; turning it into a
// binary is what somebody else's program does. Separating them means the IR can be
// read, diffed, and checked on a machine with no LLVM installed at all — which is
// also the only way to review what the backend produces without trusting it.
func emitLLVM(flag string) string {
	args := os.Args[2:]
	var file, outPath string

	for _, arg := range args {
		if strings.HasPrefix(arg, "--emit-llvm=") {
			outPath = strings.TrimPrefix(arg, "--emit-llvm=")
			continue
		}
		if arg == flag || strings.HasPrefix(arg, "-") {
			continue
		}
		if file == "" {
			file = arg
		}
	}
	if file == "" {
		fmt.Fprintln(os.Stderr, "❌ --emit-llvm needs a source file")
		os.Exit(1)
	}
	if outPath == "" {
		fmt.Fprintln(os.Stderr, "❌ --emit-llvm=<file.ll> needs an output path")
		os.Exit(1)
	}

	source, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ ফাইল পড়তে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	res := frontend.Check(string(source), frontend.Options{
		Filename:  file,
		SourceDir: filepath.Dir(file),
	})
	if len(res.ParseErrors) > 0 {
		fmt.Fprintf(os.Stderr, "❌ সিনট্যাক্স ত্রুটি:\n%s\n", strings.Join(res.ParseErrors, "\n"))
		os.Exit(1)
	}
	if !res.OK {
		fmt.Fprintln(os.Stderr, "❌ টাইপ ত্রুটি:")
		for _, d := range res.Diagnostics {
			if d.Severity == diagnostics.SeverityError {
				fmt.Fprintf(os.Stderr, "   %s\n", d.String())
			}
		}
		os.Exit(1)
	}

	optimized := hir.NewOptimizer().Optimize(hir.NewLowerer().LowerProgram(res.Lowered))
	mirProg := mir.NewLowerer().LowerHIR(optimized)
	mir.NewOptimizer().OptimizeProgram(mirProg)

	text, err := llvm.NewEmitter(llvm.TargetHost).Emit(mirProg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ LLVM IR তৈরিতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	if outPath == "" {
		fmt.Print(text)
		return ""
	}
	// Written as bytes rather than printed and redirected: a shell redirect on
	// Windows writes UTF-16 with a byte-order mark, and LLVM rejects a module
	// that starts with one with an encoding error that says nothing about the IR.
	if err := os.WriteFile(outPath, []byte(text), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "❌ লেখা যায়নি: %s\n", err)
		os.Exit(1)
	}
	return outPath
}

func cmdBuild() {
	startTime := time.Now()

	// `--print-targets` answers "what can this build for?" without building
	// anything.
	//
	// It is deliberately a flag on `build` rather than its own command: a target
	// list is a property of the compiler, not a separate tool, and a person who
	// types `nil build` and asks what it can do should not have to guess a second
	// word for the answer.
	for _, arg := range os.Args[2:] {
		switch {
		case arg == "--print-targets":
			printTargets()
			return
		case arg == "--print-toolchain":
			printToolchain()
			return
		case arg == "--emit-llvm", strings.HasPrefix(arg, "--emit-llvm="):
			out := emitLLVM(arg)
			if out != "" {
				fmt.Printf("wrote %s\n", out)
			}
			return
		}
	}

	// Parse command-line target flags
	var requestedTargets []string
	allOS := false
	sizeFlag := false

	// Find project directory (current directory by default, or overridden by arg)
	projectDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ কারেন্ট ডিরেক্টরি পেতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	// `nil build --aot` produces a native executable rather than the default
	// artefacts. It is checked here rather than further down because it changes what
	// the rest of the command is *for*: a native build is one file, and the
	// per-target bundle path below would produce a directory of things that are not
	// what the flag asked for.
	//
	// The project directory is read from the command line rather than from
	// `os.Getwd`, because the argument loop that would normally resolve `--dir` has
	// not run yet — and a build that ignored `--dir` would succeed in the wrong
	// directory, which is worse than one that failed.
	if hasAOTFlag(os.Args[2:]) {
		directory := projectDirFrom(os.Args[2:])
		if directory == "" {
			directory = projectDir
		}
		out := buildAOT(directory, aotOutputPath(os.Args[2:]), keepsModule(os.Args[2:]))
		if out == "" {
			os.Exit(1)
		}
		return
	}

	for i := 2; i < len(os.Args); i++ {
		rawArg := os.Args[i]
		arg := strings.ToLower(rawArg)
		if arg == "-allos" || arg == "--all" || arg == "-all" || arg == "all" || arg == "--allos" {
			allOS = true
		} else if strings.HasPrefix(arg, "--dir=") {
			projectDir = strings.TrimPrefix(rawArg, "--dir=")
		} else if (arg == "--dir" || arg == "-dir") && i+1 < len(os.Args) {
			projectDir = os.Args[i+1]
			i++
		} else if strings.HasPrefix(arg, "--target=") {
			requestedTargets = append(requestedTargets, strings.TrimPrefix(arg, "--target="))
		} else if strings.HasPrefix(arg, "-target=") {
			requestedTargets = append(requestedTargets, strings.TrimPrefix(arg, "-target="))
		} else if (arg == "--target" || arg == "-target") && i+1 < len(os.Args) {
			requestedTargets = append(requestedTargets, os.Args[i+1])
			i++
		} else if arg == "--size" {
			sizeFlag = true
		} else if !strings.HasPrefix(arg, "-") {
			if fi, statErr := os.Stat(rawArg); statErr == nil && fi.IsDir() {
				candidate := filepath.Join(rawArg, "nil.json")
				if _, jsonErr := os.Stat(candidate); jsonErr == nil {
					projectDir = rawArg
					continue
				}
			}
			requestedTargets = append(requestedTargets, arg)
		}
	}

	// Load configuration
	cfg, err := config.LoadConfig(projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %s\n", err)
		os.Exit(1)
	}

	// Validate configuration
	if err := cfg.Validate(projectDir); err != nil {
		fmt.Fprintf(os.Stderr, "❌ কনফিগারেশন ত্রুটি: %s\n", err)
		os.Exit(1)
	}

	// Validate capabilities against profile
	if valRes, valErr := cfg.ValidateCapabilities(); valErr != nil || !valRes.Valid {
		fmt.Fprintf(os.Stderr, "❌ ক্যাপাবিলিটি ভায়োলেশন (প্রোফাইল: %s):\n", cfg.Profile)
		if valRes != nil {
			for _, v := range valRes.Violations {
				fmt.Fprintf(os.Stderr, "   • %s\n", v)
			}
		} else {
			fmt.Fprintf(os.Stderr, "   • %v\n", valErr)
		}
		os.Exit(1)
	}

	// Check if WebAssembly target was requested
	for _, req := range requestedTargets {
		if strings.EqualFold(req, "wasm") || strings.EqualFold(req, "web") {
			buildWASM(cfg, projectDir)
			return
		}
	}

	// Resolve target platforms
	var selectedPlatforms []PlatformTarget
	if allOS {
		selectedPlatforms = allPlatforms
	} else if len(requestedTargets) > 0 {
		for _, req := range requestedTargets {
			matches := resolveTargetPlatforms(req)
			if len(matches) == 0 {
				fmt.Fprintf(os.Stderr, "⚠️ অজানা টার্গেট: %s (সাপোর্টেড: linux, windows, macos, darwin, onuron, android, wasm, -allos)\n", req)
			} else {
				selectedPlatforms = append(selectedPlatforms, matches...)
			}
		}
	} else {
		// 1. Check if targets are configured in nil.json
		if len(cfg.Targets) > 0 {
			for _, t := range cfg.Targets {
				matches := resolveTargetPlatforms(t)
				selectedPlatforms = append(selectedPlatforms, matches...)
			}
		}
		// 2. Also ensure current host platform is included
		hostPlatform := getHostPlatform()
		hasHost := false
		for _, p := range selectedPlatforms {
			if p.ID == hostPlatform.ID {
				hasHost = true
				break
			}
		}
		if !hasHost {
			selectedPlatforms = append(selectedPlatforms, hostPlatform)
		}
	}

	selectedPlatforms = deduplicatePlatforms(selectedPlatforms)

	fmt.Printf("🔨 বিল্ড শুরু: %s v%s [Profile: %s]\n", cfg.Name, cfg.Version, cfg.Profile)
	fmt.Printf("   এন্ট্রি: %s\n", cfg.Entry)
	var platNames []string
	for _, p := range selectedPlatforms {
		platNames = append(platNames, p.ID)
	}
	fmt.Printf("   টার্গেট ওএস: %v\n", platNames)
	fmt.Printf("   ক্যাপাবিলিটিস: %v\n", cfg.Capabilities)
	fmt.Println()

	// Step 1: Compile source code
	fmt.Print("   [1/4] কম্পাইল হচ্ছে... ")
	entryPath := cfg.GetEntryPath(projectDir)
	pipeline, err := compiler.CompileFile(entryPath)
	if err != nil {
		fmt.Println("❌")
		fmt.Fprintf(os.Stderr, "❌ কম্পাইলেশন ত্রুটি:\n%s\n", err)
		os.Exit(1)
	}
	fmt.Println("✅")

	// Step 2: Create bundle builder
	fmt.Print("   [2/4] বান্ডিল তৈরি হচ্ছে... ")
	builder := bundle.NewBuilder(cfg, projectDir)

	// Add compiled bytecode and source
	bytecodeBytes, err := pipeline.GetBytecodeImage()
	if err != nil {
		fmt.Println("❌")
		fmt.Fprintf(os.Stderr, "❌ বাইটকোড সিরিয়ালাইজেশন ত্রুটি: %s\n", err)
		os.Exit(1)
	}
	builder.AddFile("bytecode/main.nabc", bytecodeBytes)
	srcDir := filepath.Join(projectDir, "src")
	if info, err := os.Stat(srcDir); err == nil && info.IsDir() {
		_ = builder.AddDirectory("src", srcDir)
	} else if _, err := os.Stat(entryPath); err == nil {
		_ = builder.AddFileFromDisk("src/main.nil", entryPath)
	}
	fmt.Println("✅")

	// Step 3: Add resources
	fmt.Print("   [3/4] রিসোর্স যোগ হচ্ছে... ")
	if err := builder.AddResources(); err != nil {
		fmt.Println("⚠️")
		fmt.Fprintf(os.Stderr, "   ⚠️ রিসোর্স যোগ করতে সমস্যা: %s\n", err)
	} else {
		fmt.Println("✅")
	}

	// Step 4: Build the .nilax bundle
	fmt.Print("   [4/4] .nilax প্যাকেজ হচ্ছে... ")
	outputPath := cfg.GetOutputPath(projectDir)
	if err := builder.Build(outputPath); err != nil {
		fmt.Println("❌")
		fmt.Fprintf(os.Stderr, "❌ বান্ডিল তৈরি করতে সমস্যা: %s\n", err)
		os.Exit(1)
	}
	fmt.Println("✅")

	bundleBytes, err := os.ReadFile(outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ বান্ডিল পড়তে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	outputDir := filepath.Dir(outputPath)

	// Build standalone native executables for selected platforms
	type BuiltArtifact struct {
		Name string
		Path string
		Size int64
		Type string
	}
	var artifacts []BuiltArtifact

	artifacts = append(artifacts, BuiltArtifact{
		Name: fmt.Sprintf("%s Universal Bundle", cfg.Name),
		Path: outputPath,
		Size: int64(len(bundleBytes)),
		Type: ".nilax Bundle",
	})

	if len(selectedPlatforms) > 0 {
		fmt.Println()
		fmt.Printf("🚀 টার্গেট প্ল্যাটফর্ম আর্টিফ্যাক্ট তৈরি হচ্ছে (%d টি টার্গেট)...\n", len(selectedPlatforms))

		for _, plat := range selectedPlatforms {
			if plat.IsMobile {
				// Mobile target: Build real Android APK package
				apkName := fmt.Sprintf("%s.apk", cfg.Name)
				apkPath := filepath.Join(outputDir, apkName)
				res, err := android.BuildAPK(cfg, projectDir, apkPath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "   ⚠️ %s প্যাকেজ তৈরি করতে সমস্যা: %v\n", plat.DisplayName, err)
					continue
				}

				signLabel := "Unsigned"
				if res.Signed {
					signLabel = "Signed Debug"
				}

				artifacts = append(artifacts, BuiltArtifact{
					Name: fmt.Sprintf("%s [%s]", plat.DisplayName, signLabel),
					Path: res.Path,
					Size: res.Size,
					Type: "Android APK",
				})
				fmt.Printf("   ✅ %-30s -> %s (%s) [%s]\n", plat.DisplayName, apkName, formatSize(res.Size), signLabel)
				// L8.3: `nil build android --size` prints the section
				// breakdown against the <10MB budget.
				if sizeFlag {
					if rep, repErr := android.ReportSize(res.Path); repErr == nil {
						fmt.Printf("\n%s\n", rep.String())
					} else {
						fmt.Fprintf(os.Stderr, "   ⚠️ size report ব্যর্থ: %v\n", repErr)
					}
				}
				continue
			}

			// Desktop / Server standalone binary
			runnerBytes, err := getOrBuildRunner(plat, projectDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "   ⚠️ %s এক্সেকিউটেবল স্কিপ: %v\n", plat.DisplayName, err)
				continue
			}

			// Combine runner + bundle
			exeBytes := make([]byte, len(runnerBytes)+len(bundleBytes))
			copy(exeBytes, runnerBytes)
			copy(exeBytes[len(runnerBytes):], bundleBytes)

			var binaryName string
			if len(selectedPlatforms) == 1 {
				// Single target: use clean target name without arch duplication
				if plat.ID == "windows" {
					binaryName = fmt.Sprintf("%s.exe", cfg.Name)
				} else {
					binaryName = cfg.Name
				}
			} else {
				// Multi-target: use explicit architecture suffix
				binaryName = fmt.Sprintf("%s%s", cfg.Name, plat.FileSuffix)
			}

			binaryPath := filepath.Join(outputDir, binaryName)

			if err := os.WriteFile(binaryPath, exeBytes, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "   ❌ %s সংরক্ষণ ব্যর্থ: %v\n", binaryName, err)
				continue
			}

			artType := "Desktop Binary"
			if plat.ID == "windows" {
				artType = "Windows Executable"
			} else if plat.ID == "linux" {
				artType = "Linux Executable"
			} else if plat.ID == "onuron" {
				artType = "Onuron OS"
			} else if strings.HasPrefix(plat.ID, "darwin") {
				artType = "macOS"
			}

			artifacts = append(artifacts, BuiltArtifact{
				Name: plat.DisplayName,
				Path: binaryPath,
				Size: int64(len(exeBytes)),
				Type: artType,
			})
			fmt.Printf("   ✅ %-30s -> %s (%s)\n", plat.DisplayName, binaryName, formatSize(int64(len(exeBytes))))
		}
	}

	elapsed := time.Since(startTime)

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Printf("🎉 বিল্ড সফল! মোট %d টি আর্টিফ্যাক্ট প্রস্তুত হয়েছে (সময়: %v)\n", len(artifacts), elapsed.Round(time.Millisecond))
	fmt.Println("───────────────────────────────────────────────────────────────────")
	for _, art := range artifacts {
		fmt.Printf("  • %-36s [%s]\n    📄 %s\n", art.Name, formatSize(art.Size), art.Path)
	}
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("💡 কীভাবে চালাবেন (How to run):")
	for _, art := range artifacts {
		relPath := filepath.Join(cfg.Build.OutputDir, filepath.Base(art.Path))
		switch art.Type {
		case "Android APK":
			fmt.Printf("  • Android Device : adb install %s (অথবা ফোনে ট্রান্সফার করে সরাসরি ইনস্টল করুন)\n", relPath)
		case "Windows Executable":
			fmt.Printf("  • Windows PS     : .\\%s (PowerShell-এ .\\ দিয়ে চালান)\n", relPath)
			fmt.Printf("  • Windows CMD    : %s\n", relPath)
		case "Linux Executable":
			fmt.Printf("  • Linux          : ./%s\n", filepath.ToSlash(relPath))
		case "Onuron OS":
			fmt.Printf("  • Onuron OS      : ./%s\n", filepath.ToSlash(relPath))
		case "macOS":
			fmt.Printf("  • macOS          : ./%s\n", filepath.ToSlash(relPath))
		}
	}
	fmt.Printf("  • সর্বজনীন বান্ডিল: nil run %s\n", outputPath)
}

func resolveTargetPlatforms(target string) []PlatformTarget {
	t := strings.ToLower(strings.TrimSpace(target))
	switch t {
	case "windows", "win", "win64", "windows-amd64", "exe":
		return []PlatformTarget{allPlatforms[0]}
	case "linux", "linux64", "linux-amd64":
		return []PlatformTarget{allPlatforms[1]}
	case "macos", "darwin", "mac", "osx", "apple":
		return []PlatformTarget{allPlatforms[2], allPlatforms[3]}
	case "darwin-arm64", "macos-arm64", "m1", "m2", "m3", "apple-silicon":
		return []PlatformTarget{allPlatforms[2]}
	case "darwin-amd64", "macos-amd64", "intel-mac":
		return []PlatformTarget{allPlatforms[3]}
	case "onuron", "os":
		return []PlatformTarget{allPlatforms[4]}
	case "android", "apk", "mobile":
		return []PlatformTarget{allPlatforms[5]}
	case "all", "-allos", "--all", "allos":
		return allPlatforms
	default:
		return nil
	}
}

func getHostPlatform() PlatformTarget {
	switch runtime.GOOS {
	case "windows":
		return allPlatforms[0]
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return allPlatforms[2]
		}
		return allPlatforms[3]
	default:
		return allPlatforms[1]
	}
}

func deduplicatePlatforms(platforms []PlatformTarget) []PlatformTarget {
	seen := make(map[string]bool)
	var res []PlatformTarget
	for _, p := range platforms {
		if !seen[p.RunnerName] {
			seen[p.RunnerName] = true
			res = append(res, p)
		}
	}
	return res
}

func getOrBuildRunner(target PlatformTarget, currentDir string) ([]byte, error) {
	homeDir, _ := os.UserHomeDir()
	runnerPath := filepath.Join(homeDir, ".nil", "runners", target.RunnerName)

	if runnerStat, err := os.Stat(runnerPath); err == nil && runnerStat.Size() > 0 {
		stale := false
		if selfPath, selfErr := os.Executable(); selfErr == nil {
			if selfStat, statErr := os.Stat(selfPath); statErr == nil && selfStat.ModTime().After(runnerStat.ModTime()) {
				stale = true
			}
		}
		if !stale {
			if data, readErr := os.ReadFile(runnerPath); readErr == nil && len(data) > 0 {
				return data, nil
			}
		}
	}

	// Try locating runner source in known NilLang locations
	var runnerSrcDir string
	candidates := []string{
		"c:/Users/joysr/Documents/programming language/nilLang",
		filepath.Join(homeDir, "go", "src", "github.com", "joysriramsarkar", "nilLang"),
		filepath.Join(currentDir, "..", "nilLang"),
	}

	for _, cand := range candidates {
		checkPath := filepath.Join(cand, "cmd", "nil-runner", "main.go")
		if _, err := os.Stat(checkPath); err == nil {
			runnerSrcDir = filepath.Join(cand, "cmd", "nil-runner")
			break
		}
	}

	if runnerSrcDir == "" {
		return nil, fmt.Errorf("runner binary not cached at %s and source not located", runnerPath)
	}

	_ = os.MkdirAll(filepath.Dir(runnerPath), 0755)

	cmd := exec.Command("go", "build", "-ldflags=-s -w", "-o", runnerPath, ".")
	cmd.Dir = runnerSrcDir
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+target.OS,
		"GOARCH="+target.Arch,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("runner compile error: %s (%s)", err, string(out))
	}

	return os.ReadFile(runnerPath)
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func buildWASM(cfg *config.ProjectConfig, projectDir string) {
	fmt.Printf("🌐 WebAssembly (WASM) বিল্ড শুরু: %s v%s [Profile: %s]\n", cfg.Name, cfg.Version, cfg.Profile)
	entryPath := cfg.GetEntryPath(projectDir)
	srcBytes, err := os.ReadFile(entryPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ এন্ট্রি ফাইল পড়তে ব্যর্থ: %v\n", err)
		os.Exit(1)
	}

	res := frontend.Check(string(srcBytes), frontend.Options{
		Filename:  entryPath,
		SourceDir: filepath.Dir(entryPath),
	})
	if len(res.ParseErrors) > 0 {
		fmt.Fprintf(os.Stderr, "❌ সিনট্যাক্স এরর:\n%s\n", strings.Join(res.ParseErrors, "\n"))
		os.Exit(1)
	}
	if !res.OK {
		fmt.Fprintln(os.Stderr, "❌ টাইপ ত্রুটি:")
		for _, d := range res.Diagnostics {
			if d.Severity == diagnostics.SeverityError {
				fmt.Fprintf(os.Stderr, "   %s\n", d.String())
			}
		}
		os.Exit(1)
	}
	printWarnings(res.Diagnostics)
	prog := backendProgram(res)

	lowerer := hir.NewLowerer()
	hProg := lowerer.LowerProgram(prog)
	opt := hir.NewOptimizer()
	optProg := opt.Optimize(hProg)

	mirLowerer := mir.NewLowerer()
	mirProg := mirLowerer.LowerHIR(optProg)

	wasmCompiler := wasm.NewCompiler()
	mod, err := wasmCompiler.Compile(mirProg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ WASM কম্পাইলেশন ত্রুটি: %v\n", err)
		os.Exit(1)
	}

	outDir := filepath.Join(projectDir, "build", "wasm")
	_ = os.MkdirAll(outDir, 0755)

	wasmFile := filepath.Join(outDir, "app.wasm")
	watFile := filepath.Join(outDir, "app.wat")
	htmlFile := filepath.Join(outDir, "index.html")

	_ = os.WriteFile(wasmFile, mod.Binary, 0644)
	_ = os.WriteFile(watFile, []byte(mod.WAT), 0644)
	_ = os.WriteFile(filepath.Join(outDir, "nil_runtime.js"), []byte(wasm.NilRuntimeJS()), 0644)
	htmlContent := wasm.GenerateBrowserHarness(cfg.Name, "app.wasm")
	_ = os.WriteFile(htmlFile, []byte(htmlContent), 0644)

	// WASM stays labelled experimental per ADR-0004: the backend runs real
	// programs now, but aggregates have no linear-memory ABI yet and host
	// builtins beyond puts are stubs.
	fmt.Println("✅ WebAssembly বিল্ড সফল হয়েছে (experimental — ADR-0004):")
	fmt.Printf("  • WASM বাইনারি: %s (%d bytes)\n", wasmFile, len(mod.Binary))
	fmt.Printf("  • WAT টেক্সট:   %s\n", watFile)
	fmt.Printf("  • রানটাইম ব্রিজ: %s\n", filepath.Join(outDir, "nil_runtime.js"))
	fmt.Printf("  • HTML প্রিভিউ:  %s\n", htmlFile)
}
