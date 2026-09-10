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

func TestCoreFreezeContract(t *testing.T) {
	repositoryRoot := ".."
	freezePath := filepath.Join(repositoryRoot, "CORE_FREEZE.md")
	freeze, err := os.ReadFile(freezePath)
	if err != nil {
		t.Fatalf("core freeze contract is required: %v", err)
	}

	requiredFrozenAreas := []string{
		"Language syntax frozen",
		"Type semantics frozen",
		"Scope semantics frozen",
		"Assignment semantics frozen",
		"Function semantics frozen",
		"Module semantics frozen",
		"Error semantics frozen",
		"Concurrency semantics frozen",
		"Memory semantics frozen",
		"Effect semantics frozen",
		"Capability semantics frozen",
	}
	for _, frozenArea := range requiredFrozenAreas {
		if !strings.Contains(string(freeze), frozenArea) {
			t.Errorf("core freeze contract is missing %q", frozenArea)
		}
	}

	requiredSpecifications := []string{
		"LANGUAGE_SPEC.md",
		"MEMORY_MODEL.md",
		"MODULE_SYSTEM.md",
		"SEMANTICS.md",
		"TYPE_SYSTEM.md",
	}
	for _, specification := range requiredSpecifications {
		path := filepath.Join(repositoryRoot, "docs", "spec", specification)
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Errorf("normative specification %s is required: %v", specification, statErr)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("normative specification %s must not be empty", specification)
		}
	}
}
