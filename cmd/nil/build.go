package main

import (
	"fmt"
	"os"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/bundle"
	"github.com/joysriramsarkar/nilLang/pkg/compiler"
	"github.com/joysriramsarkar/nilLang/pkg/config"
)

func cmdBuild() {
	startTime := time.Now()

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

	fmt.Printf("🔨 বিল্ড শুরু: %s v%s\n", cfg.Name, cfg.Version)
	fmt.Printf("   এন্ট্রি: %s\n", cfg.Entry)
	fmt.Printf("   টার্গেট: %v\n", cfg.Targets)
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

	// Add compiled bytecode
	bytecodeBytes := pipeline.GetBytecodeBytes()
	builder.AddFile("bytecode/main.nabc", bytecodeBytes)
	fmt.Println("✅")

	// Step 3: Add resources
	fmt.Print("   [3/4] রিসোর্স যোগ হচ্ছে... ")
	if err := builder.AddResources(); err != nil {
		fmt.Println("⚠️")
		fmt.Fprintf(os.Stderr, "   ⚠️ রিসোর্স যোগ করতে সমস্যা: %s\n", err)
	} else {
		fmt.Println("✅")
	}

	// Step 4: Build the bundle
	fmt.Print("   [4/4] .nilax প্যাকেজ হচ্ছে... ")
	outputPath := cfg.GetOutputPath(projectDir)
	if err := builder.Build(outputPath); err != nil {
		fmt.Println("❌")
		fmt.Fprintf(os.Stderr, "❌ বান্ডিল তৈরি করতে সমস্যা: %s\n", err)
		os.Exit(1)
	}
	fmt.Println("✅")

	elapsed := time.Since(startTime)
	info := builder.GetBundleInfo()

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════")
	fmt.Printf("✅ বিল্ড সফল!\n")
	fmt.Printf("📦 %s\n", info.String())
	fmt.Printf("📄 আউটপুট: %s\n", outputPath)
	fmt.Printf("⏱️  সময়: %v\n", elapsed.Round(time.Millisecond))
	fmt.Println("═══════════════════════════════════════════")

	// Show disassembly in debug mode
	if cfg.Build.Debug {
		fmt.Println()
		fmt.Println("--- Disassembled Bytecode ---")
		fmt.Println(pipeline.GetDisassembly())
	}
}