package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/diagnostics"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	arg := os.Args[1]
	switch arg {
	case "-h", "--help", "help":
		printUsage()
		return
	case "-v", "--version", "version":
		fmt.Println("nilc v0.1.0 - Dedicated Nilang Bytecode Compiler & Disassembler")
		return
	}

	filename := arg

	// Read source file
	source, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("ফাইল পড়তে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	// Compile
	bytecode, err := compileSource(string(source))
	if err != nil {
		fmt.Printf("কম্পাইলেশন এরর: %s\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ কম্পাইলেশন সফল!")
	fmt.Printf("📊 Bytecode সাইজ: %d বাইট, ধ্রুবক: %d টি\n",
		len(bytecode.Instructions), len(bytecode.Constants))
	fmt.Println("\n--- Disassembled Bytecode ---")
	fmt.Println(vm.Disassemble(bytecode.Instructions))

	// Execute
	fmt.Println("\n--- এক্সিকিউশন শুরু ---")
	machine := vm.New(bytecode)
	err = machine.Run()
	if err != nil {
		fmt.Printf("রানটাইম এরর: %s\n", err)
		os.Exit(1)
	}
}

func compileSource(source string) (*compiler.Bytecode, error) {
	res := frontend.Check(source, frontend.Options{})
	if len(res.ParseErrors) != 0 {
		return nil, fmt.Errorf("parser errors:\n%s", strings.Join(res.ParseErrors, "\n"))
	}
	if !res.OK {
		msgs := make([]string, 0, len(res.Diagnostics))
		for _, d := range res.Diagnostics {
			if d.Severity == diagnostics.SeverityError {
				msgs = append(msgs, d.String())
			}
		}
		return nil, fmt.Errorf("type checking failed:\n%s", strings.Join(msgs, "\n"))
	}

	comp := compiler.New()
	if err := comp.Compile(res.Program); err != nil {
		return nil, err
	}

	return comp.Bytecode(), nil
}

func printUsage() {
	fmt.Println("nilc v0.1.0 - Dedicated Nilang Bytecode Compiler & Disassembler")
	fmt.Println("ব্যবহার: nilc <file.nil>")
	fmt.Println("উদাহরণ: nilc examples/hello-onuron/src/main.nil")
}
