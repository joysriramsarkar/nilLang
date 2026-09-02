package main

import (
	"fmt"
	"os"

	"github.com/joysriramsarkar/nilLang/pkg/nilpkg"
)

func cmdUpdate(cfg *nilpkg.Config) {
	db, err := nilpkg.NewDatabase(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ ডাটাবেস লোড করতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	if len(os.Args) < 3 {
		fmt.Println("ব্যবহার: nilpkg update <package-name | --all>")
		os.Exit(1)
	}

	target := os.Args[2]

	if target == "--all" {
		updateAll(cfg, db)
		return
	}

	// Update specific package
	if !db.Has(target) {
		fmt.Fprintf(os.Stderr, "❌ প্যাকেজ ইনস্টল নেই: %s\n", target)
		os.Exit(1)
	}

	fmt.Printf("📦 %s আপডেট করা হচ্ছে...\n", target)
	fmt.Println("⚠️  আপডেট ফিচার এখনো ইমপ্লিমেন্ট হয়নি")
	fmt.Println("   ম্যানুয়ালি আপডেট করতে: nilpkg install <new-version.nilax>")
}

func updateAll(cfg *nilpkg.Config, db *nilpkg.Database) {
	_ = cfg
	packages := db.List()

	if len(packages) == 0 {
		fmt.Println("📦 কোনো প্যাকেজ ইনস্টল নেই")
		return
	}

	fmt.Printf("🔄 %d টি প্যাকেজ আপডেট চেক করা হচ্ছে...\n", len(packages))
	fmt.Println("⚠️  অটো-আপডেট ফিচার এখনো ইমপ্লিমেন্ট হয়নি")
	fmt.Println("   ম্যানুয়ালি আপডেট করতে: nilpkg install <new-version.nilax>")
}
