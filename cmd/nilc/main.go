package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("ব্যবহার: nilc <file.nil>")
		fmt.Println("উদাহরণ: nilc examples/hello.nil")
		os.Exit(1)
	}

	filename := os.Args[1]

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
	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 0 {
		return nil, fmt.Errorf("parser errors:\n%s", strings.Join(p.Errors(), "\n"))
	}

	comp := compiler.New()
	err := comp.Compile(program)
	if err != nil {
		return nil, err
	}

	return comp.Bytecode(), nil
}
