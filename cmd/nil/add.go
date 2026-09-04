package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/joysriramsarkar/nilLang/pkg/config"
)

func cmdAdd() {
	if len(os.Args) < 3 {
		fmt.Println("ব্যবহার: nil add <package-name>")
		fmt.Println()
		fmt.Println("জনপ্রিয় প্যাকেজসমূহ:")
		fmt.Println("  alap/web       - Alap Web Application Profile & SSR")
		fmt.Println("  alap/ui        - Alap Declarative UI & Component System")
		fmt.Println("  alap/server    - Alap Server Microservices & REST Endpoints")
		fmt.Println("  alap/data      - Alap Data Science & Machine Learning Pipeline")
		fmt.Println("  alap/auth      - Authentication & Permission Management")
		fmt.Println("  alap/onuron    - Onuron OS Native System Adapter")
		return
	}

	pkgName := strings.TrimSpace(os.Args[2])

	cfg, err := config.LoadConfig(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ nil.json লোড করতে ব্যর্থ: %v\n", err)
		fmt.Println("   নতুন প্রজেক্ট তৈরি করতে 'nil init' চালান।")
		os.Exit(1)
	}

	if cfg.Dependencies == nil {
		cfg.Dependencies = make(map[string]string)
	}

	// Determine version
	version := "^0.2.0"
	cfg.Dependencies[pkgName] = version

	if err := cfg.Save("."); err != nil {
		fmt.Fprintf(os.Stderr, "❌ ডিপেন্ডেন্সি সেভ করতে ব্যর্থ: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ সফলভাবে যোগ করা হয়েছে: %s (%s)\n", pkgName, version)
	fmt.Printf("   আপডেট হয়েছে: nil.json\n")
}
