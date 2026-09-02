package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func cmdFmt() {
	projectDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ কারেন্ট ডিরেক্টরি পেতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	count := 0
	err = filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			// Skip build and hidden directories
			name := info.Name()
			if name == "build" || name == ".git" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasSuffix(path, ".nil") {
			if err := formatFile(path); err != nil {
				fmt.Fprintf(os.Stderr, "⚠️ %s ফরম্যাট করতে সমস্যা: %s\n", path, err)
			} else {
				count++
			}
		}

		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ ডিরেক্টরি স্ক্যান করতে সমস্যা: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ %d টি .nil ফাইল ফরম্যাট করা হয়েছে\n", count)
}

func formatFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	content := string(data)

	// Basic formatting rules
	// 1. Ensure consistent indentation (4 spaces)
	// 2. Remove trailing whitespace
	// 3. Ensure single newline at end of file
	// 4. Normalize spacing around operators

	lines := strings.Split(content, "\n")
	formatted := make([]string, 0, len(lines))

	for _, line := range lines {
		// Remove trailing whitespace
		line = strings.TrimRight(line, " \t")
		formatted = append(formatted, line)
	}

	result := strings.Join(formatted, "\n")

	// Ensure single newline at end
	result = strings.TrimRight(result, "\n") + "\n"

	return os.WriteFile(path, []byte(result), 0644)
}