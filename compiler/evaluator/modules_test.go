package evaluator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

func TestCircularModuleDependencyDetection(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nil_mod_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	fileA := filepath.Join(tmpDir, "mod_a.nil")
	fileB := filepath.Join(tmpDir, "mod_b.nil")

	// mod_a imports mod_b, mod_b imports mod_a
	srcA := `import "./mod_b.nil" as b; let a = 1;`
	srcB := `import "./mod_a.nil" as a; let b = 2;`

	if err := os.WriteFile(fileA, []byte(srcA), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileB, []byte(srcB), 0644); err != nil {
		t.Fatal(err)
	}

	PushScriptDir(tmpDir)
	defer PopScriptDir()

	l := lexer.New(srcA)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}

	env := object.NewEnvironment()
	res := Eval(prog, env)

	errObj, ok := res.(*object.Error)
	if !ok {
		t.Fatalf("expected circular dependency error, got %v", res)
	}

	if !strings.Contains(errObj.Message, "E0301") || !strings.Contains(errObj.Message, "circular dependency detected") {
		t.Fatalf("expected E0301 circular dependency error, got: %s", errObj.Message)
	}
	t.Logf("Successfully caught circular import: %s", errObj.Message)
}
