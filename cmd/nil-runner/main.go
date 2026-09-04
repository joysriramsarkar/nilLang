package main

import (
	"fmt"
	"os"

	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
	"github.com/joysriramsarkar/nilLang/pkg/bundle"
)

func main() {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error finding executable path: %v\n", err)
		os.Exit(1)
	}

	reader, err := bundle.OpenBundle(exePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error opening embedded NilLang application: %v\n", err)
		os.Exit(1)
	}
	defer reader.Close()

	// Check if source entry is present in bundle
	if srcBytes, err := reader.GetFile("src/main.nil"); err == nil {
		executeSource(string(srcBytes))
		return
	}

	// Fallback to bytecode if available
	if bcBytes, err := reader.GetBytecode(); err == nil && len(bcBytes) > 0 {
		machine := vm.New(&compiler.Bytecode{
			Instructions: bcBytes,
			Constants:    []object.Object{},
		})
		if err := machine.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Runtime error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Fprintf(os.Stderr, "❌ No executable entry point found in bundle\n")
	os.Exit(1)
}

func executeSource(source string) {
	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		fmt.Fprintf(os.Stderr, "❌ Syntax errors:\n")
		for _, e := range p.Errors() {
			fmt.Fprintf(os.Stderr, "   %s\n", e)
		}
		os.Exit(1)
	}

	env := object.NewEnvironment()
	for name, builtin := range evaluator.Builtins {
		env.Set(name, builtin)
	}

	cliArgs := []object.Object{}
	for _, arg := range os.Args[1:] {
		cliArgs = append(cliArgs, &object.String{Value: arg})
	}
	env.Set("args", &object.Array{Elements: cliArgs})
	env.Set("ARGV", &object.Array{Elements: cliArgs})

	result := evaluator.Eval(program, env)
	if result != nil {
		if result.Type() == object.ERROR_OBJ {
			fmt.Fprintf(os.Stderr, "❌ Runtime error: %s\n", result.Inspect())
			os.Exit(1)
		}
		if intObj, ok := result.(*object.Integer); ok {
			if intObj.Value != 0 {
				os.Exit(int(intObj.Value))
			}
		}
	}
}
