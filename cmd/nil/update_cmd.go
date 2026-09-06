package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// cmdUpdate handles updating the nil toolchain and compiler binaries
func cmdUpdate() {
	fmt.Println("🔄 Nilang & Alap Toolchain আপডেট যাচাই করা হচ্ছে...")

	// Find the workspace or Go path
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ হোম ডিরেক্টরি পাওয়া যায়নি: %v\n", err)
		return
	}

	workspaceDir := filepath.Join(home, "Documents", "programming language", "nilLang")
	if _, err := os.Stat(workspaceDir); err != nil {
		// Fallback to current working directory
		cwd, _ := os.Getwd()
		workspaceDir = cwd
	}

	fmt.Printf("📦 সোর্স লোকেশন: %s\n", workspaceDir)
	fmt.Println("🔨 কম্পাইলার ও CLI বিল্ড করা হচ্ছে...")

	cmd := exec.Command("go", "install", "./cmd/nil")
	cmd.Dir = workspaceDir
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ আপডেট করতে ত্রুটি: %v\n%s\n", err, string(output))
		return
	}

	targetBin := filepath.Join(home, "go", "bin", "nil")
	if runtime.GOOS == "windows" {
		targetBin += ".exe"
	}

	fmt.Printf("✅ সফলভাবে আপডেট সম্পন্ন হয়েছে! (ভার্সন: v%s)\n", VERSION)
	fmt.Printf("📍 ইনস্টল করা বাইনারি: %s\n", targetBin)
	fmt.Println()
	fmt.Println("নতুন ফিচারসমূহ:")
	fmt.Println("  • nil dev [--port 8080] : Alap ওয়েব লাইভ সার্ভার ও হট-রিলোড")
	fmt.Println("  • nil routes            : Radix Tree HTTP রাউট টেবিল পরিদর্শন")
	fmt.Println("  • nil db migrate        : এন্টারপ্রাইজ ডেটাবেস মাইগ্রেশন")
	fmt.Println("  • nil init [name]       : দ্রুত প্রজেক্ট স্ক্যাফোল্ডিং")
}
