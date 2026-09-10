package tests

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompilerArchitecturalDecoupling(t *testing.T) {
	compilerDir := filepath.Join("..", "compiler")

	forbiddenPrefixes := []string{
		"github.com/joysriramsarkar/nilLang/pkg/alap",
		"github.com/joysriramsarkar/nilLang/pkg/oracle",
		"github.com/joysriramsarkar/nilLang/cmd",
	}

	fset := token.NewFileSet()
	var violations []string

	err := filepath.Walk(compilerDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only check non-test production Go files
		if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			node, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if parseErr != nil {
				return parseErr
			}

			for _, imp := range node.Imports {
				impPath := strings.Trim(imp.Path.Value, `"`)
				for _, forbidden := range forbiddenPrefixes {
					if strings.HasPrefix(impPath, forbidden) {
						violations = append(violations, path+" imports "+impPath)
					}
				}
			}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("failed to walk compiler directory: %v", err)
	}

	if len(violations) > 0 {
		t.Fatalf("Architectural Boundary Violation! The compiler core must be decoupled from application packages:\n%s",
			strings.Join(violations, "\n"))
	}
}

func TestRuntimeArchitecturalDecoupling(t *testing.T) {
	runtimeDir := filepath.Join("..", "runtime")

	forbiddenPrefixes := []string{
		"github.com/joysriramsarkar/nilLang/pkg/alap",
		"github.com/joysriramsarkar/nilLang/pkg/oracle",
		"github.com/joysriramsarkar/nilLang/cmd",
	}

	fset := token.NewFileSet()
	var violations []string

	err := filepath.Walk(runtimeDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			node, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if parseErr != nil {
				return parseErr
			}

			for _, imp := range node.Imports {
				impPath := strings.Trim(imp.Path.Value, `"`)
				for _, forbidden := range forbiddenPrefixes {
					if strings.HasPrefix(impPath, forbidden) {
						violations = append(violations, path+" imports "+impPath)
					}
				}
			}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("failed to walk runtime directory: %v", err)
	}

	if len(violations) > 0 {
		t.Fatalf("Architectural Boundary Violation! The runtime must be decoupled from application packages:\n%s",
			strings.Join(violations, "\n"))
	}
}
