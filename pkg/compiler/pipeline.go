package compiler

import (
	"fmt"
	"os"

	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
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

	// Phase 3: Bytecode compilation
	comp := compiler.New()
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
	return p.bytecode.Instructions
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
