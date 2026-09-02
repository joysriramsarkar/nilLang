package main

import (
	"fmt"
	"os"

	"github.com/joysriramsarkar/nilLang/pkg/nilpkg"
)

func cmdUninstall(cfg *nilpkg.Config) {
	if len(os.Args) < 3 {
		fmt.Println("ব্যবহার: nilpkg uninstall <package-name>")
		os.Exit(1)
	}

	name := os.Args[2]

	db, err := nilpkg.NewDatabase(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ ডাটাবেস লোড করতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	if !db.Has(name) {
		fmt.Fprintf(os.Stderr, "❌ প্যাকেজ ইনস্টল নেই: %s\n", name)
		os.Exit(1)
	}

	// Confirm uninstallation
	pkg, _ := db.Get(name)
	fmt.Printf("⚠️  আপনি কি নিশ্চিত '%s' v%s আনইনস্টল করতে চান? (y/n): ",
		pkg.Name, pkg.Version)

	var response string
	fmt.Scanln(&response)
	if response != "y" && response != "Y" {
		fmt.Println("❌ আনইনস্টলেশন বাতিল করা হয়েছে")
		return
	}

	installer := nilpkg.NewInstaller(cfg, db)
	if err := installer.Uninstall(name); err != nil {
		fmt.Fprintf(os.Stderr, "❌ আনইনস্টলেশন ব্যর্থ: %s\n", err)
		os.Exit(1)
	}
}