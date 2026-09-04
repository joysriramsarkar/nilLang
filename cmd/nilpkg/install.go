package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/nilpkg"
)

func cmdInstall(cfg *nilpkg.Config) {
	if len(os.Args) < 3 {
		fmt.Println("ব্যবহার: nilpkg install <file.nilax | package-name>")
		os.Exit(1)
	}

	target := os.Args[2]

	// Initialize database
	db, err := nilpkg.NewDatabase(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ ডাটাবেস লোড করতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	// Create installer
	installer := nilpkg.NewInstaller(cfg, db)

	// Check if it's a file path or package name
	if strings.HasSuffix(target, ".nilax") || strings.Contains(target, "/") || strings.Contains(target, "\\") {
		// Install from file
		installFromFile(installer, target)
	} else {
		// Install from registry
		installFromRegistry(cfg, db, installer, target)
	}
}

func installFromFile(installer *nilpkg.Installer, path string) {
	fmt.Printf("📦 ইনস্টল শুরু: %s\n\n", path)

	// Check file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "❌ ফাইল পাওয়া যায়নি: %s\n", path)
		os.Exit(1)
	}

	result, err := installer.InstallFromFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ ইনস্টলেশন ব্যর্থ: %s\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════")
	fmt.Println("✅ ইনস্টলেশন সফল!")
	fmt.Printf("📦 প্যাকেজ: %s v%s\n", result.PackageName, result.Version)
	fmt.Printf("📁 ইনস্টল পাথ: %s\n", result.InstallPath)
	fmt.Printf("💾 সাইজ: %s\n", formatSize(result.Size))
	fmt.Printf("⏱️  সময়: %v\n", result.Duration.Round(time.Millisecond))
	fmt.Println("═══════════════════════════════════════════")
}

func installFromRegistry(cfg *nilpkg.Config, db *nilpkg.Database, installer *nilpkg.Installer, name string) {
	_ = db
	fmt.Printf("🔍 রেজিস্ট্রিতে খোঁজা হচ্ছে: %s\n", name)

	registry, err := nilpkg.NewRegistry(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ রেজিস্ট্রি লোড করতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	pkg, err := registry.Get(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ প্যাকেজ পাওয়া যায়নি: %s\n", name)
		fmt.Println("   নিশ্চিত করুন প্যাকেজ নাম সঠিক এবং রেজিস্ট্রি আপডেট আছে")
		os.Exit(1)
	}

	fmt.Printf("📦 পাওয়া গেছে: %s v%s\n", pkg.Name, pkg.Version)
	fmt.Printf("   বিবরণ: %s\n", pkg.Description)
	fmt.Printf("   লেখক: %s\n", pkg.Author)
	fmt.Println()

	tempFile := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%s.nilax", pkg.Name, pkg.Version))

	if strings.HasPrefix(pkg.DownloadURL, "http://") || strings.HasPrefix(pkg.DownloadURL, "https://") {
		fmt.Printf("⬇️  ডাউনলোড হচ্ছে: %s\n", pkg.DownloadURL)
		resp, err := http.Get(pkg.DownloadURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ ডাউনলোড করতে ব্যর্থ: %s\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "❌ ডাউনলোড ব্যর্থ (HTTP %d): %s\n", resp.StatusCode, resp.Status)
			os.Exit(1)
		}

		out, err := os.Create(tempFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ টেম্প ফাইল তৈরি করতে ব্যর্থ: %s\n", err)
			os.Exit(1)
		}
		defer out.Close()

		if _, err := io.Copy(out, resp.Body); err != nil {
			fmt.Fprintf(os.Stderr, "❌ ফাইল সেভ করতে ব্যর্থ: %s\n", err)
			os.Exit(1)
		}
		out.Close()
	} else if strings.HasPrefix(pkg.DownloadURL, "file://") || len(pkg.DownloadURL) > 0 {
		filePath := strings.TrimPrefix(pkg.DownloadURL, "file://")
		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ প্যাকেজ ফাইল পড়তে ব্যর্থ (%s): %s\n", filePath, err)
			os.Exit(1)
		}
		if err := os.WriteFile(tempFile, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "❌ টেম্প ফাইল সেভ করতে ব্যর্থ: %s\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Fprintf(os.Stderr, "❌ প্যাকেজে কোনো ডাউনলোড ইউআরএল নেই\n")
		os.Exit(1)
	}

	installFromFile(installer, tempFile)
	_ = os.Remove(tempFile)
}
