package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/formatter"
)

func cmdFmt() {
	checkMode := false
	targetPath := "."

	for i := 2; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "--check" || arg == "-c" {
			checkMode = true
		} else if !strings.HasPrefix(arg, "-") {
			targetPath = arg
		}
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Invalid path %s: %s\n", targetPath, err)
		os.Exit(1)
	}

	formattedCount := 0
	unformattedCount := 0
	errorCount := 0

	formatSingle := func(filePath string) error {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		original := string(data)
		formatted, err := formatter.Format(original)
		if err != nil {
			return fmt.Errorf("%s: %w", filePath, err)
		}

		if original != formatted {
			unformattedCount++
			if checkMode {
				fmt.Printf("❌ Unformatted: %s\n", filePath)
			} else {
				if err := os.WriteFile(filePath, []byte(formatted), 0644); err != nil {
					return fmt.Errorf("failed to write %s: %w", filePath, err)
				}
				fmt.Printf("Formatted: %s\n", filePath)
				formattedCount++
			}
		}
		return nil
	}

	if !info.IsDir() {
		if strings.HasSuffix(targetPath, ".nil") {
			if err := formatSingle(targetPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}
		}
	} else {
		err = filepath.Walk(targetPath, func(path string, fi os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if fi.IsDir() {
				name := fi.Name()
				if name == "build" || name == ".git" || name == "node_modules" || strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, ".nil") {
				if err := formatSingle(path); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: %s\n", err)
					errorCount++
				}
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning directory: %s\n", err)
			os.Exit(1)
		}
	}

	if checkMode {
		if unformattedCount > 0 {
			fmt.Fprintf(os.Stderr, "\nFound %d unformatted file(s). Run 'nil fmt' to format them.\n", unformattedCount)
			os.Exit(1)
		} else {
			fmt.Println("All .nil files are properly formatted.")
		}
	} else {
		if formattedCount > 0 {
			fmt.Printf("Formatted %d file(s).\n", formattedCount)
		} else {
			fmt.Println("All files are already formatted.")
		}
	}
}
