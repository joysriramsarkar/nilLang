package main

import (
	"fmt"
	"os"

	"github.com/joysriramsarkar/nilLang/pkg/lint"
)

func cmdLint() {
	targetPath := "."
	for i := 2; i < len(os.Args); i++ {
		if arg := os.Args[i]; len(arg) > 0 && arg[0] != '-' {
			targetPath = arg
		}
	}

	findings, err := lint.LintPath(targetPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ লিন্ট পাথ ত্রুটি: %s\n", err)
		os.Exit(1)
	}

	warns, errs := 0, 0
	for _, f := range findings {
		fmt.Printf("   %s\n", f.String())
		if f.Severity == lint.Error {
			errs++
		} else {
			warns++
		}
	}

	if len(findings) == 0 {
		fmt.Println("✅ লিন্ট পরিষ্কার: কোনো সমস্যা নেই।")
		return
	}
	fmt.Printf("\n⚠️  %d warning(s), %d error(s)\n", warns, errs)
	if errs > 0 {
		os.Exit(1)
	}
}

// checkExitCode decides the process exit for `nil check [--ci]`: errors
// always fail; warnings fail only in CI mode (L9.4), so a developer's local
// run stays green while the pipeline stays strict.
func checkExitCode(errs, warns []string, ci bool) int {
	if len(errs) > 0 {
		return 1
	}
	if ci && len(warns) > 0 {
		return 1
	}
	return 0
}
