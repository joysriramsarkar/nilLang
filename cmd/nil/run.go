package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
	"github.com/joysriramsarkar/nilLang/pkg/bundle"
	"github.com/joysriramsarkar/nilLang/pkg/config"
)

func runProjectScript(scriptName string) bool {
	projectDir, err := os.Getwd()
	if err != nil {
		return false
	}
	cfg, err := config.LoadConfig(projectDir)
	if err != nil || cfg.Scripts == nil {
		return false
	}
	cmdLine, ok := cfg.Scripts[scriptName]
	if !ok {
		return false
	}
	runShellCommand(cmdLine)
	return true
}

func runShellCommand(cmdLine string) {
	fmt.Printf("➜ নীলাং স্ক্রিপ্ট: %s\n", cmdLine)
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", cmdLine)
	} else {
		cmd = exec.Command("sh", "-c", cmdLine)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

func cmdRun() {
	useVM := false
	var targetFile string
	var scriptName string

	for _, arg := range os.Args[2:] {
		if arg == "-vm" || arg == "--vm" {
			useVM = true
		} else if strings.HasSuffix(arg, ".nil") || strings.HasSuffix(arg, ".nilax") {
			targetFile = arg
		} else if scriptName == "" && !strings.HasPrefix(arg, "-") {
			scriptName = arg
		}
	}

	// Case 1: Direct file execution
	if targetFile != "" {
		runDirectFile(targetFile, useVM)
		return
	}

	// Case 2: Project directory execution
	projectDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ কারেন্ট ডিরেক্টরি পেতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	cfg, err := config.LoadConfig(projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %s\n", err)
		os.Exit(1)
	}

	// Check if scriptName matches a script in nil.json
	if scriptName != "" && cfg.Scripts != nil {
		if cmdStr, ok := cfg.Scripts[scriptName]; ok {
			runShellCommand(cmdStr)
			return
		}
	}

	entryPath := cfg.GetEntryPath(projectDir)
	runDirectFile(entryPath, useVM)
}

func runDirectFile(filePath string, useVM bool) {
	if strings.HasSuffix(filePath, ".nilax") {
		runBundleFile(filePath, useVM)
		return
	}

	absPath, err := filepath.Abs(filePath)
	if err == nil {
		evaluator.PushScriptDir(filepath.Dir(absPath))
		defer evaluator.PopScriptDir()
	}

	source, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ ফাইল পড়তে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	executeSource(string(source), useVM)
}

func runBundleFile(bundlePath string, useVM bool) {
	r, err := bundle.OpenBundle(bundlePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ .nilax বান্ডিল খুলতে সমস্যা: %s\n", err)
		os.Exit(1)
	}
	defer r.Close()

	manifest := r.GetManifest()
	fmt.Printf("📦 নির্বাহ হচ্ছে: %s v%s\n", manifest.AppName, manifest.AppVersion)

	// If source is present in bundle, execute it
	if srcBytes, err := r.GetFile("src/main.nil"); err == nil {
		executeSource(string(srcBytes), useVM)
		return
	}

	// Fallback to compiled bytecode if available
	if bcBytes, err := r.GetBytecode(); err == nil && len(bcBytes) > 0 {
		machine := vm.New(&compiler.Bytecode{
			Instructions: bcBytes,
			Constants:    []object.Object{},
		})
		if err := machine.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ রানটাইম ত্রুটি: %s\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Fprintf(os.Stderr, "❌ বান্ডিলে কোনো এক্সিকিউটেবল কোড পাওয়া যায়নি\n")
	os.Exit(1)
}

func executeSource(source string, useVM bool) {
	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		fmt.Fprintf(os.Stderr, "❌ সিনট্যাক্স ত্রুটি:\n")
		for _, e := range p.Errors() {
			fmt.Fprintf(os.Stderr, "   %s\n", e)
		}
		os.Exit(1)
	}

	if useVM {
		comp := compiler.New()
		err := comp.Compile(program)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ কম্পাইলেশন ত্রুটি: %s\n", err)
			os.Exit(1)
		}
		machine := vm.New(comp.Bytecode())
		err = machine.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ রানটাইম ত্রুটি: %s\n", err)
			os.Exit(1)
		}
		return
	}

	// Tree-walking evaluator
	env := object.NewEnvironment()
	registerBuiltins(env)

	result := evaluator.Eval(program, env)
	if result != nil {
		if result.Type() == object.ERROR_OBJ {
			fmt.Fprintf(os.Stderr, "❌ রানটাইম ত্রুটি: %s\n", result.Inspect())
			os.Exit(1)
		}
	}
}

func registerBuiltins(env *object.Environment) {
	for name, builtin := range evaluator.Builtins {
		env.Set(name, builtin)
	}
}
