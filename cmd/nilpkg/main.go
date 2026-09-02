package main

import (
	"fmt"
	"os"

	"github.com/joysriramsarkar/nilLang/pkg/nilpkg"
)

const VERSION = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Load configuration
	cfg, err := nilpkg.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ কনফিগারেশন লোড করতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "install":
		cmdInstall(cfg)
	case "uninstall", "remove":
		cmdUninstall(cfg)
	case "update", "upgrade":
		cmdUpdate(cfg)
	case "list", "ls":
		cmdList(cfg)
	case "search":
		cmdSearch(cfg)
	case "info":
		cmdInfo(cfg)
	case "verify":
		cmdVerify(cfg)
	case "repo":
		cmdRepo(cfg)
	case "version":
		fmt.Printf("nilpkg v%s\n", VERSION)
		fmt.Println("Nilang Package Manager • Onuron OS")
	case "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "❌ অজানা কমান্ড: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf("nilpkg v%s - Nilang Package Manager\n\n", VERSION)
	fmt.Println("ব্যবহার: nilpkg <কমান্ড> [আর্গুমেন্ট]")
	fmt.Println()
	fmt.Println("কমান্ড:")
	fmt.Println("  install <file.nilax>     .nilax বান্ডিল ইনস্টল করুন")
	fmt.Println("  install <package-name>   রেজিস্ট্রি থেকে প্যাকেজ ইনস্টল করুন")
	fmt.Println("  uninstall <name>         প্যাকেজ আনইনস্টল করুন")
	fmt.Println("  update <name>            প্যাকেজ আপডেট করুন")
	fmt.Println("  update --all             সব প্যাকেজ আপডেট করুন")
	fmt.Println("  list                     ইনস্টলড প্যাকেজ তালিকা দেখুন")
	fmt.Println("  search <query>           রেজিস্ট্রিতে প্যাকেজ খুঁজুন")
	fmt.Println("  info <name>              প্যাকেজ তথ্য দেখুন")
	fmt.Println("  verify <file.nilax>      বান্ডিল ভেরিফাই করুন")
	fmt.Println("  repo add <url>           নতুন রেপোজিটরি যোগ করুন")
	fmt.Println("  repo list                রেপোজিটরি তালিকা দেখুন")
	fmt.Println("  version                  ভার্সন তথ্য দেখুন")
	fmt.Println("  help                     এই সাহায্য বার্তা দেখুন")
	fmt.Println()
	fmt.Println("উদাহরণ:")
	fmt.Println("  nilpkg install build/hello-onuron-1.0.0.nilax")
	fmt.Println("  nilpkg list")
	fmt.Println("  nilpkg uninstall hello-onuron")
}