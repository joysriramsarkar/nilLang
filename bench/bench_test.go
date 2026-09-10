package bench

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
)

func loadBenchmarkAST(b *testing.B, filename string) *parser.Parser {
	data, err := os.ReadFile(filename)
	if err != nil {
		b.Fatalf("failed to read %s: %v", filename, err)
	}
	l := lexer.New(string(data))
	p := parser.New(l)
	return p
}

func loadBytecode(b *testing.B, filename string) *compiler.Bytecode {
	data, err := os.ReadFile(filename)
	if err != nil {
		b.Fatalf("failed to read %s: %v", filename, err)
	}
	l := lexer.New(string(data))
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		b.Fatalf("parse error in %s: %v", filename, p.Errors())
	}

	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		b.Fatalf("compile error in %s: %v", filename, err)
	}
	return comp.Bytecode()
}

// 1. Fibonacci Benchmarks
func BenchmarkVMFibonacci(b *testing.B) {
	bc := loadBytecode(b, filepath.Join(".", "fibonacci.nil"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		machine := vm.New(bc)
		if err := machine.Run(); err != nil {
			b.Fatalf("VM error: %v", err)
		}
	}
}

func BenchmarkEvaluatorFibonacci(b *testing.B) {
	data, err := os.ReadFile(filepath.Join(".", "fibonacci.nil"))
	if err != nil {
		b.Fatalf("failed to read: %v", err)
	}
	src := string(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := lexer.New(src)
		p := parser.New(l)
		prog := p.ParseProgram()
		env := object.NewEnvironment()
		evaluator.Eval(prog, env)
	}
}

// 2. Prime Calculation Benchmarks
func BenchmarkVMPrimeSieve(b *testing.B) {
	bc := loadBytecode(b, filepath.Join(".", "prime_sieve.nil"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		machine := vm.New(bc)
		if err := machine.Run(); err != nil {
			b.Fatalf("VM error: %v", err)
		}
	}
}

func BenchmarkEvaluatorPrimeSieve(b *testing.B) {
	data, err := os.ReadFile(filepath.Join(".", "prime_sieve.nil"))
	if err != nil {
		b.Fatalf("failed to read: %v", err)
	}
	src := string(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := lexer.New(src)
		p := parser.New(l)
		prog := p.ParseProgram()
		env := object.NewEnvironment()
		evaluator.Eval(prog, env)
	}
}

// 3. Closure Counter Benchmarks
func BenchmarkVMClosureCounter(b *testing.B) {
	bc := loadBytecode(b, filepath.Join(".", "closure_counter.nil"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		machine := vm.New(bc)
		if err := machine.Run(); err != nil {
			b.Fatalf("VM error: %v", err)
		}
	}
}

func BenchmarkEvaluatorClosureCounter(b *testing.B) {
	data, err := os.ReadFile(filepath.Join(".", "closure_counter.nil"))
	if err != nil {
		b.Fatalf("failed to read: %v", err)
	}
	src := string(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := lexer.New(src)
		p := parser.New(l)
		prog := p.ParseProgram()
		env := object.NewEnvironment()
		evaluator.Eval(prog, env)
	}
}

// 4. Matrix Multiplication / Dot Product Benchmarks
func BenchmarkVMMatrixMul(b *testing.B) {
	bc := loadBytecode(b, filepath.Join(".", "matrix_mul.nil"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		machine := vm.New(bc)
		if err := machine.Run(); err != nil {
			b.Fatalf("VM error: %v", err)
		}
	}
}

func BenchmarkEvaluatorMatrixMul(b *testing.B) {
	data, err := os.ReadFile(filepath.Join(".", "matrix_mul.nil"))
	if err != nil {
		b.Fatalf("failed to read: %v", err)
	}
	src := string(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := lexer.New(src)
		p := parser.New(l)
		prog := p.ParseProgram()
		env := object.NewEnvironment()
		evaluator.Eval(prog, env)
	}
}
