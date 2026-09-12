package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joysriramsarkar/nilLang/pkg/config"
)

func cmdClean() {
	projectDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ কারেন্ট ডিরেক্টরি পেতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	for _, arg := range os.Args[2:] {
		if !strings.HasPrefix(arg, "-") {
			if fi, statErr := os.Stat(arg); statErr == nil && fi.IsDir() {
				projectDir = arg
				break
			}
		}
	}

	cfg, err := config.LoadConfig(projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %s\n", err)
		os.Exit(1)
	}

	buildDir := filepath.Join(projectDir, cfg.Build.OutputDir)

	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		fmt.Println("✅ বিল্ড ডিরেক্টরি ইতিমধ্যে খালি")
		return
	}

	if err := os.RemoveAll(buildDir); err != nil {
		fmt.Fprintf(os.Stderr, "❌ বিল্ড ডিরেক্টরি মুছতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ বিল্ড ডিরেক্টরি মুছে ফেলা হয়েছে: %s\n", buildDir)
}
