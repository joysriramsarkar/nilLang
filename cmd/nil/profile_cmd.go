package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/joysriramsarkar/nilLang/pkg/capability"
	"github.com/joysriramsarkar/nilLang/pkg/profile"
)

func cmdProfile() {
	if len(os.Args) < 3 {
		printProfileHelp()
		return
	}

	action := strings.ToLower(os.Args[2])
	switch action {
	case "list":
		fmt.Println("📋 NilLang Runtime Profiles (refactor.md Section 5):")
		fmt.Println("═══════════════════════════════════════════════════════════════")
		for _, p := range profile.ListAll() {
			fmt.Printf("  • \033[1;36m%-10s\033[0m %-28s [Target: %s]\n", p.ID, p.Name, p.Target)
			fmt.Printf("    %s\n", p.Description)
			fmt.Println()
		}

	case "inspect":
		if len(os.Args) < 4 {
			fmt.Println("ব্যবহার: nil profile inspect <profile-id>")
			fmt.Println("উপলব্ধ প্রোফাইল: core, web, mobile, server, data, os, embedded")
			return
		}
		pID := strings.ToLower(os.Args[3])
		p, err := profile.Get(pID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("🔍 NilLang Profile: \033[1;36m%s\033[0m\n", p.Name)
		fmt.Printf("   ID:          %s\n", p.ID)
		fmt.Printf("   Target:      %s\n", p.Target)
		fmt.Printf("   Description: %s\n", p.Description)
		fmt.Println()
		fmt.Println("   অনুমোদিত ক্যাপাবিলিটিস (Capabilities):")
		matrix := capability.CapabilityMatrix[string(p.ID)]
		for _, capType := range capability.AllCapabilities {
			perm := matrix[capType]
			switch perm {
			case capability.PermAllowed:
				fmt.Printf("     ✅ \033[32m%-12s\033[0m ALLOWED\n", capType)
			case capability.PermRestricted:
				fmt.Printf("     ⚠️ \033[33m%-12s\033[0m RESTRICTED (Sandbox/Prompt)\n", capType)
			case capability.PermDenied:
				fmt.Printf("     ❌ \033[90m%-12s\033[0m DENIED\n", capType)
			}
		}
		fmt.Println()
		fmt.Println("   রানটাইম এপিআইসমূহ (Runtime APIs):")
		for _, api := range p.RuntimeAPIs {
			fmt.Printf("     • %s\n", api)
		}

	default:
		printProfileHelp()
	}
}

func printProfileHelp() {
	fmt.Println("ব্যবহার: nil profile <সাব-কমান্ড>")
	fmt.Println()
	fmt.Println("সাব-কমান্ডসমূহ:")
	fmt.Println("  list              সকল NilLang রানটাইম প্রোফাইল দেখুন")
	fmt.Println("  inspect <name>    নির্দিষ্ট প্রোফাইলের বিস্তারিত অনুমতি ও এপিআই দেখুন")
	fmt.Println()
	fmt.Println("উদাহরণ:")
	fmt.Println("  nil profile list")
	fmt.Println("  nil profile inspect web")
	fmt.Println("  nil profile inspect server")
}
