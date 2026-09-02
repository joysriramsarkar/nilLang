package main

import (
	"fmt"
	"os"

	"github.com/joysriramsarkar/nilLang/pkg/nilpkg"
)

func cmdVerify(cfg *nilpkg.Config) {
	if len(os.Args) < 3 {
		fmt.Println("ব্যবহার: nilpkg verify <file.nilax>")
		os.Exit(1)
	}

	path := os.Args[2]

	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "❌ ফাইল পাওয়া যায়নি: %s\n", path)
		os.Exit(1)
	}

	fmt.Printf("🔍 ভেরিফাই করা হচ্ছে: %s\n\n", path)

	verifier := nilpkg.NewVerifier(cfg)
	result, err := verifier.VerifyBundle(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ ভেরিফিকেশন ত্রুটি: %s\n", err)
		os.Exit(1)
	}

	fmt.Print(result.String())

	if !result.Valid {
		os.Exit(1)
	}
}
