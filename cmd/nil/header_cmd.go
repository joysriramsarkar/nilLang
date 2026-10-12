package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/cheader"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/mir"
	"github.com/joysriramsarkar/nilLang/pkg/config"
)

// writeCHeader generates the C header for a project's foreign declarations.
//
// # Why it is its own command and not a `build` flag
//
// Because it answers a question a *build* does not. A build produces a binary; this
// produces the thing a person reads to find out what the binary expects. Making it a
// flag would mean it came and went with a build, and a C author would regenerate the
// whole program to get a file they only wanted to read.
//
// # Why it goes through the whole front end
//
// Because a header written from the source text would be a second parser. The
// project goes through the checker, the HIR and the MIR like every other path, so
// the declarations it reads are the ones the compiler itself read — a header that
// disagrees with the module is then a bug in one place rather than two.
func writeCHeader(projectDir string, outPath string) string {
	cfg, err := config.LoadConfig(projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ nil.json পড়া যায়নি: %s\n", err)
		return ""
	}

	entry := filepath.Join(projectDir, cfg.Entry)
	source, err := os.ReadFile(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ এন্ট্রি পড়া যায়নি: %s\n", err)
		return ""
	}

	res := frontend.Check(string(source), frontend.Options{Filename: cfg.Entry})
	if !res.OK {
		fmt.Fprintln(os.Stderr, "❌ টাইপ ত্রুটি:")
		for _, d := range res.Diagnostics {
			fmt.Fprintf(os.Stderr, "   %s\n", d.String())
		}
		return ""
	}

	optimized := hir.NewOptimizer().Optimize(hir.NewLowerer().LowerProgram(res.Lowered))
	mirProg := mir.NewLowerer().LowerHIR(optimized)
	mir.NewOptimizer().OptimizeProgram(mirProg)

	header, err := cheader.Generate(mirProg, cfg.Name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ C হেডার তৈরিতে সমস্যা: %s\n", err)
		return ""
	}

	if outPath == "" {
		outputDir := cfg.Build.OutputDir
		if outputDir == "" {
			outputDir = "build"
		}
		outPath = filepath.Join(projectDir, outputDir, cfg.Name+".h")
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "❌ হেডার ডিরেক্টরি তৈরি যায়নি: %s\n", err)
		return ""
	}

	if err := os.WriteFile(outPath, []byte(header), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "❌ হেডার লেখা যায়নি: %s\n", err)
		return ""
	}

	fmt.Printf("C হেডার লেখা হয়েছে: %s\n", outPath)
	return outPath
}

// cmdHeader is `nil header`.
func cmdHeader(args []string) {
	dir := projectDirFrom(args)
	if dir == "" {
		dir = "."
	}
	writeCHeader(dir, headerOutputPath(args))
}

// headerOutputPath returns the value of `--out=`, or "" when none was given.
//
// Read from the same arguments `--dir` is read from, and accepting both spellings,
// because a flag that works in one spelling and not the other is a flag somebody
// types wrong once and then works around forever.
func headerOutputPath(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		lower := strings.ToLower(arg)
		if strings.HasPrefix(lower, "--out=") {
			return arg[len("--out="):]
		}
		if strings.HasPrefix(lower, "-out=") {
			return arg[len("-out="):]
		}
		if lower == "--out" || lower == "-out" {
			if i+1 < len(args) {
				return args[i+1]
			}
		}
	}
	return ""
}
