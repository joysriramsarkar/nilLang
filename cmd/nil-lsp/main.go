package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/joysriramsarkar/nilLang/pkg/lsp"
)

const Version = "1.0.0"

func main() {
	versionFlag := flag.Bool("version", false, "Print version and exit")
	vFlag := flag.Bool("v", false, "Print version and exit (shorthand)")
	flag.Parse()

	if *versionFlag || *vFlag {
		fmt.Printf("nil-lsp %s (Nilang Language Server Protocol)\n", Version)
		os.Exit(0)
	}

	server := lsp.NewServer(os.Stdin, os.Stdout)
	if err := server.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "nil-lsp server error: %v\n", err)
		os.Exit(1)
	}
}
