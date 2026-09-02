package main

import (
	"fmt"
	"os"

	"github.com/joysriramsarkar/nilLang/pkg/nilpkg"
)

func cmdSearch(cfg *nilpkg.Config) {
	if len(os.Args) < 3 {
		fmt.Println("ব্যবহার: nilpkg search <query>")
		os.Exit(1)
	}

	query := os.Args[2]

	registry, err := nilpkg.NewRegistry(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ রেজিস্ট্রি লোড করতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	results := registry.Search(query)

	if len(results) == 0 {
		fmt.Printf("🔍 '%s' এর জন্য কোনো প্যাকেজ পাওয়া যায়নি\n", query)
		return
	}

	fmt.Printf("🔍 '%s' এর জন্য %d টি প্যাকেজ পাওয়া গেছে:\n\n", query, len(results))

	for _, pkg := range results {
		fmt.Printf("📦 %s v%s\n", pkg.Name, pkg.Version)
		fmt.Printf("   %s\n", pkg.Description)
		fmt.Printf("   লেখক: %s | ডাউনলোড: %d\n", pkg.Author, pkg.Downloads)
		fmt.Println()
	}
}