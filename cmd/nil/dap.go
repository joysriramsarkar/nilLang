package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joysriramsarkar/nilLang/pkg/dap"
)

// cmdDAP runs a Nilang program under the Debug Adapter Protocol server.
//
//	nil dap path/to/main.nil
//
// Editors and the NilOS Studio emulator speak standard DAP to this process.
func cmdDAP() {
	file := ""
	for _, arg := range os.Args[2:] {
		if !strings.HasPrefix(arg, "-") {
			file = arg
			break
		}
	}
	if file == "" {
		fmt.Fprintln(os.Stderr, "ব্যবহার: nil dap <file.nil>")
		os.Exit(1)
	}
	src, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ ফাইল পড়তে সমস্যা: %s\n", err)
		os.Exit(1)
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		abs = file
	}
	srv := dap.NewServer(abs, string(src))
	if err := srv.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "nil dap: %v\n", err)
		os.Exit(1)
	}
}
