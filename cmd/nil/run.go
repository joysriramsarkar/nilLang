package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
	"github.com/joysriramsarkar/nilLang/pkg/config"
)

func cmdRun() {
	useVM := false
	var targetFile string

	for _, arg := range os.Args[2:] {
		if arg == "-vm" || arg == "--vm" {
			useVM = true
		} else if strings.HasSuffix(arg, ".nil") || strings.HasSuffix(arg, ".nilax") {
			targetFile = arg
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

	entryPath := cfg.GetEntryPath(projectDir)
	runDirectFile(entryPath, useVM)
}

func runDirectFile(filePath string, useVM bool) {
	source, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ ফাইল পড়তে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	l := lexer.New(string(source))
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
