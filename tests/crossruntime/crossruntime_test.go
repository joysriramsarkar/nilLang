// Package crossruntime provides a cross-runtime conformance test suite for NilLang.
//
// It runs .nil test programs through multiple execution engines and compares
// their output against expected results. The suite is designed to be extensible
// for testing across different runtime implementations (Go evaluator, Go VM,
// Rust VM, etc.).
//
// # Test Program Format
//
// Each test program is a .nil file in the programs/ directory with a corresponding
// .expected file containing the expected stdout output, one line per output line.
//
// # Supported Engines
//
//   - evaluator: Go tree-walking interpreter
//   - vm: Go stack-based virtual machine
//   - rust: Rust JSON bytecode runtime (manual verification)
//
// # Adding New Tests
//
// 1. Create a .nil file in programs/ with the test program
// 2. Create a .expected file with the expected output
// 3. The test runner automatically discovers and runs all .nil files
package crossruntime

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
)

// Engine represents a NilLang execution engine.
type Engine interface {
	// Name returns the engine's identifier.
	Name() string
	// Run executes the given source and returns the stdout output.
	Run(t *testing.T, source string) (string, error)
}

// EvaluatorEngine runs programs through the tree-walking evaluator.
type EvaluatorEngine struct{}

func (e *EvaluatorEngine) Name() string { return "evaluator" }

func (e *EvaluatorEngine) Run(t *testing.T, source string) (string, error) {
	t.Helper()
	res := frontend.Check(source, frontend.Options{})
	if !res.OK {
		return "", fmt.Errorf("frontend rejected program: %s", res.ErrSummary())
	}
	program := res.Lowered

	// Capture stdout during evaluation
	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	evalEnv := object.NewEnvironment()
	evalObj := evaluator.Eval(program, evalEnv)

	w.Close()
	os.Stdout = old
	buf.ReadFrom(r)

	if evalObj != nil {
		if errObj, ok := evalObj.(*object.Error); ok {
			return buf.String(), fmt.Errorf("evaluator error: %s", errObj.Message)
		}
		if rtErr, ok := evalObj.(*object.RuntimeError); ok {
			return buf.String(), fmt.Errorf("evaluator runtime error: %s", rtErr.Message)
		}
	}
	return buf.String(), nil
}

// VMEngine runs programs through the stack-based virtual machine.
type VMEngine struct{}

func (e *VMEngine) Name() string { return "vm" }

func (e *VMEngine) Run(t *testing.T, source string) (string, error) {
	t.Helper()
	res := frontend.Check(source, frontend.Options{})
	if !res.OK {
		return "", fmt.Errorf("frontend rejected program: %s", res.ErrSummary())
	}
	program := res.Lowered

	comp := compiler.New()
	if err := comp.Compile(program); err != nil {
		return "", fmt.Errorf("compiler error: %s", err)
	}

	// Capture stdout during VM execution
	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	machine := vm.New(comp.Bytecode())
	vmErr := machine.Run()

	w.Close()
	os.Stdout = old
	buf.ReadFrom(r)

	if vmErr != nil {
		return buf.String(), fmt.Errorf("vm error: %s", vmErr)
	}
	return buf.String(), nil
}

// engines returns all available engines.
func engines() []Engine {
	return []Engine{
		&EvaluatorEngine{},
		&VMEngine{},
	}
}

// discoverPrograms finds all .nil test programs in the programs directory.
func discoverPrograms(t *testing.T) []string {
	t.Helper()
	dir := "programs"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot read programs directory: %v", err)
	}
	var programs []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".nil") {
			programs = append(programs, entry.Name())
		}
	}
	if len(programs) == 0 {
		t.Fatal("no test programs found in programs/ directory")
	}
	return programs
}

// loadExpected reads the expected output for a test program.
func loadExpected(t *testing.T, name string) string {
	t.Helper()
	base := strings.TrimSuffix(name, ".nil")
	data, err := os.ReadFile(filepath.Join("programs", base+".expected"))
	if err != nil {
		t.Fatalf("cannot read expected output for %s: %v", name, err)
	}
	return strings.TrimRight(string(data), "\n")
}

// loadSource reads the source code for a test program.
func loadSource(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("programs", name))
	if err != nil {
		t.Fatalf("cannot read source for %s: %v", name, err)
	}
	return string(data)
}

// TestCrossRuntimeConformance runs all test programs through all engines
// and compares output against expected results.
func TestCrossRuntimeConformance(t *testing.T) {
	programs := discoverPrograms(t)
	engines := engines()

	for _, progName := range programs {
		t.Run(progName, func(t *testing.T) {
			source := loadSource(t, progName)
			expected := loadExpected(t, progName)

			for _, engine := range engines {
				t.Run(engine.Name(), func(t *testing.T) {
					output, err := engine.Run(t, source)
					if err != nil {
						t.Fatalf("[%s] %s failed: %v", progName, engine.Name(), err)
					}
					output = strings.TrimRight(output, "\n")
					if output != expected {
						t.Errorf("[%s] %s output mismatch:\n  got:      %q\n  expected: %q",
							progName, engine.Name(), output, expected)
					}
				})
			}
		})
	}
}

// TestEnginesAgree verifies that all engines produce the same output
// for all test programs, regardless of expected values.
func TestEnginesAgree(t *testing.T) {
	programs := discoverPrograms(t)
	engines := engines()

	for _, progName := range programs {
		t.Run(progName, func(t *testing.T) {
			source := loadSource(t, progName)
			var reference string
			var refEngine string

			for _, engine := range engines {
				output, err := engine.Run(t, source)
				if err != nil {
					t.Fatalf("[%s] %s failed: %v", progName, engine.Name(), err)
				}
				output = strings.TrimRight(output, "\n")

				if refEngine == "" {
					reference = output
					refEngine = engine.Name()
				} else if output != reference {
					t.Errorf("[%s] engine disagreement:\n  %s: %q\n  %s: %q",
						progName, refEngine, reference, engine.Name(), output)
				}
			}
		})
	}
}

// TestProgramCount verifies that the suite has a reasonable number of tests.
func TestProgramCount(t *testing.T) {
	programs := discoverPrograms(t)
	if len(programs) < 20 {
		t.Errorf("expected at least 20 test programs, got %d", len(programs))
	}
	t.Logf("cross-runtime suite has %d test programs", len(programs))
}

// captureOutput captures stdout from a function execution.
func captureOutput(fn func()) (string, error) {
	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old
	io.Copy(&buf, r)
	return buf.String(), nil
}
