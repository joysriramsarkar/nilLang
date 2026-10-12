package formatter

import (
	"strings"
	"testing"
)

// TestFormatterPrettyPrintExpansion (L9.8): pretty-print coverage beyond the
// core statements — annotations, calls, nesting, imports and components.
func TestFormatterTypedLetAndConst(t *testing.T) {
	cases := map[string]string{
		`let x: Int = 5;`:        "let x: Int = 5;\n",
		`const  PI=3.14;`:        "const PI = 3.14;\n",
		`let s="hi";`:            "let s = \"hi\";\n",
		`let b = true;`:          "let b = true;\n",
		`let n = null;`:          "let n = null;\n",
		`let xs = [1, 2];`:       "let xs = [1, 2];\n",
		`x  =  x+1;`:             "x = x + 1;\n",
		`fn f() { return 0; }`:   "fn f() {\n    return 0;\n}\n",
		`return  x*2 ;`:          "return x * 2;\n",
		`println( x ,y );`:       "println(x, y);\n",
		`import { a } from "m";`: "import { a } from \"m\";\n",
	}
	for src, want := range cases {
		got, err := Format(src)
		if err != nil {
			t.Fatalf("Format(%q) failed: %v", src, err)
		}
		if got != want {
			t.Errorf("Format(%q):\n got: %q\nwant: %q", src, got, want)
		}
	}
}

func TestFormatterNestedCallsAndIndex(t *testing.T) {
	src := "let v = first(rest([1, 2, 3]));\nlet w = m[\"k\"];\nlet u = a.b.c;\n"
	got, err := Format(src)
	if err != nil {
		t.Fatalf("format failed: %v", err)
	}
	for _, want := range []string{
		"let v = first(rest([1, 2, 3]));",
		"let w = m[\"k\"];",
		"let u = a.b.c;",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("pretty output missing %q:\n%s", want, got)
		}
	}
	// Idempotent.
	again, err := Format(got)
	if err != nil || again != got {
		t.Errorf("not idempotent:\nfirst:\n%s\nsecond:\n%s", got, again)
	}
}

func TestFormatterFunctionReturnType(t *testing.T) {
	src := "fn add(a: Int, b: Int) -> Int {\nreturn a + b;\n}\n"
	got, err := Format(src)
	if err != nil {
		t.Fatalf("format failed: %v", err)
	}
	if !strings.Contains(got, "fn add(a: Int, b: Int) -> Int {") {
		t.Errorf("signature not preserved:\n%s", got)
	}
	again, err := Format(got)
	if err != nil || again != got {
		t.Errorf("not idempotent:\n%s\n%s", got, again)
	}
}

func TestFormatterWhileNestedIf(t *testing.T) {
	src := "while (i < 10) {\nif (i > 5) {\nreturn i;\n}\ni = i + 1;\n}\n"
	got, err := Format(src)
	if err != nil {
		t.Fatalf("format failed: %v", err)
	}
	if !strings.Contains(got, "while (i < 10) {") || !strings.Contains(got, "if (i > 5) {") {
		t.Errorf("control flow not preserved:\n%s", got)
	}
	if !strings.Contains(got, "        return i;") {
		t.Errorf("nested indent wrong (want 8 spaces):\n%s", got)
	}
	again, err := Format(got)
	if err != nil || again != got {
		t.Errorf("not idempotent:\n%s\n%s", got, again)
	}
}

func TestFormatterEntityComponent(t *testing.T) {
	src := "entity User {\nid: UUID primary;\nname: String required;\n}\n"
	got, err := Format(src)
	if err != nil {
		t.Fatalf("format failed: %v", err)
	}
	if !strings.Contains(got, "entity User {") || !strings.Contains(got, "    id: UUID primary;") {
		t.Errorf("entity not pretty-printed:\n%s", got)
	}
	again, err := Format(got)
	if err != nil || again != got {
		t.Errorf("not idempotent:\n%s\n%s", got, again)
	}
}

func TestFormatterEndsWithSingleNewline(t *testing.T) {
	got, err := Format("let x = 1;")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, "\n") || strings.HasSuffix(got, "\n\n") {
		t.Errorf("must end with exactly one newline: %q", got)
	}
}
