package main

import "github.com/joysriramsarkar/nilLang/pkg/nilpkg"

func formatSize(bytes int64) string {
	return nilpkg.FormatSize(bytes)
}
