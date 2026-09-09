package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	pkgcompiler "github.com/joysriramsarkar/nilLang/pkg/compiler"
)

func cmdNative() {
	var inputPath, outputPath, compilerPath string
	keepC := false
	for index := 2; index < len(os.Args); index++ {
		argument := os.Args[index]
		switch {
		case argument == "-o" && index+1 < len(os.Args):
			index++
			outputPath = os.Args[index]
		case strings.HasPrefix(argument, "-o="):
			outputPath = strings.TrimPrefix(argument, "-o=")
		case argument == "--cc" && index+1 < len(os.Args):
			index++
			compilerPath = os.Args[index]
		case strings.HasPrefix(argument, "--cc="):
			compilerPath = strings.TrimPrefix(argument, "--cc=")
		case argument == "--keep-c":
			keepC = true
		case !strings.HasPrefix(argument, "-") && inputPath == "":
			inputPath = argument
		default:
			fmt.Fprintf(os.Stderr, "invalid native option: %s\n", argument)
			os.Exit(2)
		}
	}
	if inputPath == "" {
		fmt.Fprintln(os.Stderr, "usage: nil native <input.nil> [-o output] [--cc compiler] [--keep-c]")
		os.Exit(2)
	}
	source, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", inputPath, err)
		os.Exit(1)
	}
	if outputPath == "" {
		base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
		outputPath = filepath.Join("build", base)
		if runtime.GOOS == "windows" {
			outputPath += ".exe"
		}
	}
	if err := pkgcompiler.CompileNative(string(source), outputPath, pkgcompiler.NativeOptions{Compiler: compilerPath, KeepC: keepC}); err != nil {
		fmt.Fprintf(os.Stderr, "native compilation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("native executable: %s\n", outputPath)
}
