package lint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTrailingWhitespace(t *testing.T) {
	f := LintSource("a.nil", "let x = 1;  \nlet y = 2;\n")
	if len(f) != 1 || f[0].Rule != "LINT001" || f[0].Line != 1 {
		t.Fatalf("expected one LINT001 on line 1, got %+v", f)
	}
}

func TestTabIndentation(t *testing.T) {
	f := LintSource("a.nil", "\tlet x = 1;\n")
	if len(f) != 1 || f[0].Rule != "LINT002" {
		t.Fatalf("expected one LINT002, got %+v", f)
	}
}

func TestCleanFileNoFindings(t *testing.T) {
	f := LintSource("a.nil", "let x = 1;\nfn f() {\n  return x;\n}\n")
	if len(f) != 0 {
		t.Fatalf("clean file must have no findings, got %+v", f)
	}
}

func TestNulByteIsError(t *testing.T) {
	f := LintSource("a.nil", "let x = 1;\x00\n")
	if !HasErrors(f) {
		t.Fatalf("NUL byte must be error-severity, got %+v", f)
	}
}

func TestLintPathDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.nil"), []byte("let x = 1;\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.nil"), []byte("let y = 2; \n"), 0644); err != nil {
		t.Fatal(err)
	}
	found, err := LintPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].Rule != "LINT001" {
		t.Fatalf("expected one finding, got %+v", found)
	}
}
