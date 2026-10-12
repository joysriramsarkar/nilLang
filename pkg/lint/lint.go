// Package lint implements the `nil lint` rule set (L9.4): small,
// line-based, zero-false-positive checks. Every rule fires only on text the
// formatter itself would change or the style guide forbids, so a clean file
// stays clean and a flagged file is always worth a look.
package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Severity of a finding.
type Severity string

const (
	Warning Severity = "warning"
	Error   Severity = "error"
)

// Finding is one rule violation.
type Finding struct {
	File     string
	Line     int // 1-indexed
	Rule     string
	Severity Severity
	Message  string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d: [%s] %s: %s", f.File, f.Line, f.Severity, f.Rule, f.Message)
}

// LintSource checks one file's content. Rules:
//
//	LINT001 (warning): trailing whitespace — the formatter strips it.
//	LINT002 (warning): tab indentation — the project uses spaces.
//	LINT003 (error):   NUL byte — the lexer rejects it; flag early.
func LintSource(file, content string) []Finding {
	var out []Finding
	lines := strings.Split(content, "\n")
	for i, ln := range lines {
		n := i + 1
		trimmed := strings.TrimRight(ln, " \t\r")
		if trimmed != ln {
			out = append(out, Finding{File: file, Line: n, Rule: "LINT001", Severity: Warning, Message: "trailing whitespace"})
		}
		if strings.HasPrefix(ln, "\t") {
			out = append(out, Finding{File: file, Line: n, Rule: "LINT002", Severity: Warning, Message: "tab indentation; use spaces"})
		}
		if strings.IndexByte(ln, 0) >= 0 {
			out = append(out, Finding{File: file, Line: n, Rule: "LINT003", Severity: Error, Message: "NUL byte in source"})
		}
	}
	return out
}

// LintPath lints one .nil file or, for a directory, every .nil file under it
// (skipping build output and VCS dirs, like `nil fmt`).
func LintPath(target string) ([]Finding, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}
	var out []Finding
	if !info.IsDir() {
		data, err := os.ReadFile(target)
		if err != nil {
			return nil, err
		}
		return LintSource(target, string(data)), nil
	}
	err = filepath.Walk(target, func(path string, fi os.FileInfo, walkErr error) error {
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
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			out = append(out, LintSource(path, string(data))...)
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out, err
}

// HasErrors reports whether any finding is error-severity.
func HasErrors(findings []Finding) bool {
	for _, f := range findings {
		if f.Severity == Error {
			return true
		}
	}
	return false
}
