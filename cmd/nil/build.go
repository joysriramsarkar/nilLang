package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/bundle"
	"github.com/joysriramsarkar/nilLang/pkg/compiler"
	"github.com/joysriramsarkar/nilLang/pkg/config"
)

type PlatformTarget struct {
	ID          string
	OS          string
	Arch        string
	Ext         string
	DisplayName string
	RunnerName  string
	FileSuffix  string
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
		OS:          "linux",
		Arch:        "arm64",
		Ext:         "",
		DisplayName: "Android (AArch64 / ARM64 CLI)",
		RunnerName:  "nil-runner-android-arm64",
		FileSuffix:  "-android-arm64",
	},
}

func cmdBuild() {
	startTime := time.Now()

	// Parse command-line target flags
	var requestedTargets []string
	allOS := false

	for i := 2; i < len(os.Args); i++ {
		arg := strings.ToLower(os.Args[i])
		if arg == "-allos" || arg == "--all" || arg == "-all" || arg == "all" || arg == "--allos" {
			allOS = true
		} else if strings.HasPrefix(arg, "--target=") {
			requestedTargets = append(requestedTargets, strings.TrimPrefix(arg, "--target="))
		} else if strings.HasPrefix(arg, "-target=") {
			requestedTargets = append(requestedTargets, strings.TrimPrefix(arg, "-target="))
		} else if (arg == "--target" || arg == "-target") && i+1 < len(os.Args) {
			requestedTargets = append(requestedTargets, os.Args[i+1])
			i++
		} else if !strings.HasPrefix(arg, "-") {
			requestedTargets = append(requestedTargets, arg)
		}
	}

	// Find project directory (current directory)
	projectDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ কারেন্ট ডিরেক্টরি পেতে সমস্যা: %s\n", err)
		os.Exit(1)
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

	// Resolve target platforms
	var selectedPlatforms []PlatformTarget
	if allOS {
		selectedPlatforms = allPlatforms
	} else if len(requestedTargets) > 0 {
		for _, req := range requestedTargets {
			matches := resolveTargetPlatforms(req)
			if len(matches) == 0 {
				fmt.Fprintf(os.Stderr, "⚠️ অজানা টার্গেট: %s (সাপোর্টেড: linux, windows, macos, darwin, onuron, android, -allos)\n", req)
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
	bytecodeBytes := pipeline.GetBytecodeBytes()
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
		fmt.Printf("🚀 নেটিভ স্ট্যান্ডঅ্যালোন এক্সেকিউটেবল তৈরি হচ্ছে (%d টি ওএস টার্গেট)...\n", len(selectedPlatforms))

		for _, plat := range selectedPlatforms {
			runnerBytes, err := getOrBuildRunner(plat, projectDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "   ⚠️ %s এক্সেকিউটেবল স্কিপ: %v\n", plat.DisplayName, err)
				continue
			}

			// Combine runner + bundle
			exeBytes := make([]byte, len(runnerBytes)+len(bundleBytes))
			copy(exeBytes, runnerBytes)
			copy(exeBytes[len(runnerBytes):], bundleBytes)

			binaryName := fmt.Sprintf("%s%s", cfg.Name, plat.FileSuffix)
			binaryPath := filepath.Join(outputDir, binaryName)

			if err := os.WriteFile(binaryPath, exeBytes, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "   ❌ %s সংরক্ষণ ব্যর্থ: %v\n", binaryName, err)
				continue
			}

			// If single target or host OS, also create default short name
			if len(selectedPlatforms) == 1 {
				shortName := fmt.Sprintf("%s%s", cfg.Name, plat.Ext)
				if shortName != binaryName {
					_ = os.WriteFile(filepath.Join(outputDir, shortName), exeBytes, 0755)
				}
			}

			artifacts = append(artifacts, BuiltArtifact{
				Name: plat.DisplayName,
				Path: binaryPath,
				Size: int64(len(exeBytes)),
				Type: "Standalone Executable",
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
		fmt.Printf("  • %-32s [%s]\n    📄 %s\n", art.Name, formatSize(art.Size), art.Path)
	}
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("💡 কীভাবে চালাবেন (How to run standalone binaries):")
	fmt.Printf("  • Windows : .\\%s\\%s-windows-amd64.exe\n", cfg.Build.OutputDir, cfg.Name)
	fmt.Printf("  • Linux   : ./%s/%s-linux-amd64\n", cfg.Build.OutputDir, cfg.Name)
	fmt.Printf("  • Onuron  : ./%s/%s-onuron-x86_64\n", cfg.Build.OutputDir, cfg.Name)
	fmt.Printf("  • macOS   : ./%s/%s-darwin-arm64\n", cfg.Build.OutputDir, cfg.Name)
	fmt.Printf("  • অথবা    : nil run %s\n", outputPath)
}

func resolveTargetPlatforms(target string) []PlatformTarget {
	t := strings.ToLower(strings.TrimSpace(target))
	switch t {
	case "windows", "win", "win64", "windows-amd64":
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
	case "android", "android-arm64":
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

	if data, err := os.ReadFile(runnerPath); err == nil && len(data) > 0 {
		return data, nil
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
