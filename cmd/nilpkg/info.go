package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/joysriramsarkar/nilLang/pkg/nilpkg"
)

func cmdInfo(cfg *nilpkg.Config) {
	if len(os.Args) < 3 {
		fmt.Println("ব্যবহার: nilpkg info <package-name>")
		os.Exit(1)
	}

	name := os.Args[2]

	db, err := nilpkg.NewDatabase(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ ডাটাবেস লোড করতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	pkg, err := db.Get(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ প্যাকেজ ইনস্টল নেই: %s\n", name)
		os.Exit(1)
	}

	fmt.Println("═══════════════════════════════════════════")
	fmt.Printf("📦 %s\n", pkg.Name)
	fmt.Println("═══════════════════════════════════════════")
	fmt.Printf("   ভার্সন:      %s\n", pkg.Version)
	fmt.Printf("   লেখক:       %s\n", pkg.Author)
	fmt.Printf("   বিবরণ:     %s\n", pkg.Description)
	fmt.Printf("   ইনস্টল পাথ: %s\n", pkg.InstallPath)
	fmt.Printf("   সাইজ:       %s\n", formatSize(pkg.Size))
	fmt.Printf("   চেকসাম:     %s...\n", pkg.Checksum[:16])
	fmt.Printf("   ইনস্টল:     %s\n", pkg.InstalledAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("   আপডেট:     %s\n", pkg.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("   টার্গেট:     %s\n", strings.Join(pkg.Targets, ", "))
	fmt.Printf("   এন্ট্রি:     %s\n", pkg.EntryPoint)

	if len(pkg.Dependencies) > 0 {
		fmt.Printf("   ডিপেন্ডেন্সি:\n")
		for dep, ver := range pkg.Dependencies {
			fmt.Printf("     - %s %s\n", dep, ver)
		}
	}

	fmt.Println("═══════════════════════════════════════════")
}
