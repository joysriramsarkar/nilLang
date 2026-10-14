package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/diagnostics"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/hir"
	"github.com/joysriramsarkar/nilLang/compiler/mir"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
	"github.com/joysriramsarkar/nilLang/pkg/stdlib"
)

// Backend names which path turns the lowered program into bytecode.
type Backend string

const (
	// BackendDirect is the AST-to-bytecode compiler: the long-tested path, and
	// the default.
	BackendDirect Backend = "direct"

	// BackendMir routes through HIR, MIR and the MIR bytecode emitter, so that
	// the VM and WebAssembly consume the same canonical IR.
	BackendMir Backend = "mir"
)

// BackendFromArgs reads a `--backend=` flag out of a command line.
//
// It is one function so that `nil run`, `nil build` and the bare `.nil` shortcut
// cannot each decide for themselves which spelling means what.
func BackendFromArgs(args []string) Backend {
	for _, arg := range args {
		if arg == "--backend=mir" {
			return BackendMir
		}
	}
	return BackendDirect
}

// Pipeline represents the complete compilation pipeline
type Pipeline struct {
	source   string
	filename string
	bytecode *compiler.Bytecode
	errors   []string
	warnings []string
	backend  Backend
}

// NewPipeline creates a new compilation pipeline
func NewPipeline(source, filename string) *Pipeline {
	return &Pipeline{
		source:   source,
		filename: filename,
		errors:   []string{},
		warnings: []string{},
		backend:  BackendDirect,
	}
}

// WithBackend selects the bytecode path and returns the pipeline, so a caller
// can say which one it meant without a second statement.
func (p *Pipeline) WithBackend(backend Backend) *Pipeline {
	p.backend = backend
	return p
}

// CompileViaMir runs the canonical IR path: HIR, then MIR, then bytecode.
//
// It is exported because two callers need it and a rule written twice is a rule
// free to differ: `build` goes through the pipeline's backend switch, `run --vm
// --backend=mir` bypasses the pipeline entirely because it already holds a
// checked program.
func CompileViaMir(program *ast.Program) (*compiler.Bytecode, error) {
	optimized := hir.NewOptimizer().Optimize(hir.NewLowerer().LowerProgram(program))
	return mir.NewBytecodeEmitter(nil).EmitProgram(mir.NewLowerer().LowerHIR(optimized))
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

// CompileFileVia reads and compiles a .nil file through a named backend.
func CompileFileVia(path string, backend Backend) (*Pipeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	pipeline := NewPipeline(string(data), path).WithBackend(backend)
	if err := pipeline.Compile(); err != nil {
		return nil, err
	}

	return pipeline, nil
}

// Compile runs the full compilation pipeline.
//
// Parsing and type checking both go through the frontend gate so that every
// backend observes identical diagnostics. See compiler/frontend.
func (p *Pipeline) Compile() error {
	res := frontend.Check(p.source, frontend.Options{
		Filename:  p.filename,
		SourceDir: filepath.Dir(p.filename),
	})

	if !res.OK {
		for _, d := range res.Diagnostics {
			if d.Severity == diagnostics.SeverityError {
				p.errors = append(p.errors, d.String())
			}
		}
		if len(p.errors) == 0 {
			// Defensive: OK is false but no error-severity diagnostic was
			// recorded (only possible if a parse failed without a diagnostic).
			p.errors = res.ParseErrors
		}
		summary := res.ErrSummary()
		if summary == "" {
			// Defensive and unreachable in practice: the gate was red but
			// named nothing. Coded as internal so it can never be mistaken
			// for a user error with a missing code.
			summary = fmt.Sprintf("[E0000] compilation failed with %d error(s)", len(p.errors))
		}
		return fmt.Errorf("%s", summary)
	}

	for _, d := range res.Diagnostics {
		if d.Severity == diagnostics.SeverityWarning {
			p.warnings = append(p.warnings, d.String())
		}
	}

	// Phase 4: Bytecode compilation
	//
	// The lowered program is what the engines can execute: for loops have
	// become while loops. Compiling res.Program here would hit the guard in
	// compileFor, which exists to catch exactly this mistake.
	//
	// # Why this is not the MIR path yet
	//
	// `L3.4` asks for both backends to consume one canonical IR, and the MIR
	// path is now correct across the language's core — `TestThreeEnginesAgree`
	// runs twenty-two programs through the evaluator, the direct compiler and
	// the MIR emitter and requires all three to agree.
	//
	// It is not the default here because that test covers the core, not the
	// whole language. Modules, imports, `task`/`await`, components and
	// capabilities have not been through it, and switching a shipped path to a
	// backend on the strength of twenty-two agreeing programs would be trading a
	// tested path for a partly-tested one. `--backend=mir` exposes the new path
	// so the comparison can be run at will; promoting it is the remaining half
	// of `L3.4`, and the honest gate is conformance over the full suite, not a
	// flag.
	if p.backend == BackendMir {
		bc, err := CompileViaMir(res.Lowered)
		if err != nil {
			// The gate passed, so a backend failure is an internal fault,
			// not a user error: code it as one.
			return fmt.Errorf("[E0000] MIR bytecode compilation failed: %w", err)
		}
		p.bytecode = bc
		return nil
	}

	program := res.Lowered
	comp := compiler.New()
	comp.SourceDir = filepath.Dir(p.filename)
	comp.LoadModule = loadModuleAST
	if err := comp.Compile(program); err != nil {
		return fmt.Errorf("[E0000] bytecode compilation failed: %w", err)
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
	// Modules are type checked as part of the importing program, so a module
	// load only needs to parse. Routing through the frontend gate keeps the
	// diagnostic codes and spans identical to top level sources.
	res := frontend.Check(content, frontend.Options{Filename: path, SkipTypeCheck: true})
	if len(res.ParseErrors) > 0 {
		return nil, fmt.Errorf("parse error in %s: %s", path, strings.Join(res.ParseErrors, "; "))
	}

	// The lowered form, not the source form. A module is compiled into this
	// program's bytecode, so `for`, `match`, `struct`, `enum` and `while let`
	// have to be rewritten before the compiler sees them. Returning res.Program
	// here left every module that used them reaching the bytecode compiler as
	// nodes it has no instruction for.
	if res.Lowered != nil {
		return res.Lowered, nil
	}
	return res.Program, nil
}
