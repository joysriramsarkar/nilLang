package main

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/joysriramsarkar/nilLang/pkg/nilpkg"
)

func cmdList(cfg *nilpkg.Config) {
	db, err := nilpkg.NewDatabase(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ ডাটাবেস লোড করতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	packages := db.List()

	if len(packages) == 0 {
		fmt.Println("📦 কোনো প্যাকেজ ইনস্টল নেই")
		fmt.Println("   ইনস্টল করতে: nilpkg install <file.nilax>")
		return
	}

	// Sort by name
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Name < packages[j].Name
	})

	fmt.Printf("📦 ইনস্টলড প্যাকেজ (%d টি)\n", len(packages))
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("%-20s %-10s %-12s %-15s %s\n",
		"নাম", "ভার্সন", "সাইজ", "ইনস্টল তারিখ", "লেখক")
	fmt.Println("───────────────────────────────────────────────────────────")

	for _, pkg := range packages {
		fmt.Printf("%-20s %-10s %-12s %-15s %s\n",
			pkg.Name,
			pkg.Version,
			formatSize(pkg.Size),
			pkg.InstalledAt.Format("2006-01-02"),
			pkg.Author)
	}

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("মোট সাইজ: %s | ইনস্টলড: %s\n",
		formatSize(db.TotalSize()),
		time.Now().Format("2006-01-02 15:04"))
}
