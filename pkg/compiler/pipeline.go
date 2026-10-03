package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/compiler/typecheck"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
	"github.com/joysriramsarkar/nilLang/pkg/stdlib"
)

// Pipeline represents the complete compilation pipeline
type Pipeline struct {
	source   string
	filename string
	bytecode *compiler.Bytecode
	errors   []string
	warnings []string
}

// NewPipeline creates a new compilation pipeline
func NewPipeline(source, filename string) *Pipeline {
	return &Pipeline{
		source:   source,
		filename: filename,
		errors:   []string{},
		warnings: []string{},
	}
}

// CompileFile reads and compiles a .nil file
func CompileFile(path string) (*Pipeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	pipeline := NewPipeline(string(data), path)
	if err := pipeline.Compile(); err != nil {
		return nil, err
	}

	return pipeline, nil
}

// Compile runs the full compilation pipeline
func (p *Pipeline) Compile() error {
	// Phase 1: Lexing
	l := lexer.New(p.source)

	// Phase 2: Parsing
	psr := parser.New(l)
	program := psr.ParseProgram()

	if len(psr.Errors()) > 0 {
		p.errors = psr.Errors()
		return fmt.Errorf("compilation failed with %d error(s):\n%s",
			len(psr.Errors()), formatErrors(psr.Errors()))
	}

	// Phase 3: Static type checking
	checker := typecheck.NewChecker()
	if !checker.CheckProgram(program) {
		for _, diagnostic := range checker.Diagnostics {
			p.errors = append(p.errors, diagnostic.String())
		}
		return fmt.Errorf("type checking failed with %d error(s):\n%s",
			len(p.errors), formatErrors(p.errors))
	}

	// Phase 4: Bytecode compilation
	comp := compiler.New()
	comp.SourceDir = filepath.Dir(p.filename)
	comp.LoadModule = loadModuleAST
	if err := comp.Compile(program); err != nil {
		return fmt.Errorf("bytecode compilation failed: %w", err)
	}

	p.bytecode = comp.Bytecode()
	return nil
}

// GetBytecode returns the compiled bytecode
func (p *Pipeline) GetBytecode() *compiler.Bytecode {
	return p.bytecode
}

// GetBytecodeBytes returns the bytecode as raw bytes for serialization
func (p *Pipeline) GetBytecodeBytes() []byte {
	if p.bytecode == nil {
		return nil
	}
	image, err := EncodeBytecode(p.bytecode)
	if err != nil {
		return nil
	}
	return image
}

func (p *Pipeline) GetBytecodeImage() ([]byte, error) {
	return EncodeBytecode(p.bytecode)
}

// GetDisassembly returns the disassembled bytecode
func (p *Pipeline) GetDisassembly() string {
	if p.bytecode == nil {
		return ""
	}
	return vm.Disassemble(p.bytecode.Instructions)
}

func formatErrors(errors []string) string {
	result := ""
	for i, err := range errors {
		result += fmt.Sprintf("  %d. %s\n", i+1, err)
	}
	return result
}

// ConfigureCompiler equips a bytecode compiler with the module-resolution
// hooks (source directory + file/stdlib module loader) required to compile
// programs that use import statements. Call this before comp.Compile(program)
// on every bytecode compilation entry point so the VM backend has the same
// module semantics as the tree-walking evaluator.
func ConfigureCompiler(comp *compiler.Compiler, sourceDir string) {
	comp.SourceDir = sourceDir
	comp.LoadModule = loadModuleAST
}

// loadModuleAST resolves a file-based import to its parsed AST. It supports
// embedded standard library modules (std/...) and relative .nil source files,
// so the bytecode pipeline can inline modules the same way the evaluator does.
func loadModuleAST(importPath, sourceDir string) (*ast.Program, string, error) {
	// Embedded standard library module
	if strings.HasPrefix(importPath, "std/") && stdlib.Exists(importPath) {
		content, err := stdlib.Read(importPath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read standard library module %q: %s", importPath, err)
		}
		prog, err := parseModuleSource(string(content), importPath)
		return prog, importPath, err
	}

	// Relative or bare file path
	filePath := importPath
	if !strings.HasSuffix(filePath, ".nil") && !strings.Contains(filePath, ".") {
		filePath += ".nil"
	}

	var candidates []string
	if sourceDir != "" && !filepath.IsAbs(filePath) {
		candidates = append(candidates, filepath.Join(sourceDir, filePath))
	}
	candidates = append(candidates, filePath)

	for _, cand := range candidates {
		if _, err := os.Stat(cand); err != nil {
			continue
		}
		content, err := os.ReadFile(cand)
		if err != nil {
			return nil, "", fmt.Errorf("cannot read module file '%s': %s", cand, err)
		}
		abs, absErr := filepath.Abs(cand)
		if absErr != nil {
			abs = cand
		}
		prog, err := parseModuleSource(string(content), abs)
		return prog, abs, err
	}

	return nil, "", fmt.Errorf("cannot find module or file '%s'", importPath)
}

func parseModuleSource(content, path string) (*ast.Program, error) {
	l := lexer.New(content)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, fmt.Errorf("parse error in %s: %s", path, strings.Join(p.Errors(), "; "))
	}
	return prog, nil
}
