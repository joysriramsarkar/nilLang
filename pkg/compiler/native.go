package compiler

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

type NativeOptions struct {
	Compiler string
	KeepC    bool
}

type nativeEmitter struct {
	variables map[string]string
	lines     []string
	nextVar   int
}

func CompileNative(source, outputPath string, options NativeOptions) error {
	programParser := parser.New(lexer.New(source))
	program := programParser.ParseProgram()
	if len(programParser.Errors()) > 0 {
		return fmt.Errorf("native parse failed:\n%s", strings.Join(programParser.Errors(), "\n"))
	}

	generated, err := GenerateNativeC(program)
	if err != nil {
		return err
	}
	compilerPath, err := findCCompiler(options.Compiler)
	if err != nil {
		return err
	}
	if outputPath == "" {
		outputPath = "a.out"
		if runtime.GOOS == "windows" {
			outputPath = "a.exe"
		}
	}
	outputPath, err = filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolve native output: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create native output directory: %w", err)
	}

	cPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".nil.c"
	if err := os.WriteFile(cPath, []byte(generated), 0o644); err != nil {
		return fmt.Errorf("write generated C: %w", err)
	}
	if !options.KeepC {
		defer os.Remove(cPath)
	}

	command := exec.Command(compilerPath, "-std=c11", "-O2", "-Wall", "-Wextra", "-Werror", cPath, "-o", outputPath)
	var compilerOutput bytes.Buffer
	command.Stdout = &compilerOutput
	command.Stderr = &compilerOutput
	if err := command.Run(); err != nil {
		return fmt.Errorf("native compiler failed: %w\n%s", err, strings.TrimSpace(compilerOutput.String()))
	}
	return nil
}

func GenerateNativeC(program *ast.Program) (string, error) {
	if program == nil || len(program.Statements) == 0 {
		return "", fmt.Errorf("native compilation requires a non-empty program")
	}
	emitter := &nativeEmitter{variables: make(map[string]string)}
	for index, statement := range program.Statements {
		isLast := index == len(program.Statements)-1
		switch node := statement.(type) {
		case *ast.LetStatement:
			if node.Name == nil || node.Value == nil {
				return "", fmt.Errorf("native backend requires initialized let bindings")
			}
			if _, exists := emitter.variables[node.Name.Value]; exists {
				return "", fmt.Errorf("native backend does not support redeclaring %q", node.Name.Value)
			}
			value, err := emitter.expression(node.Value)
			if err != nil {
				return "", err
			}
			name := fmt.Sprintf("nil_v%d", emitter.nextVar)
			emitter.nextVar++
			emitter.variables[node.Name.Value] = name
			emitter.lines = append(emitter.lines, fmt.Sprintf("long long %s = %s;", name, value))
		case *ast.ExpressionStatement:
			value, err := emitter.expression(node.Expression)
			if err != nil {
				return "", err
			}
			if isLast {
				emitter.lines = append(emitter.lines, fmt.Sprintf("printf(\"%%lld\\n\", %s);", value))
			} else {
				emitter.lines = append(emitter.lines, fmt.Sprintf("(void)(%s);", value))
			}
		default:
			return "", fmt.Errorf("native backend does not support statement %T", statement)
		}
	}

	var generated strings.Builder
	generated.WriteString("#include <stdio.h>\n\nint main(void) {\n")
	for _, line := range emitter.lines {
		generated.WriteString("    ")
		generated.WriteString(line)
		generated.WriteByte('\n')
	}
	generated.WriteString("    return 0;\n}\n")
	return generated.String(), nil
}

func (e *nativeEmitter) expression(expression ast.Expression) (string, error) {
	switch node := expression.(type) {
	case *ast.IntegerLiteral:
		return fmt.Sprintf("%dLL", node.Value), nil
	case *ast.Identifier:
		name, ok := e.variables[node.Value]
		if !ok {
			return "", fmt.Errorf("native backend: undefined integer variable %q", node.Value)
		}
		return name, nil
	case *ast.PrefixExpression:
		if node.Operator != "-" {
			return "", fmt.Errorf("native backend does not support prefix operator %q", node.Operator)
		}
		right, err := e.expression(node.Right)
		if err != nil {
			return "", err
		}
		return "(-" + right + ")", nil
	case *ast.InfixExpression:
		switch node.Operator {
		case "+", "-", "*", "/", "%":
		default:
			return "", fmt.Errorf("native backend does not support infix operator %q", node.Operator)
		}
		left, err := e.expression(node.Left)
		if err != nil {
			return "", err
		}
		right, err := e.expression(node.Right)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("(%s %s %s)", left, node.Operator, right), nil
	default:
		return "", fmt.Errorf("native backend supports only integer expressions, got %T", expression)
	}
}

func findCCompiler(requested string) (string, error) {
	if requested != "" {
		path, err := exec.LookPath(requested)
		if err != nil {
			return "", fmt.Errorf("native C compiler %q not found: %w", requested, err)
		}
		return path, nil
	}
	for _, candidate := range []string{"clang", "cc", "gcc"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("native compilation requires clang, cc, or gcc on PATH")
}
