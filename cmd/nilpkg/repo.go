package main

import (
	"fmt"
	"os"

	"github.com/joysriramsarkar/nilLang/pkg/nilpkg"
)

func cmdRepo(cfg *nilpkg.Config) {
	if len(os.Args) < 3 {
		fmt.Println("ব্যবহার: nilpkg repo <add|list|remove> [url]")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "add":
		if len(os.Args) < 4 {
			fmt.Println("ব্যবহার: nilpkg repo add <url>")
			os.Exit(1)
		}
		url := os.Args[3]
		if err := cfg.AddRepository(url); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %s\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ রেপোজিটরি যোগ হয়েছে: %s\n", url)

	case "remove":
		if len(os.Args) < 4 {
			fmt.Println("ব্যবহার: nilpkg repo remove <url>")
			os.Exit(1)
		}
		url := os.Args[3]
		if err := cfg.RemoveRepository(url); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %s\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ রেপোজিটরি মুছে ফেলা হয়েছে: %s\n", url)

	case "list":
		fmt.Println("📋 রেপোজিটরি তালিকা:")
		for i, repo := range cfg.Repositories {
			fmt.Printf("   %d. %s\n", i+1, repo)
		}

	default:
		fmt.Fprintf(os.Stderr, "❌ অজানা সাবকমান্ড: %s\n", subcommand)
		os.Exit(1)
	}
}