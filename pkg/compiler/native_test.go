package compiler

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

func TestGenerateNativeC(t *testing.T) {
	programParser := parser.New(lexer.New("let base = 40; (base + 2) * 2;"))
	program := programParser.ParseProgram()
	if len(programParser.Errors()) > 0 {
		t.Fatalf("parse errors: %v", programParser.Errors())
	}
	generated, err := GenerateNativeC(program)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	for _, expected := range []string{"long long nil_v0 = 40LL;", `printf("%lld\n", ((nil_v0 + 2LL) * 2LL));`} {
		if !strings.Contains(generated, expected) {
			t.Fatalf("generated C missing %q:\n%s", expected, generated)
		}
	}
}

func TestGenerateNativeCRejectsUnsupportedValues(t *testing.T) {
	programParser := parser.New(lexer.New(`"not an integer";`))
	_, err := GenerateNativeC(programParser.ParseProgram())
	if err == nil || !strings.Contains(err.Error(), "only integer expressions") {
		t.Fatalf("expected unsupported value error, got %v", err)
	}
}

func TestCompileNativeExecutable(t *testing.T) {
	compilerPath, err := findCCompiler("")
	if err != nil {
		t.Skip(err)
	}
	output := filepath.Join(t.TempDir(), "native-test")
	if runtime.GOOS == "windows" {
		output += ".exe"
	}
	if err := CompileNative("let base = 40; base + 2;", output, NativeOptions{Compiler: compilerPath}); err != nil {
		t.Fatalf("native compile failed: %v", err)
	}
	result, err := exec.Command(output).Output()
	if err != nil {
		t.Fatalf("native executable failed: %v", err)
	}
	if string(bytes.TrimSpace(result)) != "42" {
		t.Fatalf("native output = %q, want 42", result)
	}
	binary, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(binary, []byte("NABC")) {
		t.Fatal("native executable unexpectedly embeds NABC bytecode")
	}
}
